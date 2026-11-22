package singbox

import (
	"fmt"
	"strings"
)

// inboundJSON 把入站连接层配置 + 其下用户映射为 sing-box inbound JSON 对象。
// 用户凭据按协议写入 users 数组；无用户的入站不产出（内核要求至少一个认证用户）。
func inboundJSON(e InboundEntry, cfg any) (map[string]any, error) {
	if len(e.Users) == 0 {
		return nil, fmt.Errorf("入站没有启用中的用户，无法生成配置")
	}
	listen := e.Listen
	if listen == "" {
		listen = DefaultListen
	}
	in := map[string]any{"type": e.Protocol, "tag": e.Tag, "listen": listen, "listen_port": e.Port}
	switch c := cfg.(type) {
	case *VLESSConfig:
		tls, err := c.TLS.tlsJSON(c.Reality.Enabled, !c.Reality.Enabled, e.Cert)
		if err != nil {
			return nil, err
		}
		if c.Reality.Enabled {
			sni := c.RealitySNI()
			tls["server_name"] = sni
			rc := c.Reality
			if strings.TrimSpace(rc.HandshakeServer) == "" {
				rc.HandshakeServer = sni
			}
			tls["reality"] = rc.json()
			// 内核不允许 Reality 与 ECH 并存（reality_server 会直接报错），这里以 Reality 优先
			delete(tls, "ech")
		}
		if tls != nil {
			in["tls"] = tls
		}
		mergeConnect(in, e.Protocol, c.ConnectConfig)
		users, err := usersJSON(e, func(u InboundUserEntry) (map[string]any, error) {
			if u.Credential.UUID == "" {
				return nil, fmt.Errorf("用户 %s 缺少 UUID", u.Name)
			}
			user := map[string]any{"name": u.Name, "uuid": u.Credential.UUID}
			// 用户级流控优先，缺省沿用入站级
			flow := u.Credential.Flow
			if flow == "" {
				flow = c.Flow
			}
			if flow != "" {
				user["flow"] = flow
			}
			return user, nil
		})
		if err != nil {
			return nil, err
		}
		in["users"] = users
	case *Hysteria2Config:
		if c.ObfsEnabled && c.ObfsPassword != "" {
			in["obfs"] = map[string]any{"type": "salamander", "password": c.ObfsPassword}
		}
		if c.UpMbps > 0 {
			in["up_mbps"] = c.UpMbps
		}
		if c.DownMbps > 0 {
			in["down_mbps"] = c.DownMbps
		}
		if c.IgnoreClientBandwidth {
			in["ignore_client_bandwidth"] = true
		}
		if c.BBRProfile != "" {
			in["bbr_profile"] = c.BBRProfile
		}
		if masquerade := c.Masquerade.json(); masquerade != nil {
			in["masquerade"] = masquerade
		}
		tls, err := c.TLS.tlsJSON(true, true, e.Cert)
		if err != nil {
			return nil, err
		}
		if _, ok := tls["alpn"]; !ok {
			tls["alpn"] = []string{"h3"}
		}
		in["tls"] = tls
		mergeConnect(in, e.Protocol, c.ConnectConfig)
		users, err := usersJSON(e, passwordUser)
		if err != nil {
			return nil, err
		}
		in["users"] = users
	case *TUICConfig:
		if c.CongestionControl != "" {
			in["congestion_control"] = c.CongestionControl
		}
		if c.ZeroRTTHandshake {
			in["zero_rtt_handshake"] = true
		}
		if c.Heartbeat != "" {
			in["heartbeat"] = c.Heartbeat
		}
		tls, err := c.TLS.tlsJSON(true, true, e.Cert)
		if err != nil {
			return nil, err
		}
		if _, ok := tls["alpn"]; !ok {
			tls["alpn"] = []string{"h3"}
		}
		in["tls"] = tls
		mergeConnect(in, e.Protocol, c.ConnectConfig)
		users, err := usersJSON(e, func(u InboundUserEntry) (map[string]any, error) {
			if u.Credential.UUID == "" || u.Credential.Password == "" {
				return nil, fmt.Errorf("用户 %s 缺少 UUID 或密码", u.Name)
			}
			return map[string]any{"name": u.Name, "uuid": u.Credential.UUID, "password": u.Credential.Password}, nil
		})
		if err != nil {
			return nil, err
		}
		in["users"] = users
	case *AnyTLSConfig:
		tls, err := c.TLS.tlsJSON(true, true, e.Cert)
		if err != nil {
			return nil, err
		}
		in["tls"] = tls
		mergeConnect(in, e.Protocol, c.ConnectConfig)
		users, err := usersJSON(e, passwordUser)
		if err != nil {
			return nil, err
		}
		in["users"] = users
	case *TrojanConfig:
		tls, err := c.TLS.tlsJSON(false, true, e.Cert)
		if err != nil {
			return nil, err
		}
		if tls != nil {
			in["tls"] = tls
		}
		mergeConnect(in, e.Protocol, c.ConnectConfig)
		users, err := usersJSON(e, passwordUser)
		if err != nil {
			return nil, err
		}
		in["users"] = users
	case *VMessConfig:
		path := c.WSPath
		if path == "" {
			path = "/"
		}
		in["transport"] = map[string]any{"type": "ws", "path": path}
		tls, err := c.TLS.tlsJSON(false, true, e.Cert)
		if err != nil {
			return nil, err
		}
		if tls != nil {
			in["tls"] = tls
		}
		mergeConnect(in, e.Protocol, c.ConnectConfig)
		users, err := usersJSON(e, func(u InboundUserEntry) (map[string]any, error) {
			if u.Credential.UUID == "" {
				return nil, fmt.Errorf("用户 %s 缺少 UUID", u.Name)
			}
			return map[string]any{"name": u.Name, "uuid": u.Credential.UUID}, nil
		})
		if err != nil {
			return nil, err
		}
		in["users"] = users
	case *ShadowsocksConfig:
		method := c.Method
		if method == "" {
			method = "2022-blake3-aes-128-gcm"
		}
		in["method"] = method
		if c.Network != "" {
			in["network"] = c.Network
		}
		if n, is2022 := SSKeyLength(method); is2022 {
			// Shadowsocks-2022 多用户：入站级 password 是该入站的 PSK（服务端身份），
			// 各用户用自己的密钥（users[].password）认证；客户端密码须为「PSK:用户密钥」，
			// 因此 PSK 必须持久化在入站配置里（c.PSK），不能每次随机生成。
			if c.PSK == "" {
				return nil, fmt.Errorf("入站缺少 Shadowsocks-2022 PSK，请重新保存入站")
			}
			_ = n
			in["password"] = c.PSK
			users, err := usersJSON(e, passwordUser)
			if err != nil {
				return nil, err
			}
			in["users"] = users
		} else {
			// 非 2022（单用户）方法：沿用首个用户密钥
			first := e.Users[0]
			if first.Credential.Password == "" {
				return nil, fmt.Errorf("用户 %s 缺少密钥", first.Name)
			}
			in["password"] = first.Credential.Password
		}
		mergeConnect(in, e.Protocol, c.ConnectConfig)
	default:
		return nil, fmt.Errorf("入站 %s: 配置类型不符: %T", e.Tag, cfg)
	}
	return in, nil
}

