package subscription

import (
	"encoding/json"
	"strings"
	"testing"

	"proxy-ui/internal/singbox"
)

// nodeSpec 以协议默认配置 + 新生成凭据构造一条订阅节点输入。
func nodeSpec(t *testing.T, tag, protocol string, port int) NodeSpec {
	t.Helper()
	cfg, err := singbox.NewTemplateConfig(protocol)
	if err != nil {
		t.Fatalf("生成 %s 默认配置失败: %v", protocol, err)
	}
	// 面板存储证书在订阅侧只需渲染证书引用，这里补一个 ID 保持配置完整
	if tls := singbox.InboundTLS(cfg); tls != nil && tls.CertMode == singbox.CertModeManaged {
		tls.CertID = 1
	}
	cred, err := singbox.NewCredential(protocol)
	if err != nil {
		t.Fatalf("生成 %s 凭据失败: %v", protocol, err)
	}
	rawCfg, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("序列化配置失败: %v", err)
	}
	rawCred, err := json.Marshal(cred)
	if err != nil {
		t.Fatalf("序列化凭据失败: %v", err)
	}
	return NodeSpec{Tag: tag, Protocol: protocol, Port: port, Config: string(rawCfg), CredJSON: string(rawCred)}
}

// ssSpec 构造 Shadowsocks-2022 节点（入站 PSK + 用户密钥）。
func ssSpec(t *testing.T, tag string, port int) (NodeSpec, string, string) {
	t.Helper()
	const method = "2022-blake3-aes-128-gcm"
	psk, err := singbox.NewSSKey(method)
	if err != nil {
		t.Fatalf("生成 PSK 失败: %v", err)
	}
	key, err := singbox.NewSSKey(method)
	if err != nil {
		t.Fatalf("生成用户密钥失败: %v", err)
	}
	rawCfg, err := json.Marshal(&singbox.ShadowsocksConfig{Method: method, PSK: psk})
	if err != nil {
		t.Fatalf("序列化 SS 配置失败: %v", err)
	}
	rawCred, err := json.Marshal(singbox.Credential{Password: key})
	if err != nil {
		t.Fatalf("序列化 SS 凭据失败: %v", err)
	}
	return NodeSpec{Tag: tag, Protocol: singbox.ProtoShadowsocks, Port: port, Config: string(rawCfg), CredJSON: string(rawCred)}, psk, key
}

// findOutbound 按 tag 取出站对象。
func findOutbound(t *testing.T, cfg singbox.Config, tag string) map[string]any {
	t.Helper()
	for _, ob := range cfg.Outbounds {
		if ob["tag"] == tag {
			return ob
		}
	}
	t.Fatalf("出站 %s 不存在: %v", tag, cfg.Outbounds)
	return nil
}

// TestDetectFormat 输出格式判定：查询参数优先于 UA，未知一律 sing-box。
func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name  string
		param string
		ua    string
		want  Format
	}{
		{"参数 clash", "clash", "", FormatClash},
		{"参数 mihomo", "mihomo", "", FormatClash},
		{"参数 clash.meta", "clash.meta", "", FormatClash},
		{"参数 clashmeta", "clashmeta", "", FormatClash},
		{"参数大小写与空白", " CLASH ", "", FormatClash},
		{"参数 singbox", "singbox", "clash", FormatSingBox},
		{"参数 sing-box", "sing-box", "Mihomo", FormatSingBox},
		{"参数 sing", "sing", "", FormatSingBox},
		{"UA clash", "", "clash-verge/v1.5.0", FormatClash},
		{"UA mihomo", "", "mihomo/1.19.0", FormatClash},
		{"UA stash", "", "Stash/2.4.0", FormatClash},
		{"UA SFM", "", "SFM/1.11.0 sing-box", FormatSingBox},
		{"UA sing-box", "", "SFA/1.11.0 (sing-box 1.11.0)", FormatSingBox},
		{"UA 无关", "", "curl/8.4.0", FormatSingBox},
		{"参数与 UA 均空", "", "", FormatSingBox},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectFormat(tt.param, tt.ua); got != tt.want {
				t.Errorf("DetectFormat(%q, %q) = %q, want %q", tt.param, tt.ua, got, tt.want)
			}
		})
	}
}

