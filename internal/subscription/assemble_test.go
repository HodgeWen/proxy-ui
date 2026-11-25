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

// TestAssembleGroupMemberMapping 策略组成员过滤为可用 tag，组内无代理成员时回落全部节点。
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
	// 组内没有任何代理成员（客户端不存在这些出站）→ 填充全部节点
	if len(d.Groups[1].Members) != 2 || d.Groups[1].Members[0] != "n1" || d.Groups[1].Members[1] != "n2" {
		t.Errorf("无代理成员的组应填充全部节点: %+v", d.Groups[1])
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

// TestAssembleTemplatePlaceholderExpansion 模板组在服务端只有 direct 占位（没有自定义出站）时，
// 下发订阅应填充该用户的节点而不是只剩 DIRECT；三种模式（含固定代理）一致。
func TestAssembleTemplatePlaceholderExpansion(t *testing.T) {
	specs := []NodeSpec{
		nodeSpec(t, "n1", singbox.ProtoVLESS, 20001),
		nodeSpec(t, "n2", singbox.ProtoTrojan, 20002),
	}
	groups := []singbox.GroupEntry{
		{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"direct"}},
		{Name: "manual", Mode: singbox.GroupModeFixed, Members: []string{"direct"}},
		{Name: "backup", Mode: singbox.GroupModeFailover, Members: []string{"direct"}},
	}
	d, err := Assemble("example.com", specs, groups, nil, "proxy", nil)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	if len(d.Groups) != 3 {
		t.Fatalf("应有 3 个策略组: %+v", d.Groups)
	}
	// 自动选择 / 故障转移丢掉 DIRECT（延迟近 0 会夺走整组），固定代理保留它作为可选手动出口
	wantMembers := [][]string{{"n1", "n2"}, {"n1", "n2", "direct"}, {"n1", "n2"}}
	for i, name := range []string{"proxy", "manual", "backup"} {
		got := d.Groups[i].Members
		if len(got) != len(wantMembers[i]) {
			t.Fatalf("%s 组成员 = %v, want %v", name, got, wantMembers[i])
		}
		for j := range got {
			if got[j] != wantMembers[i][j] {
				t.Fatalf("%s 组成员 = %v, want %v", name, got, wantMembers[i])
			}
		}
	}
}

// TestAssembleFixedGroupKeepsNodes 固定代理组只应「默认选第一个成员」，不能退化成只有 DIRECT。
func TestAssembleFixedGroupKeepsNodes(t *testing.T) {
	specs := []NodeSpec{
		nodeSpec(t, "n1", singbox.ProtoVLESS, 20001),
		nodeSpec(t, "n2", singbox.ProtoTrojan, 20002),
	}
	d, err := Assemble("example.com", specs, []singbox.GroupEntry{
		{Name: "proxy", Mode: singbox.GroupModeFixed, Members: []string{"direct"}},
	}, nil, "proxy", nil)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	cfg, err := RenderSingBox(d)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	group := findOutbound(t, cfg, "proxy")
	members, _ := group["outbounds"].([]string)
	if len(members) != 3 || members[0] != "n1" || members[1] != "n2" || members[2] != directTag {
		t.Fatalf("固定代理组的 outbounds 应为订阅节点 + direct: %v", group)
	}
	if group["default"] != "n1" {
		t.Errorf("固定代理组默认应选首个成员: %v", group["default"])
	}

	// clash：select 组的 proxies 同样不能只剩 DIRECT
	raw, err := RenderClash(d)
	if err != nil {
		t.Fatalf("渲染 clash 失败: %v", err)
	}
	doc := parseClash(t, raw)
	if len(doc.ProxyGroups) != 1 {
		t.Fatalf("应有 1 个 proxy-group: %+v", doc.ProxyGroups)
	}
	proxyMembers, _ := doc.ProxyGroups[0]["proxies"].([]any)
	if len(proxyMembers) != 3 || proxyMembers[0] != "n1" || proxyMembers[1] != "n2" || proxyMembers[2] != "DIRECT" {
		t.Errorf("固定代理组应包含节点并保留 DIRECT: %v", proxyMembers)
	}
}

