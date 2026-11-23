package subscription

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"

	"proxy-ui/internal/singbox"
)

// mihomo 规则集缓存目录（客户端本地相对路径）。
const mihomoRuleSetDir = "./ruleset/"

// RenderClash 渲染 mihomo(clash.meta) 客户端配置（YAML）：proxies、proxy-groups、rules、
// rule-providers 与 dns。
func RenderClash(d Data) ([]byte, error) {
	cfg := clashConfig{}
	proxyGroup := primaryProxyGroup(d)
	for _, n := range d.Nodes {
		p, err := clashProxyOf(n, d.Server)
		if err != nil {
			return nil, err
		}
		cfg.Proxies = append(cfg.Proxies, p)
	}
	for _, g := range d.Groups {
		cfg.ProxyGroups = append(cfg.ProxyGroups, clashGroupOf(g))
	}
	providers := map[string]singbox.RuleSetSource{}
	var providerOrder []string
	for _, r := range d.Rules {
		lines, provider := clashRulesOf(r)
		if provider != nil {
			if _, seen := providers[provider.Tag]; !seen {
				providerOrder = append(providerOrder, provider.Tag)
			}
			providers[provider.Tag] = *provider
		}
		cfg.Rules = append(cfg.Rules, lines...)
	}
	cfg.Rules = append(cfg.Rules, "MATCH,"+clashOutbound(d.Final))
	// rule-providers 必须是映射（tag -> 定义）；用 MapSlice 并保持规则出现顺序
	for _, tag := range providerOrder {
		src := providers[tag]
		entry := clashRuleProvider{
			Type: "http", Behavior: src.MihomoBehavior, Format: clashProviderFormat(src.MihomoURL),
			URL: src.MihomoURL, Path: mihomoRuleSetDir + tag + clashProviderExt(src.MihomoURL), Interval: 86400,
			Proxy: proxyGroup,
		}
		if entry.Behavior == "" {
			entry.Behavior = "domain"
		}
		cfg.RuleProviders = append(cfg.RuleProviders, yaml.MapItem{Key: tag, Value: entry})
	}
	if dns := clashDNSOf(d.DNS, proxyGroup); dns != nil {
		cfg.DNS = dns
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("序列化 clash 配置失败: %w", err)
	}
	return out, nil
}

// clashProviderFormat 判断规则集源的格式（.mrs 为 mrs，其余按 yaml）。
func clashProviderFormat(url string) string {
	if strings.HasSuffix(strings.ToLower(url), ".mrs") {
		return "mrs"
	}
	return "yaml"
}

// clashProviderExt 规则集源缓存文件扩展名。
func clashProviderExt(url string) string {
	if strings.HasSuffix(strings.ToLower(url), ".mrs") {
		return ".mrs"
	}
	return ".yaml"
}

// clashConfig mihomo 配置结构（字段顺序即输出顺序）。
type clashConfig struct {
	Proxies       []clashProxy      `yaml:"proxies"`
	ProxyGroups   []clashProxyGroup `yaml:"proxy-groups"`
	RuleProviders yaml.MapSlice     `yaml:"rule-providers,omitempty"`
	Rules         []string          `yaml:"rules"`
	DNS           *clashDNS         `yaml:"dns,omitempty"`
}

// clashRuleProvider 远程规则源定义。
type clashRuleProvider struct {
	Type     string `yaml:"type"`            // http 远程规则源
	Behavior string `yaml:"behavior"`        // 匹配行为：domain / ipcidr / classical
	Format   string `yaml:"format"`          // 源格式：yaml / mrs
	URL      string `yaml:"url"`             // 规则源地址
	Path     string `yaml:"path"`            // 本地缓存路径
	Interval int    `yaml:"interval"`        // 更新间隔（秒）
	Proxy    string `yaml:"proxy,omitempty"` // 经由代理组下载（避免直连 GitHub 失败）
}