// TestAssembleNodeTags 节点按输入顺序保留自身 tag 与凭据。
func TestAssembleNodeTags(t *testing.T) {
	specs := []NodeSpec{
		nodeSpec(t, "alice-vless", singbox.ProtoVLESS, 20001),
		nodeSpec(t, "alice-hy2", singbox.ProtoHysteria2, 20002),
	}
	d, err := Assemble("example.com", specs, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	if d.Server != "example.com" {
		t.Errorf("服务器地址 = %q, want example.com", d.Server)
	}
	if len(d.Nodes) != 2 {
		t.Fatalf("应有 2 个节点, got %d", len(d.Nodes))
	}
	if d.Nodes[0].Tag != "alice-vless" || d.Nodes[0].Port != 20001 {
		t.Errorf("节点 0 字段不符: %+v", d.Nodes[0])
	}
	if d.Nodes[1].Tag != "alice-hy2" || d.Nodes[1].Protocol != singbox.ProtoHysteria2 {
		t.Errorf("节点 1 字段不符: %+v", d.Nodes[1])
	}
	// 凭据已解析且各节点独立
	if d.Nodes[0].Cred.UUID == "" {
		t.Errorf("VLESS 节点应有 UUID: %+v", d.Nodes[0].Cred)
	}
	if d.Nodes[1].Cred.Password == "" {
		t.Errorf("Hysteria2 节点应有密码: %+v", d.Nodes[1].Cred)
	}
	// 无策略组时合成 proxy 兜底组
	if len(d.Groups) != 1 || d.Groups[0].Name != FallbackGroup {
		t.Fatalf("应合成 %s 兜底组: %+v", FallbackGroup, d.Groups)
	}
	if d.Groups[0].Mode != singbox.GroupModeFixed {
		t.Errorf("兜底组模式应为 fixed: %+v", d.Groups[0])
	}
	if len(d.Groups[0].Members) != 2 {
		t.Errorf("兜底组成员应为全部节点: %+v", d.Groups[0].Members)
	}
	if d.Final != FallbackGroup {
		t.Errorf("无兜底出站时应回落到 %s, got %q", FallbackGroup, d.Final)
	}
}

// TestAssembleGroupMemberMapping 策略组成员过滤为可用 tag，全无效时回落全部节点。
func TestAssembleGroupMemberMapping(t *testing.T) {
	specs := []NodeSpec{
		nodeSpec(t, "n1", singbox.ProtoVLESS, 20001),
		nodeSpec(t, "n2", singbox.ProtoTrojan, 20002),
	}
	groups := []singbox.GroupEntry{
		{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"n1", "ghost"}},
		{Name: "broken", Mode: singbox.GroupModeAuto, Members: []string{"ghost-a", "ghost-b"}},
	}
	d, err := Assemble("example.com", specs, groups, nil, "", nil)
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	if len(d.Groups) != 2 {
		t.Fatalf("应保留 2 个策略组, got %d", len(d.Groups))
	}
	// 有效成员保留，不存在的被剔除
	if len(d.Groups[0].Members) != 1 || d.Groups[0].Members[0] != "n1" {
		t.Errorf("策略组成员应剔除无效 tag: %+v", d.Groups[0])
	}
	// 全无效成员回落为全部节点 tag
	if len(d.Groups[1].Members) != 2 || d.Groups[1].Members[0] != "n1" || d.Groups[1].Members[1] != "n2" {
		t.Errorf("无效成员组应回落全部节点: %+v", d.Groups[1])
	}
	// 无兜底设置时取首个策略组
	if d.Final != "proxy" {
		t.Errorf("Final 应取首个策略组, got %q", d.Final)
	}

	// 策略组可引用 direct 与其它策略组（不得引用自身）
	d, err = Assemble("example.com", specs, []singbox.GroupEntry{
		{Name: "inner", Mode: singbox.GroupModeFixed, Members: []string{"n1"}},
		{Name: "outer", Mode: singbox.GroupModeFixed, Members: []string{"inner", "direct", "outer"}},
	}, nil, "", nil)
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	if len(d.Groups[1].Members) != 2 || d.Groups[1].Members[0] != "inner" || d.Groups[1].Members[1] != "direct" {
		t.Errorf("策略组引用规则不符（应保留组名与 direct、剔除自引用）: %+v", d.Groups[1])
	}
}

