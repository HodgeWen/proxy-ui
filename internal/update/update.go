package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"

	"proxy-ui/internal/db"
)

// 设置键。
const (
	settingMirror      = "mirror_prefix" // GitHub 下载镜像前缀
	settingCoreVersion = "core_version"  // 已安装内核版本 tag
	defaultAPIBase     = "https://api.github.com"
)

// Manager sing-box 内核更新管理：版本查询、镜像下载、完整性校验与安装。
type Manager struct {
	db      *gorm.DB // 读写面板设置（镜像前缀、已装版本）
	dir     string   // 内核安装目录
	apiBase string   // GitHub API 基址（测试可替换）
	client  *http.Client
}

// New 构造内核更新管理器；dir 为内核安装目录（自动创建）。
func New(database *gorm.DB, dir string) *Manager {
	_ = os.MkdirAll(dir, 0o755)
	return &Manager{
		db:      database,
		dir:     dir,
		apiBase: defaultAPIBase,
		client:  &http.Client{Timeout: 10 * time.Minute},
	}
}

// CorePath 内核可执行文件路径。
func (m *Manager) CorePath() string { return filepath.Join(m.dir, "sing-box") }

// Installed 内核是否已安装。
func (m *Manager) Installed() bool {
	_, err := os.Stat(m.CorePath())
	return err == nil
}

// InstalledVersion 已安装内核版本 tag（未记录返回空）。
func (m *Manager) InstalledVersion() string { return m.getSetting(settingCoreVersion) }

// MirrorPrefix 读取下载镜像前缀设置（空表示直连 GitHub）。
func (m *Manager) MirrorPrefix() string { return m.getSetting(settingMirror) }

// SetMirrorPrefix 保存镜像前缀；非空须为 http(s):// 地址，返回去掉尾部斜杠后的规范化值。
func (m *Manager) SetMirrorPrefix(prefix string) (string, error) {
	prefix = strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	if prefix != "" && !strings.HasPrefix(prefix, "http://") && !strings.HasPrefix(prefix, "https://") {
		return "", errors.New("镜像前缀须以 http:// 或 https:// 开头")
	}
	if err := m.setSetting(settingMirror, prefix); err != nil {
		return "", err
	}
	return prefix, nil
}

// ListVersions 查询 GitHub 可用版本列表，按版本号降序返回。
func (m *Manager) ListVersions(ctx context.Context) ([]Version, error) {
	rels, err := m.fetchReleases(ctx)
	if err != nil {
		return nil, err
	}
	installed := m.InstalledVersion()
	versions := make([]Version, 0, len(rels))
	for _, r := range rels {
		v, ok := parseVersionTag(r.TagName)
		if !ok || r.Draft {
			continue
		}
		versions = append(versions, Version{
			Tag:         r.TagName,
			Version:     v.String(),
			Prerelease:  r.Prerelease || v.pre != "",
			PublishedAt: r.PublishedAt.Format(time.RFC3339),
			Installed:   r.TagName == installed,
		})
	}
	slices.SortFunc(versions, func(a, b Version) int {
		va, _ := parseVersionTag(a.Tag)
		vb, _ := parseVersionTag(b.Tag)
		return vb.compare(va) // 降序：新版本在前
	})
	return versions, nil
}

// Install 下载并安装指定版本内核；version 为空时选择最新稳定版，返回安装的版本 tag。
func (m *Manager) Install(ctx context.Context, version string) (string, error) {
	rels, err := m.fetchReleases(ctx)
	if err != nil {
		return "", err
	}
	rel, ok := pickRelease(rels, version)
	if !ok {
		return "", fmt.Errorf("版本 %q 不存在或无稳定版本可选", version)
	}
	name := assetName(rel.TagName)
	var asset *ghAsset
	for i := range rel.Assets {
		if rel.Assets[i].Name == name {
			asset = &rel.Assets[i]
			break
		}
	}
	if asset == nil {
		return "", fmt.Errorf("版本 %s 未提供 %s 安装包", rel.TagName, name)
	}

	tmpPath, err := m.downloadFile(ctx, asset.BrowserDownloadURL, asset.Digest)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpPath)

	coreTmp := m.CorePath() + ".new"
	if err := extractCoreTo(tmpPath, coreTmp); err != nil {
		_ = os.Remove(coreTmp)
		return "", err
	}
	if err := os.Chmod(coreTmp, 0o755); err != nil {
		_ = os.Remove(coreTmp)
		return "", fmt.Errorf("设置内核可执行权限失败: %w", err)
	}
	if err := os.Rename(coreTmp, m.CorePath()); err != nil {
		_ = os.Remove(coreTmp)
		return "", fmt.Errorf("安装内核失败: %w", err)
	}
	if err := m.setSetting(settingCoreVersion, rel.TagName); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

// fetchReleases 拉取 GitHub Releases 列表（经镜像前缀）。
func (m *Manager) fetchReleases(ctx context.Context) ([]ghRelease, error) {
	resp, err := m.get(ctx, m.apiBase+"/repos/"+repo+"/releases?per_page=100")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var rels []ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&rels); err != nil {
		return nil, fmt.Errorf("解析 GitHub 响应失败: %w", err)
	}
	return rels, nil
}

// get 发起 GET 请求并校验状态码，URL 先经镜像前缀拼接。
func (m *Manager) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, MirrorURL(m.MirrorPrefix(), url), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "proxy-ui")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub 失败（网络不通时可在设置页配置镜像前缀）: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return resp, nil
}

// downloadFile 下载 url 到临时文件并按 digest 校验完整性，返回临时文件路径（调用方负责删除）。
func (m *Manager) downloadFile(ctx context.Context, url, digest string) (string, error) {
	resp, err := m.get(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	tmp, err := os.CreateTemp(m.dir, ".download-*.tmp")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), resp.Body); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("下载内核安装包失败: %w", err)
	}
	tmp.Close()
	if err := verifyDigestHex(hex.EncodeToString(hash.Sum(nil)), digest); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

// extractCoreTo 从 tar.gz 安装包中提取 sing-box 可执行文件写到 dest（gzip 解压自带 CRC 完整性校验）。
func extractCoreTo(tarGzPath, dest string) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("打开安装包失败: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("安装包不是有效的 gzip 文件: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return errors.New("安装包中未找到 sing-box 可执行文件")
		}
		if err != nil {
			return fmt.Errorf("读取安装包失败: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != "sing-box" {
			continue
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return fmt.Errorf("写入内核文件失败: %w", err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return fmt.Errorf("解压内核失败: %w", err)
		}
		return out.Close()
	}
}

// getSetting 读取设置值，不存在返回空字符串。
func (m *Manager) getSetting(key string) string {
	var s db.Setting
	if err := m.db.Where("`key` = ?", key).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

// setSetting 写入设置值（upsert）。
func (m *Manager) setSetting(key, value string) error {
	return m.db.Save(&db.Setting{Key: key, Value: value}).Error
}
