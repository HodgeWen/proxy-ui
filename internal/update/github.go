// Package update 提供面板自身（proxy-ui）的 GitHub Releases 版本查询与自更新。
//
// 内核已改为编进面板（见 internal/singbox），不再从 Releases 下载内核二进制，
// 因此这里只保留面板自更新所需的最小实现。
package update

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// defaultAPIBase GitHub API 基址。
const defaultAPIBase = "https://api.github.com"

// ghRelease GitHub Releases API 响应精简结构。
type ghRelease struct {
	TagName     string    `json:"tag_name"`   // 版本 tag（v1.2.3）
	Draft       bool      `json:"draft"`      // 是否草稿
	Prerelease  bool      `json:"prerelease"` // 是否预发布
	PublishedAt time.Time `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
}

// ghAsset release 资产。
type ghAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"` // 形如 sha256:<hex>
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Version 面板展示的版本条目。
type Version struct {
	Tag         string `json:"tag"`         // 版本 tag
	Version     string `json:"version"`     // 不带 v 的版本号
	Prerelease  bool   `json:"prerelease"`  // 是否预发布
	PublishedAt string `json:"publishedAt"` // 发布时间（RFC3339）
	Installed   bool   `json:"installed"`   // 是否当前已安装版本
}

// parsedVersion 解析后的语义化版本，用于排序比较。
type parsedVersion struct {
	major, minor, patch int
	pre                 string // 预发布段（alpha.13 等），空表示稳定版
}

// parseVersionTag 解析 v1.2.3 / 1.2.3-alpha.1 形式 tag；无法解析返回 ok=false。
func parseVersionTag(tag string) (parsedVersion, bool) {
	core, pre, _ := strings.Cut(strings.TrimPrefix(tag, "v"), "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return parsedVersion{}, false
	}
	nums := [3]int{}
	for i, p := range parts {
		n, err := parseNonNegative(p)
		if err != nil {
			return parsedVersion{}, false
		}
		nums[i] = n
	}
	return parsedVersion{nums[0], nums[1], nums[2], pre}, true
}

// String 输出不带 v 的版本号（含预发布段）。
func (v parsedVersion) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
	if v.pre != "" {
		s += "-" + v.pre
	}
	return s
}

// compare 比较版本先后：小于返回 -1；稳定版大于同号预发布版。
func (v parsedVersion) compare(o parsedVersion) int {
	if c := cmp.Or(
		cmp.Compare(v.major, o.major),
		cmp.Compare(v.minor, o.minor),
		cmp.Compare(v.patch, o.patch),
	); c != 0 {
		return c
	}
	switch {
	case v.pre == o.pre:
		return 0
	case v.pre == "":
		return 1
	case o.pre == "":
		return -1
	default:
		return comparePrerelease(v.pre, o.pre)
	}
}

// comparePrerelease 逐段比较预发布号：数字段按数值、其余按字典序。
func comparePrerelease(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		an, aerr := parseNonNegative(as[i])
		bn, berr := parseNonNegative(bs[i])
		var c int
		if aerr == nil && berr == nil {
			c = cmp.Compare(an, bn)
		} else {
			c = strings.Compare(as[i], bs[i])
		}
		if c != 0 {
			return c
		}
	}
	return cmp.Compare(len(as), len(bs))
}

// parseNonNegative 解析非负整数（拒绝带符号前缀）。
func parseNonNegative(s string) (int, error) {
	if s == "" || s[0] == '+' || s[0] == '-' {
		return 0, fmt.Errorf("非非负整数: %q", s)
	}
	return strconv.Atoi(s)
}

// verifyDigestHex 比对 sha256 十六进制摘要与 GitHub 资产 digest（"sha256:<hex>"）；
// digest 为空时跳过校验。
func verifyDigestHex(got, digest string) error {
	if digest == "" {
		return nil
	}
	algo, want, ok := strings.Cut(digest, ":")
	if !ok || algo != "sha256" {
		return fmt.Errorf("不支持的校验摘要格式: %s", digest)
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("安装包完整性校验失败：期望 sha256 %s，实际 %s", want, got)
	}
	return nil
}