// TestAssembleBreaksGroupCycle 循环引用的策略组必须被打破，否则客户端拒绝加载整份配置。
func TestAssembleBreaksGroupCycle(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "n1", singbox.ProtoVLESS, 20001)}
	d, err := Assemble("example.com", specs, []singbox.GroupEntry{
		{Name: "a", Mode: singbox.GroupModeFixed, Members: []string{"b"}},
		{Name: "b", Mode: singbox.GroupModeFixed, Members: []string{"a"}},
	}, nil, "a", nil)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	if cycle := singbox.FindGroupCycle(d.Groups); cycle != nil {
		t.Fatalf("订阅不应输出循环引用: %v", cycle)
	}
	if len(d.Groups) != 2 {
		t.Fatalf("应保留两个策略组: %+v", d.Groups)
	}
	// 环被打破后，环上那个已无出口的组改用订阅节点（另一个组仍可合法引用它）
	total := 0
	for _, g := range d.Groups {
		for _, m := range g.Members {
			if m == "n1" {
				total++
			}
		}
	}
	if total == 0 {
		t.Errorf("打破环后应至少有一组回落到订阅节点: %+v", d.Groups)
	}
	// 渲染出的客户端配置应能通过内核校验
	cfg, err := RenderSingBox(d)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	if err := singbox.New(t.TempDir()).CheckConfig(cfg); err != nil {
		t.Fatalf("客户端配置应通过内核校验: %v", err)
	}
}

// TestAssembleAutoGroupNeverMixesDirect 自动选择 / 故障转移组即便显式列了 direct 也不能保留，
// 否则 DIRECT 延迟近 0 必然夺走整组；固定代理组保留它作为可选手动出口。
func TestAssembleAutoGroupNeverMixesDirect(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "n1", singbox.ProtoVLESS, 20001)}
	for _, mode := range []string{singbox.GroupModeAuto, singbox.GroupModeFailover} {
		d, err := Assemble("example.com", specs, []singbox.GroupEntry{
			{Name: "proxy", Mode: mode, Members: []string{"n1", "direct"}},
		}, nil, "proxy", nil)
		if err != nil {
			t.Fatalf("%s 组装失败: %v", mode, err)
		}
		cfg, err := RenderSingBox(d)
		if err != nil {
			t.Fatalf("渲染失败: %v", err)
		}
		group := findOutbound(t, cfg, "proxy")
		members, _ := group["outbounds"].([]string)
		for _, m := range members {
			if m == directTag {
				t.Fatalf("%s 组不应包含 direct: %v", mode, members)
			}
		}
		if len(members) != 1 || members[0] != "n1" {
			t.Errorf("%s 组应只含节点: %v", mode, members)
		}
	}

	// 固定组保留 direct（手动切换有意义）
	d, err := Assemble("example.com", specs, []singbox.GroupEntry{
		{Name: "manual", Mode: singbox.GroupModeFixed, Members: []string{"n1", "direct"}},
	}, nil, "manual", nil)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	cfg, err := RenderSingBox(d)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	group := findOutbound(t, cfg, "manual")
	members, _ := group["outbounds"].([]string)
	if len(members) != 2 || members[1] != directTag {
		t.Errorf("固定组应保留 direct: %v", members)
	}
}

// TestAssembleDNSDetourRemapped 指向服务端专属出站的 DNS detour 在客户端必须回落到代理组，
// 否则客户端 DNS 指向不存在的 tag。
func TestAssembleDNSDetourRemapped(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "n1", singbox.ProtoVLESS, 20001)}
	dns := &singbox.DNSConfig{
		Enabled: true,
		Servers: []singbox.DNSServer{
			{Tag: "ali", Type: singbox.DNSTypeUDP, Server: "223.5.5.5"},
			{Tag: "doh", Type: singbox.DNSTypeHTTPS, Server: "1.1.1.1", Detour: "hk-socks"},
		},
		Final: "doh",
	}
	d, err := Assemble("example.com", specs, []singbox.GroupEntry{
		{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"n1"}},
	}, nil, "proxy", dns)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	cfg, err := RenderSingBox(d)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	if err := singbox.New(t.TempDir()).CheckConfig(cfg); err != nil {
		t.Fatalf("客户端配置应通过内核校验: %v", err)
	}
	servers, _ := cfg.DNS["servers"].([]map[string]any)
	for _, srv := range servers {
		if srv["tag"] == "doh" && srv["detour"] != "proxy" {
			t.Errorf("服务端专属 detour 应回落到代理组, got %v", srv["detour"])
		}
	}
	// clash 侧同样回落到代理组
	raw, err := RenderClash(d)
	if err != nil {
		t.Fatalf("渲染 clash 失败: %v", err)
	}
	if strings.Contains(string(raw), "#hk-socks") {
		t.Errorf("clash DNS 不应引用客户端不存在的出站:\n%s", raw)
	}
}

