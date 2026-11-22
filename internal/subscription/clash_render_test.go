package subscription

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"proxy-ui/internal/singbox"
)

// clashConfigDoc 反序列化后的 clash 配置（断言用）。
type clashConfigDoc struct {
	Proxies       []map[string]any `yaml:"proxies"`
	ProxyGroups   []map[string]any `yaml:"proxy-groups"`
	RuleProviders map[string]struct {
		Type     string `yaml:"type"`
		Behavior string `yaml:"behavior"`
		Format   string `yaml:"format"`
		URL      string `yaml:"url"`
		Path     string `yaml:"path"`
		Interval int    `yaml:"interval"`
		Proxy    string `yaml:"proxy"`
	} `yaml:"rule-providers"`
	Rules []string `yaml:"rules"`
	DNS   struct {
		Enable        bool                `yaml:"enable"`
		EnhancedMode  string              `yaml:"enhanced-mode"`
		FakeIPRange   string              `yaml:"fake-ip-range"`
		FakeIPFilter  []string            `yaml:"fake-ip-filter"`
		Nameserver    []string            `yaml:"nameserver"`
		DefaultNS     []string            `yaml:"default-nameserver"`
		ProxyServerNS []string            `yaml:"proxy-server-nameserver"`
		DirectNS      []string            `yaml:"direct-nameserver"`
		NameserverPol map[string][]string `yaml:"nameserver-policy"`
	} `yaml:"dns"`
}

// parseClash 把渲染结果解析为断言用结构。
func parseClash(t *testing.T, raw []byte) clashConfigDoc {
	t.Helper()
	var doc clashConfigDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("clash 配置应为合法 YAML: %v\n%s", err, raw)
	}
	return doc
}

// findProxy 按 name 取 proxy。
func findProxy(t *testing.T, doc clashConfigDoc, name string) map[string]any {
	t.Helper()
	for _, p := range doc.Proxies {
		if p["name"] == name {
			return p
		}
	}
	t.Fatalf("proxy %s 不存在: %v", name, doc.Proxies)
	return nil
}

// intOf 把 YAML 解析出的数值（int/uint64/float64）统一转 int，便于断言。
func intOf(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case float64:
		return int(n)
	}
	return -1
}