// TestAssembleFinalMapping 兜底出站映射：未知 tag 回落到默认组。
func TestAssembleFinalMapping(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "n1", singbox.ProtoVLESS, 20001)}
	groups := []singbox.GroupEntry{{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"n1"}}}

	tests := []struct {
		name  string
		final string
		want  string
	}{
		{"空取首个组", "", "proxy"},
		{"组名原样", "proxy", "proxy"},
		{"direct 原样", "direct", "direct"},
		{"未知 tag 回落组", "hk-unknown", "proxy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Assemble("example.com", specs, groups, nil, tt.final, nil)
			if err != nil {
				t.Fatalf("组装订阅失败: %v", err)
			}
			if d.Final != tt.want {
				t.Errorf("Final = %q, want %q", d.Final, tt.want)
			}
		})
	}

	// 规则出站同样映射：block/direct 原样，未知回落到兜底组
	rules := []singbox.RuleEntry{
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodSuffix, Value: "ad.com", Outbound: singbox.OutboundBlock},
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodSuffix, Value: "cn", Outbound: "direct"},
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodSuffix, Value: "x.com", Outbound: "unknown-ob"},
	}
	d, err := Assemble("example.com", specs, groups, rules, "direct", nil)
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	if len(d.Rules) != 3 {
		t.Fatalf("应有 3 条规则, got %d", len(d.Rules))
	}
	if d.Rules[0].Outbound != singbox.OutboundBlock {
		t.Errorf("拦截规则出站不应被改写: %q", d.Rules[0].Outbound)
	}
	if d.Rules[1].Outbound != "direct" {
		t.Errorf("direct 出站不应被改写: %q", d.Rules[1].Outbound)
	}
	if d.Rules[2].Outbound != "proxy" {
		t.Errorf("未知出站应回落到兜底组, got %q", d.Rules[2].Outbound)
	}
}

// TestAssembleNoNodes 无节点时兜底为 direct，不合成空组，且已丢弃策略组不会残留为规则/Final 目标。
func TestAssembleNoNodes(t *testing.T) {
	d, err := Assemble("example.com", nil, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	if len(d.Nodes) != 0 {
		t.Errorf("应无节点: %+v", d.Nodes)
	}
	if len(d.Groups) != 0 {
		t.Errorf("无节点时不应合成策略组: %+v", d.Groups)
	}
	if d.Final != "direct" {
		t.Errorf("无节点时兜底应为 direct, got %q", d.Final)
	}

	// 存在依赖节点/级联依赖的自定义策略组与规则时：空组整链丢弃，规则与 Final 回退到 direct
	groups := []singbox.GroupEntry{
		{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"n1"}},
		{Name: "outer", Mode: singbox.GroupModeFixed, Members: []string{"proxy"}},
	}
	rules := []singbox.RuleEntry{
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodSuffix, Value: "example.com", Outbound: "proxy"},
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodSuffix, Value: "sub.example.com", Outbound: "outer"},
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodSuffix, Value: "cn", Outbound: "direct"},
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodSuffix, Value: "ad.com", Outbound: singbox.OutboundBlock},
	}
	d, err = Assemble("example.com", nil, groups, rules, "outer", nil)
	if err != nil {
		t.Fatalf("无节点含自定义组/规则组装失败: %v", err)
	}
	if len(d.Groups) != 0 {
		t.Errorf("空策略组及其上层依赖组应全部丢弃: %+v", d.Groups)
	}
	if d.Final != "direct" {
		t.Errorf("无节点时 Final 应回退到 direct, got %q", d.Final)
	}
	if d.Rules[0].Outbound != "direct" || d.Rules[1].Outbound != "direct" {
		t.Errorf("指向已丢弃策略组的规则应回退到 direct: %+v", d.Rules)
	}
	if d.Rules[2].Outbound != "direct" || d.Rules[3].Outbound != singbox.OutboundBlock {
		t.Errorf("direct 与 block 规则不应被改写: %+v", d.Rules)
	}

	// 部分组含 direct 保留、空组丢弃时：规则与 Final 回退到保留组
	mixedGroups := []singbox.GroupEntry{
		{Name: "kept", Mode: singbox.GroupModeFixed, Members: []string{"direct"}},
		{Name: "dropped", Mode: singbox.GroupModeAuto, Members: []string{"n1"}},
	}
	d, err = Assemble("example.com", nil, mixedGroups, rules[:1], "dropped", nil)
	if err != nil {
		t.Fatalf("无节点含部分有效组组装失败: %v", err)
	}
	if len(d.Groups) != 1 || d.Groups[0].Name != "kept" {
		t.Errorf("应只保留含 direct 的策略组: %+v", d.Groups)
	}
	if d.Final != "kept" || d.Rules[0].Outbound != "kept" {
		t.Errorf("指向已丢弃组的规则与 Final 应回退到保留组 kept: final=%q rules=%+v", d.Final, d.Rules)
	}
}

