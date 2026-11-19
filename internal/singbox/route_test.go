package singbox

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// jsonEqual 经 JSON 序列化后比较（避免 []string 与 []any 类型差异）。
func jsonEqual(got, want any) bool {
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(want)
	return string(a) == string(b)
}

// templateRouting 「国内直连+海外代理+广告拦截」模板应用后的路由聚合输入形态。
func templateRouting() *Routing {
	return &Routing{
		Rules: []RuleEntry{
			{Type: RuleTypeRuleSet, Value: AntiADTag, Outbound: OutboundBlock},
			{Type: RuleTypeGeosite, Value: "cn", Outbound: "direct"},
			{Type: RuleTypeGeoIP, Value: "cn", Outbound: "direct"},
		},
		Groups: []GroupEntry{{Name: "proxy", Mode: GroupModeAuto, Members: []string{"hk", "jp"}}},
		Final:  "proxy",
	}
}

// templateOutbounds 模板可用的自定义出站。
func templateOutbounds() []OutboundEntry {
	return []OutboundEntry{
		{Tag: "hk", Type: "trojan", Config: `{"server":"hk.example.com","server_port":443}`},
		{Tag: "jp", Type: "trojan", Config: `{"server":"jp.example.com","server_port":443}`},
	}
}

func TestBuildConfigTemplateRouting(t *testing.T) {
	cfg, err := BuildConfig(nil, templateOutbounds(), templateRouting())
	if err != nil {
		t.Fatalf("聚合生成失败: %v", err)
	}
	if cfg.Route == nil {
		t.Fatalf("应生成 route 段")
	}
	if len(cfg.Route.Rules) != 3 {
		t.Fatalf("应有 3 条路由规则, got %d", len(cfg.Route.Rules))
	}
	adRule, cnRule, ipRule := cfg.Route.Rules[0], cfg.Route.Rules[1], cfg.Route.Rules[2]
	if adRule["action"] != "reject" || !jsonEqual(adRule["rule_set"], []string{"anti-ad"}) {
		t.Errorf("anti-AD 规则应为 reject + rule_set anti-ad: %v", adRule)
	}
	if cnRule["outbound"] != "direct" || !jsonEqual(cnRule["rule_set"], []string{"geosite-cn"}) {
		t.Errorf("国内域名规则应为 geosite-cn -> direct: %v", cnRule)
	}
	if ipRule["outbound"] != "direct" || !jsonEqual(ipRule["rule_set"], []string{"geoip-cn"}) {
		t.Errorf("国内 IP 规则应为 geoip-cn -> direct: %v", ipRule)
	}
	if len(cfg.Route.RuleSet) != 3 {
		t.Fatalf("应有 3 个规则集, got %d", len(cfg.Route.RuleSet))
	}
	wantURLs := map[string]string{
		"anti-ad":    DefaultAntiADURL,
		"geosite-cn": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs",
		"geoip-cn":   "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs",
	}
	for _, rs := range cfg.Route.RuleSet {
		if want, ok := wantURLs[rs.Tag]; !ok || rs.URL != want || rs.Type != "remote" || rs.Format != "binary" {
			t.Errorf("规则集 %s 定义不符: %+v", rs.Tag, rs)
		}
	}
	if cfg.Route.Final != "proxy" {
		t.Errorf("兜底出站应为 proxy, got %s", cfg.Route.Final)
	}
	// 策略组映射为 urltest 出站，且 direct 兜底补齐
	group, ok := outboundByTag(cfg, "proxy")
	if !ok || group["type"] != "urltest" || !jsonEqual(group["outbounds"], []string{"hk", "jp"}) {
		t.Errorf("策略组应映射为 urltest 出站: %v", group)
	}
	if _, ok := outboundByTag(cfg, "direct"); !ok {
		t.Errorf("应补 direct 兜底出站: %v", cfg.Outbounds)
	}
	// 序列化为合法 JSON 且包含 route/rule_set/reject 基线
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	for _, want := range []string{`"route"`, `"rule_set"`, `"reject"`, `"final":"proxy"`, `"urltest"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("聚合配置缺少 %s: %s", want, data)
		}
	}
}

func TestBuildConfigAntiADSourceOverride(t *testing.T) {
	r := &Routing{
		Rules:  []RuleEntry{{Type: RuleTypeRuleSet, Value: "anti-ad", Outbound: OutboundBlock}},
		AntiAD: "https://mirror.example.com/anti-ad.srs",
	}
	cfg, err := BuildConfig(nil, nil, r)
	if err != nil {
		t.Fatalf("聚合生成失败: %v", err)
	}
	if cfg.Route.RuleSet[0].URL != r.AntiAD {
		t.Errorf("anti-AD 源地址更换后配置应随之更新, got %s", cfg.Route.RuleSet[0].URL)
	}
}

func TestGroupOutboundModes(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want map[string]any
	}{
		{name: "固定代理映射 selector 且默认首个成员", mode: GroupModeFixed,
			want: map[string]any{"type": "selector", "default": "hk"}},
		{name: "自动选择映射 urltest", mode: GroupModeAuto,
			want: map[string]any{"type": "urltest"}},
		{name: "故障转移映射 urltest 且带容差", mode: GroupModeFailover,
			want: map[string]any{"type": "urltest", "tolerance": 500}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := BuildConfig(nil, templateOutbounds()[:1], &Routing{
				Groups: []GroupEntry{{Name: "g", Mode: tt.mode, Members: []string{"hk"}}},
			})
			if err != nil {
				t.Fatalf("聚合生成失败: %v", err)
			}
			group, _ := outboundByTag(cfg, "g")
			for k, v := range tt.want {
				if !reflect.DeepEqual(group[k], v) {
					t.Errorf("字段 %s = %v, want %v", k, group[k], v)
				}
			}
			if !jsonEqual(group["outbounds"], []string{"hk"}) {
				t.Errorf("成员应保持顺序: %v", group["outbounds"])
			}
		})
	}
}

func TestBuildConfigRoutingErrors(t *testing.T) {
	tests := []struct {
		name    string
		routing *Routing
		wantErr string
	}{
		{
			name:    "规则出站不存在",
			routing: &Routing{Rules: []RuleEntry{{Type: RuleTypeDomain, Value: "a.com", Outbound: "nope"}}},
			wantErr: "出站 nope 不存在",
		},
		{
			name:    "兜底出站不存在",
			routing: &Routing{Final: "nope"},
			wantErr: "兜底出站 nope 不存在",
		},
		{
			name:    "规则集无下载地址",
			routing: &Routing{Rules: []RuleEntry{{Type: RuleTypeRuleSet, Value: "whatever", Outbound: "direct"}}},
			wantErr: "无可用下载地址",
		},
		{
			name:    "不支持的规则类型",
			routing: &Routing{Rules: []RuleEntry{{Type: "port", Value: "80", Outbound: "direct"}}},
			wantErr: "不支持的路由规则类型",
		},
		{
			name:    "策略组成员不存在",
			routing: &Routing{Groups: []GroupEntry{{Name: "g", Mode: GroupModeAuto, Members: []string{"nope"}}}},
			wantErr: "成员 nope 不存在",
		},
		{
			name:    "策略组未配置成员",
			routing: &Routing{Groups: []GroupEntry{{Name: "g", Mode: GroupModeAuto}}},
			wantErr: "未配置成员",
		},
		{
			name:    "策略组模式不支持",
			routing: &Routing{Groups: []GroupEntry{{Name: "g", Mode: "random", Members: []string{"direct"}}}},
			wantErr: "模式不支持",
		},
		{
			name:    "策略组与出站同名",
			routing: &Routing{Groups: []GroupEntry{{Name: "hk", Mode: GroupModeAuto, Members: []string{"direct"}}}},
			wantErr: "同名",
		},
		{
			name: "策略组自引用",
			routing: &Routing{
				Groups: []GroupEntry{{Name: "g", Mode: GroupModeAuto, Members: []string{"g"}}},
			},
			wantErr: "不能引用自身",
		},
		{
			name: "策略组循环引用",
			routing: &Routing{
				Groups: []GroupEntry{
					{Name: "a", Mode: GroupModeAuto, Members: []string{"b"}},
					{Name: "b", Mode: GroupModeAuto, Members: []string{"a"}},
				},
			},
			wantErr: "循环引用",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildConfig(nil, templateOutbounds(), tt.routing)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("错误应含 %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestRoutingRuleJSON(t *testing.T) {
	// 多值逗号拆分与 literal 字段映射
	rule, refs, err := RoutingRuleJSON(RuleEntry{Type: RuleTypeDomainSuffix, Value: " google.com, youtube.com ", Outbound: "proxy"})
	if err != nil {
		t.Fatalf("映射失败: %v", err)
	}
	if !jsonEqual(rule["domain_suffix"], []string{"google.com", "youtube.com"}) || rule["outbound"] != "proxy" || refs != nil {
		t.Errorf("多值规则映射不符: %v %v", rule, refs)
	}
	if _, _, err := RoutingRuleJSON(RuleEntry{Type: RuleTypeDomain, Value: "  ", Outbound: "direct"}); err == nil {
		t.Errorf("空匹配值应报错")
	}
}

func TestRuleSetDedup(t *testing.T) {
	r := &Routing{Rules: []RuleEntry{
		{Type: RuleTypeGeosite, Value: "cn", Outbound: "direct"},
		{Type: RuleTypeGeosite, Value: "category-ads", Outbound: OutboundBlock},
		{Type: RuleTypeGeoIP, Value: "cn, private", Outbound: "direct"},
	}}
	cfg, err := BuildConfig(nil, nil, r)
	if err != nil {
		t.Fatalf("聚合生成失败: %v", err)
	}
	if len(cfg.Route.Rules) != 3 || len(cfg.Route.RuleSet) != 4 {
		t.Fatalf("规则集应去重为 4 个, got %d: %+v", len(cfg.Route.RuleSet), cfg.Route.RuleSet)
	}
	tags := map[string]bool{}
	for _, rs := range cfg.Route.RuleSet {
		tags[rs.Tag] = true
	}
	for _, want := range []string{"geosite-cn", "geosite-category-ads", "geoip-cn", "geoip-private"} {
		if !tags[want] {
			t.Errorf("缺少规则集 %s: %+v", want, cfg.Route.RuleSet)
		}
	}
}

// outboundByTag 按 tag 取聚合生成的出站。
func outboundByTag(cfg Config, tag string) (map[string]any, bool) {
	for _, ob := range cfg.Outbounds {
		if ob["tag"] == tag {
			return ob, true
		}
	}
	return nil, false
}
