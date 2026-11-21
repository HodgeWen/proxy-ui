package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// releasesOf 按 tag 依次构造非 draft release，带预发布段的同时标记 GitHub Prerelease。
func releasesOf(tags ...string) []ghRelease {
	rels := make([]ghRelease, 0, len(tags))
	for _, tag := range tags {
		v, ok := parseVersionTag(tag)
		rels = append(rels, ghRelease{TagName: tag, Prerelease: ok && v.pre != "", PublishedAt: time.Now()})
	}
	return rels
}

// tagsOf 取版本列表的 tag 序列（断言用）。
func tagsOf(versions []Version) []string {
	tags := make([]string, 0, len(versions))
	for _, v := range versions {
		tags = append(tags, v.Tag)
	}
	return tags
}

// TestParseVersionTag 版本 tag 解析与拒绝。
func TestParseVersionTag(t *testing.T) {
	ok := map[string]string{
		"v1.2.3":          "1.2.3",
		"1.2.3":           "1.2.3",
		"v1.14.4":         "1.14.4",
		"v1.0.0-alpha.13": "1.0.0-alpha.13",
		"v2.0.0-rc.1":     "2.0.0-rc.1",
	}
	for tag, want := range ok {
		v, parsed := parseVersionTag(tag)
		if !parsed {
			t.Errorf("parseVersionTag(%q) 应解析成功", tag)
			continue
		}
		if got := v.String(); got != want {
			t.Errorf("parseVersionTag(%q).String() = %q, want %q", tag, got, want)
		}
	}
	for _, tag := range []string{"", "v1.2", "v1.2.3.4", "vabc", "v1.-2.3", "v1.+.3", "latest"} {
		if _, parsed := parseVersionTag(tag); parsed {
			t.Errorf("parseVersionTag(%q) 应解析失败", tag)
		}
	}
}