// mergeConnect 把监听层选项写入入站对象：TCP Fast Open 全协议透传；
// 多路复用只在 sing-box 支持的协议上写入（内核其它入站类型没有 multiplex 字段）。
func mergeConnect(in map[string]any, protocol string, c ConnectConfig) {
	if c.TCPFastOpen {
		in["tcp_fast_open"] = true
	}
	if c.Multiplex.Enabled && MultiplexSupported(protocol) {
		in["multiplex"] = map[string]any{"enabled": true, "padding": c.Multiplex.Padding}
	}
}

// json 渲染 Hysteria2 masquerade 段；未启用返回 nil（不写入配置）。
func (m MasqueradeConfig) json() map[string]any {
	switch m.Type {
	case MasqueradeFile:
		return map[string]any{"type": MasqueradeFile, "directory": m.Directory}
	case MasqueradeProxy:
		return map[string]any{"type": MasqueradeProxy, "url": m.ProxyURL}
	case MasqueradeString:
		status := m.StatusCode
		if status == 0 {
			status = 200
		}
		return map[string]any{"type": MasqueradeString, "status_code": status, "content": m.Content}
	}
	return nil
}

// usersJSON 逐用户渲染 users 数组。
func usersJSON(e InboundEntry, render func(InboundUserEntry) (map[string]any, error)) ([]any, error) {
	users := make([]any, 0, len(e.Users))
	for _, u := range e.Users {
		user, err := render(u)
		if err != nil {
			return nil, fmt.Errorf("入站 %s: %w", e.Tag, err)
		}
		users = append(users, user)
	}
	return users, nil
}

// passwordUser 渲染仅需密码的用户（Hysteria2 / AnyTLS / Trojan / Shadowsocks）。
func passwordUser(u InboundUserEntry) (map[string]any, error) {
	if u.Credential.Password == "" {
		return nil, fmt.Errorf("用户 %s 缺少密码", u.Name)
	}
	return map[string]any{"name": u.Name, "password": u.Credential.Password}, nil
}
