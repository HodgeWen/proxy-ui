package subscription

import (
	"fmt"

	"proxy-ui/internal/singbox"
)

// RenderSingBox 渲染 sing-box 客户端配置：节点出站、策略组、DNS、路由规则与远程规则集
// （走 direct 下载），结尾补 direct 兜底出站。
func RenderSingBox(d Data) (singbox.Config, error) {
	cfg := singbox.Config{
		Log:          &singbox.LogConfig{Level: "info", Timestamp: true},
		Experimental: &singbox.Experimental{CacheFile: &singbox.CacheFile{Enabled: true}},
		DNS:          d.DNS.JSON(),
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

	route := &singbox.RouteConfig{Final: d.Final}
	seen := map[string]bool{}
	for _, r := range d.Rules {
		rule, refSet, err := singbox.RoutingRuleJSON(r)
		if err != nil {
			return singbox.Config{}, err
		}
		route.Rules = append(route.Rules, rule)
		if refSet != nil && !seen[refSet.Tag] {
			seen[refSet.Tag] = true
			// 客户端规则集走 direct 下载，避免代理未就绪时经代理下载失败
			refSet.DownloadDetour = "direct"
			route.RuleSet = append(route.RuleSet, *refSet)
		}
	}
	if resolver := d.DNS.ResolverTag(); resolver != "" {
		route.DefaultDomainResolver = resolver
	}
	if len(route.Rules) > 0 || route.Final != "" || len(route.RuleSet) > 0 || route.DefaultDomainResolver != "" {
		cfg.Route = route
	}
	return cfg, nil
}

// nodeOutboundJSON 把订阅节点映射为 sing-box 客户端出站 JSON 对象。
func nodeOutboundJSON(n Node, server string) (map[string]any, error) {
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
			// SNI 取握手目标站点，与 clash 渲染及服务端默认一致，空则 www.microsoft.com
			sni := c.Reality.HandshakeServer
			if sni == "" {
				sni = "www.microsoft.com"
			}
			ob["tls"] = map[string]any{
				"enabled":     true,
				"server_name": sni,
				"utls":        map[string]any{"enabled": true, "fingerprint": "chrome"},
				"reality": map[string]any{
					"enabled":    true,
					"public_key": c.Reality.PublicKey,
					"short_id":   c.Reality.ShortID,
				},
			}
		} else {
			if tls := clientTLS(c.TLS, server, false); tls != nil {
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
		// sing-box 客户端在带宽不限时默认用 BBR，与服务端 bbr_profile 对齐
		if c.BBRProfile != "" {
			ob["bbr_profile"] = c.BBRProfile
		}
		ob["tls"] = clientTLS(c.TLS, server, true)
	case *singbox.TUICConfig:
		ob["type"] = singbox.ProtoTUIC
		ob["uuid"] = n.Cred.UUID
		ob["password"] = n.Cred.Password
		if c.CongestionControl != "" {
			ob["congestion_control"] = c.CongestionControl
		}
		tls := clientTLS(c.TLS, server, true)
		if _, ok := tls["alpn"]; !ok {
			tls["alpn"] = []string{"h3"}
		}
		ob["tls"] = tls
	case *singbox.AnyTLSConfig:
		ob["type"] = singbox.ProtoAnyTLS
		ob["password"] = n.Cred.Password
		ob["tls"] = clientTLS(c.TLS, server, true)
	case *singbox.TrojanConfig:
		ob["type"] = singbox.ProtoTrojan
		ob["password"] = n.Cred.Password
		if tls := clientTLS(c.TLS, server, false); tls != nil {
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
		if tls := clientTLS(c.TLS, server, false); tls != nil {
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
// forceEnabled 用于必须 TLS 的协议，未启用且不强制时返回 nil。
func clientTLS(t singbox.TLSConfig, server string, forceEnabled bool) map[string]any {
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
	return tls
}