// TestAssembleGroupNameCollidesWithNode 策略组与节点同名时不能产出重复 tag（含改名后撞名）。
func TestAssembleGroupNameCollidesWithNode(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "proxy", singbox.ProtoVLESS, 20001)}
	cases := [][]singbox.GroupEntry{
		{{Name: "proxy", Mode: singbox.GroupModeFixed, Members: []string{"direct"}}},
		{{Name: "inner", Mode: singbox.GroupModeFixed, Members: []string{"direct"}},
			{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"inner"}}},
		// 改名后的 proxy-group 与已有组名撞车
		{{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"proxy-group"}},
			{Name: "proxy-group", Mode: singbox.GroupModeFixed, Members: []string{"direct"}}},
	}
	for i, groups := range cases {
		d, err := Assemble("example.com", specs, groups, nil, "proxy", nil)
		if err != nil {
			t.Fatalf("case %d 组装失败: %v", i, err)
		}
		if d.Final == "proxy" {
			t.Errorf("case %d 兜底出站应指向改名后的组: %q", i, d.Final)
		}
		cfg, err := RenderSingBox(d)
		if err != nil {
			t.Fatalf("case %d 渲染失败: %v", i, err)
		}
		if err := singbox.New(t.TempDir()).CheckConfig(cfg); err != nil {
			t.Fatalf("case %d 客户端配置应通过内核校验（tag 不得重复）: %v", i, err)
		}
	}
}

// TestAssembleKeepsRealGroupDetour 指向客户端真实存在的策略组的 DNS detour 不应被改写。
func TestAssembleKeepsRealGroupDetour(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "n1", singbox.ProtoVLESS, 20001)}
	dns := &singbox.DNSConfig{
		Enabled: true,
		Servers: []singbox.DNSServer{
			{Tag: "ali", Type: singbox.DNSTypeUDP, Server: "223.5.5.5"},
			{Tag: "doh", Type: singbox.DNSTypeHTTPS, Server: "1.1.1.1", Detour: "manual"},
		},
		Final: "doh",
	}
	d, err := Assemble("example.com", specs, []singbox.GroupEntry{
		{Name: "manual", Mode: singbox.GroupModeFixed, Members: []string{"n1"}},
		{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"n1"}},
	}, nil, "proxy", dns)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	cfg, err := RenderSingBox(d)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	servers, _ := cfg.DNS["servers"].([]map[string]any)
	for _, srv := range servers {
		if srv["tag"] == "doh" && srv["detour"] != "manual" {
			t.Errorf("指向真实策略组的 detour 应保留, got %v", srv["detour"])
		}
	}
	raw, err := RenderClash(d)
	if err != nil {
		t.Fatalf("渲染 clash 失败: %v", err)
	}
	if !strings.Contains(string(raw), "#manual") {
		t.Errorf("clash 应保留真实策略组 detour:\n%s", raw)
	}
}

// TestAssembleGroupRenameFollowsReferences 组与节点重名被改名后，规则与兜底出站必须跟着指向新名，
// 不能静默落到别的组上。
func TestAssembleGroupRenameFollowsReferences(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "proxy", singbox.ProtoVLESS, 20001)}
	rules := []singbox.RuleEntry{
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodSuffix, Value: "x.com", Outbound: "proxy"},
	}
	d, err := Assemble("example.com", specs, []singbox.GroupEntry{
		{Name: "first", Mode: singbox.GroupModeAuto, Members: []string{"direct"}},
		{Name: "proxy", Mode: singbox.GroupModeFixed, Members: []string{"direct"}},
	}, rules, "proxy", nil)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	if len(d.Groups) != 2 || d.Groups[1].Name == "proxy" {
		t.Fatalf("重名组应被改名: %+v", d.Groups)
	}
	renamedName := d.Groups[1].Name
	if d.Rules[0].Outbound != renamedName {
		t.Errorf("规则应跟随改名后的组, got %q want %q", d.Rules[0].Outbound, renamedName)
	}
	if d.Final != renamedName {
		t.Errorf("兜底出站应跟随改名后的组, got %q want %q", d.Final, renamedName)
	}
	if d.Final == "first" {
		t.Error("兜底出站不应静默落到另一个组上")
	}
}