// TestRenderClashStructure clash 配置整体结构：proxies/proxy-groups/rules/rule-providers/dns。
func TestRenderClashStructure(t *testing.T) {
	specs := []NodeSpec{
		nodeSpec(t, "n-vless", singbox.ProtoVLESS, 20001),
		nodeSpec(t, "n-hy2", singbox.ProtoHysteria2, 20002),
	}
	adSrc := &singbox.RuleSetSource{
		Tag: singbox.AntiADTag, URL: singbox.AntiADBuiltinURL,
		MihomoURL: singbox.AntiADMihomoURL, MihomoBehavior: "domain",
	}
	cnSrc := &singbox.RuleSetSource{
		Tag: singbox.GeoIPCNTag, URL: "https://example.com/cn.srs",
		MihomoURL: singbox.GeoIPCNMihomoURL, MihomoBehavior: "ipcidr",
	}
	groups := []singbox.GroupEntry{
		{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"n-vless", "n-hy2"}},
		{Name: "manual", Mode: singbox.GroupModeFixed, Members: []string{"n-vless", "direct"}},
	}
	rules := []singbox.RuleEntry{
		{MatchType: singbox.MatchRuleSet, RuleSet: adSrc, Outbound: singbox.OutboundBlock},
		{MatchType: singbox.MatchRuleSet, RuleSet: cnSrc, Outbound: "direct"},
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodSuffix, Value: "a.com,b.com", Outbound: "proxy"},
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodExact, Value: "exact.com", Outbound: "proxy"},
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodKeyword, Value: "ads", Outbound: singbox.OutboundBlock},
		{MatchType: singbox.MatchDomain, MatchMethod: singbox.MethodRegex, Value: `.*\.foo\.com`, Outbound: "direct"},
		{MatchType: singbox.MatchIP, MatchMethod: singbox.MethodCIDR, Value: "10.0.0.0/8", Outbound: "direct"},
	}
	d, err := Assemble("example.com", specs, groups, rules, "proxy", singbox.DefaultDNSConfig())
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	raw, err := RenderClash(d)
	if err != nil {
		t.Fatalf("渲染 clash 配置失败: %v", err)
	}
	doc := parseClash(t, raw)

	// proxies：每个节点一条，服务器/端口就位
	if len(doc.Proxies) != 2 {
		t.Fatalf("应有 2 个 proxy, got %d", len(doc.Proxies))
	}
	vless := findProxy(t, doc, "n-vless")
	if vless["type"] != "vless" || vless["server"] != "example.com" || intOf(vless["port"]) != 20001 {
		t.Errorf("VLESS proxy 基础字段不符: %v", vless)
	}
	if vless["uuid"] == "" || vless["flow"] != "xtls-rprx-vision" {
		t.Errorf("VLESS proxy 凭据/流控不符: %v", vless)
	}
	if vless["tls"] != true || vless["servername"] != "www.microsoft.com" || vless["client-fingerprint"] != "chrome" {
		t.Errorf("VLESS Reality proxy TLS 字段不符: %v", vless)
	}
	reality, _ := vless["reality-opts"].(map[string]any)
	if reality == nil || reality["public-key"] == "" {
		t.Errorf("VLESS Reality proxy 缺少 reality-opts: %v", vless)
	}
	hy2 := findProxy(t, doc, "n-hy2")
	if hy2["type"] != "hysteria2" || hy2["password"] == "" {
		t.Errorf("Hysteria2 proxy 字段不符: %v", hy2)
	}
	if hy2["obfs"] != "salamander" || hy2["obfs-password"] == "" {
		t.Errorf("Hysteria2 proxy 混淆字段不符: %v", hy2)
	}
	if alpn, _ := hy2["alpn"].([]any); len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("Hysteria2 proxy ALPN 应含 h3: %v", hy2)
	}

	// proxy-groups：auto → url-test、fixed → select
	if len(doc.ProxyGroups) != 2 {
		t.Fatalf("应有 2 个 proxy-group, got %d", len(doc.ProxyGroups))
	}
	if doc.ProxyGroups[0]["name"] != "proxy" || doc.ProxyGroups[0]["type"] != "url-test" {
		t.Errorf("auto 组应映射为 url-test: %v", doc.ProxyGroups[0])
	}
	if doc.ProxyGroups[1]["type"] != "select" {
		t.Errorf("fixed 组应映射为 select: %v", doc.ProxyGroups[1])
	}
	// direct 成员转大写 DIRECT
	members, _ := doc.ProxyGroups[1]["proxies"].([]any)
	if len(members) != 2 || members[1] != "DIRECT" {
		t.Errorf("direct 成员应映射为 DIRECT: %v", doc.ProxyGroups[1]["proxies"])
	}

	// rule-providers：mrs 源 → format mrs，yaml 源 → format yaml，behavior 透传
	if len(doc.RuleProviders) != 2 {
		t.Fatalf("应有 2 个 rule-provider, got %d: %v", len(doc.RuleProviders), doc.RuleProviders)
	}
	antiAD, ok := doc.RuleProviders[singbox.AntiADTag]
	if !ok {
		t.Fatalf("缺少 %s rule-provider: %v", singbox.AntiADTag, doc.RuleProviders)
	}
	if antiAD.Type != "http" || antiAD.Behavior != "domain" || antiAD.Format != "yaml" {
		t.Errorf("anti-ad rule-provider 字段不符: %+v", antiAD)
	}
	if antiAD.Path != "./ruleset/anti-ad.yaml" {
		t.Errorf("anti-ad 缓存路径不符: %q", antiAD.Path)
	}
	geoip, ok := doc.RuleProviders[singbox.GeoIPCNTag]
	if !ok {
		t.Fatalf("缺少 %s rule-provider: %v", singbox.GeoIPCNTag, doc.RuleProviders)
	}
	if geoip.Format != "mrs" || geoip.Behavior != "ipcidr" {
		t.Errorf(".mrs 源应映射为 format mrs + behavior ipcidr: %+v", geoip)
	}
	if geoip.Path != "./ruleset/geoip-cn.mrs" {
		t.Errorf("geoip 缓存路径不符: %q", geoip.Path)
	}
	if antiAD.Interval != 86400 {
		t.Errorf("rule-provider 更新间隔不符: %d", antiAD.Interval)
	}
	if antiAD.Proxy != "proxy" || geoip.Proxy != "proxy" {
		t.Errorf("rule-provider 应经由代理组下载: antiAD=%q geoip=%q", antiAD.Proxy, geoip.Proxy)
	}

	// rules：各匹配方式映射正确（ipcidr 规则集带 no-resolve），最后补 MATCH
	joined := strings.Join(doc.Rules, "\n")
	for _, want := range []string{
		"RULE-SET,anti-ad,REJECT",
		"RULE-SET,geoip-cn,DIRECT,no-resolve",
		"DOMAIN-SUFFIX,a.com,proxy",
		"DOMAIN-SUFFIX,b.com,proxy",
		"DOMAIN,exact.com,proxy",
		"DOMAIN-KEYWORD,ads,REJECT",
		"DOMAIN-REGEX,.*\\.foo\\.com,DIRECT",
		"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("clash 规则缺少 %q:\n%s", want, joined)
		}
	}
	if last := doc.Rules[len(doc.Rules)-1]; last != "MATCH,proxy" {
		t.Errorf("最后一条规则应为 MATCH,proxy, got %q", last)
	}

	// dns 段
	if !doc.DNS.Enable {
		t.Error("应渲染 dns.enable = true")
	}
	if doc.DNS.EnhancedMode != "fake-ip" || doc.DNS.FakeIPRange == "" || len(doc.DNS.FakeIPFilter) == 0 {
		t.Errorf("应渲染 fake-ip 配置与过滤列表: %+v", doc.DNS)
	}
	if len(doc.DNS.Nameserver) != 1 || doc.DNS.Nameserver[0] != "https://1.1.1.1/dns-query#proxy" {
		t.Errorf("远程 DoH nameserver 应附带 #proxy: %v", doc.DNS.Nameserver)
	}
	if len(doc.DNS.DefaultNS) == 0 || len(doc.DNS.ProxyServerNS) == 0 || len(doc.DNS.DirectNS) == 0 {
		t.Errorf("dns default/proxy-server/direct nameserver 不应为空: %+v", doc.DNS)
	}
	if len(doc.DNS.NameserverPol) == 0 {
		t.Error("dns.nameserver-policy 不应为空（按域名分流）")
	}
	if got := doc.DNS.NameserverPol["+.cn"]; len(got) == 0 {
		t.Errorf("dns.nameserver-policy 应为含 +.cn 的 mapping: %v", doc.DNS.NameserverPol)
	}
	// 用 YAML AST 校验 dns.nameserver-policy 确实为 MappingNode（非 SequenceNode），防止 mihomo 报 pipeline must contain YAML mapping
	file, err := parser.ParseBytes(raw, 0)
	if err != nil {
		t.Fatalf("AST 解析失败: %v\n%s", err, raw)
	}
	var rawMap map[string]any
	if err := yaml.Unmarshal(raw, &rawMap); err != nil {
		t.Fatalf("Unmarshal map 失败: %v", err)
	}
	dnsMap, _ := rawMap["dns"].(map[string]any)
	if _, ok := dnsMap["nameserver-policy"].(map[string]any); !ok {
		t.Fatalf("dns.nameserver-policy 应反序列化为 map[string]any，实际 %T:\n%s", dnsMap["nameserver-policy"], raw)
	}
	policyNode, err := yaml.PathString("$.dns.nameserver-policy")
	if err != nil {
		t.Fatal(err)
	}
	node, err := policyNode.FilterFile(file)
	if err != nil {
		t.Fatalf("查找 $.dns.nameserver-policy 节点失败: %v", err)
	}
	if node.Type() != ast.MappingType {
		t.Fatalf("$.dns.nameserver-policy AST 类型应为 MappingType, got %v:\n%s", node.Type(), raw)
	}
}

