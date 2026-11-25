package subscription

import (
	"fmt"
	"net"
	"strings"

	"proxy-ui/internal/singbox"
)

// RenderSingBox 渲染 sing-box 客户端配置：TUN 入站、节点出站、策略组、DNS、路由规则与远程规则集
// （优先走策略组下载），结尾补 direct 兜底出站。
func RenderSingBox(d Data) (singbox.Config, error) {
	hasIPv6DomainNode := false
	for _, n := range d.Nodes {
		if n.IPv6 && isDomain(nodeServer(n, d.Server)) {
			hasIPv6DomainNode = true
			break
		}
	}
	proxyGroup := primaryProxyGroup(d)
	clientDNS, directResolver := singboxClientDNS(d.DNS, proxyGroup, clientOutboundTags(d), hasIPv6DomainNode)
	cfg := singbox.Config{
		Log:          &singbox.LogConfig{Level: "info", Timestamp: true},
		Experimental: &singbox.Experimental{CacheFile: &singbox.CacheFile{Enabled: true}},
		DNS:          clientDNS,
		Inbounds: []any{
			map[string]any{
				"type":         "tun",
				"tag":          "tun-in",
				"address":      []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
				"auto_route":   true,
				"strict_route": true,
			},
		},
	}
	for _, n := range d.Nodes {
		ob, err := nodeOutboundJSON(n, d.Server)
		if err != nil {
			return singbox.Config{}, err
		}
		cfg.Outbounds = append(cfg.Outbounds, ob)
	}
	for _, g := range d.Groups {
		ob, err := singbox.GroupOutboundJSON(g)
		if err != nil {
			return singbox.Config{}, err
		}
		cfg.Outbounds = append(cfg.Outbounds, ob)
	}
	cfg.Outbounds = append(cfg.Outbounds, map[string]any{"type": "direct", "tag": "direct"})

	route := &singbox.RouteConfig{Final: d.Final, AutoDetectInterface: true}
	route.Rules = append(route.Rules, map[string]any{"action": "sniff"})
	if d.DNS != nil && d.DNS.Enabled && len(d.DNS.Servers) > 0 {
		route.Rules = append(route.Rules, map[string]any{"protocol": "dns", "action": "hijack-dns"})
	}
	route.Rules = append(route.Rules, map[string]any{"ip_is_private": true, "outbound": "direct"})
	seen := map[string]bool{}
	for _, r := range d.Rules {
		rule, refSet, err := singbox.RoutingRuleJSON(r)
		if err != nil {
			return singbox.Config{}, err
		}
		route.Rules = append(route.Rules, rule)
		if refSet != nil && !seen[refSet.Tag] {
			seen[refSet.Tag] = true
			// 客户端规则集优先走策略组下载，无策略组时回退 direct
			if proxyGroup != "" {
				refSet.DownloadDetour = proxyGroup
			} else {
				refSet.DownloadDetour = "direct"
			}
			route.RuleSet = append(route.RuleSet, *refSet)
		}
	}
	if directResolver != "" {
		route.DefaultDomainResolver = directResolver
	}
	if len(route.Rules) > 0 || route.Final != "" || len(route.RuleSet) > 0 || route.DefaultDomainResolver != "" {
		cfg.Route = route
	}
	return cfg, nil
}

// clientOutboundTags 客户端配置里实际存在的出站 tag 集合（节点、策略组与 direct）。
// DNS detour 只有在集合里时才原样保留。
func clientOutboundTags(d Data) map[string]bool {
	tags := map[string]bool{directTag: true}
	for _, n := range d.Nodes {
		tags[n.Tag] = true
	}
	for _, g := range d.Groups {
		tags[g.Name] = true
	}
	return tags
}

