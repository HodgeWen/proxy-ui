package singbox

import (
	"encoding/json"
	"strings"
	"testing"
)

// mustInboundEntry 构造一条可直接聚合的入站条目（默认配置 + 单用户）。
func mustInboundEntry(t *testing.T, tag, protocol string, port int, userName string) InboundEntry {
	t.Helper()
	cfg := templateWithCert(t, protocol)
	cred, err := NewCredential(protocol)
	if err != nil {
		t.Fatalf("生成 %s 凭据失败: %v", protocol, err)
	}
	if protocol == ProtoShadowsocks {
		key, err := NewSSKey("2022-blake3-aes-128-gcm")
		if err != nil {
			t.Fatalf("生成 SS 密钥失败: %v", err)
		}
		cred.Password = key
	}
	return InboundEntry{
		Tag: tag, Protocol: protocol, Listen: ListenLoopback, Port: port,
		Config: configJSON(t, cfg),
		Users:  []InboundUserEntry{{Name: userName, Credential: cred}},
		Cert:   stubResolver,
	}
}

// TestBuildConfigStatsWiring 统计服务范围必须与真正生效的入站/用户一致。
func TestBuildConfigStatsWiring(t *testing.T) {
	cfg, err := BuildConfig(
		[]InboundEntry{
			mustInboundEntry(t, "vless-a", ProtoVLESS, 10001, "alice"),
			func() InboundEntry {
				e := mustInboundEntry(t, "vless-b", ProtoVLESS, 10002, "bob")
				e.Users = append(e.Users, InboundUserEntry{Name: "carol", Credential: Credential{UUID: "0b3f6e8c-0000-4000-8000-000000000000"}})
				return e
			}(),
		},
		nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("聚合配置失败: %v", err)
	}
	if len(cfg.Inbounds) != 2 {
		t.Fatalf("应有 2 条入站, got %d", len(cfg.Inbounds))
	}
	if cfg.Experimental == nil || cfg.Experimental.V2RayAPI == nil || cfg.Experimental.V2RayAPI.Stats == nil {
		t.Fatalf("统计服务配置缺失: %+v", cfg.Experimental)
	}
	stats := cfg.Experimental.V2RayAPI.Stats
	if !stats.Enabled {
		t.Error("统计服务应启用")
	}
	if len(stats.Inbounds) != 2 || stats.Inbounds[0] != "vless-a" || stats.Inbounds[1] != "vless-b" {
		t.Errorf("统计入站列表不符: %v", stats.Inbounds)
	}
	wantUsers := []string{"alice", "bob", "carol"}
	if len(stats.Users) != len(wantUsers) {
		t.Fatalf("统计用户列表 = %v, want %v", stats.Users, wantUsers)
	}
	for i, want := range wantUsers {
		if stats.Users[i] != want {
			t.Errorf("统计用户第 %d 项 = %q, want %q", i, stats.Users[i], want)
		}
	}
	// 统计服务只监听回环
	if cfg.Experimental.V2RayAPI.Listen != StatsListen() {
		t.Errorf("统计服务监听地址 = %q, want %q", cfg.Experimental.V2RayAPI.Listen, StatsListen())
	}
}

// TestBuildConfigNoRouteWithoutRules 无规则且 DNS 关闭时不生成 route 段。
func TestBuildConfigNoRouteWithoutRules(t *testing.T) {
	cfg, err := BuildConfig(nil, nil, nil, &DNSConfig{Enabled: false})
	if err != nil {
		t.Fatalf("聚合配置失败: %v", err)
	}
	if cfg.Route != nil {
		t.Errorf("无规则且 DNS 关闭时不应生成 route 段: %+v", cfg.Route)
	}
	// direct 兜底出站仍在
	if len(cfg.Outbounds) != 1 || cfg.Outbounds[0]["tag"] != "direct" {
		t.Errorf("应仅剩 direct 兜底出站: %v", cfg.Outbounds)
	}

	// 空 Routing（无规则/无组/无 final）同样不生成 route
	cfg, err = BuildConfig(nil, nil, &Routing{}, nil)
	if err != nil {
		t.Fatalf("聚合配置失败: %v", err)
	}
	if cfg.Route != nil {
		t.Errorf("空 Routing 不应生成 route 段: %+v", cfg.Route)
	}
}

