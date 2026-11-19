package subscription

import (
	"fmt"

	"proxy-ui/internal/singbox"
)

// RenderSingBox 渲染 sing-box 客户端配置：节点出站、策略组、路由规则与远程规则集
// （anti-AD 等，走 direct 下载），结尾补 direct 兜底出站。
func RenderSingBox(d Data) (singbox.Config, error) {
	cfg := singbox.Config{
		Log:          &singbox.LogConfig{Level: "info", Timestamp: true},
		Experimental: &singbox.Experimental{CacheFile: &singbox.CacheFile{Enabled: true}},
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
		rule, refs, err := singbox.RoutingRuleJSON(r)
		if err != nil {
			return singbox.Config{}, err
		}
		route.Rules = append(route.Rules, rule)
		for _, tag := range refs {
			if !seen[tag] {
				seen[tag] = true
				rs, err := singbox.RuleSetConfigOf(tag, d.AntiAD)
				if err != nil {
					return singbox.Config{}, err
				}
				// 客户端规则集走 direct 下载，避免代理未就绪时经代理下载失败
				rs.DownloadDetour = "direct"
				route.RuleSet = append(route.RuleSet, rs)
			}
		}
	}
	if len(route.Rules) > 0 || route.Final != "" {
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
		ob["uuid"] = c.UUID
		if c.Flow != "" {
			ob["flow"] = c.Flow
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
		ob["password"] = c.Password
		if c.ObfsPassword != "" {
			ob["obfs"] = map[string]any{"type": "salamander", "password": c.ObfsPassword}
		}
		ob["tls"] = clientTLS(c.TLS, server, true)
	case *singbox.TUICConfig:
		ob["type"] = singbox.ProtoTUIC
		ob["uuid"] = c.UUID
		ob["password"] = c.Password
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
		ob["password"] = c.Password
		ob["tls"] = clientTLS(c.TLS, server, true)
	case *singbox.TrojanConfig:
		ob["type"] = singbox.ProtoTrojan
		ob["password"] = c.Password
		if tls := clientTLS(c.TLS, server, false); tls != nil {
			ob["tls"] = tls
		}
	case *singbox.VMessConfig:
		ob["type"] = singbox.ProtoVMess
		ob["uuid"] = c.UUID
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
		ob["password"] = c.Password
	default:
		return nil, fmt.Errorf("节点 %s: 配置类型不符: %T", n.Tag, n.Config)
	}
	return ob, nil
}

// clientTLS 渲染客户端 TLS 对象；SNI 未配置时用服务器地址兜底，Hysteria2/TUIC 默认
// h3 ALPN。forceEnabled 用于必须 TLS 的协议，未启用且不强制时返回 nil。
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
