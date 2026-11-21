package singbox

import (
	"encoding/json"
	"testing"
)

// jsonEqual 经 JSON 序列化后比较（避免 []string 与 []any 类型差异）。
func jsonEqual(t *testing.T, got, want any) bool {
	t.Helper()
	a, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("序列化 got 失败: %v", err)
	}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("序列化 want 失败: %v", err)
	}
	return string(a) == string(b)
}

// TestRoutingRuleJSONMatchFields 全部（匹配对象 × 匹配方式）组合映射到正确内核字段。
func TestRoutingRuleJSONMatchFields(t *testing.T) {
	tests := []struct {
		name       string
		matchType  string
		method     string
		wantField  string
		wantValues []string
	}{
		{"域名精确", MatchDomain, MethodExact, "domain", []string{"a.com", "b.com"}},
		{"域名后缀", MatchDomain, MethodSuffix, "domain_suffix", []string{"a.com", "b.com"}},
		{"域名关键字", MatchDomain, MethodKeyword, "domain_keyword", []string{"a.com", "b.com"}},
		{"域名正则", MatchDomain, MethodRegex, "domain_regex", []string{"a.com", "b.com"}},
		{"IP 网段", MatchIP, MethodCIDR, "ip_cidr", []string{"10.0.0.0/8", "192.168.0.0/16"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, refSet, err := RoutingRuleJSON(RuleEntry{
				MatchType: tt.matchType, MatchMethod: tt.method,
				Value: " a.com , b.com ", Outbound: "proxy",
			})
			if err != nil {
				t.Fatalf("映射规则失败: %v", err)
			}
			if refSet != nil {
				t.Errorf("非 rule_set 规则不应返回规则集定义: %+v", refSet)
			}
			if rule["outbound"] != "proxy" {
				t.Errorf("outbound = %v, want proxy", rule["outbound"])
			}
			if _, ok := rule["action"]; ok {
				t.Errorf("非 block 规则不应带 action: %v", rule)
			}
			// 匹配值统一按逗号拆分并去空白，IP 规则同样适用同一份输入
			got, ok := rule[tt.wantField]
			if !ok {
				t.Fatalf("规则缺少字段 %s: %v", tt.wantField, rule)
			}
			items, ok := got.([]string)
			if !ok {
				t.Fatalf("字段 %s 类型 = %T, want []string", tt.wantField, got)
			}
			if len(items) != 2 || items[0] != "a.com" || items[1] != "b.com" {
				t.Errorf("字段 %s 值不符: %v", tt.wantField, items)
			}
		})
	}
}

// TestRoutingRuleJSONInvalidCombos 非法组合与空值必须报错。
func TestRoutingRuleJSONInvalidCombos(t *testing.T) {
	tests := []struct {
		name string
		rule RuleEntry
	}{
		{"域名+cidr", RuleEntry{MatchType: MatchDomain, MatchMethod: MethodCIDR, Value: "a.com", Outbound: "direct"}},
		{"域名+空方式", RuleEntry{MatchType: MatchDomain, Value: "a.com", Outbound: "direct"}},
		{"IP+后缀", RuleEntry{MatchType: MatchIP, MatchMethod: MethodSuffix, Value: "10.0.0.0/8", Outbound: "direct"}},
		{"IP+空方式", RuleEntry{MatchType: MatchIP, Value: "10.0.0.0/8", Outbound: "direct"}},
		{"未知匹配对象", RuleEntry{MatchType: "geoip", MatchMethod: MethodCIDR, Value: "cn", Outbound: "direct"}},
		{"空匹配值", RuleEntry{MatchType: MatchDomain, MatchMethod: MethodSuffix, Value: " , ", Outbound: "direct"}},
		{"规则集缺下载源", RuleEntry{MatchType: MatchRuleSet, Outbound: "direct"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := RoutingRuleJSON(tt.rule); err == nil {
				t.Errorf("应报错: %+v", tt.rule)
			}
		})
	}
}