// TestAssembleFinalNeverBlock 兜底出站为拦截时不能写进 route.final / MATCH（内核没有 block 出站）。
func TestAssembleFinalNeverBlock(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "n1", singbox.ProtoVLESS, 20001)}
	groups := []singbox.GroupEntry{{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"n1"}}}
	d, err := Assemble("example.com", specs, groups, nil, singbox.OutboundBlock, nil)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	if d.Final != "proxy" {
		t.Errorf("拦截动作不能作为兜底出站, got %q", d.Final)
	}

	// 无策略组时回落到 direct
	d, err = Assemble("example.com", nil, nil, nil, singbox.OutboundBlock, nil)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	if d.Final != "direct" {
		t.Errorf("无可用策略组时兜底应为 direct, got %q", d.Final)
	}
}

// TestAssembleNoCustomOutboundMembers 服务端自有出站（自定义出站）在客户端不存在，
// 只由它组成的组必须换成该用户的节点，否则客户端该组无可用出口。
func TestAssembleNoCustomOutboundMembers(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "n1", singbox.ProtoVLESS, 20001)}
	rules := []singbox.RuleEntry{
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodSuffix, Value: "x.com", Outbound: "hk-socks"},
	}
	d, err := Assemble("example.com", specs, []singbox.GroupEntry{
		{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"hk-socks"}},
	}, rules, "hk-socks", nil)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	if len(d.Groups[0].Members) != 1 || d.Groups[0].Members[0] != "n1" {
		t.Errorf("仅含自定义出站的组应换成订阅节点: %+v", d.Groups[0])
	}
	if d.Rules[0].Outbound != "proxy" {
		t.Errorf("指向客户端不存在出站的规则应回落到代理组, got %q", d.Rules[0].Outbound)
	}
	if d.Final != "proxy" {
		t.Errorf("指向客户端不存在出站的兜底应回落到代理组, got %q", d.Final)
	}
}

