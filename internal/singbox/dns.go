package singbox

// DNS 配置：面板侧（服务端 route/dns 段）与客户端订阅共用同一份结构。
//
// 设计口径：只暴露真正影响分流效果的项——默认域名策略、几组 DNS 服务器与一组
// 按域名的分流规则；不堆砌内核全部 DNS 开关（缓存开关等内核默认已合理）。
type DNSConfig struct {
	Enabled  bool        `json:"enabled"`            // 是否启用自定义 DNS（关闭则不生成 dns/route.default_domain_resolver 段）
	Strategy string      `json:"strategy,omitempty"` // 默认域名解析策略：prefer_ipv4 / prefer_ipv6 / ipv4_only / ipv6_only
	Servers  []DNSServer `json:"servers"`            // DNS 服务器列表（首个为默认，除非 Final 指定）
	Rules    []DNSRule   `json:"rules"`              // 按域名分流的规则（自上而下匹配）
	Final    string      `json:"final,omitempty"`    // 默认 DNS 服务器 tag，空则取首个
}

// DNS 服务器类型。
const (
	DNSTypeUDP   = "udp"   // 明文 UDP
	DNSTypeTCP   = "tcp"   // 明文 TCP
	DNSTypeTLS   = "tls"   // DoT
	DNSTypeHTTPS = "https" // DoH
	DNSTypeH3    = "h3"    // DoH3
	DNSTypeLocal = "local" // 系统本地 DNS
)

// SupportedDNSTypes 支持的 DNS 服务器类型列表。
var SupportedDNSTypes = []string{DNSTypeUDP, DNSTypeTCP, DNSTypeTLS, DNSTypeHTTPS, DNSTypeH3, DNSTypeLocal}

// DNS 域名解析策略。
const (
	StrategyPreferIPv4 = "prefer_ipv4"
	StrategyPreferIPv6 = "prefer_ipv6"
	StrategyIPv4Only   = "ipv4_only"
	StrategyIPv6Only   = "ipv6_only"
)

// SupportedDNSStrategies 支持的域名解析策略列表。
var SupportedDNSStrategies = []string{StrategyPreferIPv4, StrategyPreferIPv6, StrategyIPv4Only, StrategyIPv6Only}

// DNSServer 单个 DNS 服务器定义。
type DNSServer struct {
	Tag        string `json:"tag"`                  // 服务器 tag（规则与 final 引用）
	Type       string `json:"type"`                 // 类型：udp/tcp/tls/https/h3/local
	Server     string `json:"server,omitempty"`     // 服务器地址（local 类型留空）
	ServerPort int    `json:"serverPort,omitempty"` // 端口，空则按类型默认（53/853/443）
	Path       string `json:"path,omitempty"`       // DoH 路径，默认 /dns-query
	Detour     string `json:"detour,omitempty"`     // 经由此出站发起查询（如走代理防污染），空为直连
}

// DNSRule 按域名把查询分流到指定 DNS 服务器。
type DNSRule struct {
	Domain        string `json:"domain,omitempty"`        // 精确域名（逗号分隔多值）
	DomainSuffix  string `json:"domainSuffix,omitempty"`  // 域名后缀（逗号分隔多值）
	DomainKeyword string `json:"domainKeyword,omitempty"` // 域名关键字（逗号分隔多值）
	Server        string `json:"server"`                  // 目标 DNS 服务器 tag
}

// DNS 预设 ID。
const (
	DNSPresetDirect    = "direct"     // 全部走本地/运营商 DNS（最简单，易被污染）
	DNSPresetAntiPoll  = "anti_poll"  // 国内域名走国内 DNS、其余走 DoH（防污染）
	DNSPresetRemoteDoH = "remote_doh" // 全部走 DoH（隐私优先）
)

// DNSPreset DNS 预设定义：一键填充 Servers/Rules/Final。
type DNSPreset struct {
	ID          string    // 预设 ID
	Name        string    // 名称
	Description string    // 说明
	Config      DNSConfig // 预设配置（Enabled 由调用方置位）
}

// DNSPresets 内置 DNS 预设列表。
var DNSPresets = []DNSPreset{
	{
		ID: DNSPresetDirect, Name: "本地 DNS（最简单）",
		Description: "全部查询走系统本地 DNS，延迟最低；国内网络可能被运营商劫持污染",
		Config: DNSConfig{
			Strategy: StrategyPreferIPv4,
			Servers:  []DNSServer{{Tag: "local", Type: DNSTypeLocal}},
			Final:    "local",
		},
	},
	{
		ID: DNSPresetAntiPoll, Name: "国内外分流（推荐）",
		Description: "国内域名走阿里 DNS、其余走 Cloudflare DoH，兼顾速度与防污染",
		Config: DNSConfig{
			Strategy: StrategyPreferIPv4,
			Servers: []DNSServer{
				{Tag: "ali", Type: DNSTypeUDP, Server: "223.5.5.5", ServerPort: 53},
				{Tag: "cf-doh", Type: DNSTypeHTTPS, Server: "1.1.1.1", ServerPort: 443, Path: "/dns-query"},
			},
			Rules: []DNSRule{
				{DomainSuffix: "cn", Server: "ali"},
				{DomainSuffix: "qq.com,baidu.com,taobao.com,jd.com,bilibili.com", Server: "ali"},
			},
			Final: "cf-doh",
		},
	},
	{
		ID: DNSPresetRemoteDoH, Name: "全部 DoH（隐私优先）",
		Description: "全部查询走 Cloudflare DoH，抗污染最好；首次解析延迟略高",
		Config: DNSConfig{
			Strategy: StrategyPreferIPv4,
			Servers:  []DNSServer{{Tag: "cf-doh", Type: DNSTypeHTTPS, Server: "1.1.1.1", ServerPort: 443, Path: "/dns-query"}},
			Final:    "cf-doh",
		},
	},
}

