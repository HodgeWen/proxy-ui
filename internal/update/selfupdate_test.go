package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// newMockPanelGitHub 起本地服务器模拟面板自身 GitHub Releases（API 与下载均按 ghproxy 式镜像路径访问），返回基址。
// 调用方将其设为镜像前缀后，全部请求经 <base>/<原始URL> 转发到本地。
func newMockPanelGitHub(t *testing.T, tags []string, assetContent []byte, digest string) string {
	t.Helper()
	name := panelAssetName()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch p := r.URL.Path; {
		case strings.HasSuffix(p, "/repos/"+panelRepo+"/releases"):
			rels := make([]ghRelease, 0, len(tags))
			for _, tag := range tags {
				rels = append(rels, ghRelease{
					TagName:     tag,
					PublishedAt: time.Now(),
					Assets: []ghAsset{{
						Name:               name,
						Digest:             digest,
						BrowserDownloadURL: "https://github.com/" + panelRepo + "/releases/download/" + tag + "/" + name,
					}},
				})
			}
			_ = json.NewEncoder(w).Encode(rels)
		case strings.Contains(p, "/"+panelRepo+"/releases/download/"):
			_, _ = w.Write(assetContent)
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL
}

// newSelfUpdaterForTest 构造注入假二进制路径、重启钩子与本地镜像前缀的面板更新器。
func newSelfUpdaterForTest(t *testing.T, version, mockBase string) *SelfUpdater {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "proxy-ui")
	if err := os.WriteFile(exe, []byte("old binary"), 0o755); err != nil {
		t.Fatalf("写入假二进制失败: %v", err)
	}
	database := testDB(t)
	if err := setSetting(database, settingMirror, mockBase); err != nil {
		t.Fatalf("设置镜像前缀失败: %v", err)
	}
	u := NewSelfUpdate(database, version)
	u.exePath = exe
	u.restart = func(string) error { return nil }
	return u
}

func TestSelfUpdateListVersions(t *testing.T) {
	content := []byte("new binary")
	sum := sha256.Sum256(content)
	base := newMockPanelGitHub(t, []string{"v2.0.0", "v1.0.0"}, content, "sha256:"+hex.EncodeToString(sum[:]))

	u := newSelfUpdaterForTest(t, "v1.0.0", base)

	versions, err := u.ListVersions(context.Background())
	if err != nil {
		t.Fatalf("ListVersions 失败: %v", err)
	}
	if len(versions) != 2 || versions[0].Tag != "v2.0.0" {
		t.Fatalf("版本列表应降序且含两版本: %+v", versions)
	}
	if !versions[1].Installed || versions[0].Installed {
		t.Fatalf("当前版本 v1.0.0 应标记 Installed: %+v", versions)
	}
	if u.CurrentVersion() != "v1.0.0" {
		t.Errorf("CurrentVersion = %q, 期望 v1.0.0", u.CurrentVersion())
	}
}

func TestSelfUpdateApply(t *testing.T) {
	content := []byte("new binary v2")
	sum := sha256.Sum256(content)
	base := newMockPanelGitHub(t, []string{"v2.0.0", "v1.0.0"}, content, "sha256:"+hex.EncodeToString(sum[:]))

	u := newSelfUpdaterForTest(t, "v1.0.0", base)

	tag, err := u.Apply(context.Background(), "")
	if err != nil {
		t.Fatalf("Apply 失败: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("应更新到最新稳定版 v2.0.0, got %s", tag)
	}
	data, err := os.ReadFile(u.exePath)
	if err != nil {
		t.Fatalf("读取替换后二进制失败: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("二进制内容不符: %q", data)
	}
	info, err := os.Stat(u.exePath)
	if err != nil {
		t.Fatalf("读取二进制属性失败: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("替换后的二进制应保留可执行权限")
	}
	// 下载临时文件应被清理
	entries, _ := os.ReadDir(filepath.Dir(u.exePath))
	if len(entries) != 1 {
		t.Errorf("应只剩二进制本身一个文件, got %d", len(entries))
	}
}

func TestSelfUpdateApplyDigestMismatch(t *testing.T) {
	content := []byte("new binary")
	base := newMockPanelGitHub(t, []string{"v2.0.0"}, content, "sha256:"+string(bytes.Repeat([]byte("0"), 64)))

	u := newSelfUpdaterForTest(t, "v1.0.0", base)

	if _, err := u.Apply(context.Background(), "2.0.0"); err == nil {
		t.Fatal("摘要不符时 Apply 应报错")
	}
	data, err := os.ReadFile(u.exePath)
	if err != nil {
		t.Fatalf("读取二进制失败: %v", err)
	}
	if string(data) != "old binary" {
		t.Errorf("校验失败不应替换二进制, got %q", data)
	}
}

func TestSelfUpdateRestart(t *testing.T) {
	u := newSelfUpdaterForTest(t, "v1.0.0", "http://unused.example")
	var gotPath string
	u.restart = func(exe string) error {
		gotPath = exe
		return nil
	}
	if err := u.Restart(); err != nil {
		t.Fatalf("Restart 失败: %v", err)
	}
	if gotPath != u.exePath {
		t.Errorf("Restart 应以当前二进制路径重启, got %q", gotPath)
	}
}

func TestPanelAssetName(t *testing.T) {
	want := fmt.Sprintf("proxy-ui-%s-%s", runtime.GOOS, runtime.GOARCH)
	if got := panelAssetName(); got != want {
		t.Fatalf("panelAssetName = %q, 期望 %q", got, want)
	}
}