// TestAssembleErrors 节点配置/凭据非法时报错。
func TestAssembleErrors(t *testing.T) {
	if _, err := Assemble("example.com", []NodeSpec{
		{Tag: "bad", Protocol: singbox.ProtoVLESS, Port: 1, Config: "{not-json"},
	}, nil, nil, "", nil); err == nil {
		t.Error("非法连接层配置应报错")
	}
	if _, err := Assemble("example.com", []NodeSpec{
		{Tag: "bad", Protocol: singbox.ProtoVLESS, Port: 1, Config: "{}", CredJSON: "{not-json"},
	}, nil, nil, "", nil); err == nil {
		t.Error("非法凭据 JSON 应报错")
	}
}

// TestRenderSingBoxNodes 渲染 sing-box 客户端配置：节点出站各自独立凭据。
func TestRenderSingBoxNodes(t *testing.T) {
	specs := []NodeSpec{
		nodeSpec(t, "n-vless", singbox.ProtoVLESS, 20001),
		nodeSpec(t, "n-hy2", singbox.ProtoHysteria2, 20002),
		nodeSpec(t, "n-tuic", singbox.ProtoTUIC, 20003),
		nodeSpec(t, "n-anytls", singbox.ProtoAnyTLS, 20004),
		nodeSpec(t, "n-trojan", singbox.ProtoTrojan, 20005),
		nodeSpec(t, "n-vmess", singbox.ProtoVMess, 20006),
	}
	d, err := Assemble("example.com", specs, nil, nil, "", singbox.DefaultDNSConfig())
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	cfg, err := RenderSingBox(d)
	if err != nil {
		t.Fatalf("渲染 sing-box 配置失败: %v", err)
	}

	// 每个节点一个出站，服务器/端口/凭据按协议就位
	for _, spec := range specs {
		ob := findOutbound(t, cfg, spec.Tag)
		if ob["server"] != "example.com" {
			t.Errorf("节点 %s server = %v, want example.com", spec.Tag, ob["server"])
		}
		if ob["server_port"] != spec.Port {
			t.Errorf("节点 %s server_port = %v, want %d", spec.Tag, ob["server_port"], spec.Port)
		}
		if ob["type"] != spec.Protocol {
			t.Errorf("节点 %s type = %v, want %s", spec.Tag, ob["type"], spec.Protocol)
		}
	}

	// VLESS + Reality：TLS/utls/reality 段齐全
	vless := findOutbound(t, cfg, "n-vless")
	if vless["uuid"] == "" || vless["flow"] != "xtls-rprx-vision" {
		t.Errorf("VLESS 节点凭据/流控不符: %v", vless)
	}
	tls, _ := vless["tls"].(map[string]any)
	if tls == nil || tls["enabled"] != true {
		t.Fatalf("VLESS 节点应带 TLS 段: %v", vless)
	}
	if tls["server_name"] != "www.microsoft.com" {
		t.Errorf("Reality 节点 SNI 应取握手站点: %v", tls)
	}
	utls, _ := tls["utls"].(map[string]any)
	if utls == nil || utls["enabled"] != true || utls["fingerprint"] != "chrome" {
		t.Errorf("Reality 节点应带 utls: %v", tls)
	}
	reality, _ := tls["reality"].(map[string]any)
	if reality == nil || reality["enabled"] != true || reality["public_key"] == "" {
		t.Errorf("Reality 段不符: %v", tls)
	}

	// Hysteria2：密码、混淆、ALPN h3
	hy2 := findOutbound(t, cfg, "n-hy2")
	if hy2["password"] == "" {
		t.Errorf("Hysteria2 节点应有密码: %v", hy2)
	}
	obfs, _ := hy2["obfs"].(map[string]any)
	if obfs == nil || obfs["type"] != "salamander" || obfs["password"] == "" {
		t.Errorf("Hysteria2 节点应带混淆: %v", hy2)
	}
	hy2TLS, _ := hy2["tls"].(map[string]any)
	if alpn, _ := hy2TLS["alpn"].([]string); len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("Hysteria2 节点 ALPN 应含 h3: %v", hy2TLS)
	}

	// TUIC：UUID + 密码 + 拥塞控制 + ALPN h3
	tuic := findOutbound(t, cfg, "n-tuic")
	if tuic["uuid"] == "" || tuic["password"] == "" {
		t.Errorf("TUIC 节点应同时带 UUID 与密码: %v", tuic)
	}
	if tuic["congestion_control"] != singbox.CongestionBBR {
		t.Errorf("TUIC 节点拥塞控制不符: %v", tuic)
	}
	tuicTLS, _ := tuic["tls"].(map[string]any)
	if alpn, _ := tuicTLS["alpn"].([]string); len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("TUIC 节点 ALPN 应含 h3: %v", tuicTLS)
	}

	// AnyTLS / Trojan：密码 + TLS
	for _, tag := range []string{"n-anytls", "n-trojan"} {
		ob := findOutbound(t, cfg, tag)
		if ob["password"] == "" {
			t.Errorf("%s 节点应有密码: %v", tag, ob)
		}
		if _, ok := ob["tls"].(map[string]any); !ok {
			t.Errorf("%s 节点应带 TLS 段: %v", tag, ob)
		}
	}

	// VMess：UUID + WS 传输 + TLS
	vmess := findOutbound(t, cfg, "n-vmess")
	if vmess["uuid"] == "" {
		t.Errorf("VMess 节点应有 UUID: %v", vmess)
	}
	transport, _ := vmess["transport"].(map[string]any)
	if transport["type"] != "ws" || transport["path"] != "/" {
		t.Errorf("VMess 节点传输段不符: %v", vmess)
	}

	// 每个节点的凭据互不相同
	uuids := map[string]bool{}
	for _, ob := range cfg.Outbounds {
		if u, ok := ob["uuid"].(string); ok {
			if uuids[u] {
				t.Errorf("不同节点不应共用 UUID: %s", u)
			}
			uuids[u] = true
		}
	}

	// direct 兜底出站与兜底组
	findOutbound(t, cfg, "direct")
	proxy := findOutbound(t, cfg, FallbackGroup)
	if proxy["type"] != "selector" {
		t.Errorf("兜底组应为 selector: %v", proxy)
	}
}