// singboxClientDNS 构造客户端专用 DNS 段，并返回应写入 route.default_domain_resolver 的直连解析器 tag：
// 为未显式指定 detour 的远程加密 DNS 补齐代理出口，并确保解析节点/DoH 域名的默认解析器始终直连不死锁；
// 存在域名型 IPv6 节点且原策略为 ipv4_only 时回落为 prefer_ipv4，保证 AAAA 可解析。
func singboxClientDNS(d *singbox.DNSConfig, proxyGroup string, clientTags map[string]bool, forceIPv6 bool) (map[string]any, string) {
	base := d.JSON()
	if base == nil {
		return nil, ""
	}
	if forceIPv6 && d.Strategy == singbox.StrategyIPv4Only {
		base["strategy"] = singbox.StrategyPreferIPv4
	}
	ruleTargets := make(map[string]bool, len(d.Rules))
	for _, r := range d.Rules {
		if r.Server != "" {
			ruleTargets[r.Server] = true
		}
	}
	usedTags := make(map[string]bool, len(d.Servers))
	directTag := ""
	for _, s := range d.Servers {
		usedTags[s.Tag] = true
		if directTag != "" || s.Detour != "" {
			continue
		}
		// 直连默认解析器必须能无需自身再做域名解析：local 或纯 IP 地址的 udp/tcp
		if s.Type == singbox.DNSTypeLocal ||
			((s.Type == singbox.DNSTypeUDP || s.Type == singbox.DNSTypeTCP) &&
				net.ParseIP(strings.Trim(s.Server, "[]")) != nil) {
			directTag = s.Tag
		}
	}
	rawServers, _ := base["servers"].([]map[string]any)
	needSyntheticLocal := directTag == ""
	if needSyntheticLocal {
		directTag = "dns-local"
		for usedTags[directTag] {
			directTag += "-fallback"
		}
	}
	for i, s := range d.Servers {
		if i >= len(rawServers) {
			break
		}
		entry := rawServers[i]
		isRemote := s.Type == singbox.DNSTypeHTTPS || s.Type == singbox.DNSTypeH3 || s.Type == singbox.DNSTypeTLS
		// 显式 detour 若指向服务端专属出站（客户端没有这个 tag），下发后远程 DNS 无法工作、
		// 甚至被客户端拒绝加载，因此仅在这种情况下回落到代理组；指向真实存在的组则尊重原选择。
		if s.Detour != "" && !clientTags[s.Detour] && proxyGroup != "" {
			entry["detour"] = proxyGroup
		}
		if s.Detour == "" && proxyGroup != "" && isRemote && (s.Tag == d.Final || !ruleTargets[s.Tag]) {
			entry["detour"] = proxyGroup
		}
		if s.Tag != directTag && s.Type != singbox.DNSTypeLocal && s.Server != "" &&
			net.ParseIP(strings.Trim(s.Server, "[]")) == nil {
			entry["domain_resolver"] = directTag
		}
	}
	if needSyntheticLocal {
		rawServers = append(rawServers, map[string]any{"type": singbox.DNSTypeLocal, "tag": directTag})
	}
	base["servers"] = rawServers
	return base, directTag
}