// TestAssembleAndRenderIPv6Variants IPv6 附加节点在策略组自动纳入、节点不绑定 domain_resolver tag、
// ipv4_only 下存在域名 IPv6 节点时客户端 DNS 自动放开到 prefer_ipv4、裸 IPv6 节点 SNI 回落主域名，且整体通过内核校验。
func TestAssembleAndRenderIPv6Variants(t *testing.T) {
	tlsRaw, _ := json.Marshal(&singbox.TrojanConfig{
		TLS: singbox.TLSConfig{Enabled: true, CertMode: singbox.CertModeManaged, CertID: 1},
	})
	credRaw, _ := json.Marshal(singbox.Credential{Password: "secret"})

	// 1. 裸 IPv6 附加节点：TLS 未填 server_name 时 SNI 回落主域名 example.com；显式引用 n1 的策略组自动带上 n1-ipv6
	specsLiteral := []NodeSpec{
		{Tag: "n1", Protocol: singbox.ProtoTrojan, Port: 20001, Config: string(tlsRaw), CredJSON: string(credRaw)},
		{Tag: "n1-ipv6", Server: "2001:db8::1", IPv6: true, VariantOf: "n1", Protocol: singbox.ProtoTrojan, Port: 20001, Config: string(tlsRaw), CredJSON: string(credRaw)},
	}
	groups := []singbox.GroupEntry{
		{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"n1"}},
	}
	d, err := Assemble("example.com", specsLiteral, groups, nil, "proxy", singbox.DefaultDNSConfig())
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	if got := d.Groups[0].Members; len(got) != 2 || got[0] != "n1" || got[1] != "n1-ipv6" {
		t.Fatalf("显式引用主节点的策略组应自动带上 -ipv6 节点: %v", got)
	}
	cfg, err := RenderSingBox(d)
	if err != nil {
		t.Fatalf("渲染 sing-box 失败: %v", err)
	}
	obLiteral := findOutbound(t, cfg, "n1-ipv6")
	if obLiteral["server"] != "2001:db8::1" {
		t.Errorf("IPv6 节点 server = %v, want 2001:db8::1", obLiteral["server"])
	}
	if _, hasDR := obLiteral["domain_resolver"]; hasDR {
		t.Errorf("裸 IPv6 地址节点不应写 domain_resolver: %v", obLiteral)
	}
	if tls, _ := obLiteral["tls"].(map[string]any); tls["server_name"] != "example.com" {
		t.Errorf("裸 IPv6 节点 SNI 应回落到主域名 example.com, got %v", tls["server_name"])
	}
	if err := singbox.New(t.TempDir()).CheckConfig(cfg); err != nil {
		t.Fatalf("裸 IPv6 节点配置应通过内核 CheckConfig 校验: %v", err)
	}

	// 2. 纯 AAAA 域名附加节点：节点出站不绑定 domain_resolver（由 route.default_domain_resolver 兜底），兼容出站提取类客户端
	specsDomain := []NodeSpec{
		nodeSpec(t, "v1", singbox.ProtoVLESS, 20002),
		func() NodeSpec {
			sp := nodeSpec(t, "v1-ipv6", singbox.ProtoVLESS, 20002)
			sp.Server = "v6.example.com"
			sp.IPv6 = true
			sp.VariantOf = "v1"
			return sp
		}(),
	}
	d2, err := Assemble("example.com", specsDomain, []singbox.GroupEntry{
		{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"v1"}},
	}, nil, "proxy", singbox.DefaultDNSConfig())
	if err != nil {
		t.Fatalf("域名 IPv6 组装失败: %v", err)
	}
	cfg2, err := RenderSingBox(d2)
	if err != nil {
		t.Fatalf("域名 IPv6 渲染失败: %v", err)
	}
	obDomain := findOutbound(t, cfg2, "v1-ipv6")
	if obDomain["server"] != "v6.example.com" {
		t.Errorf("域名 IPv6 节点 server = %v, want v6.example.com", obDomain["server"])
	}
	if _, hasDR := obDomain["domain_resolver"]; hasDR {
		t.Errorf("域名 IPv6 节点出站不应绑定 domain_resolver: %v", obDomain["domain_resolver"])
	}
	if cfg2.Route == nil || cfg2.Route.DefaultDomainResolver != "ali" {
		t.Errorf("应由 route.default_domain_resolver 兜底为 ali, got %+v", cfg2.Route)
	}
	if err := singbox.New(t.TempDir()).CheckConfig(cfg2); err != nil {
		t.Fatalf("域名 IPv6 节点配置应通过内核 CheckConfig 校验: %v", err)
	}

	// 3. 全局 DNS 策略为 ipv4_only 且存在域名 IPv6 节点时：客户端 DNS 自动放开为 prefer_ipv4，保证可解析 AAAA
	dnsV4Only := singbox.DNSConfig{
		Enabled:  true,
		Strategy: singbox.StrategyIPv4Only,
		Servers:  []singbox.DNSServer{{Tag: "ali", Type: singbox.DNSTypeUDP, Server: "223.5.5.5"}},
	}
	d3, err := Assemble("example.com", specsDomain, []singbox.GroupEntry{
		{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"v1"}},
	}, nil, "proxy", &dnsV4Only)
	if err != nil {
		t.Fatalf("ipv4_only + 域名 IPv6 组装失败: %v", err)
	}
	cfg3, err := RenderSingBox(d3)
	if err != nil {
		t.Fatalf("ipv4_only + 域名 IPv6 渲染失败: %v", err)
	}
	if cfg3.DNS["strategy"] != singbox.StrategyPreferIPv4 {
		t.Errorf("存在域名 IPv6 节点时客户端 dns.strategy = %v, want %q", cfg3.DNS["strategy"], singbox.StrategyPreferIPv4)
	}
	if dnsV4Only.Strategy != singbox.StrategyIPv4Only {
		t.Errorf("不应修改入参 DNSConfig.Strategy, got %q", dnsV4Only.Strategy)
	}
	obDomain3 := findOutbound(t, cfg3, "v1-ipv6")
	if _, hasDR := obDomain3["domain_resolver"]; hasDR {
		t.Errorf("ipv4_only 场景下域名 IPv6 节点出站也不应写 domain_resolver: %v", obDomain3)
	}
	if err := singbox.New(t.TempDir()).CheckConfig(cfg3); err != nil {
		t.Fatalf("ipv4_only + 域名 IPv6 配置应通过内核 CheckConfig 校验: %v", err)
	}

	// 4. 全局 DNS 策略为 ipv4_only 且附加节点为裸 IPv6 地址时：客户端 dns.strategy 保持 ipv4_only（目标流量纯 IPv4 防风控），
	// 同时裸 IPv6 节点直连不受 DNS 策略阻断且通过内核校验
	d4, err := Assemble("proxy.example.com", specsLiteral, []singbox.GroupEntry{
		{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"n1"}},
	}, nil, "proxy", &dnsV4Only)
	if err != nil {
		t.Fatalf("ipv4_only + 裸 IPv6 组装失败: %v", err)
	}
	cfg4, err := RenderSingBox(d4)
	if err != nil {
		t.Fatalf("ipv4_only + 裸 IPv6 渲染失败: %v", err)
	}
	if cfg4.DNS["strategy"] != singbox.StrategyIPv4Only {
		t.Errorf("裸 IPv6 节点下 ipv4_only 不应放开: strategy = %v, want %q", cfg4.DNS["strategy"], singbox.StrategyIPv4Only)
	}
	obLiteral4 := findOutbound(t, cfg4, "n1-ipv6")
	if obLiteral4["server"] != "2001:db8::1" {
		t.Errorf("裸 IPv6 节点 server = %v, want 2001:db8::1", obLiteral4["server"])
	}
	if tls4, _ := obLiteral4["tls"].(map[string]any); tls4["server_name"] != "proxy.example.com" {
		t.Errorf("裸 IPv6 节点 SNI 应回落到具体主域名 proxy.example.com, got %v", tls4["server_name"])
	}
	if err := singbox.New(t.TempDir()).CheckConfig(cfg4); err != nil {
		t.Fatalf("ipv4_only + 裸 IPv6 配置应通过内核 CheckConfig 校验: %v", err)
	}

	// 5. 入站显式填写通配符证书名（*.example.com）时：客户端 SNI 不得含 *，主节点与裸 IPv6 节点均回落具体主域名
	wildcardTLSRaw, _ := json.Marshal(&singbox.TrojanConfig{
		TLS: singbox.TLSConfig{Enabled: true, CertMode: singbox.CertModeManaged, CertID: 1, ServerName: "*.example.com"},
	})
	specsWildcard := []NodeSpec{
		{Tag: "w1", Protocol: singbox.ProtoTrojan, Port: 20001, Config: string(wildcardTLSRaw), CredJSON: string(credRaw)},
		{Tag: "w1-ipv6", Server: "2001:db8::1", IPv6: true, VariantOf: "w1", Protocol: singbox.ProtoTrojan, Port: 20001, Config: string(wildcardTLSRaw), CredJSON: string(credRaw)},
	}
	d5, err := Assemble("proxy.example.com", specsWildcard, nil, nil, "", &dnsV4Only)
	if err != nil {
		t.Fatalf("泛域名 SNI 组装失败: %v", err)
	}
	cfg5, err := RenderSingBox(d5)
	if err != nil {
		t.Fatalf("泛域名 SNI 渲染失败: %v", err)
	}
	for _, tag := range []string{"w1", "w1-ipv6"} {
		ob := findOutbound(t, cfg5, tag)
		tls5, _ := ob["tls"].(map[string]any)
		if tls5["server_name"] != "proxy.example.com" {
			t.Errorf("节点 %s 通配符 SNI 应回落为 proxy.example.com, got %v", tag, tls5["server_name"])
		}
	}
	if err := singbox.New(t.TempDir()).CheckConfig(cfg5); err != nil {
		t.Fatalf("泛域名 SNI 回落配置应通过内核 CheckConfig 校验: %v", err)
	}
}