// clashDefaultFakeIPFilter fake-ip 模式下必须回落真实 IP 的本地、校时与系统探测域名。
var clashDefaultFakeIPFilter = []string{
	"*.lan",
	"*.local",
	"*.localhost",
	"*.arpa",
	"time.*.com",
	"ntp.*.com",
	"+.pool.ntp.org",
	"localhost.ptlogin2.qq.com",
	"*.msftconnecttest.com",
	"*.msftncsi.com",
}

// clashDNS mihomo dns 配置。
type clashDNS struct {
	Enable             bool          `yaml:"enable"`
	EnhancedMode       string        `yaml:"enhanced-mode"`
	FakeIPRange        string        `yaml:"fake-ip-range"`
	FakeIPFilter       []string      `yaml:"fake-ip-filter"`
	DefaultNameser     []string      `yaml:"default-nameserver"`
	ProxyServerNameser []string      `yaml:"proxy-server-nameserver"`
	DirectNameser      []string      `yaml:"direct-nameserver"`
	Nameserver         []string      `yaml:"nameserver"`
	NameserverPol      yaml.MapSlice `yaml:"nameserver-policy,omitempty"`
}

// clashDNSOf 把面板 DNS 配置映射为 mihomo dns 段；未启用返回 nil。
func clashDNSOf(d *singbox.DNSConfig, proxyGroup string) *clashDNS {
	if d == nil || !d.Enabled || len(d.Servers) == 0 {
		return nil
	}
	byTag := make(map[string]singbox.DNSServer, len(d.Servers))
	ruleTargets := make(map[string]bool, len(d.Rules))
	for _, r := range d.Rules {
		if r.Server != "" {
			ruleTargets[r.Server] = true
		}
	}
	out := &clashDNS{
		Enable:       true,
		EnhancedMode: "fake-ip",
		FakeIPRange:  "198.18.0.1/16",
		FakeIPFilter: append([]string(nil), clashDefaultFakeIPFilter...),
	}
	// default-nameserver 必须是 IP 型明文解析器，用于解析 DoH 域名本身
	for _, s := range d.Servers {
		byTag[s.Tag] = s
		if (s.Type == singbox.DNSTypeUDP || s.Type == singbox.DNSTypeTCP) &&
			net.ParseIP(strings.Trim(s.Server, "[]")) != nil {
			out.DefaultNameser = append(out.DefaultNameser, clashDNSServer(s))
		}
	}
	if len(out.DefaultNameser) == 0 {
		out.DefaultNameser = []string{"223.5.5.5", "119.29.29.29"}
	}
	// 代理服务器域名与直连出站域名同用直连明文 DNS，防止解析节点地址死锁与国内流量误用远程 DNS
	out.ProxyServerNameser = append([]string(nil), out.DefaultNameser...)
	out.DirectNameser = append([]string(nil), out.DefaultNameser...)

	if d.Final != "" {
		if srv, ok := byTag[d.Final]; ok {
			out.Nameserver = []string{clashDNSServerAddr(srv, proxyGroup, true)}
		}
	}
	if len(out.Nameserver) == 0 {
		for _, s := range d.Servers {
			out.Nameserver = append(out.Nameserver, clashDNSServerAddr(s, proxyGroup, !ruleTargets[s.Tag]))
		}
	}
	// nameserver-policy 必须是 YAML mapping（域名模式 -> 服务器列表），保持先到优先并去重
	seenPolicy := make(map[string]bool)
	for _, r := range d.Rules {
		srv, ok := byTag[r.Server]
		if !ok {
			continue
		}
		serverAddr := clashDNSServerAddr(srv, proxyGroup, false)
		for _, domain := range clashDomainPatterns(r) {
			if seenPolicy[domain] {
				continue
			}
			seenPolicy[domain] = true
			out.NameserverPol = append(out.NameserverPol, yaml.MapItem{Key: domain, Value: []string{serverAddr}})
		}
	}
	return out
}