// nodeOutboundJSON 把订阅节点映射为 sing-box 客户端出站 JSON 对象。
// 节点出站保持自包含、不绑定具体 domain_resolver tag，域名解析交由 route.default_domain_resolver 兜底，
// 避免提取出站时脱离 dns 段引发 domain resolver not found。
func nodeOutboundJSON(n Node, defaultServer string) (map[string]any, error) {
	server := nodeServer(n, defaultServer)
	sni := fallbackSNI(server, defaultServer)
	ob := map[string]any{"tag": n.Tag, "server": server, "server_port": n.Port}
	switch c := n.Config.(type) {
	case *singbox.VLESSConfig:
		ob["type"] = singbox.ProtoVLESS
		ob["uuid"] = n.Cred.UUID
		flow := n.Cred.Flow
		if flow == "" {
			flow = c.Flow
		}
		if flow != "" {
			ob["flow"] = flow
		}
		if c.Reality.Enabled {
			ob["tls"] = map[string]any{
				"enabled":     true,
				"server_name": c.RealitySNI(),
				"utls":        map[string]any{"enabled": true, "fingerprint": "chrome"},
				"reality": map[string]any{
					"enabled":    true,
					"public_key": c.Reality.PublicKey,
					"short_id":   c.Reality.ShortID,
				},
			}
		} else {
			if tls := clientTLS(c.TLS, sni, false, n.Insecure); tls != nil {
				ob["tls"] = tls
			}
		}
	case *singbox.Hysteria2Config:
		ob["type"] = singbox.ProtoHysteria2
		ob["password"] = n.Cred.Password
		if c.ObfsEnabled && c.ObfsPassword != "" {
			ob["obfs"] = map[string]any{"type": "salamander", "password": c.ObfsPassword}
		}
		if c.UpMbps > 0 {
			ob["up_mbps"] = c.UpMbps
		}
		if c.DownMbps > 0 {
			ob["down_mbps"] = c.DownMbps
		}
		// 默认档位 standard 省略，兼容不支持 bbr_profile 的旧版 sing-box 客户端
		if c.BBRProfile != "" && c.BBRProfile != singbox.BBRProfileStandard {
			ob["bbr_profile"] = c.BBRProfile
		}
		ob["tls"] = clientTLS(c.TLS, sni, true, n.Insecure)
	case *singbox.TUICConfig:
		ob["type"] = singbox.ProtoTUIC
		ob["uuid"] = n.Cred.UUID
		ob["password"] = n.Cred.Password
		if c.CongestionControl != "" {
			ob["congestion_control"] = c.CongestionControl
		}
		tls := clientTLS(c.TLS, sni, true, n.Insecure)
		if _, ok := tls["alpn"]; !ok {
			tls["alpn"] = []string{"h3"}
		}
		ob["tls"] = tls
	case *singbox.AnyTLSConfig:
		ob["type"] = singbox.ProtoAnyTLS
		ob["password"] = n.Cred.Password
		ob["tls"] = clientTLS(c.TLS, sni, true, n.Insecure)
	case *singbox.TrojanConfig:
		ob["type"] = singbox.ProtoTrojan
		ob["password"] = n.Cred.Password
		if tls := clientTLS(c.TLS, sni, false, n.Insecure); tls != nil {
			ob["tls"] = tls
		}
	case *singbox.VMessConfig:
		ob["type"] = singbox.ProtoVMess
		ob["uuid"] = n.Cred.UUID
		path := c.WSPath
		if path == "" {
			path = "/"
		}
		ob["transport"] = map[string]any{"type": "ws", "path": path}
		if tls := clientTLS(c.TLS, sni, false, n.Insecure); tls != nil {
			ob["tls"] = tls
		}
	case *singbox.ShadowsocksConfig:
		ob["type"] = singbox.ProtoShadowsocks
		method := c.Method
		if method == "" {
			method = "2022-blake3-aes-128-gcm"
		}
		ob["method"] = method
		if _, is2022 := singbox.SSKeyLength(method); is2022 {
			// Shadowsocks-2022 多用户：客户端密码必须是「入站 PSK:用户密钥」，
			// 入站 PSK 持久化在入站配置里，缺一握手必然失败。
			ob["password"] = c.PSK + ":" + n.Cred.Password
			return ob, nil
		}
		ob["password"] = n.Cred.Password
	default:
		return nil, fmt.Errorf("节点 %s: 配置类型不符: %T", n.Tag, n.Config)
	}
	return ob, nil
}

// clientTLS 渲染客户端 TLS 对象；SNI 未配置时用服务器地址兜底。
// forceEnabled 用于必须 TLS 的协议，未启用且不强制时返回 nil；insecure 用于自签证书跳过校验。
func clientTLS(t singbox.TLSConfig, server string, forceEnabled, insecure bool) map[string]any {
	if !t.Enabled && !forceEnabled {
		return nil
	}
	tls := map[string]any{"enabled": true}
	if t.ServerName != "" {
		tls["server_name"] = t.ServerName
	} else {
		tls["server_name"] = server
	}
	if len(t.ALPN) > 0 {
		tls["alpn"] = t.ALPN
	}
	if insecure {
		tls["insecure"] = true
	}
	return tls
}
