package subscription

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"

	"proxy-ui/internal/singbox"
)

// DefaultAntiADClashURL anti-AD 官方 clash 规则源（与 sing-box 源同站不同格式）。
const DefaultAntiADClashURL = "https://anti-ad.net/anti-ad-for-clash.yaml"

// RenderClash 渲染 clash(mihomo) 客户端配置（YAML）：proxies、proxy-groups、rules
// 与 anti-AD rule-providers。
func RenderClash(d Data) ([]byte, error) {
	cfg := clashConfig{}
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
	ruleProviders := map[string]bool{}
	for _, r := range d.Rules {
		lines, provider := clashRulesOf(r)
		for _, tag := range provider {
			ruleProviders[tag] = true
		}
		cfg.Rules = append(cfg.Rules, lines...)
	}
	cfg.Rules = append(cfg.Rules, "MATCH,"+clashOutbound(d.Final))
	for tag := range ruleProviders {
		// 面板自定义的 .srs 规则源无 clash 等价格式，clash 端一律用 anti-AD 官方 clash 源
		cfg.RuleProviders = append(cfg.RuleProviders, yaml.MapItem{
			Key: tag,
			Value: clashRuleProvider{
				Type: "http", Behavior: "domain", Format: "yaml",
				URL: DefaultAntiADClashURL, Path: "./ruleset/" + tag + ".yaml", Interval: 86400,
			},
		})
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("序列化 clash 配置失败: %w", err)
	}
	return out, nil
}

// clashConfig clash(mihomo) 配置结构（字段顺序即输出顺序）。
type clashConfig struct {
	Proxies       []clashProxy      `yaml:"proxies"`
	ProxyGroups   []clashProxyGroup `yaml:"proxy-groups"`
	Rules         []string          `yaml:"rules"`
	RuleProviders yaml.MapSlice     `yaml:"rule-providers,omitempty"`
}

// clashRuleProvider anti-AD 等 http 规则源定义。
type clashRuleProvider struct {
	Type     string `yaml:"type"`     // http 远程规则源
	Behavior string `yaml:"behavior"` // 匹配行为：domain
	Format   string `yaml:"format"`   // 源格式：yaml
	URL      string `yaml:"url"`      // 规则源地址
	Path     string `yaml:"path"`     // 本地缓存路径
	Interval int    `yaml:"interval"` // 更新间隔（秒）
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
	Password          string            `yaml:"password,omitempty"`
	Flow              string            `yaml:"flow,omitempty"`
	TLS               bool              `yaml:"tls,omitempty"`
	ServerName        string            `yaml:"servername,omitempty"`
	SNI               string            `yaml:"sni,omitempty"`
	ALPN              []string          `yaml:"alpn,omitempty"`
	Obfs              string            `yaml:"obfs,omitempty"`
	ObfsPassword      string            `yaml:"obfs-password,omitempty"`
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
	p := clashProxy{Name: n.Tag, Server: server, Port: n.Port}
	sni := func(t singbox.TLSConfig) string {
		if t.ServerName != "" {
			return t.ServerName
		}
		return server
	}
	switch c := n.Config.(type) {
	case *singbox.VLESSConfig:
		p.Type = "vless"
		p.UUID = c.UUID
		p.Flow = c.Flow
		if c.Reality.Enabled {
			p.TLS = true
			p.ServerName = c.Reality.HandshakeServer
			if p.ServerName == "" {
				p.ServerName = "www.microsoft.com"
			}
			p.RealityOpts = &clashRealityOpts{PublicKey: c.Reality.PublicKey, ShortID: c.Reality.ShortID}
			p.Fingerprint = "chrome" // mihomo Reality 必须指定客户端指纹
		} else if c.TLS.Enabled {
			p.TLS = true
			p.ServerName = sni(c.TLS)
			p.ALPN = c.TLS.ALPN
		}
	case *singbox.Hysteria2Config:
		p.Type = "hysteria2"
		p.Password = c.Password
		if c.ObfsPassword != "" {
			p.Obfs = "salamander"
			p.ObfsPassword = c.ObfsPassword
		}
		p.SNI = sni(c.TLS)
		if len(c.TLS.ALPN) > 0 {
			p.ALPN = c.TLS.ALPN
		} else {
			p.ALPN = []string{"h3"}
		}
	case *singbox.TUICConfig:
		p.Type = "tuic"
		p.UUID = c.UUID
		p.Password = c.Password
		p.CongestionControl = c.CongestionControl
		p.SNI = sni(c.TLS)
		if len(c.TLS.ALPN) > 0 {
			p.ALPN = c.TLS.ALPN
		} else {
			p.ALPN = []string{"h3"}
		}
	case *singbox.AnyTLSConfig:
		p.Type = "anytls"
		p.Password = c.Password
		p.SNI = sni(c.TLS)
	case *singbox.TrojanConfig:
		p.Type = "trojan"
		p.Password = c.Password
		p.SNI = sni(c.TLS)
		p.ALPN = c.TLS.ALPN
	case *singbox.VMessConfig:
		p.Type = "vmess"
		p.UUID = c.UUID
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
		}
	case *singbox.ShadowsocksConfig:
		p.Type = "ss"
		p.Cipher = c.Method
		if p.Cipher == "" {
			p.Cipher = "2022-blake3-aes-128-gcm"
		}
		p.Password = c.Password
	default:
		return clashProxy{}, fmt.Errorf("节点 %s: 配置类型不符: %T", n.Tag, n.Config)
	}
	return p, nil
}

// clashRulesOf 把路由规则映射为 clash 规则行，返回引用的 rule-provider tag。
func clashRulesOf(r singbox.RuleEntry) ([]string, []string) {
	outbound := clashOutbound(r.Outbound)
	var providers []string
	var lines []string
	for _, v := range splitRuleValues(r.Value) {
		line := ""
		switch r.Type {
		case singbox.RuleTypeDomain:
			line = "DOMAIN," + v + "," + outbound
		case singbox.RuleTypeDomainSuffix:
			line = "DOMAIN-SUFFIX," + v + "," + outbound
		case singbox.RuleTypeDomainKeyword:
			line = "DOMAIN-KEYWORD," + v + "," + outbound
		case singbox.RuleTypeIPCIDR:
			line = "IP-CIDR," + v + "," + outbound
		case singbox.RuleTypeGeosite:
			line = "GEOSITE," + v + "," + outbound
		case singbox.RuleTypeGeoIP:
			line = "GEOIP," + v + "," + outbound
		case singbox.RuleTypeRuleSet:
			if v == singbox.AntiADTag {
				providers = append(providers, v)
				line = "RULE-SET," + v + "," + outbound
			}
			// 其余任意 rule_set tag 无 clash 等价物（面板仅支持 anti-ad/geosite/geoip），跳过
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, providers
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

// splitRuleValues 拆分逗号分隔的规则值并去空白。
func splitRuleValues(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			items = append(items, v)
		}
	}
	return items
}