// clashDNSServerAddr 生成带出站路由标记的 mihomo DNS 服务器地址串；
// 显式指定 Detour 时附带 #detour，否则远程加密 DNS（DoH/DoH3/DoT）按需经代理组转发。
func clashDNSServerAddr(s singbox.DNSServer, proxyGroup string, routeRemoteViaProxy bool) string {
	base := clashDNSServer(s)
	if s.Type == singbox.DNSTypeLocal {
		return base
	}
	if s.Detour != "" {
		detour := clashOutbound(s.Detour)
		if !strings.ContainsAny(detour, "#&") {
			return base + "#" + detour
		}
		return base
	}
	if routeRemoteViaProxy && proxyGroup != "" && !strings.ContainsAny(proxyGroup, "#&") &&
		(s.Type == singbox.DNSTypeHTTPS || s.Type == singbox.DNSTypeH3 || s.Type == singbox.DNSTypeTLS) {
		return base + "#" + proxyGroup
	}
	return base
}

// clashDNSServer 把单个 DNS 服务器映射为 mihomo 的服务器地址串。
func clashDNSServer(s singbox.DNSServer) string {
	port := s.ServerPort
	host := strings.Trim(s.Server, "[]")
	switch s.Type {
	case singbox.DNSTypeLocal:
		return "system"
	case singbox.DNSTypeUDP:
		if port == 0 || port == 53 {
			return s.Server
		}
		return net.JoinHostPort(host, strconv.Itoa(port))
	case singbox.DNSTypeTCP:
		if port == 0 {
			port = 53
		}
		return "tcp://" + net.JoinHostPort(host, strconv.Itoa(port))
	case singbox.DNSTypeTLS:
		if port == 0 {
			port = 853
		}
		return "tls://" + net.JoinHostPort(host, strconv.Itoa(port))
	case singbox.DNSTypeHTTPS, singbox.DNSTypeH3:
		path := s.Path
		if path == "" {
			path = "/dns-query"
		}
		if port == 0 || port == 443 {
			addr := s.Server
			if strings.Contains(s.Server, ":") {
				addr = "[" + host + "]"
			}
			return fmt.Sprintf("%s://%s%s", s.Type, addr, path)
		}
		return fmt.Sprintf("%s://%s%s", s.Type, net.JoinHostPort(host, strconv.Itoa(port)), path)
	}
	return s.Server
}

// clashDomainPatterns 把单条 DNS 规则展开为 mihomo 域名匹配串。
func clashDomainPatterns(r singbox.DNSRule) []string {
	var out []string
	for v := range strings.SplitSeq(r.Domain, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	for v := range strings.SplitSeq(r.DomainSuffix, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, "+."+strings.TrimPrefix(v, "."))
		}
	}
	for v := range strings.SplitSeq(r.DomainKeyword, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, "+."+v)
		}
	}
	return out
}