// pickRelease 选取目标 release：version 归一化匹配 tag，空则取最新稳定版。
func pickRelease(rels []ghRelease, version string) (ghRelease, bool) {
	if want := strings.TrimPrefix(strings.TrimSpace(version), "v"); want != "" {
		for _, r := range rels {
			if !r.Draft && strings.TrimPrefix(r.TagName, "v") == want {
				return r, true
			}
		}
		return ghRelease{}, false
	}
	var (
		best  ghRelease
		bestV parsedVersion
		found bool
	)
	for _, r := range rels {
		if r.Draft || r.Prerelease {
			continue
		}
		v, ok := parseVersionTag(r.TagName)
		if !ok || found && v.compare(bestV) <= 0 {
			continue
		}
		best, bestV, found = r, v, true
	}
	return best, found
}

// findAsset 在 release 中查找指定名称资产。
func findAsset(rel ghRelease, name string) *ghAsset {
	for i := range rel.Assets {
		if rel.Assets[i].Name == name {
			return &rel.Assets[i]
		}
	}
	return nil
}

// curateVersions 列出版本：稳定版 + 最新一条预发布（若有），按版本降序，最多 10 条。
func curateVersions(rels []ghRelease, installedTag string) []Version {
	installed := strings.TrimPrefix(installedTag, "v")
	type entry struct {
		rel ghRelease
		v   parsedVersion
	}
	var stable, pre []entry
	for _, r := range rels {
		if r.Draft {
			continue
		}
		v, ok := parseVersionTag(r.TagName)
		if !ok {
			continue
		}
		if v.pre == "" {
			stable = append(stable, entry{rel: r, v: v})
		} else {
			pre = append(pre, entry{rel: r, v: v})
		}
	}
	desc := func(a, b entry) int { return -a.v.compare(b.v) }
	slices.SortFunc(stable, desc)
	slices.SortFunc(pre, desc)
	entries := stable
	if len(pre) > 0 {
		entries = append(entries, pre[0]) // 最新预发布作「尝鲜」项
	}
	out := make([]Version, 0, min(len(entries), maxCuratedVersions))
	for _, e := range entries {
		if len(out) >= maxCuratedVersions {
			break
		}
		out = append(out, Version{
			Tag:         e.rel.TagName,
			Version:     e.v.String(),
			Prerelease:  e.rel.Prerelease || e.v.pre != "",
			PublishedAt: e.rel.PublishedAt.Format(time.RFC3339),
			Installed:   strings.TrimPrefix(e.rel.TagName, "v") == installed,
		})
	}
	return out
}

// maxCuratedVersions 版本列表最大条数。
const maxCuratedVersions = 10

// ghClient GitHub Releases 访问底座：请求与下载校验。
type ghClient struct {
	apiBase string // GitHub API 基址（测试可替换）
	client  *http.Client
}

// newGHClient 构建默认 GitHub 访问底座。
func newGHClient() ghClient {
	return ghClient{apiBase: defaultAPIBase, client: &http.Client{Timeout: 10 * time.Minute}}
}

// get 发起 GET 请求并校验状态码。
func (g *ghClient) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "proxy-ui")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub 失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return resp, nil
}

// fetchReleases 拉取指定仓库的 Releases 列表。
func (g *ghClient) fetchReleases(ctx context.Context, repo string) ([]ghRelease, error) {
	resp, err := g.get(ctx, g.apiBase+"/repos/"+repo+"/releases?per_page=100")
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

// listVersions 查询仓库可用版本列表（策展规则见 curateVersions）。
func (g *ghClient) listVersions(ctx context.Context, repo, installedTag string) ([]Version, error) {
	rels, err := g.fetchReleases(ctx, repo)
	if err != nil {
		return nil, err
	}
	return curateVersions(rels, installedTag), nil
}

// downloadFile 下载 url 到 tmpDir 下临时文件并按 digest 校验完整性，返回临时文件路径（调用方负责删除）。
func (g *ghClient) downloadFile(ctx context.Context, url, digest, tmpDir string) (string, error) {
	resp, err := g.get(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	tmp, err := os.CreateTemp(tmpDir, ".download-*.tmp")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), resp.Body); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("下载安装包失败: %w", err)
	}
	tmp.Close()
	if err := verifyDigestHex(hex.EncodeToString(hash.Sum(nil)), digest); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}
