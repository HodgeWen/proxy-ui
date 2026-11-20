package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"proxy-ui/internal/db"
)

// testDB 建内存 SQLite 并迁移 Setting 表。
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	g, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := g.AutoMigrate(&db.Setting{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return g
}

func TestParseVersionTag(t *testing.T) {
	tests := []struct {
		name   string
		tag    string
		want   string
		pre    string
		wantOK bool
	}{
		{"稳定版带 v 前缀", "v1.14.4", "1.14.4", "", true},
		{"预发布无 v 前缀", "1.15.0-alpha.13", "1.15.0-alpha.13", "alpha.13", true},
		{"预发布带 v 前缀", "v1.2.3-beta.1", "1.2.3-beta.1", "beta.1", true},
		{"两段版本拒绝", "v1.2", "", "", false},
		{"非数字段拒绝", "v1.2.x", "", "", false},
		{"非版本字符串拒绝", "testing", "", "", false},
		{"负号段拒绝", "v-1.2.3", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := parseVersionTag(tt.tag)
			if ok != tt.wantOK {
				t.Fatalf("parseVersionTag(%q) ok=%v, 期望 %v", tt.tag, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if v.String() != tt.want || v.pre != tt.pre {
				t.Fatalf("parseVersionTag(%q) = %q (pre=%q), 期望 %q (pre=%q)", tt.tag, v.String(), v.pre, tt.want, tt.pre)
			}
		})
	}
}

func TestCompareVersion(t *testing.T) {
	ordered := []string{
		"v1.14.4",
		"v1.15.0-alpha.2",
		"v1.15.0-alpha.13",
		"v1.15.0-beta.1",
		"v1.15.0",
		"v2.0.0",
	}
	for i := range len(ordered) - 1 {
		a, _ := parseVersionTag(ordered[i])
		b, _ := parseVersionTag(ordered[i+1])
		if a.compare(b) >= 0 {
			t.Errorf("%s 应小于 %s", ordered[i], ordered[i+1])
		}
		if b.compare(a) <= 0 {
			t.Errorf("%s 应大于 %s", ordered[i+1], ordered[i])
		}
	}
}

func TestVerifyDigestHex(t *testing.T) {
	sum := sha256.Sum256([]byte("payload"))
	local := hex.EncodeToString(sum[:])
	tests := []struct {
		name    string
		digest  string
		wantErr bool
	}{
		{"匹配摘要通过", "sha256:" + local, false},
		{"摘要不匹配报错", "sha256:" + string(bytes.Repeat([]byte("0"), 64)), true},
		{"非 sha256 算法报错", "md5:" + local, true},
		{"格式错误报错", "not-a-digest", true},
		{"空 digest 跳过校验", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyDigestHex(local, tt.digest)
			if (err != nil) != tt.wantErr {
				t.Fatalf("verifyDigestHex(%q, %q) err = %v, wantErr %v", local, tt.digest, err, tt.wantErr)
			}
		})
	}
}

func TestPickRelease(t *testing.T) {
	rels := []ghRelease{
		{TagName: "v1.15.0-alpha.13", Prerelease: true},
		{TagName: "v1.14.4"},
		{TagName: "v1.13.0"},
		{TagName: "draft-tag", Draft: true},
	}
	tests := []struct {
		name    string
		version string
		wantTag string
		wantOK  bool
	}{
		{"空版本选最新稳定版", "", "v1.14.4", true},
		{"指定纯版本号", "1.13.0", "v1.13.0", true},
		{"指定带 v 前缀版本", "v1.14.4", "v1.14.4", true},
		{"不存在的版本", "v9.9.9", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, ok := pickRelease(rels, tt.version)
			if ok != tt.wantOK || r.TagName != tt.wantTag {
				t.Fatalf("pickRelease(rels, %q) = %q ok=%v, 期望 %q ok=%v", tt.version, r.TagName, ok, tt.wantTag, tt.wantOK)
			}
		})
	}
}

// buildCoreArchive 构造含 sing-box 二进制成员的 tar.gz 安装包。
func buildCoreArchive(t *testing.T, dirName, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name: dirName + "/", Typeflag: tar.TypeDir, Mode: 0o755,
	}); err != nil {
		t.Fatalf("写 tar 头失败: %v", err)
	}
	if err := tw.WriteHeader(&tar.Header{
		Name: dirName + "/sing-box", Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len(content)),
	}); err != nil {
		t.Fatalf("写 tar 头失败: %v", err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatalf("写 tar 内容失败: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("关闭 tar 失败: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("关闭 gzip 失败: %v", err)
	}
	return buf.Bytes()
}

// newMockGitHub 起本地服务器模拟 GitHub API 与下载端点。
func newMockGitHub(t *testing.T, tag string, archive []byte, digest string) string {
	t.Helper()
	assetName := fmt.Sprintf("sing-box-%s-%s-%s.tar.gz", tag[1:], runtime.GOOS, runtime.GOARCH)
	var server *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+repo+"/releases", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]ghRelease{{
			TagName:     tag,
			PublishedAt: time.Now(),
			Assets: []ghAsset{{
				Name:               assetName,
				Digest:             digest,
				BrowserDownloadURL: server.URL + "/download/" + assetName,
			}},
		}})
	})
	mux.HandleFunc("/download/"+assetName, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archive)
	})
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL
}

func TestInstallLatest(t *testing.T) {
	content := "#!/bin/sh\nfake sing-box core\n"
	archive := buildCoreArchive(t, "sing-box-1.0.0-"+runtime.GOOS+"-"+runtime.GOARCH, content)
	sum := sha256.Sum256(archive)
	base := newMockGitHub(t, "v1.0.0", archive, "sha256:"+hex.EncodeToString(sum[:]))

	m := New(testDB(t), t.TempDir())
	m.apiBase = base

	versions, err := m.ListVersions(context.Background())
	if err != nil {
		t.Fatalf("ListVersions 失败: %v", err)
	}
	if len(versions) != 1 || versions[0].Tag != "v1.0.0" || versions[0].Installed {
		t.Fatalf("版本列表不符: %+v", versions)
	}

	tag, err := m.Install(context.Background(), "")
	if err != nil {
		t.Fatalf("Install 失败: %v", err)
	}
	if tag != "v1.0.0" {
		t.Errorf("应安装最新稳定版 v1.0.0, got %s", tag)
	}
	data, err := os.ReadFile(m.CorePath())
	if err != nil {
		t.Fatalf("读取已安装内核失败: %v", err)
	}
	if string(data) != content {
		t.Errorf("内核文件内容不符: %q", data)
	}
	if !m.Installed() {
		t.Error("Installed() 应为 true")
	}
	if m.InstalledVersion() != "v1.0.0" {
		t.Errorf("已装版本应持久化为 v1.0.0, got %q", m.InstalledVersion())
	}
}

func TestInstallDigestMismatch(t *testing.T) {
	archive := buildCoreArchive(t, "sing-box-1.0.0-"+runtime.GOOS+"-"+runtime.GOARCH, "core")
	base := newMockGitHub(t, "v1.0.0", archive, "sha256:"+string(bytes.Repeat([]byte("0"), 64)))

	m := New(testDB(t), t.TempDir())
	m.apiBase = base
	if _, err := m.Install(context.Background(), "1.0.0"); err == nil {
		t.Fatal("摘要不符时 Install 应报错")
	}
	if m.Installed() {
		t.Error("校验失败不应落盘内核")
	}
}
