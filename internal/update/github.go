// Package update 提供 GitHub Releases 版本查询与 sing-box 内核下载、完整性校验与安装。
package update

import (
	"cmp"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// repo sing-box 内核所在 GitHub 仓库。
const repo = "SagerNet/sing-box"

// ghRelease GitHub Releases API 响应精简结构。
type ghRelease struct {
	TagName     string    `json:"tag_name"`   // 版本 tag（v1.14.4）
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

// Version 面板展示的内核版本条目。
type Version struct {
	Tag         string `json:"tag"`         // 版本 tag（v1.14.4）
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

// parseVersionTag 解析 v1.14.4 / 1.15.0-alpha.13 形式 tag；无法解析返回 ok=false。
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

// MirrorURL 拼接镜像前缀：mirror 为空返回原 URL，否则 <mirror>/<rawURL>。
func MirrorURL(mirror, rawURL string) string {
	if mirror == "" {
		return rawURL
	}
	return strings.TrimSuffix(mirror, "/") + "/" + rawURL
}

// verifyDigestHex 比对 sha256 十六进制摘要与 GitHub 资产 digest（"sha256:<hex>"）；
// digest 为空时跳过校验（依赖解压时的 gzip CRC 兜底）。
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

// assetName 计算当前平台对应的 release 资产名（sing-box-<版本>-<os>-<arch>.tar.gz）。
func assetName(tag string) string {
	return fmt.Sprintf("sing-box-%s-%s-%s.tar.gz", strings.TrimPrefix(tag, "v"), runtime.GOOS, runtime.GOARCH)
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