// DefaultDNSConfig 默认 DNS 配置（国内外分流预设）。
func DefaultDNSConfig() *DNSConfig {
	cfg := DNSPresets[1].Config
	cfg.Enabled = true
	return &cfg
}

// Validate 校验 DNS 配置：服务器 tag 唯一且非空、类型受支持、规则引用的服务器存在。
func (d *DNSConfig) Validate() error {
	if d == nil || !d.Enabled {
		return nil
	}
	if d.Strategy != "" && !contains(SupportedDNSStrategies, d.Strategy) {
		return &FieldError{Field: "dns.strategy", Msg: "不支持的域名解析策略: " + d.Strategy}
	}
	if len(d.Servers) == 0 {
		return &FieldError{Field: "dns.servers", Msg: "启用 DNS 时至少需要一个 DNS 服务器"}
	}
	tags := make(map[string]bool, len(d.Servers))
	for i, s := range d.Servers {
		if s.Tag == "" {
			return &FieldError{Field: "dns.servers", Msg: "DNS 服务器 tag 不能为空"}
		}
		if tags[s.Tag] {
			return &FieldError{Field: "dns.servers", Msg: "DNS 服务器 tag 重复: " + s.Tag}
		}
		tags[s.Tag] = true
		if !contains(SupportedDNSTypes, s.Type) {
			return &FieldError{Field: "dns.servers", Msg: "不支持的 DNS 服务器类型: " + s.Type}
		}
		if s.Type != DNSTypeLocal && s.Server == "" {
			return &FieldError{Field: "dns.servers", Msg: "DNS 服务器 " + s.Tag + " 缺少服务器地址"}
		}
		_ = i
	}
	if d.Final != "" && !tags[d.Final] {
		return &FieldError{Field: "dns.final", Msg: "默认 DNS 服务器不存在: " + d.Final}
	}
	for _, r := range d.Rules {
		if !tags[r.Server] {
			return &FieldError{Field: "dns.rules", Msg: "DNS 规则引用的服务器不存在: " + r.Server}
		}
	}
	return nil
}

// ResolverTag 返回用于默认域名解析的服务器 tag（final 优先，否则首个服务器）；未启用返回空。
func (d *DNSConfig) ResolverTag() string {
	if d == nil || !d.Enabled || len(d.Servers) == 0 {
		return ""
	}
	if d.Final != "" {
		return d.Final
	}
	return d.Servers[0].Tag
}

// JSON 渲染为内核 dns 段（未启用返回 nil）。
func (d *DNSConfig) JSON() map[string]any {
	if d == nil || !d.Enabled || len(d.Servers) == 0 {
		return nil
	}
	servers := make([]map[string]any, 0, len(d.Servers))
	for _, s := range d.Servers {
		servers = append(servers, s.json())
	}
	dns := map[string]any{"servers": servers}
	if d.Strategy != "" {
		dns["strategy"] = d.Strategy
	}
	if rules := d.rulesJSON(); len(rules) > 0 {
		dns["rules"] = rules
	}
	if d.Final != "" {
		dns["final"] = d.Final
	}
	return dns
}

// json 渲染单个 DNS 服务器（内核按 type 区分字段）。
func (s DNSServer) json() map[string]any {
	server := map[string]any{"type": s.Type, "tag": s.Tag}
	if s.Type == DNSTypeLocal {
		return server
	}
	server["server"] = s.Server
	if s.ServerPort > 0 {
		server["server_port"] = s.ServerPort
	}
	if s.Type == DNSTypeHTTPS || s.Type == DNSTypeH3 {
		if s.Path != "" {
			server["path"] = s.Path
		}
	}
	if s.Detour != "" {
		server["detour"] = s.Detour
	}
	return server
}

// rulesJSON 渲染 DNS 分流规则（route 动作指向服务器 tag）。
func (d *DNSConfig) rulesJSON() []map[string]any {
	rules := make([]map[string]any, 0, len(d.Rules))
	for _, r := range d.Rules {
		rule := map[string]any{"action": "route", "server": r.Server}
		if items := splitRuleValue(r.Domain); len(items) > 0 {
			rule["domain"] = items
		}
		if items := splitRuleValue(r.DomainSuffix); len(items) > 0 {
			rule["domain_suffix"] = items
		}
		if items := splitRuleValue(r.DomainKeyword); len(items) > 0 {
			rule["domain_keyword"] = items
		}
		if len(rule) == 2 {
			continue // 无任何匹配条件，跳过
		}
		rules = append(rules, rule)
	}
	return rules
}

// FieldError 带字段定位的配置错误（前端可精确提示到表单项）。
type FieldError struct {
	Field string // 字段路径，如 dns.strategy
	Msg   string // 错误说明
}

// Error 实现 error。
func (e *FieldError) Error() string { return e.Msg }

// contains 判断字符串是否在列表中。
func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