// TestBuildConfigDefaultDomainResolver 启用 DNS 时写入 route.default_domain_resolver。
func TestBuildConfigDefaultDomainResolver(t *testing.T) {
	dns := DefaultDNSConfig()
	dns.Final = "cf-doh"
	cfg, err := BuildConfig(nil, nil, nil, dns)
	if err != nil {
		t.Fatalf("聚合配置失败: %v", err)
	}
	if cfg.DNS == nil {
		t.Fatal("启用 DNS 时应生成 dns 段")
	}
	if cfg.Route == nil {
		t.Fatal("启用 DNS 时应生成 route 段（承载 default_domain_resolver）")
	}
	if cfg.Route.DefaultDomainResolver != "cf-doh" {
		t.Errorf("default_domain_resolver = %q, want cf-doh", cfg.Route.DefaultDomainResolver)
	}

	// 没有 final 时取首个服务器 tag
	dns2 := DefaultDNSConfig()
	dns2.Final = ""
	cfg, err = BuildConfig(nil, nil, nil, dns2)
	if err != nil {
		t.Fatalf("聚合配置失败: %v", err)
	}
	if cfg.Route == nil || cfg.Route.DefaultDomainResolver != dns2.Servers[0].Tag {
		t.Errorf("未指定 final 时应用首个服务器 tag: %+v", cfg.Route)
	}

	// DNS 关闭时不写 default_domain_resolver
	cfg, err = BuildConfig(nil, nil, nil, &DNSConfig{Enabled: false, Servers: dns.Servers})
	if err != nil {
		t.Fatalf("聚合配置失败: %v", err)
	}
	if cfg.DNS != nil {
		t.Errorf("DNS 关闭时不应生成 dns 段: %v", cfg.DNS)
	}
	if cfg.Route != nil && cfg.Route.DefaultDomainResolver != "" {
		t.Errorf("DNS 关闭时不应设置 default_domain_resolver: %+v", cfg.Route)
	}
}