// TestRenderClashShadowsocks2022 SS-2022 的 clash 密码同样是「入站PSK:用户密钥」。
func TestRenderClashShadowsocks2022(t *testing.T) {
	spec, psk, key := ssSpec(t, "n-ss", 20007)
	d, err := Assemble("example.com", []NodeSpec{spec}, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	raw, err := RenderClash(d)
	if err != nil {
		t.Fatalf("渲染 clash 配置失败: %v", err)
	}
	doc := parseClash(t, raw)
	p := findProxy(t, doc, "n-ss")
	if p["type"] != "ss" {
		t.Errorf("SS proxy 类型 = %v, want ss", p["type"])
	}
	if p["cipher"] != "2022-blake3-aes-128-gcm" {
		t.Errorf("SS proxy cipher = %v", p["cipher"])
	}
	want := psk + ":" + key
	if p["password"] != want {
		t.Fatalf("SS-2022 clash 密码应为「入站PSK:用户密钥」\n got = %v\nwant = %q", p["password"], want)
	}
}

// TestRenderClashNoDNS DNS 关闭时不渲染 dns 段。
func TestRenderClashNoDNS(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "n1", singbox.ProtoTrojan, 20001)}
	d, err := Assemble("example.com", specs, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	raw, err := RenderClash(d)
	if err != nil {
		t.Fatalf("渲染 clash 配置失败: %v", err)
	}
	if strings.Contains(string(raw), "\ndns:") {
		t.Errorf("DNS 关闭时不应渲染 dns 段:\n%s", raw)
	}
	doc := parseClash(t, raw)
	if doc.DNS.Enable {
		t.Error("DNS 关闭时 dns.enable 应为 false")
	}
}

