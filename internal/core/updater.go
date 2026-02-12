package core

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Release represents a sing-box release from GitHub.
type Release struct {
	Tag       string `json:"tag"`
	Version   string `json:"version"`
	Prerelease bool  `json:"prerelease"`
}

// CoreUpdater manages sing-box binary updates from GitHub releases.
type CoreUpdater struct {
	binaryPath string
}

// NewCoreUpdater creates a CoreUpdater for the given binary path.
// When binaryPath is empty, Update() and Rollback() return an error.
func NewCoreUpdater(binaryPath string) *CoreUpdater {
	return &CoreUpdater{binaryPath: binaryPath}
}

// githubRelease is the GitHub API response shape.
type githubRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// ListReleases fetches releases from GitHub API.
// Returns releases in reverse chronological order (latest first).
func (u *CoreUpdater) ListReleases() ([]Release, error) {
	resp, err := http.Get("https://api.github.com/repos/SagerNet/sing-box/releases")
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API status %d", resp.StatusCode)
	}
	var raw []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}
	var out []Release
	for _, r := range raw {
		if r.Draft {
			continue
		}
		ver := strings.TrimPrefix(r.TagName, "v")
		out = append(out, Release{
			Tag:       r.TagName,
			Version:   ver,
			Prerelease: r.Prerelease,
		})
	}
	return out, nil
}

// assetForPlatform maps GOOS/GOARCH to sing-box asset name and returns URL.
func (u *CoreUpdater) assetForPlatform(releases []githubRelease, version string) (name, url string, err error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if goarch == "arm" {
		goarch = "armv7"
	}
	if goarch == "386" {
		goarch = "386"
	}
	suffix := fmt.Sprintf("%s-%s.tar.gz", goos, goarch)
	assetName := fmt.Sprintf("sing-box-%s-%s", version, suffix)

	for _, r := range releases {
		if r.Draft {
			continue
		}
		ver := strings.TrimPrefix(r.TagName, "v")
		if ver != version {
			continue
		}
		for _, a := range r.Assets {
			if a.Name == assetName {
				return a.Name, a.BrowserDownloadURL, nil
			}
		}
	}
	return "", "", fmt.Errorf("asset %s not found for version %s", assetName, version)
}

// Update downloads the latest release, replaces the binary atomically, and restarts.
func (u *CoreUpdater) Update() error {
	if u.binaryPath == "" {
		return fmt.Errorf("请设置 SINGBOX_BINARY_PATH 以启用核心更新")
	}

	pm := NewProcessManagerWithBinary(ConfigPathFromEnv(), u.binaryPath)
	configPath := pm.ConfigPath()

	// 1. Stop
	_ = execPkill("sing-box")
	time.Sleep(100 * time.Millisecond)

	// 2. Fetch releases
	resp, err := http.Get("https://api.github.com/repos/SagerNet/sing-box/releases")
	if err != nil {
		return fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API status %d", resp.StatusCode)
	}
	var raw []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("decode releases: %w", err)
	}
	var latest *githubRelease
	for i := range raw {
		if !raw[i].Draft {
			latest = &raw[i]
			break
		}
	}
	if latest == nil {
		return fmt.Errorf("no non-draft release found")
	}
	version := strings.TrimPrefix(latest.TagName, "v")

	_, assetURL, err := u.assetForPlatform(raw, version)
	if err != nil {
		return err
	}

	// 3. Download to temp file
	tmpDir, err := os.MkdirTemp("", "s-ui-update-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	tarPath := filepath.Join(tmpDir, "release.tar.gz")
	if err := downloadFile(assetURL, tarPath); err != nil {
		return err
	}

	// 4. Extract binary
	extractedPath, err := extractBinary(tarPath, tmpDir)
	if err != nil {
		return err
	}

	// 5. Backup
	backupPath := u.binaryPath + ".backup"
	if _, err := os.Stat(u.binaryPath); err == nil {
		_ = os.Remove(backupPath)
		if err := copyFile(u.binaryPath, backupPath); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
	}

	// 6. Atomic replace
	tmpBinary := filepath.Join(tmpDir, "sing-box-new")
	if err := copyFile(extractedPath, tmpBinary); err != nil {
		restoreBackup(u.binaryPath, backupPath)
		return fmt.Errorf("copy extracted: %w", err)
	}
	if err := os.Chmod(tmpBinary, 0755); err != nil {
		restoreBackup(u.binaryPath, backupPath)
		return fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(tmpBinary, u.binaryPath); err != nil {
		restoreBackup(u.binaryPath, backupPath)
		return fmt.Errorf("atomic replace: %w", err)
	}

	// 7. Verify
	if out, err := pm.Check(configPath); err != nil {
		restoreBackup(u.binaryPath, backupPath)
		return fmt.Errorf("verify failed: %w\n%s", err, out)
	}

	// 8. Start
	if err := pm.Restart(configPath); err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	return nil
}

// Rollback restores the binary from backup.
func (u *CoreUpdater) Rollback() error {
	if u.binaryPath == "" {
		return fmt.Errorf("请设置 SINGBOX_BINARY_PATH 以启用核心更新")
	}

	backupPath := u.binaryPath + ".backup"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("暂无备份可回滚")
	}

	pm := NewProcessManagerWithBinary(ConfigPathFromEnv(), u.binaryPath)
	configPath := pm.ConfigPath()

	// Stop
	_ = execPkill("sing-box")
	time.Sleep(100 * time.Millisecond)

	// Swap backup with current
	if err := os.Rename(u.binaryPath, u.binaryPath+".old"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("rename current: %w", err)
	}
	if err := os.Rename(backupPath, u.binaryPath); err != nil {
		_ = os.Rename(u.binaryPath+".old", u.binaryPath)
		return fmt.Errorf("restore backup: %w", err)
	}
	_ = os.Remove(u.binaryPath + ".old")
	if err := os.Chmod(u.binaryPath, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	if err := pm.Restart(configPath); err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	return nil
}

func execPkill(name string) error {
	cmd := exec.Command("pkill", "-x", name)
	return cmd.Run()
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download status %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

func extractBinary(tarGzPath, extractDir string) (string, error) {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	var extractedPath string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if h.Typeflag == tar.TypeReg && strings.HasSuffix(h.Name, "/sing-box") {
			base := filepath.Base(h.Name)
			if base == "" {
				base = "sing-box"
			}
			dest := filepath.Join(extractDir, base)
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return "", err
			}
			out.Close()
			extractedPath = dest
			break
		}
	}
	if extractedPath == "" {
		return "", fmt.Errorf("sing-box binary not found in archive")
	}
	return extractedPath, nil
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	return err
}

func restoreBackup(target, backup string) {
	_ = os.Remove(target)
	_ = copyFile(backup, target)
	_ = os.Chmod(target, 0755)
}

// ConfigPathFromEnv returns the sing-box config path from env or default.
func ConfigPathFromEnv() string {
	if p := os.Getenv("SINGBOX_CONFIG_PATH"); p != "" {
		return p
	}
	return "./config.json"
}