// TestRenderSingBoxRoute 路由段：规则、规则集（走 direct 下载）、默认解析器。
func TestRenderSingBoxRoute(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "n1", singbox.ProtoVLESS, 20001)}
	adSrc := &singbox.RuleSetSource{
		Tag: singbox.AntiADTag, URL: singbox.AntiADBuiltinURL,
		MihomoURL: singbox.AntiADMihomoURL, MihomoBehavior: "domain",
	}
	cnSrc := &singbox.RuleSetSource{
		Tag: singbox.GeoCNTag, URL: "https://example.com/cn.srs",
		MihomoURL: singbox.GeoCNMihomoURL, MihomoBehavior: "domain",
	}
	groups := []singbox.GroupEntry{{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"n1"}}}
	rules := []singbox.RuleEntry{
		{MatchType: singbox.MatchRuleSet, RuleSet: adSrc, Outbound: singbox.OutboundBlock},
		{MatchType: singbox.MatchRuleSet, RuleSet: cnSrc, Outbound: "direct"},
		{MatchType: singbox.MatchRuleSet, RuleSet: adSrc, Outbound: singbox.OutboundBlock}, // 重复引用
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodSuffix, Value: "cn", Outbound: "direct"},
		{MatchType: singbox.MatchIP, MatchMethod: singbox.MethodCIDR, Value: "10.0.0.0/8", Outbound: "direct"},
	}
	dns := singbox.DefaultDNSConfig()
	d, err := Assemble("example.com", specs, groups, rules, "proxy", dns)
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	cfg, err := RenderSingBox(d)
	if err != nil {
		t.Fatalf("渲染 sing-box 配置失败: %v", err)
	}
	if cfg.Route == nil {
		t.Fatal("应生成 route 段")
	}
	if len(cfg.Inbounds) != 1 {
		t.Fatalf("应包含 1 个客户端 tun 入站, got %d", len(cfg.Inbounds))
	}
	if inb, _ := cfg.Inbounds[0].(map[string]any); inb["type"] != "tun" || inb["tag"] != "tun-in" {
		t.Errorf("客户端入站应为 tun-in: %v", cfg.Inbounds[0])
	}
	if !cfg.Route.AutoDetectInterface {
		t.Error("应启用 route.auto_detect_interface")
	}
	if cfg.Route.Final != "proxy" {
		t.Errorf("route.final = %q, want proxy", cfg.Route.Final)
	}
	// 前置 3 条客户端基础规则（sniff / hijack-dns / ip_is_private）+ 5 条用户规则
	if len(cfg.Route.Rules) != 8 {
		t.Fatalf("应有 8 条规则, got %d: %+v", len(cfg.Route.Rules), cfg.Route.Rules)
	}
	if cfg.Route.Rules[0]["action"] != "sniff" {
		t.Errorf("第 0 条应为 sniff 规则: %v", cfg.Route.Rules[0])
	}
	if cfg.Route.Rules[1]["protocol"] != "dns" || cfg.Route.Rules[1]["action"] != "hijack-dns" {
		t.Errorf("第 1 条应为 hijack-dns 规则: %v", cfg.Route.Rules[1])
	}
	if cfg.Route.Rules[2]["ip_is_private"] != true || cfg.Route.Rules[2]["outbound"] != "direct" {
		t.Errorf("第 2 条应为私有 IP 直连规则: %v", cfg.Route.Rules[2])
	}
	if cfg.Route.Rules[3]["action"] != "reject" {
		t.Errorf("拦截规则应映射为 reject: %v", cfg.Route.Rules[3])
	}
	// 规则集去重且下载优先走代理组
	if len(cfg.Route.RuleSet) != 2 {
		t.Fatalf("规则集定义应去重为 2 份: %+v", cfg.Route.RuleSet)
	}
	for _, rs := range cfg.Route.RuleSet {
		if rs.DownloadDetour != "proxy" {
			t.Errorf("规则集 %s 的 download_detour = %q, want proxy", rs.Tag, rs.DownloadDetour)
		}
		if rs.Format != "binary" {
			t.Errorf("规则集 %s 格式 = %q, want binary", rs.Tag, rs.Format)
		}
		if rs.Type != "remote" {
			t.Errorf("规则集 %s 类型 = %q, want remote", rs.Tag, rs.Type)
		}
	}
	// DNS 段、远程 DoH detour 与直连默认域名解析器
	if cfg.DNS == nil {
		t.Fatal("启用 DNS 时应渲染 dns 段")
	}
	dnsServers, _ := cfg.DNS["servers"].([]map[string]any)
	var cfDetour any
	for _, srv := range dnsServers {
		if srv["tag"] == "cf-doh" {
			cfDetour = srv["detour"]
		}
	}
	if cfDetour != "proxy" {
		t.Errorf("远程 DoH cf-doh 应设置 detour=proxy, got %v", cfDetour)
	}
	if cfg.Route.DefaultDomainResolver != "ali" {
		t.Errorf("default_domain_resolver 应取直连明文 DNS ali, got %q", cfg.Route.DefaultDomainResolver)
	}
}