// TestClashDNSOf DNS 服务器映射到 mihomo 地址串。
func TestClashDNSOf(t *testing.T) {
	cfg := &singbox.DNSConfig{
		Enabled: true,
		Servers: []singbox.DNSServer{
			{Tag: "local", Type: singbox.DNSTypeLocal},
			{Tag: "udp", Type: singbox.DNSTypeUDP, Server: "223.5.5.5", ServerPort: 53},
			{Tag: "udp-custom", Type: singbox.DNSTypeUDP, Server: "8.8.8.8", ServerPort: 5353},
			{Tag: "doh", Type: singbox.DNSTypeHTTPS, Server: "1.1.1.1", ServerPort: 443, Path: "/dns-query"},
			{Tag: "dot", Type: singbox.DNSTypeTLS, Server: "1.0.0.1"},
		},
		Rules: []singbox.DNSRule{{DomainSuffix: "cn", Server: "udp"}},
		Final: "doh",
	}
	out := clashDNSOf(cfg, "proxy")
	if out == nil {
		t.Fatal("启用时不应返回 nil")
	}
	if !out.Enable {
		t.Error("dns.enable 应为 true")
	}
	// final 指定时 nameserver 只保留该服务器，远程 DoH 自动走代理组
	if len(out.Nameserver) != 1 || out.Nameserver[0] != "https://1.1.1.1/dns-query#proxy" {
		t.Errorf("final 指定时远程 DoH nameserver 应附带 #proxy: %v", out.Nameserver)
	}
	// 无 proxyGroup 时不追加 #proxy；显式指定 Detour 时优先保留
	if noGroup := clashDNSOf(cfg, ""); len(noGroup.Nameserver) != 1 || noGroup.Nameserver[0] != "https://1.1.1.1/dns-query" {
		t.Errorf("无 proxyGroup 时不应追加 #: %v", noGroup.Nameserver)
	}
	explicitDetour := clashDNSOf(&singbox.DNSConfig{
		Enabled: true,
		Servers: []singbox.DNSServer{{Tag: "doh", Type: singbox.DNSTypeHTTPS, Server: "1.1.1.1", Detour: "direct"}},
		Final:   "doh",
	}, "proxy")
	if len(explicitDetour.Nameserver) != 1 || explicitDetour.Nameserver[0] != "https://1.1.1.1/dns-query#DIRECT" {
		t.Errorf("显式 Detour=direct 应映射为 #DIRECT: %v", explicitDetour.Nameserver)
	}
	// default-nameserver / proxy-server-nameserver / direct-nameserver 用明文 UDP 服务器
	if len(out.DefaultNameser) != 2 || out.DefaultNameser[0] != "223.5.5.5" || out.DefaultNameser[1] != "8.8.8.8:5353" {
		t.Errorf("default-nameserver 应取明文 UDP 服务器: %v", out.DefaultNameser)
	}
	if len(out.ProxyServerNameser) != 2 || len(out.DirectNameser) != 2 {
		t.Errorf("proxy-server-nameserver 与 direct-nameserver 应与 default-nameserver 一致: %+v", out)
	}
	// 规则展开为 mihomo 域名匹配串（后缀用 +. 匹配根域与子域）且保持顺序与去重
	if len(out.NameserverPol) != 1 || out.NameserverPol[0].Key != "+.cn" {
		t.Errorf("nameserver-policy 域名映射不符: %+v", out.NameserverPol)
	}
	if servers, _ := out.NameserverPol[0].Value.([]string); len(servers) == 0 || servers[0] != "223.5.5.5" {
		t.Errorf("nameserver-policy 服务器不符: %+v", out.NameserverPol[0])
	}

	// 重复域名规则先到优先且去重
	dupPolicy := clashDNSOf(&singbox.DNSConfig{
		Enabled: true,
		Servers: []singbox.DNSServer{
			{Tag: "a", Type: singbox.DNSTypeUDP, Server: "223.5.5.5"},
			{Tag: "b", Type: singbox.DNSTypeUDP, Server: "8.8.8.8"},
		},
		Rules: []singbox.DNSRule{
			{DomainSuffix: "cn,qq.com", Server: "a"},
			{DomainSuffix: "cn", Server: "b"},
		},
	}, "proxy")
	if len(dupPolicy.NameserverPol) != 2 {
		t.Errorf("重复域名应去重为 2 项, got %+v", dupPolicy.NameserverPol)
	}
	if s, _ := dupPolicy.NameserverPol[0].Value.([]string); len(s) == 0 || s[0] != "223.5.5.5" {
		t.Errorf("重复域名应保留先出现的规则: %+v", dupPolicy.NameserverPol[0])
	}

	// 非 IP 型 UDP 服务器不进入 default-nameserver，回落公共 DNS
	domainUDP := clashDNSOf(&singbox.DNSConfig{
		Enabled: true,
		Servers: []singbox.DNSServer{{Tag: "u", Type: singbox.DNSTypeUDP, Server: "dns.alidns.com"}},
	}, "proxy")
	if len(domainUDP.DefaultNameser) == 0 || domainUDP.DefaultNameser[0] != "223.5.5.5" {
		t.Errorf("非 IP UDP 服务器不应放入 default-nameserver, got %v", domainUDP.DefaultNameser)
	}

	// 无明文 UDP 服务器时 default-nameserver 回落到公共 DNS
	onlyDoH := &singbox.DNSConfig{
		Enabled: true,
		Servers: []singbox.DNSServer{{Tag: "doh", Type: singbox.DNSTypeHTTPS, Server: "1.1.1.1"}},
	}
	fallback := clashDNSOf(onlyDoH, "proxy")
	if len(fallback.DefaultNameser) == 0 {
		t.Error("无明文服务器时 default-nameserver 应给出兜底值")
	}

	// 未启用返回 nil
	if clashDNSOf(nil, "proxy") != nil || clashDNSOf(&singbox.DNSConfig{Enabled: false}, "proxy") != nil {
		t.Error("未启用时应返回 nil")
	}
}

