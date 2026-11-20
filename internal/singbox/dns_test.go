package singbox

import (
	"encoding/json"
	"testing"
)

// TestDNSValidate DNS 配置校验：启用时各类非法配置必须被拒绝。
func TestDNSValidate(t *testing.T) {
	valid := func() *DNSConfig {
		return &DNSConfig{
			Enabled:  true,
			Strategy: StrategyPreferIPv4,
			Servers: []DNSServer{
				{Tag: "ali", Type: DNSTypeUDP, Server: "223.5.5.5", ServerPort: 53},
				{Tag: "cf-doh", Type: DNSTypeHTTPS, Server: "1.1.1.1", ServerPort: 443, Path: "/dns-query"},
			},
			Rules: []DNSRule{{DomainSuffix: "cn", Server: "ali"}},
			Final: "cf-doh",
		}
	}
	if err := valid().Validate(); err != nil {
		t.Fatalf("合法配置不应报错: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*DNSConfig)
	}{
		{"启用但无服务器", func(d *DNSConfig) { d.Servers = nil }},
		{"服务器 tag 重复", func(d *DNSConfig) { d.Servers[1].Tag = d.Servers[0].Tag }},
		{"服务器 tag 为空", func(d *DNSConfig) { d.Servers[0].Tag = "" }},
		{"服务器类型不支持", func(d *DNSConfig) { d.Servers[0].Type = "quic" }},
		{"服务器缺地址", func(d *DNSConfig) { d.Servers[0].Server = "" }},
		{"默认服务器不存在", func(d *DNSConfig) { d.Final = "nope" }},
		{"规则引用服务器不存在", func(d *DNSConfig) { d.Rules[0].Server = "nope" }},
		{"解析策略不支持", func(d *DNSConfig) { d.Strategy = "prefer_ipv7" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid()
			tt.mutate(cfg)
			if err := cfg.Validate(); err == nil {
				t.Errorf("应校验失败: %+v", cfg)
			}
		})
	}

	// local 类型允许不填地址
	localOnly := &DNSConfig{Enabled: true, Servers: []DNSServer{{Tag: "local", Type: DNSTypeLocal}}, Final: "local"}
	if err := localOnly.Validate(); err != nil {
		t.Errorf("local 类型不应要求地址: %v", err)
	}

	// 未启用 / nil 时不做校验
	if err := (&DNSConfig{Enabled: false}).Validate(); err != nil {
		t.Errorf("未启用时不应校验: %v", err)
	}
	var nilCfg *DNSConfig
	if err := nilCfg.Validate(); err != nil {
		t.Errorf("nil 配置不应报错: %v", err)
	}

	// 校验错误应带字段定位
	d := valid()
	d.Final = "nope"
	var fieldErr *FieldError
	err := d.Validate()
	if err == nil {
		t.Fatal("应校验失败")
	}
	if !asFieldError(err, &fieldErr) || fieldErr.Field == "" {
		t.Errorf("校验错误应带字段定位: %v", err)
	}
}

// asFieldError 判断错误是否为 *FieldError 并取出。
func asFieldError(err error, target **FieldError) bool {
	fe, ok := err.(*FieldError)
	if ok {
		*target = fe
	}
	return ok
}

// TestDNSJSON 渲染后的内核 dns 段形状。
func TestDNSJSON(t *testing.T) {
	cfg := &DNSConfig{
		Enabled:  true,
		Strategy: StrategyPreferIPv6,
		Servers: []DNSServer{
			{Tag: "local", Type: DNSTypeLocal},
			{Tag: "ali", Type: DNSTypeUDP, Server: "223.5.5.5", ServerPort: 53},
			{Tag: "cf-doh", Type: DNSTypeHTTPS, Server: "1.1.1.1", ServerPort: 443, Path: "/dns-query", Detour: "proxy"},
		},
		Rules: []DNSRule{
			{Domain: "a.com,b.com", DomainSuffix: "cn", DomainKeyword: "ads", Server: "ali"},
			{Server: "local"}, // 无任何匹配条件，应被跳过
		},
		Final: "cf-doh",
	}
	dns := cfg.JSON()
	if dns == nil {
		t.Fatal("启用时 JSON 不应为 nil")
	}
	if dns["strategy"] != StrategyPreferIPv6 {
		t.Errorf("strategy = %v, want %s", dns["strategy"], StrategyPreferIPv6)
	}
	if dns["final"] != "cf-doh" {
		t.Errorf("final = %v, want cf-doh", dns["final"])
	}
	servers, ok := dns["servers"].([]map[string]any)
	if !ok || len(servers) != 3 {
		t.Fatalf("servers 应有 3 项: %v", dns["servers"])
	}
	// local 类型只输出 type/tag
	if servers[0]["type"] != DNSTypeLocal || servers[0]["tag"] != "local" {
		t.Errorf("local 服务器字段不符: %v", servers[0])
	}
	if _, ok := servers[0]["server"]; ok {
		t.Errorf("local 服务器不应带 server 字段: %v", servers[0])
	}
	// UDP 带地址与端口
	if servers[1]["server"] != "223.5.5.5" || servers[1]["server_port"] != 53 {
		t.Errorf("UDP 服务器字段不符: %v", servers[1])
	}
	if _, ok := servers[1]["path"]; ok {
		t.Errorf("非 DoH 服务器不应带 path: %v", servers[1])
	}
	// DoH 带 path 与 detour
	if servers[2]["path"] != "/dns-query" || servers[2]["detour"] != "proxy" {
		t.Errorf("DoH 服务器字段不符: %v", servers[2])
	}

	rules, ok := dns["rules"].([]map[string]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("rules 应有 1 条（空条件跳过）: %v", dns["rules"])
	}
	rule := rules[0]
	if rule["action"] != "route" || rule["server"] != "ali" {
		t.Errorf("DNS 规则动作不符: %v", rule)
	}
	if a, _ := rule["domain"].([]string); len(a) != 2 || a[0] != "a.com" {
		t.Errorf("DNS 规则 domain 不符: %v", rule["domain"])
	}
	if a, _ := rule["domain_suffix"].([]string); len(a) != 1 || a[0] != "cn" {
		t.Errorf("DNS 规则 domain_suffix 不符: %v", rule["domain_suffix"])
	}
	if a, _ := rule["domain_keyword"].([]string); len(a) != 1 || a[0] != "ads" {
		t.Errorf("DNS 规则 domain_keyword 不符: %v", rule["domain_keyword"])
	}

	// 未启用 / nil / 无服务器 → nil
	for _, tc := range []*DNSConfig{
		nil,
		{Enabled: false, Servers: cfg.Servers},
		{Enabled: true},
	} {
		if got := tc.JSON(); got != nil {
			t.Errorf("未启用或无服务器时 JSON 应为 nil: %+v → %v", tc, got)
		}
	}
}

// TestDNSResolverTag 默认解析器 tag 取值规则。
func TestDNSResolverTag(t *testing.T) {
	if got := (*DNSConfig)(nil).ResolverTag(); got != "" {
		t.Errorf("nil 配置的 ResolverTag 应为空, got %q", got)
	}
	if got := (&DNSConfig{Enabled: false, Servers: []DNSServer{{Tag: "ali"}}, Final: "ali"}).ResolverTag(); got != "" {
		t.Errorf("未启用时 ResolverTag 应为空, got %q", got)
	}
	if got := (&DNSConfig{Enabled: true}).ResolverTag(); got != "" {
		t.Errorf("无服务器时 ResolverTag 应为空, got %q", got)
	}
	// 未指定 final 取首个服务器
	cfg := &DNSConfig{Enabled: true, Servers: []DNSServer{{Tag: "first"}, {Tag: "second"}}}
	if got := cfg.ResolverTag(); got != "first" {
		t.Errorf("未指定 final 时应取首个服务器, got %q", got)
	}
	// 指定 final 优先
	cfg.Final = "second"
	if got := cfg.ResolverTag(); got != "second" {
		t.Errorf("final 应优先, got %q", got)
	}
}

// TestDefaultDNSConfig 默认 DNS 配置可用且与预设一致。
func TestDefaultDNSConfig(t *testing.T) {
	cfg := DefaultDNSConfig()
	if cfg == nil || !cfg.Enabled {
		t.Fatal("默认 DNS 配置应启用")
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("默认 DNS 配置应通过校验: %v", err)
	}
	if cfg.ResolverTag() == "" {
		t.Error("默认 DNS 配置应有解析器 tag")
	}
	if cfg.JSON() == nil {
		t.Error("默认 DNS 配置应能渲染 dns 段")
	}
}

// TestDNSPresets 全部预设都应通过校验，ID 不重复。
func TestDNSPresets(t *testing.T) {
	if len(DNSPresets) == 0 {
		t.Fatal("DNS 预设列表不应为空")
	}
	ids := map[string]bool{}
	for _, p := range DNSPresets {
		if p.ID == "" || p.Name == "" || p.Description == "" {
			t.Errorf("预设定义不完整: %+v", p)
		}
		if ids[p.ID] {
			t.Errorf("预设 ID 重复: %s", p.ID)
		}
		ids[p.ID] = true
		cfg := p.Config
		cfg.Enabled = true
		if err := cfg.Validate(); err != nil {
			t.Errorf("预设 %s 校验失败: %v", p.ID, err)
		}
		if len(cfg.Servers) == 0 {
			t.Errorf("预设 %s 缺少 DNS 服务器", p.ID)
		}
	}
	for _, want := range []string{DNSPresetDirect, DNSPresetAntiPoll, DNSPresetRemoteDoH} {
		if !ids[want] {
			t.Errorf("缺少内置预设 %s", want)
		}
	}
}

// TestDNSJSONMarshal 渲染结果可序列化（含中文值不破坏 JSON）。
func TestDNSJSONMarshal(t *testing.T) {
	raw, err := json.Marshal(DefaultDNSConfig())
	if err != nil {
		t.Fatalf("序列化 DNS 配置失败: %v", err)
	}
	var back DNSConfig
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("反序列化 DNS 配置失败: %v", err)
	}
	if len(back.Servers) != len(DefaultDNSConfig().Servers) {
		t.Errorf("DNS 服务器数量往返不一致: %d", len(back.Servers))
	}
}
