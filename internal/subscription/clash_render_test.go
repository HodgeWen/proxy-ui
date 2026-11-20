package subscription

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

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
	} `yaml:"rule-providers"`
	Rules []string `yaml:"rules"`
	DNS   struct {
		Enable        bool     `yaml:"enable"`
		Nameserver    []string `yaml:"nameserver"`
		DefaultNS     []string `yaml:"default-nameserver"`
		NameserverPol []any    `yaml:"nameserver-policy"`
		ProxyServerNS []string `yaml:"proxy-server-nameserver"`
		EnhancedMode  string   `yaml:"enhanced-mode"`
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

	// rules：各匹配方式映射正确，最后补 MATCH
	joined := strings.Join(doc.Rules, "\n")
	for _, want := range []string{
		"RULE-SET,anti-ad,REJECT",
		"RULE-SET,geoip-cn,DIRECT",
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
	if len(doc.DNS.Nameserver) == 0 {
		t.Error("dns.nameserver 不应为空")
	}
	if len(doc.DNS.DefaultNS) == 0 {
		t.Error("dns.default-nameserver 不应为空（需明文 IP 解析器）")
	}
	if len(doc.DNS.NameserverPol) == 0 {
		t.Error("dns.nameserver-policy 不应为空（按域名分流）")
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
	out := clashDNSOf(cfg)
	if out == nil {
		t.Fatal("启用时不应返回 nil")
	}
	if !out.Enable {
		t.Error("dns.enable 应为 true")
	}
	// final 指定时 nameserver 只保留该服务器
	if len(out.Nameserver) != 1 || out.Nameserver[0] != "https://1.1.1.1/dns-query" {
		t.Errorf("final 指定时 nameserver 应只含该服务器: %v", out.Nameserver)
	}
	// default-nameserver 用明文 UDP 服务器
	if len(out.DefaultNameser) != 2 || out.DefaultNameser[0] != "223.5.5.5" || out.DefaultNameser[1] != "8.8.8.8:5353" {
		t.Errorf("default-nameserver 应取明文 UDP 服务器: %v", out.DefaultNameser)
	}
	// 规则展开为 mihomo 域名匹配串
	if len(out.NameserverPol) != 1 || out.NameserverPol[0].Domain != ".cn" {
		t.Errorf("nameserver-policy 域名映射不符: %+v", out.NameserverPol)
	}
	if out.NameserverPol[0].Server[0] != "223.5.5.5" {
		t.Errorf("nameserver-policy 服务器不符: %+v", out.NameserverPol[0])
	}

	// 无明文 UDP 服务器时 default-nameserver 回落到公共 DNS
	onlyDoH := &singbox.DNSConfig{
		Enabled: true,
		Servers: []singbox.DNSServer{{Tag: "doh", Type: singbox.DNSTypeHTTPS, Server: "1.1.1.1"}},
	}
	fallback := clashDNSOf(onlyDoH)
	if len(fallback.DefaultNameser) == 0 {
		t.Error("无明文服务器时 default-nameserver 应给出兜底值")
	}

	// 未启用返回 nil
	if clashDNSOf(nil) != nil || clashDNSOf(&singbox.DNSConfig{Enabled: false}) != nil {
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