// TestClashProviderFormat 规则集源格式判定。
func TestClashProviderFormat(t *testing.T) {
	tests := []struct {
		url        string
		wantFormat string
		wantExt    string
	}{
		{"https://example.com/a.mrs", "mrs", ".mrs"},
		{"https://example.com/a.MRS", "mrs", ".mrs"},
		{"https://example.com/a.yaml", "yaml", ".yaml"},
		{"https://example.com/clash.yaml", "yaml", ".yaml"},
		{"", "yaml", ".yaml"},
	}
	for _, tt := range tests {
		if got := clashProviderFormat(tt.url); got != tt.wantFormat {
			t.Errorf("clashProviderFormat(%q) = %q, want %q", tt.url, got, tt.wantFormat)
		}
		if got := clashProviderExt(tt.url); got != tt.wantExt {
			t.Errorf("clashProviderExt(%q) = %q, want %q", tt.url, got, tt.wantExt)
		}
	}
}

// TestClashOutbound 出站 tag 到 clash 动作的映射。
func TestClashOutbound(t *testing.T) {
	tests := map[string]string{
		singbox.OutboundBlock: "REJECT",
		"direct":              "DIRECT",
		"proxy":               "proxy",
	}
	for in, want := range tests {
		if got := clashOutbound(in); got != want {
			t.Errorf("clashOutbound(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRenderClashFallbackMatch 无规则时仍渲染 MATCH 兜底行。
func TestRenderClashFallbackMatch(t *testing.T) {
	specs := []NodeSpec{nodeSpec(t, "n1", singbox.ProtoVLESS, 20001)}
	d, err := Assemble("example.com", specs, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	raw, err := RenderClash(d)
	if err != nil {
		t.Fatalf("渲染 clash 配置失败: %v", err)
	}
	doc := parseClash(t, raw)
	if len(doc.Rules) != 1 || doc.Rules[0] != "MATCH,"+FallbackGroup {
		t.Errorf("无规则时应仅有 MATCH 兜底行: %v", doc.Rules)
	}
}

// TestRenderClashMoreProtocols 覆盖 TUIC / AnyTLS / VMess 与非 Reality TLS VLESS 的 Clash 渲染。
func TestRenderClashMoreProtocols(t *testing.T) {
	vlessTLS, _ := json.Marshal(&singbox.VLESSConfig{
		Flow: "xtls-rprx-vision",
		TLS:  singbox.TLSConfig{Enabled: true, ServerName: "vless.example.com", ALPN: []string{"h2", "http/1.1"}},
	})
	vlessCred, _ := json.Marshal(singbox.Credential{UUID: "11111111-1111-1111-1111-111111111111"})

	vmessCfg, _ := json.Marshal(&singbox.VMessConfig{
		WSPath: "/ws",
		TLS:    singbox.TLSConfig{Enabled: true, ServerName: "vmess.example.com", ALPN: []string{"http/1.1"}},
	})
	vmessCred, _ := json.Marshal(singbox.Credential{UUID: "22222222-2222-2222-2222-222222222222"})

	specs := []NodeSpec{
		nodeSpec(t, "n-tuic", singbox.ProtoTUIC, 20003),
		nodeSpec(t, "n-anytls", singbox.ProtoAnyTLS, 20004),
		{Tag: "n-vmess", Protocol: singbox.ProtoVMess, Port: 20005, Config: string(vmessCfg), CredJSON: string(vmessCred)},
		{Tag: "n-vless-tls", Protocol: singbox.ProtoVLESS, Port: 20006, Config: string(vlessTLS), CredJSON: string(vlessCred)},
	}
	d, err := Assemble("example.com", specs, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("组装订阅失败: %v", err)
	}
	raw, err := RenderClash(d)
	if err != nil {
		t.Fatalf("渲染 clash 失败: %v", err)
	}
	doc := parseClash(t, raw)

	tuic := findProxy(t, doc, "n-tuic")
	if tuic["type"] != "tuic" || tuic["uuid"] == "" || tuic["password"] == "" ||
		tuic["congestion-controller"] != singbox.CongestionBBR || tuic["sni"] != "example.com" {
		t.Errorf("TUIC proxy 字段不符: %v", tuic)
	}
	if alpn, _ := tuic["alpn"].([]any); len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("TUIC proxy ALPN 应含 h3: %v", tuic)
	}

	anytls := findProxy(t, doc, "n-anytls")
	if anytls["type"] != "anytls" || anytls["password"] == "" || anytls["sni"] != "example.com" {
		t.Errorf("AnyTLS proxy 字段不符: %v", anytls)
	}

	vmess := findProxy(t, doc, "n-vmess")
	if vmess["type"] != "vmess" || vmess["uuid"] == "" || vmess["network"] != "ws" ||
		vmess["tls"] != true || vmess["servername"] != "vmess.example.com" {
		t.Errorf("VMess proxy 字段不符: %v", vmess)
	}
	if alterID, ok := vmess["alterId"]; !ok || intOf(alterID) != 0 {
		t.Errorf("VMess 必须含 alterId: 0: %v", vmess)
	}
	if vmess["cipher"] != "auto" || vmess["udp"] != true {
		t.Errorf("VMess 应含 cipher=auto 与 udp=true: %v", vmess)
	}
	wsOpts, _ := vmess["ws-opts"].(map[string]any)
	if wsOpts == nil || wsOpts["path"] != "/ws" {
		t.Errorf("VMess ws-opts 不符: %v", vmess)
	}

	vless := findProxy(t, doc, "n-vless-tls")
	if vless["tls"] != true || vless["servername"] != "vless.example.com" || vless["udp"] != true {
		t.Errorf("非 Reality TLS VLESS 字段不符: %v", vless)
	}
	if _, ok := vless["reality-opts"]; ok {
		t.Errorf("非 Reality VLESS 不应带 reality-opts: %v", vless)
	}
	if _, ok := vless["client-fingerprint"]; ok {
		t.Errorf("非 Reality VLESS 不应带 client-fingerprint: %v", vless)
	}
}

// TestRenderClashSkipCertVerifyAndUDP 自签证书写入 skip-cert-verify（Reality 不写），仅 TCP SS 省略 udp。
func TestRenderClashSkipCertVerifyAndUDP(t *testing.T) {
	vlessTLS, _ := json.Marshal(&singbox.VLESSConfig{
		Flow: "xtls-rprx-vision",
		TLS:  singbox.TLSConfig{Enabled: true, ServerName: "vless.example.com"},
	})
	vlessCred, _ := json.Marshal(singbox.Credential{UUID: "11111111-1111-1111-1111-111111111111"})
	ssTCPCfg, _ := json.Marshal(&singbox.ShadowsocksConfig{Method: "2022-blake3-aes-128-gcm", PSK: "AAAAAAAAAAAAAAAAAAAAAA==", Network: "tcp"})
	ssCred, _ := json.Marshal(singbox.Credential{Password: "BBBBBBBBBBBBBBBBBBBBBB=="})

	realitySpec := nodeSpec(t, "n-reality", singbox.ProtoVLESS, 20001)
	realitySpec.Insecure = true // Reality 即使标了也不应输出 skip-cert-verify
	hy2Spec := nodeSpec(t, "n-hy2", singbox.ProtoHysteria2, 20002)
	hy2Spec.Insecure = true
	tuicSpec := nodeSpec(t, "n-tuic", singbox.ProtoTUIC, 20003)
	tuicSpec.Insecure = true
	anytlsSpec := nodeSpec(t, "n-anytls", singbox.ProtoAnyTLS, 20004)
	anytlsSpec.Insecure = true
	trojanSpec := nodeSpec(t, "n-trojan", singbox.ProtoTrojan, 20005)
	trojanSpec.Insecure = true
	vlessTLSSpec := NodeSpec{Tag: "n-vless-tls", Protocol: singbox.ProtoVLESS, Port: 20006, Config: string(vlessTLS), CredJSON: string(vlessCred), Insecure: true}
	ssTCPSpec := NodeSpec{Tag: "n-ss-tcp", Protocol: singbox.ProtoShadowsocks, Port: 20007, Config: string(ssTCPCfg), CredJSON: string(ssCred)}

	d, err := Assemble("example.com", []NodeSpec{realitySpec, hy2Spec, tuicSpec, anytlsSpec, trojanSpec, vlessTLSSpec, ssTCPSpec}, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	raw, err := RenderClash(d)
	if err != nil {
		t.Fatalf("渲染 clash 失败: %v", err)
	}
	doc := parseClash(t, raw)

	reality := findProxy(t, doc, "n-reality")
	if _, ok := reality["skip-cert-verify"]; ok {
		t.Errorf("Reality 节点不应写入 skip-cert-verify: %v", reality)
	}
	for _, tag := range []string{"n-hy2", "n-tuic", "n-anytls", "n-trojan", "n-vless-tls"} {
		p := findProxy(t, doc, tag)
		if p["skip-cert-verify"] != true {
			t.Errorf("%s 自签节点应写入 skip-cert-verify: true: %v", tag, p)
		}
		if p["udp"] != true {
			t.Errorf("%s 节点应写入 udp: true: %v", tag, p)
		}
	}
	ss := findProxy(t, doc, "n-ss-tcp")
	if _, ok := ss["udp"]; ok {
		t.Errorf("仅 TCP 的 SS 节点不应写入 udp: true: %v", ss)
	}
}

// TestClashDNSServerIPv6 IPv6 DNS 服务器地址在带端口与默认端口下均格式合法。
func TestClashDNSServerIPv6(t *testing.T) {
	tests := []struct {
		name string
		srv  singbox.DNSServer
		want string
	}{
		{"UDP IPv6 默认端口", singbox.DNSServer{Type: singbox.DNSTypeUDP, Server: "2606:4700:4700::1111"}, "2606:4700:4700::1111"},
		{"UDP IPv6 自定义端口", singbox.DNSServer{Type: singbox.DNSTypeUDP, Server: "2606:4700:4700::1111", ServerPort: 5353}, "[2606:4700:4700::1111]:5353"},
		{"TCP IPv6 默认端口", singbox.DNSServer{Type: singbox.DNSTypeTCP, Server: "2606:4700:4700::1111"}, "tcp://[2606:4700:4700::1111]:53"},
		{"TLS IPv6 默认端口", singbox.DNSServer{Type: singbox.DNSTypeTLS, Server: "2606:4700:4700::1111"}, "tls://[2606:4700:4700::1111]:853"},
		{"HTTPS IPv6 默认端口", singbox.DNSServer{Type: singbox.DNSTypeHTTPS, Server: "2606:4700:4700::1111"}, "https://[2606:4700:4700::1111]/dns-query"},
		{"HTTPS IPv6 自定义端口", singbox.DNSServer{Type: singbox.DNSTypeHTTPS, Server: "2606:4700:4700::1111", ServerPort: 8443, Path: "/q"}, "https://[2606:4700:4700::1111]:8443/q"},
		{"H3 IPv6 默认端口", singbox.DNSServer{Type: singbox.DNSTypeH3, Server: "2606:4700:4700::1111"}, "h3://[2606:4700:4700::1111]/dns-query"},
		{"H3 IPv6 已带方括号", singbox.DNSServer{Type: singbox.DNSTypeH3, Server: "[::1]"}, "h3://[::1]/dns-query"},
		{"H3 IPv6 自定义端口", singbox.DNSServer{Type: singbox.DNSTypeH3, Server: "2606:4700:4700::1111", ServerPort: 8443}, "h3://[2606:4700:4700::1111]:8443/dns-query"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clashDNSServer(tt.srv); got != tt.want {
				t.Errorf("clashDNSServer(%+v) = %q, want %q", tt.srv, got, tt.want)
			}
		})
	}
}