// TestRenderSingBoxNoDNSAndNoRules DNS 关闭且无规则时不渲染 dns/route。
func TestRenderSingBoxNoDNSAndNoRules(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "n1", singbox.ProtoVLESS, 20001)}
	d, err := Assemble("example.com", specs, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	cfg, err := RenderSingBox(d)
	if err != nil {
		t.Fatalf("渲染 sing-box 配置失败: %v", err)
	}
	if cfg.DNS != nil {
		t.Errorf("DNS 关闭时不应渲染 dns 段: %v", cfg.DNS)
	}
	if cfg.Route == nil {
		t.Fatal("仍需 route 段承载 final")
	}
	if cfg.Route.DefaultDomainResolver != "" {
		t.Errorf("DNS 关闭时不应设置 default_domain_resolver: %+v", cfg.Route)
	}
	if cfg.Route.Final != FallbackGroup {
		t.Errorf("route.final = %q, want %q", cfg.Route.Final, FallbackGroup)
	}
}

// TestRenderSingBoxShadowsocks2022 Shadowsocks-2022 节点密码必须是「入站PSK:用户密钥」。
func TestRenderSingBoxShadowsocks2022(t *testing.T) {
	spec, psk, key := ssSpec(t, "n-ss", 20007)
	d, err := Assemble("example.com", []NodeSpec{spec}, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	cfg, err := RenderSingBox(d)
	if err != nil {
		t.Fatalf("渲染 sing-box 配置失败: %v", err)
	}
	ob := findOutbound(t, cfg, "n-ss")
	want := psk + ":" + key
	if ob["password"] != want {
		t.Fatalf("SS-2022 节点密码应为「入站PSK:用户密钥」\n got = %v\nwant = %q", ob["password"], want)
	}
	if !strings.Contains(ob["password"].(string), ":") {
		t.Errorf("SS-2022 节点密码应含冒号分隔符: %v", ob["password"])
	}
	if ob["method"] != "2022-blake3-aes-128-gcm" {
		t.Errorf("SS-2022 节点加密方法不符: %v", ob["method"])
	}
	if _, ok := ob["tls"]; ok {
		t.Errorf("Shadowsocks 节点不应带 TLS 段: %v", ob)
	}
}

// TestRenderDispatch 按格式分派渲染。
func TestRenderDispatch(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "n1", singbox.ProtoVLESS, 20001)}
	d, err := Assemble("example.com", specs, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	raw, err := Render(d, FormatSingBox)
	if err != nil {
		t.Fatalf("渲染 sing-box 失败: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("sing-box 渲染结果应为合法 JSON: %v", err)
	}
	if _, ok := decoded["outbounds"]; !ok {
		t.Errorf("sing-box 渲染结果缺少 outbounds: %s", raw)
	}

	raw, err = Render(d, FormatClash)
	if err != nil {
		t.Fatalf("渲染 clash 失败: %v", err)
	}
	if !strings.Contains(string(raw), "proxies:") {
		t.Errorf("clash 渲染结果缺少 proxies: %s", raw)
	}
}

// TestRenderSingBoxCheckConfig 渲染出的 sing-box 客户端配置应能通过真实 sing-box 内核校验。
func TestRenderSingBoxCheckConfig(t *testing.T) {
	specs := []NodeSpec{
		nodeSpec(t, "n-vless", singbox.ProtoVLESS, 20001),
		nodeSpec(t, "n-hy2", singbox.ProtoHysteria2, 20002),
		nodeSpec(t, "n-tuic", singbox.ProtoTUIC, 20003),
		nodeSpec(t, "n-anytls", singbox.ProtoAnyTLS, 20004),
		nodeSpec(t, "n-trojan", singbox.ProtoTrojan, 20005),
		nodeSpec(t, "n-vmess", singbox.ProtoVMess, 20006),
	}
	ssSpecItem, _, _ := ssSpec(t, "n-ss", 20007)
	specs = append(specs, ssSpecItem)

	adSrc := &singbox.RuleSetSource{
		Tag: singbox.AntiADTag, URL: singbox.AntiADBuiltinURL,
		MihomoURL: singbox.AntiADMihomoURL, MihomoBehavior: "domain",
	}
	groups := []singbox.GroupEntry{{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"direct"}}}
	rules := []singbox.RuleEntry{
		{MatchType: singbox.MatchRuleSet, RuleSet: adSrc, Outbound: singbox.OutboundBlock},
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodSuffix, Value: "cn", Outbound: "direct"},
	}
	d, err := Assemble("example.com", specs, groups, rules, "proxy", singbox.DefaultDNSConfig())
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	cfg, err := RenderSingBox(d)
	if err != nil {
		t.Fatalf("渲染 sing-box 失败: %v", err)
	}
	if err := singbox.New(t.TempDir()).CheckConfig(cfg); err != nil {
		t.Fatalf("客户端 sing-box 配置应通过内核 CheckConfig 校验: %v", err)
	}

	// 纯 DoH（remote_doh 预设）场景：自动补 dns-local 并设为 default_domain_resolver，且通过内核校验
	remoteDoH := singbox.DNSPresets[2].Config
	remoteDoH.Enabled = true
	d2, err := Assemble("example.com", specs[:2], groups, rules, "proxy", &remoteDoH)
	if err != nil {
		t.Fatalf("remote_doh 组装失败: %v", err)
	}
	cfg2, err := RenderSingBox(d2)
	if err != nil {
		t.Fatalf("remote_doh 渲染失败: %v", err)
	}
	if cfg2.Route.DefaultDomainResolver != "dns-local" {
		t.Errorf("纯 DoH 预设应合成 dns-local 作为 default_domain_resolver, got %q", cfg2.Route.DefaultDomainResolver)
	}
	if err := singbox.New(t.TempDir()).CheckConfig(cfg2); err != nil {
		t.Fatalf("remote_doh 客户端 sing-box 配置应通过内核 CheckConfig 校验: %v", err)
	}

	// 域名版 UDP 服务器不可自选为 directTag（防止 domain_resolver 自引用死循环）
	domainUDP := singbox.DNSConfig{
		Enabled: true,
		Servers: []singbox.DNSServer{{Tag: "ali-domain", Type: singbox.DNSTypeUDP, Server: "dns.alidns.com"}},
		Final:   "ali-domain",
	}
	d3, err := Assemble("example.com", specs[:2], groups, rules, "proxy", &domainUDP)
	if err != nil {
		t.Fatalf("domainUDP 组装失败: %v", err)
	}
	cfg3, err := RenderSingBox(d3)
	if err != nil {
		t.Fatalf("domainUDP 渲染失败: %v", err)
	}
	if cfg3.Route.DefaultDomainResolver != "dns-local" {
		t.Errorf("域名版 UDP 不应自选为 default_domain_resolver, got %q", cfg3.Route.DefaultDomainResolver)
	}
	if err := singbox.New(t.TempDir()).CheckConfig(cfg3); err != nil {
		t.Fatalf("domainUDP 客户端 sing-box 配置应通过内核 CheckConfig 校验: %v", err)
	}
}