// clashProxyGroup 策略组：fixed→select、auto→url-test、failover→fallback。
type clashProxyGroup struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Proxies  []string `yaml:"proxies"`
	URL      string   `yaml:"url,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
}

// clashProxy 节点代理（协议特定字段 omitempty，按协议取用）。
type clashProxy struct {
	Name              string            `yaml:"name"`
	Type              string            `yaml:"type"`
	Server            string            `yaml:"server"`
	Port              int               `yaml:"port"`
	UUID              string            `yaml:"uuid,omitempty"`
	AlterID           *int              `yaml:"alterId,omitempty"`
	Password          string            `yaml:"password,omitempty"`
	Flow              string            `yaml:"flow,omitempty"`
	UDP               bool              `yaml:"udp,omitempty"`
	TLS               bool              `yaml:"tls,omitempty"`
	SkipCertVerify    bool              `yaml:"skip-cert-verify,omitempty"`
	ServerName        string            `yaml:"servername,omitempty"`
	SNI               string            `yaml:"sni,omitempty"`
	ALPN              []string          `yaml:"alpn,omitempty"`
	Obfs              string            `yaml:"obfs,omitempty"`
	ObfsPassword      string            `yaml:"obfs-password,omitempty"`
	Up                string            `yaml:"up,omitempty"`
	Down              string            `yaml:"down,omitempty"`
	CongestionControl string            `yaml:"congestion-controller,omitempty"`
	Cipher            string            `yaml:"cipher,omitempty"`
	Network           string            `yaml:"network,omitempty"`
	WSOpts            *clashWSOpts      `yaml:"ws-opts,omitempty"`
	RealityOpts       *clashRealityOpts `yaml:"reality-opts,omitempty"`
	Fingerprint       string            `yaml:"client-fingerprint,omitempty"`
}

// clashWSOpts WebSocket 传输参数。
type clashWSOpts struct {
	Path string `yaml:"path"`
}

// clashRealityOpts Reality 参数。
type clashRealityOpts struct {
	PublicKey string `yaml:"public-key"`
	ShortID   string `yaml:"short-id,omitempty"`
}

// clashGroupOf 把策略组映射为 clash proxy-group；direct 成员转 DIRECT。
func clashGroupOf(g singbox.GroupEntry) clashProxyGroup {
	group := clashProxyGroup{Name: g.Name, Proxies: make([]string, 0, len(g.Members))}
	for _, m := range g.Members {
		if m == "direct" {
			group.Proxies = append(group.Proxies, "DIRECT")
		} else {
			group.Proxies = append(group.Proxies, m)
		}
	}
	switch g.Mode {
	case singbox.GroupModeAuto:
		group.Type = "url-test"
	case singbox.GroupModeFailover:
		group.Type = "fallback"
	default:
		group.Type = "select"
	}
	if group.Type != "select" {
		group.URL = "https://www.gstatic.com/generate_204"
		group.Interval = 300
	}
	return group
}

// clashProxyOf 把订阅节点映射为 clash proxy。
func clashProxyOf(n Node, server string) (clashProxy, error) {
	p := clashProxy{Name: n.Tag, Server: server, Port: n.Port, UDP: true}
	sni := func(t singbox.TLSConfig) string {
		if t.ServerName != "" {
			return t.ServerName
		}
		return server
	}
	switch c := n.Config.(type) {
	case *singbox.VLESSConfig:
		p.Type = "vless"
		p.UUID = n.Cred.UUID
		p.Flow = n.Cred.Flow
		if p.Flow == "" {
			p.Flow = c.Flow
		}
		if c.Reality.Enabled {
			p.TLS = true
			p.ServerName = c.RealitySNI()
			p.RealityOpts = &clashRealityOpts{PublicKey: c.Reality.PublicKey, ShortID: c.Reality.ShortID}
			p.Fingerprint = "chrome" // mihomo Reality 必须指定客户端指纹
		} else if c.TLS.Enabled {
			p.TLS = true
			p.ServerName = sni(c.TLS)
			p.ALPN = c.TLS.ALPN
			p.SkipCertVerify = n.Insecure
		}
	case *singbox.Hysteria2Config:
		p.Type = "hysteria2"
		p.Password = n.Cred.Password
		if c.ObfsEnabled && c.ObfsPassword != "" {
			p.Obfs = "salamander"
			p.ObfsPassword = c.ObfsPassword
		}
		if c.UpMbps > 0 {
			p.Up = fmt.Sprintf("%d Mbps", c.UpMbps)
		}
		if c.DownMbps > 0 {
			p.Down = fmt.Sprintf("%d Mbps", c.DownMbps)
		}
		p.SNI = sni(c.TLS)
		if len(c.TLS.ALPN) > 0 {
			p.ALPN = c.TLS.ALPN
		} else {
			p.ALPN = []string{"h3"}
		}
		p.SkipCertVerify = n.Insecure
	case *singbox.TUICConfig:
		p.Type = "tuic"
		p.UUID = n.Cred.UUID
		p.Password = n.Cred.Password
		p.CongestionControl = c.CongestionControl
		p.SNI = sni(c.TLS)
		if len(c.TLS.ALPN) > 0 {
			p.ALPN = c.TLS.ALPN
		} else {
			p.ALPN = []string{"h3"}
		}
		p.SkipCertVerify = n.Insecure
	case *singbox.AnyTLSConfig:
		p.Type = "anytls"
		p.Password = n.Cred.Password
		p.SNI = sni(c.TLS)
		p.ALPN = c.TLS.ALPN
		p.SkipCertVerify = n.Insecure
	case *singbox.TrojanConfig:
		p.Type = "trojan"
		p.Password = n.Cred.Password
		p.SNI = sni(c.TLS)
		p.ALPN = c.TLS.ALPN
		p.SkipCertVerify = n.Insecure
	case *singbox.VMessConfig:
		p.Type = "vmess"
		p.UUID = n.Cred.UUID
		p.AlterID = new(0)
		p.Cipher = "auto"
		p.Network = "ws"
		path := c.WSPath
		if path == "" {
			path = "/"
		}
		p.WSOpts = &clashWSOpts{Path: path}
		if c.TLS.Enabled {
			p.TLS = true
			p.ServerName = sni(c.TLS)
			p.ALPN = c.TLS.ALPN
			p.SkipCertVerify = n.Insecure
		}
	case *singbox.ShadowsocksConfig:
		p.Type = "ss"
		if c.Network == "tcp" {
			p.UDP = false
		}
		p.Cipher = c.Method
		if p.Cipher == "" {
			p.Cipher = "2022-blake3-aes-128-gcm"
		}
		// Shadowsocks-2022 多用户：mihomo 同样要求「入站PSK:用户密钥」
		if _, is2022 := singbox.SSKeyLength(p.Cipher); is2022 {
			p.Password = c.PSK + ":" + n.Cred.Password
			break
		}
		p.Password = n.Cred.Password
	default:
		return clashProxy{}, fmt.Errorf("节点 %s: 配置类型不符: %T", n.Tag, n.Config)
	}
	return p, nil
}

// clashRulesOf 把路由规则映射为 clash 规则行；引用规则集时返回其下载源。
func clashRulesOf(r singbox.RuleEntry) ([]string, *singbox.RuleSetSource) {
	outbound := clashOutbound(r.Outbound)
	if r.MatchType == singbox.MatchRuleSet {
		if r.RuleSet == nil {
			return nil, nil
		}
		// ipcidr 规则集不加 no-resolve：fake-ip 下只有域名，需解析出真实 IP 才能命中（如国内 IP 直连）
		return []string{"RULE-SET," + r.RuleSet.Tag + "," + outbound}, r.RuleSet
	}
	var lines []string
	for _, v := range singbox.SplitRuleValue(r.Value) {
		line := ""
		switch r.MatchType {
		case singbox.MatchDomain:
			switch r.MatchMethod {
			case singbox.MethodExact:
				line = "DOMAIN," + v + "," + outbound
			case singbox.MethodSuffix:
				line = "DOMAIN-SUFFIX," + v + "," + outbound
			case singbox.MethodKeyword:
				line = "DOMAIN-KEYWORD," + v + "," + outbound
			case singbox.MethodRegex:
				line = "DOMAIN-REGEX," + v + "," + outbound
			}
		case singbox.MatchIP:
			line = "IP-CIDR," + v + "," + outbound + ",no-resolve"
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

// clashOutbound 出站 tag 映射：block→REJECT、direct→DIRECT、组名原样。
func clashOutbound(tag string) string {
	switch tag {
	case singbox.OutboundBlock:
		return "REJECT"
	case "direct":
		return "DIRECT"
	default:
		return tag
	}
}