// TestRoutingRuleJSONRuleSet rule_set 规则产出 binary 远程规则集定义。
func TestRoutingRuleJSONRuleSet(t *testing.T) {
	src := &RuleSetSource{
		Tag: AntiADTag, URL: AntiADBuiltinURL,
		MihomoURL: AntiADMihomoURL, MihomoBehavior: "domain",
	}
	rule, refSet, err := RoutingRuleJSON(RuleEntry{MatchType: MatchRuleSet, RuleSet: src, Outbound: "direct"})
	if err != nil {
		t.Fatalf("映射规则集规则失败: %v", err)
	}
	if !jsonEqual(t, rule["rule_set"], []string{AntiADTag}) {
		t.Errorf("rule_set 字段不符: %v", rule)
	}
	if rule["outbound"] != "direct" {
		t.Errorf("outbound = %v, want direct", rule["outbound"])
	}
	if refSet == nil {
		t.Fatal("rule_set 规则应返回规则集定义")
	}
	if refSet.Type != "remote" {
		t.Errorf("规则集类型 = %q, want remote", refSet.Type)
	}
	if refSet.Format != "binary" {
		t.Errorf("规则集格式 = %q, want binary", refSet.Format)
	}
	if refSet.Tag != AntiADTag || refSet.URL != AntiADBuiltinURL {
		t.Errorf("规则集 tag/url 不符: %+v", refSet)
	}
}

// TestRoutingRuleJSONBlock block 出站映射为 reject 动作且不产生 outbound 字段。
func TestRoutingRuleJSONBlock(t *testing.T) {
	rule, _, err := RoutingRuleJSON(RuleEntry{
		MatchType: MatchDomain, MatchMethod: MethodKeyword, Value: "ad.", Outbound: OutboundBlock,
	})
	if err != nil {
		t.Fatalf("映射拦截规则失败: %v", err)
	}
	if rule["action"] != "reject" {
		t.Errorf("block 应映射为 reject: %v", rule)
	}
	if _, ok := rule["outbound"]; ok {
		t.Errorf("reject 规则不应带 outbound: %v", rule)
	}
}

// TestGroupOutboundJSON 策略组模式映射。
func TestGroupOutboundJSON(t *testing.T) {
	// 固定代理 → selector，默认选中首个成员
	ob, err := GroupOutboundJSON(GroupEntry{Name: "g-fixed", Mode: GroupModeFixed, Members: []string{"hk", "jp"}})
	if err != nil {
		t.Fatalf("映射固定组失败: %v", err)
	}
	if ob["type"] != "selector" || ob["tag"] != "g-fixed" || ob["default"] != "hk" {
		t.Errorf("固定组字段不符: %v", ob)
	}
	if !jsonEqual(t, ob["outbounds"], []string{"hk", "jp"}) {
		t.Errorf("固定组成员不符: %v", ob)
	}

	// 自动选择 → urltest，无容差
	ob, err = GroupOutboundJSON(GroupEntry{Name: "g-auto", Mode: GroupModeAuto, Members: []string{"hk"}})
	if err != nil {
		t.Fatalf("映射自动组失败: %v", err)
	}
	if ob["type"] != "urltest" {
		t.Errorf("自动组类型 = %v, want urltest", ob["type"])
	}
	if _, ok := ob["tolerance"]; ok {
		t.Errorf("自动组不应设置容差: %v", ob)
	}

	// 故障转移 → urltest + 容差
	ob, err = GroupOutboundJSON(GroupEntry{Name: "g-failover", Mode: GroupModeFailover, Members: []string{"hk"}})
	if err != nil {
		t.Fatalf("映射故障转移组失败: %v", err)
	}
	if ob["type"] != "urltest" || ob["tolerance"] != 500 {
		t.Errorf("故障转移组字段不符: %v", ob)
	}

	// 无成员 / 未知模式报错
	if _, err := GroupOutboundJSON(GroupEntry{Name: "g", Mode: GroupModeAuto}); err == nil {
		t.Error("无成员策略组应报错")
	}
	if _, err := GroupOutboundJSON(GroupEntry{Name: "g", Mode: "roundrobin", Members: []string{"hk"}}); err == nil {
		t.Error("不支持的策略组模式应报错")
	}
}