// TestBuildConfigGroupValidation 策略组成员必须存在、不得循环引用、不得与出站同名。
func TestBuildConfigGroupValidation(t *testing.T) {
	// 成员不存在
	_, err := BuildConfig(nil, nil, &Routing{
		Groups: []GroupEntry{{Name: "proxy", Mode: GroupModeAuto, Members: []string{"missing"}}},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Errorf("成员不存在应报错: %v", err)
	}

	// 循环引用
	_, err = BuildConfig(nil, nil, &Routing{
		Groups: []GroupEntry{
			{Name: "g1", Mode: GroupModeAuto, Members: []string{"g2"}},
			{Name: "g2", Mode: GroupModeAuto, Members: []string{"g1"}},
		},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "循环") {
		t.Errorf("策略组循环引用应报错: %v", err)
	}

	// 自引用
	_, err = BuildConfig(nil, nil, &Routing{
		Groups: []GroupEntry{{Name: "g1", Mode: GroupModeAuto, Members: []string{"g1"}}},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "自身") {
		t.Errorf("策略组自引用应报错: %v", err)
	}

	// 与出站同名
	_, err = BuildConfig(nil, []OutboundEntry{
		{Tag: "hk", Type: "socks", Config: `{"server":"127.0.0.1","server_port":1080}`},
	}, &Routing{
		Groups: []GroupEntry{{Name: "hk", Mode: GroupModeAuto, Members: []string{"direct"}}},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "同名") {
		t.Errorf("策略组与出站同名应报错: %v", err)
	}

	// 规则出站不存在
	_, err = BuildConfig(nil, nil, &Routing{
		Rules: []RuleEntry{{MatchType: MatchDomain, MatchMethod: MethodSuffix, Value: "a.com", Outbound: "nope"}},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "出站") {
		t.Errorf("规则出站不存在应报错: %v", err)
	}

	// 兜底出站不存在
	_, err = BuildConfig(nil, nil, &Routing{Final: "nope"}, nil)
	if err == nil || !strings.Contains(err.Error(), "兜底") {
		t.Errorf("兜底出站不存在应报错: %v", err)
	}

	// 合法：组成员引用策略组与 direct、规则引用 block
	cfg, err := BuildConfig(nil, []OutboundEntry{
		{Tag: "hk", Type: "socks", Config: `{"server":"127.0.0.1","server_port":1080}`},
	}, &Routing{
		Groups: []GroupEntry{
			{Name: "proxy", Mode: GroupModeAuto, Members: []string{"hk"}},
			{Name: "outer", Mode: GroupModeFixed, Members: []string{"proxy", "direct"}},
		},
		Rules: []RuleEntry{{MatchType: MatchDomain, MatchMethod: MethodSuffix, Value: "ad.com", Outbound: OutboundBlock}},
		Final: "outer",
	}, nil)
	if err != nil {
		t.Fatalf("合法路由不应报错: %v", err)
	}
	if cfg.Route == nil || cfg.Route.Final != "outer" {
		t.Errorf("兜底出站应为 outer: %+v", cfg.Route)
	}
	tags := map[string]bool{}
	for _, ob := range cfg.Outbounds {
		tags[ob["tag"].(string)] = true
	}
	for _, want := range []string{"hk", "proxy", "outer", "direct"} {
		if !tags[want] {
			t.Errorf("出站缺少 %s: %v", want, tags)
		}
	}
}

// TestBuildConfigDirectNotDuplicated 已有 direct 出站时不再补兜底。
func TestBuildConfigDirectNotDuplicated(t *testing.T) {
	cfg, err := BuildConfig(nil, []OutboundEntry{{Tag: "direct", Type: "direct", Config: "{}"}}, nil, nil)
	if err != nil {
		t.Fatalf("聚合配置失败: %v", err)
	}
	if len(cfg.Outbounds) != 1 || cfg.Outbounds[0]["tag"] != "direct" {
		t.Errorf("已有 direct 时不应重复兜底: %v", cfg.Outbounds)
	}
}

// TestBuildConfigInboundError 入站生成失败时整体报错（不产生半成品配置）。
func TestBuildConfigInboundError(t *testing.T) {
	// 无启用用户
	_, err := BuildConfig([]InboundEntry{
		{Tag: "empty", Protocol: ProtoVLESS, Port: 10001, Config: "{}"},
	}, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "用户") {
		t.Errorf("无用户的入站应报错: %v", err)
	}

	// 非法连接层配置
	_, err = BuildConfig([]InboundEntry{
		{Tag: "bad", Protocol: ProtoVLESS, Port: 10002, Config: "{not-json"},
	}, nil, nil, nil)
	if err == nil {
		t.Error("非法入站配置应报错")
	}

	// 非法出站配置
	_, err = BuildConfig(nil, []OutboundEntry{{Tag: "bad", Type: "socks", Config: "{not-json"}}, nil, nil)
	if err == nil {
		t.Error("非法出站配置应报错")
	}
}

// TestBuildConfigJSONRoundTrip 聚合结果必须是可序列化的合法 JSON。
func TestBuildConfigJSONRoundTrip(t *testing.T) {
	cfg, err := BuildConfig(
		[]InboundEntry{mustInboundEntry(t, "hy2-in", ProtoHysteria2, 8443, "alice")},
		nil,
		&Routing{Final: "direct"},
		DefaultDNSConfig(),
	)
	if err != nil {
		t.Fatalf("聚合配置失败: %v", err)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("聚合配置不是合法 JSON: %v", err)
	}
	for _, want := range []string{`"inbounds"`, `"hy2-in"`, `"experimental"`, `"v2ray_api"`, `"dns"`, `"route"`, `"default_domain_resolver"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("聚合配置缺少 %s: %s", want, raw)
		}
	}
}
