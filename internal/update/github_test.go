package update

import (
	"fmt"
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

// stableLineTags 构造 minor 从 n-1 到 0 的稳定线代表 tag（v1.<minor>.0）。
func stableLineTags(n int) []string {
	tags := make([]string, 0, n)
	for minor := range n {
		tags = append(tags, fmt.Sprintf("v1.%d.0", minor))
	}
	return tags
}

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
			want:      []string{"1.14.4", "1.13.2"},
		},
		{
			name: "纯预发布新线取最新预发布置尾（draft 剔除）",
			rels: append([]ghRelease{{TagName: "v1.16.0", Draft: true}},
				releasesOf("v1.15.0-alpha.2", "v1.15.0-beta.1", "v1.14.4", "v1.13.0")...),
			want: []string{"1.14.4", "1.13.0", "1.15.0-beta.1"},
		},
		{
			name: "稳定线内自己的预发布不展示",
			rels: releasesOf("v1.14.0-beta.1", "v1.14.4", "v1.14.0-rc.2"),
			want: []string{"1.14.4"},
		},
		{
			name: "低于最高稳定线的纯预发布线不展示",
			rels: releasesOf("v1.14.4", "v1.9.0-beta.3"),
			want: []string{"1.14.4"},
		},
		{
			name: "稳定线超过 10 条截断",
			rels: releasesOf(stableLineTags(12)...),
			want: []string{"1.11.0", "1.10.0", "1.9.0", "1.8.0", "1.7.0", "1.6.0", "1.5.0", "1.4.0", "1.3.0", "1.2.0"},
		},
		{
			name: "稳定线满 10 不追加尝鲜预发布",
			rels: append(releasesOf(stableLineTags(10)...), releasesOf("v2.0.0-alpha.1")...),
			want: []string{"1.9.0", "1.8.0", "1.7.0", "1.6.0", "1.5.0", "1.4.0", "1.3.0", "1.2.0", "1.1.0", "1.0.0"},
		},
		{
			name: "尝鲜预发布计入 10 条上限",
			rels: append(releasesOf(stableLineTags(9)...), releasesOf("v2.0.0-alpha.1")...),
			want: []string{"1.8.0", "1.7.0", "1.6.0", "1.5.0", "1.4.0", "1.3.0", "1.2.0", "1.1.0", "1.0.0", "2.0.0-alpha.1"},
		},
		{
			name: "无法解析的 tag 跳过",
			rels: append(releasesOf("v1.14.4"), ghRelease{TagName: "testing"}),
			want: []string{"1.14.4"},
		},
		{
			name: "空列表返回空",
			rels: nil,
			want: nil,
		},
		{
			name: "全部为预发布时仅展示最高线尝鲜项",
			rels: releasesOf("v1.15.0-alpha.1", "v1.14.0-beta.2"),
			want: []string{"1.15.0-alpha.1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := curateVersions(tt.rels, tt.installed)
			gotVersions := make([]string, 0, len(got))
			for _, v := range got {
				gotVersions = append(gotVersions, v.Version)
			}
			if len(gotVersions) != len(tt.want) {
				t.Fatalf("curateVersions 条数 = %d (%v), 期望 %d (%v)", len(gotVersions), gotVersions, len(tt.want), tt.want)
			}
			for i := range got {
				if got[i].Version != tt.want[i] {
					t.Fatalf("curateVersions[%d] = %q, 期望 %q（完整列表 %v）", i, got[i].Version, tt.want[i], gotVersions)
				}
				if got[i].Tag != "v"+got[i].Version {
					t.Errorf("Tag 与 Version 不一致: %q vs %q", got[i].Tag, got[i].Version)
				}
			}
		})
	}
}

func TestCurateVersionsFlags(t *testing.T) {
	got := curateVersions(releasesOf("v1.15.0-beta.1", "v1.14.4", "v1.13.0"), "v1.14.4")

	if got[0].Prerelease {
		t.Error("稳定代表不应标记 Prerelease")
	}
	last := got[len(got)-1]
	if last.Version != "1.15.0-beta.1" || !last.Prerelease {
		t.Errorf("尝鲜项应为最新预发布且标记 Prerelease: %+v", last)
	}
	if !got[0].Installed || got[1].Installed || last.Installed {
		t.Errorf("仅已安装版本 v1.14.4 标记 Installed: %+v", got)
	}
	if got[0].PublishedAt == "" {
		t.Error("PublishedAt 不应为空")
	}
}