// TestBuildConfigRuleSetDedup 同一规则集被多条规则引用时只生成一份规则集定义、按出现顺序排列。
func TestBuildConfigRuleSetDedup(t *testing.T) {
	adSrc := &RuleSetSource{Tag: AntiADTag, URL: AntiADBuiltinURL, MihomoURL: AntiADMihomoURL, MihomoBehavior: "domain"}
	cnSrc := &RuleSetSource{Tag: GeoCNTag, URL: "https://example.com/cn.srs", MihomoURL: GeoCNMihomoURL, MihomoBehavior: "domain"}
	cfg, err := BuildConfig(nil, nil, &Routing{
		Rules: []RuleEntry{
			{MatchType: MatchRuleSet, RuleSet: adSrc, Outbound: OutboundBlock},
			{MatchType: MatchRuleSet, RuleSet: cnSrc, Outbound: "direct"},
			{MatchType: MatchRuleSet, RuleSet: adSrc, Outbound: OutboundBlock},
		},
		Final: "direct",
	}, nil)
	if err != nil {
		t.Fatalf("聚合配置失败: %v", err)
	}
	if cfg.Route == nil {
		t.Fatal("应生成 route 段")
	}
	if len(cfg.Route.Rules) != 3 {
		t.Fatalf("应有 3 条路由规则, got %d", len(cfg.Route.Rules))
	}
	if len(cfg.Route.RuleSet) != 2 {
		t.Fatalf("规则集定义应去重为 2 份, got %d: %+v", len(cfg.Route.RuleSet), cfg.Route.RuleSet)
	}
	if cfg.Route.RuleSet[0].Tag != AntiADTag || cfg.Route.RuleSet[1].Tag != GeoCNTag {
		t.Errorf("规则集顺序应按规则出现顺序: %+v", cfg.Route.RuleSet)
	}
	// 服务端配置的规则集下载不走 direct 指定（download_detour 仅客户端渲染设置）
	if cfg.Route.RuleSet[0].DownloadDetour != "" {
		t.Errorf("服务端规则集不应设置 download_detour: %+v", cfg.Route.RuleSet[0])
	}
}

// TestBuildConfigRuleOrder route.rules 按输入顺序生效。
func TestBuildConfigRuleOrder(t *testing.T) {
	cfg, err := BuildConfig(nil, nil, &Routing{
		Rules: []RuleEntry{
			{MatchType: MatchDomain, MatchMethod: MethodSuffix, Value: "ad.com", Outbound: OutboundBlock},
			{MatchType: MatchDomain, MatchMethod: MethodSuffix, Value: "cn", Outbound: "direct"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("聚合配置失败: %v", err)
	}
	if len(cfg.Route.Rules) != 2 {
		t.Fatalf("应有 2 条规则, got %d", len(cfg.Route.Rules))
	}
	if cfg.Route.Rules[0]["action"] != "reject" {
		t.Errorf("第 1 条应为拦截规则: %v", cfg.Route.Rules[0])
	}
	if !jsonEqual(t, cfg.Route.Rules[1]["domain_suffix"], []string{"cn"}) {
		t.Errorf("第 2 条应为 cn 后缀直连: %v", cfg.Route.Rules[1])
	}
}

// TestBuiltinRuleSets 内置规则集定义与其 mihomo 源一致。
func TestBuiltinRuleSets(t *testing.T) {
	if len(BuiltinRuleSets) == 0 {
		t.Fatal("内置规则集列表不应为空")
	}
	names := map[string]bool{}
	for _, def := range BuiltinRuleSets {
		if def.Name == "" || def.SingBoxURL == "" || def.Description == "" {
			t.Errorf("内置规则集定义不完整: %+v", def)
		}
		if names[def.Name] {
			t.Errorf("内置规则集名称重复: %s", def.Name)
		}
		names[def.Name] = true
		url, behavior, ok := BuiltinMihomoSource(def.Name)
		if !ok || url == "" || behavior == "" {
			t.Errorf("内置规则集 %s 缺少 mihomo 源: url=%q behavior=%q ok=%v", def.Name, url, behavior, ok)
		}
	}
	if _, _, ok := BuiltinMihomoSource("custom"); ok {
		t.Error("非内置规则集不应有 mihomo 默认源")
	}
	if _, _, ok := BuiltinMihomoSource(AntiADTag); !ok || !names[AntiADTag] {
		t.Error("anti-ad 应作为内置规则集存在")
	}
}

// TestSplitRuleValue 规则值拆分（去空白、忽略空项）。
func TestSplitRuleValue(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"a.com,b.com", []string{"a.com", "b.com"}},
		{" a.com , , b.com ", []string{"a.com", "b.com"}},
		{"", nil},
		{" , ", nil},
		{"single", []string{"single"}},
	}
	for _, tt := range tests {
		got := SplitRuleValue(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("SplitRuleValue(%q) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("SplitRuleValue(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
			}
		}
	}
}