// TestVersionCompare 版本比较：数字段优先，稳定版大于同号预发布版。
func TestVersionCompare(t *testing.T) {
	mustParse := func(tag string) parsedVersion {
		t.Helper()
		v, ok := parseVersionTag(tag)
		if !ok {
			t.Fatalf("解析 %s 失败", tag)
		}
		return v
	}
	tests := []struct {
		a, b string
		want int
	}{
		{"v1.2.3", "v1.2.3", 0},
		{"v1.2.4", "v1.2.3", 1},
		{"v1.2.3", "v1.3.0", -1},
		{"v2.0.0", "v1.99.99", 1},
		{"v1.0.0", "v1.0.0-rc.1", 1},
		{"v1.0.0-rc.1", "v1.0.0", -1},
		{"v1.0.0-alpha.2", "v1.0.0-alpha.10", -1},
		{"v1.0.0-alpha", "v1.0.0-alpha.1", -1},
		{"v1.0.0-alpha.beta", "v1.0.0-alpha.1", 1},
	}
	for _, tt := range tests {
		if got := mustParse(tt.a).compare(mustParse(tt.b)); got != tt.want {
			t.Errorf("%s.compare(%s) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// TestVerifyDigestHex 安装包摘要校验。
func TestVerifyDigestHex(t *testing.T) {
	const digest = "sha256:ABCDEF0123"
	// digest 为空拒绝安装（GitHub 新资产均携带 digest）
	if err := verifyDigestHex("anything", ""); err == nil {
		t.Error("空 digest 应拒绝安装")
	}
	// 大小写不敏感
	if err := verifyDigestHex("abcdef0123", digest); err != nil {
		t.Errorf("摘要应大小写不敏感: %v", err)
	}
	// 不一致报错
	if err := verifyDigestHex("deadbeef", digest); err == nil {
		t.Error("摘要不一致应报错")
	}
	// 不支持的算法报错
	if err := verifyDigestHex("abc", "md5:abc"); err == nil {
		t.Error("不支持的算法应报错")
	}
	if err := verifyDigestHex("abc", "no-colon"); err == nil {
		t.Error("缺少算法前缀应报错")
	}
}

// TestPickRelease 目标版本选取：显式版本匹配 tag，空取最新稳定版。
func TestPickRelease(t *testing.T) {
	rels := releasesOf("v1.14.4", "v1.13.2", "v2.0.0-rc.1")
	// 显式版本（带不带 v 均可）
	for _, version := range []string{"v1.13.2", "1.13.2", " v1.13.2 "} {
		rel, ok := pickRelease(rels, version)
		if !ok || rel.TagName != "v1.13.2" {
			t.Errorf("pickRelease(%q) = %q ok=%v, want v1.13.2", version, rel.TagName, ok)
		}
	}
	// 显式版本允许预发布
	if rel, ok := pickRelease(rels, "v2.0.0-rc.1"); !ok || rel.TagName != "v2.0.0-rc.1" {
		t.Errorf("显式预发布版本应可选中: %q ok=%v", rel.TagName, ok)
	}
	// 版本不存在
	if _, ok := pickRelease(rels, "v9.9.9"); ok {
		t.Error("不存在的版本不应选中")
	}
	// 空版本只取最新稳定版，跳过预发布
	rel, ok := pickRelease(rels, "")
	if !ok || rel.TagName != "v1.14.4" {
		t.Errorf("空版本应取最新稳定版, got %q ok=%v", rel.TagName, ok)
	}
	// 只有预发布时无稳定版可选
	if _, ok := pickRelease(releasesOf("v2.0.0-rc.1"), ""); ok {
		t.Error("只有预发布时不应选出稳定版")
	}
	// draft 一律跳过
	drafts := []ghRelease{
		{TagName: "v9.9.9", Draft: true},
		{TagName: "v1.0.0"},
	}
	if rel, ok := pickRelease(drafts, "v9.9.9"); ok {
		t.Errorf("draft 版本不应被选中: %q", rel.TagName)
	}
	if rel, ok := pickRelease(drafts, ""); !ok || rel.TagName != "v1.0.0" {
		t.Errorf("应取非 draft 版本, got %q", rel.TagName)
	}
	// tag 带预发布后缀但 GitHub prerelease=false 时，仍应跳过（与 curateVersions 一致）
	unflaggedPre := []ghRelease{
		{TagName: "v2.0.0-rc.1", Prerelease: false},
		{TagName: "v1.14.4", Prerelease: false},
	}
	if rel, ok := pickRelease(unflaggedPre, ""); !ok || rel.TagName != "v1.14.4" {
		t.Errorf("带 -rc.1 后缀的 tag 不应作为稳定版选中, got %q ok=%v", rel.TagName, ok)
	}
}

// TestCurateVersions 版本列表策展：稳定版降序 + 最新一条预发布置尾，上限 10 条。
func TestCurateVersions(t *testing.T) {
	tests := []struct {
		name      string
		rels      []ghRelease
		installed string
		want      []string
	}{
		{
			name:      "多个稳定线各取最新并降序",
			rels:      releasesOf("v1.14.1", "v1.14.4", "v1.13.0", "v1.13.2"),
			installed: "1.13.2",
			want:      []string{"v1.14.4", "v1.14.1", "v1.13.2", "v1.13.0"},
		},
		{
			name:      "预发布只保留最新一条并置尾",
			rels:      releasesOf("v1.14.4", "v2.0.0-rc.1", "v2.0.0-rc.2"),
			installed: "",
			want:      []string{"v1.14.4", "v2.0.0-rc.2"},
		},
		{
			name:      "标记已安装",
			rels:      releasesOf("v1.14.4", "v1.13.2"),
			installed: "v1.14.4",
			want:      []string{"v1.14.4", "v1.13.2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tagsOf(curateVersions(tt.rels, tt.installed))
			if len(got) != len(tt.want) {
				t.Fatalf("版本列表 = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("版本列表[%d] = %q, want %q（整体 %v）", i, got[i], tt.want[i], got)
				}
			}
		})
	}

	// 已安装标记
	versions := curateVersions(releasesOf("v1.14.4", "v1.13.2"), "1.13.2")
	for _, v := range versions {
		installed := v.Tag == "v1.13.2"
		if v.Installed != installed {
			t.Errorf("%s 的已安装标记 = %v, want %v", v.Tag, v.Installed, installed)
		}
		if v.Version == "" || v.PublishedAt == "" {
			t.Errorf("版本条目字段不完整: %+v", v)
		}
	}

	// 上限 10 条
	many := make([]string, 0, 20)
	for i := 20; i > 0; i-- {
		many = append(many, fmt.Sprintf("v1.%d.0", i))
	}
	got := curateVersions(releasesOf(many...), "")
	if len(got) != maxCuratedVersions {
		t.Errorf("版本列表应截断到 %d 条, got %d", maxCuratedVersions, len(got))
	}
	if got[0].Tag != "v1.20.0" {
		t.Errorf("首条应为最高版本, got %q", got[0].Tag)
	}

	// draft 与非法 tag 被跳过
	mixed := []ghRelease{
		{TagName: "v1.0.0"},
		{TagName: "v2.0.0", Draft: true},
		{TagName: "not-a-version"},
	}
	mixedTags := tagsOf(curateVersions(mixed, ""))
	if len(mixedTags) != 1 || mixedTags[0] != "v1.0.0" {
		t.Errorf("应只保留合法非 draft 版本, got %v", mixedTags)
	}

	// 12 个稳定版 + 1 个预发布版：稳定版截断到 9 条并保留最新预发布版（共 10 条）
	twelvePlusPre := make([]string, 0, 13)
	for i := 1; i <= 12; i++ {
		twelvePlusPre = append(twelvePlusPre, fmt.Sprintf("v1.%d.0", i))
	}
	twelvePlusPre = append(twelvePlusPre, "v2.0.0-rc.1")
	curated := curateVersions(releasesOf(twelvePlusPre...), "")
	if len(curated) != maxCuratedVersions {
		t.Fatalf("应截断到 %d 条, got %d", maxCuratedVersions, len(curated))
	}
	if curated[0].Tag != "v1.12.0" || curated[maxCuratedVersions-2].Tag != "v1.4.0" {
		t.Errorf("稳定版区间不符: %v", tagsOf(curated))
	}
	if last := curated[maxCuratedVersions-1]; last.Tag != "v2.0.0-rc.1" || !last.Prerelease {
		t.Errorf("末条应保留最新预发布版: %+v", last)
	}

	// GitHub 标记 Prerelease=true（即使 tag 无后缀）也应按预发布处理
	flagged := []ghRelease{
		{TagName: "v1.0.0"},
		{TagName: "v2.0.0", Prerelease: true},
	}
	flaggedGot := curateVersions(flagged, "")
	if len(flaggedGot) != 2 || flaggedGot[0].Tag != "v1.0.0" || !flaggedGot[1].Prerelease {
		t.Errorf("Prerelease=true 应判为预发布: %+v", flaggedGot)
	}
}

// TestFindAsset 按名称查找资产。
func TestFindAsset(t *testing.T) {
	rel := ghRelease{Assets: []ghAsset{
		{Name: "proxy-ui-linux-amd64", Size: 10, Digest: "sha256:aa"},
		{Name: "proxy-ui-darwin-arm64", Size: 20, Digest: "sha256:bb"},
	}}
	asset := findAsset(rel, "proxy-ui-darwin-arm64")
	if asset == nil || asset.Size != 20 {
		t.Fatalf("查找资产失败: %+v", asset)
	}
	if findAsset(rel, "missing") != nil {
		t.Error("不存在的资产应返回 nil")
	}
}

// TestPanelAssetName 面板资产名与构建产物命名一致。
func TestPanelAssetName(t *testing.T) {
	name := panelAssetName()
	if !strings.HasPrefix(name, "proxy-ui-") {
		t.Errorf("资产名前缀不符: %q", name)
	}
	if parts := strings.Split(name, "-"); len(parts) != 4 {
		t.Errorf("资产名应为 proxy-ui-<os>-<arch>, got %q", name)
	}
}

// TestCurrentVersion 面板当前版本号透传。
func TestCurrentVersion(t *testing.T) {
	if got := NewSelfUpdate("1.2.3").CurrentVersion(); got != "1.2.3" {
		t.Errorf("CurrentVersion = %q, want 1.2.3", got)
	}
}

func sha256Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// TestDownloadFile 验证下载与摘要校验（含非法摘要格式不发 HTTP 请求）。
func TestDownloadFile(t *testing.T) {
	payload := []byte("fake-proxy-ui-binary-payload")
	goodDigest := sha256Digest(payload)

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	client := ghClient{apiBase: srv.URL, client: srv.Client()}
	tmpDir := t.TempDir()

	// 1. 正常下载与校验
	gotPath, err := client.downloadFile(context.Background(), srv.URL+"/bin", goodDigest, tmpDir)
	if err != nil {
		t.Fatalf("合法下载不应失败: %v", err)
	}
	gotBytes, err := os.ReadFile(gotPath)
	if err != nil || string(gotBytes) != string(payload) {
		t.Fatalf("下载内容不符: %q err=%v", gotBytes, err)
	}
	_ = os.Remove(gotPath)

	// 2. digest 校验失败应删除临时文件并报错
	wrongDigest := "sha256:" + strings.Repeat("0", 64)
	if _, err := client.downloadFile(context.Background(), srv.URL+"/bin", wrongDigest, tmpDir); err == nil {
		t.Fatal("摘要不匹配应报错")
	}
	if entries, err := os.ReadDir(tmpDir); err != nil || len(entries) != 0 {
		t.Errorf("校验失败后应清理临时文件, got %v err=%v", entries, err)
	}

	// 3. digest 格式非法应在发起 HTTP GET 前直接拒绝
	before := hits.Load()
	for _, bad := range []string{"", "md5:1234", "sha256:abcd", "sha256:" + strings.Repeat("z", 64)} {
		if _, err := client.downloadFile(context.Background(), srv.URL+"/bin", bad, tmpDir); err == nil {
			t.Errorf("非法摘要格式 %q 应被拒绝", bad)
		}
	}
	if hits.Load() != before {
		t.Errorf("digest 格式非法时不应发起 HTTP 请求: hits %d -> %d", before, hits.Load())
	}
}

// TestSelfUpdaterLifecycle 覆盖 ListVersions、Apply 与 Restart 全链路。
func TestSelfUpdaterLifecycle(t *testing.T) {
	newBin := []byte("new-proxy-ui-binary")
	digest := sha256Digest(newBin)
	assetName := panelAssetName()

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/" + panelRepo + "/releases":
			rels := []ghRelease{
				{
					TagName:     "v1.2.0",
					PublishedAt: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
					Assets:      []ghAsset{{Name: assetName, Size: int64(len(newBin)), Digest: digest, BrowserDownloadURL: srvURL + "/dl/v1.2.0"}},
				},
				{
					TagName:     "v1.1.0",
					PublishedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
					Assets:      []ghAsset{{Name: assetName, Size: 3, Digest: sha256Digest([]byte("old")), BrowserDownloadURL: srvURL + "/dl/v1.1.0"}},
				},
			}
			_ = json.NewEncoder(w).Encode(rels)
		case "/dl/v1.2.0":
			_, _ = w.Write(newBin)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	exe := filepath.Join(t.TempDir(), "proxy-ui")
	if err := os.WriteFile(exe, []byte("old-binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	var restartedExe string
	u := NewSelfUpdate("v1.1.0")
	u.apiBase = srv.URL
	u.client = srv.Client()
	u.exePath = exe
	u.restart = func(p string) error {
		restartedExe = p
		return nil
	}

	versions, err := u.ListVersions(context.Background())
	if err != nil {
		t.Fatalf("ListVersions 失败: %v", err)
	}
	if len(versions) != 2 || versions[0].Tag != "v1.2.0" || versions[0].Installed || !versions[1].Installed {
		t.Fatalf("ListVersions 结果不符: %+v", versions)
	}

	tag, err := u.Apply(context.Background(), "")
	if err != nil {
		t.Fatalf("Apply 失败: %v", err)
	}
	if tag != "v1.2.0" {
		t.Errorf("Apply tag = %q, want v1.2.0", tag)
	}
	got, err := os.ReadFile(exe)
	if err != nil || string(got) != string(newBin) {
		t.Fatalf("替换后的二进制内容不符: %q err=%v", got, err)
	}
	if info, err := os.Stat(exe); err != nil || info.Mode()&0o111 == 0 {
		t.Errorf("替换后应具备可执行权限: mode=%v err=%v", info.Mode(), err)
	}

	if err := u.Restart(); err != nil {
		t.Fatalf("Restart 失败: %v", err)
	}
	if restartedExe != exe {
		t.Errorf("Restart 回调路径 = %q, want %q", restartedExe, exe)
	}
}