// TestRenderSingBoxInsecureAndBBR 自签证书输出 insecure:true，Hysteria2 默认 standard 省略 bbr_profile。
func TestRenderSingBoxInsecureAndBBR(t *testing.T) {
	hy2Default := nodeSpec(t, "hy2-std", singbox.ProtoHysteria2, 20001)
	hy2Default.Insecure = true

	aggRaw, _ := json.Marshal(&singbox.Hysteria2Config{
		BBRProfile: singbox.BBRProfileAggressive,
		TLS:        singbox.TLSConfig{Enabled: true, CertMode: singbox.CertModeManaged, CertID: 1},
	})
	hy2Cred, _ := json.Marshal(singbox.Credential{Password: "pw"})
	hy2Agg := NodeSpec{Tag: "hy2-agg", Protocol: singbox.ProtoHysteria2, Port: 20002, Config: string(aggRaw), CredJSON: string(hy2Cred)}

	vlessReality := nodeSpec(t, "vless-r", singbox.ProtoVLESS, 20003)
	vlessReality.Insecure = true // Reality 不应输出 tls.insecure

	d, err := Assemble("example.com", []NodeSpec{hy2Default, hy2Agg, vlessReality}, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	cfg, err := RenderSingBox(d)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}

	std := findOutbound(t, cfg, "hy2-std")
	if _, ok := std["bbr_profile"]; ok {
		t.Errorf("默认 standard bbr_profile 应省略以兼容旧版客户端: %v", std)
	}
	if tls, _ := std["tls"].(map[string]any); tls["insecure"] != true {
		t.Errorf("自签节点应输出 tls.insecure=true: %v", tls)
	}

	agg := findOutbound(t, cfg, "hy2-agg")
	if agg["bbr_profile"] != singbox.BBRProfileAggressive {
		t.Errorf("非默认 bbr_profile 应保留: %v", agg["bbr_profile"])
	}
	if tls, _ := agg["tls"].(map[string]any); tls["insecure"] != nil {
		t.Errorf("非自签节点不应输出 tls.insecure: %v", tls)
	}

	vr := findOutbound(t, cfg, "vless-r")
	if tls, _ := vr["tls"].(map[string]any); tls["insecure"] != nil {
		t.Errorf("Reality 节点不应输出 tls.insecure: %v", tls)
	}
}

// TestAssembleTemplatePlaceholderExpansion 规则模板无自定义出站时用 ["direct"] 占位的 auto/failover 组，
// 下发订阅且有可用节点时自动填充订阅节点，而 fixed 组保留 direct。
func TestAssembleTemplatePlaceholderExpansion(t *testing.T) {
	specs := []NodeSpec{
		nodeSpec(t, "n1", singbox.ProtoVLESS, 20001),
		nodeSpec(t, "n2", singbox.ProtoTrojan, 20002),
	}
	groups := []singbox.GroupEntry{
		{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"direct"}},
		{Name: "direct-only", Mode: singbox.GroupModeFixed, Members: []string{"direct"}},
	}
	d, err := Assemble("example.com", specs, groups, nil, "proxy", nil)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	if len(d.Groups) != 2 {
		t.Fatalf("应有 2 个策略组: %+v", d.Groups)
	}
	if got := d.Groups[0].Members; len(got) != 2 || got[0] != "n1" || got[1] != "n2" {
		t.Errorf("模板 auto 组应自动填充订阅节点, got %v", got)
	}
	if got := d.Groups[1].Members; len(got) != 1 || got[0] != "direct" {
		t.Errorf("手动 fixed 组含 direct 应保留, got %v", got)
	}
}
