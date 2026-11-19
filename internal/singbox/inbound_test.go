package singbox

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// stubResolver 面板存储证书解析桩：任意 ID 均解析为固定路径。
func stubResolver(uint) (string, string, error) { return "/data/certs/c.pem", "/data/certs/k.pem", nil }

// buildOne 以单条入站走完 解析→映射→聚合，返回生成的 inbound JSON。
func buildOne(t *testing.T, protocol, rawConfig string) map[string]any {
	t.Helper()
	cfg, err := BuildConfig(
		[]InboundEntry{{Tag: "test-in", Protocol: protocol, ListenPort: 8443, Config: rawConfig, Cert: stubResolver}},
		nil,
	)
	if err != nil {
		t.Fatalf("生成配置失败: %v", err)
	}
	if len(cfg.Inbounds) != 1 {
		t.Fatalf("应生成 1 条入站, got %d", len(cfg.Inbounds))
	}
	return cfg.Inbounds[0].(map[string]any)
}

// users 取入站 users 数组首元素。
func users(t *testing.T, in map[string]any) map[string]any {
	t.Helper()
	list, _ := in["users"].([]any)
	if len(list) != 1 {
		t.Fatalf("应恰有 1 个用户, got %v", in["users"])
	}
	return list[0].(map[string]any)
}

// tlsOf 取入站 tls 段。
func tlsOf(t *testing.T, in map[string]any) map[string]any {
	t.Helper()
	tls, _ := in["tls"].(map[string]any)
	if tls == nil {
		t.Fatalf("缺少 tls 段: %v", in)
	}
	return tls
}

func TestInboundJSONEightProtocols(t *testing.T) {
	ssKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	tests := []struct {
		name     string
		protocol string
		config   string
		assert   func(t *testing.T, in map[string]any)
	}{
		{
			name:     "VLESS+Reality",
			protocol: ProtoVLESS,
			config: `{"uuid":"bf000d23-0752-40b4-affe-68f7707a9661","flow":"xtls-rprx-vision",` +
				`"reality":{"enabled":true,"privateKey":"pk","shortId":"0123abcd","handshakeServer":"www.bing.com","handshakePort":443},` +
				`"tls":{"enabled":true,"serverName":"www.bing.com"}}`,
			assert: func(t *testing.T, in map[string]any) {
				u := users(t, in)
				if u["uuid"] != "bf000d23-0752-40b4-affe-68f7707a9661" || u["flow"] != "xtls-rprx-vision" {
					t.Errorf("VLESS 用户字段不符: %v", u)
				}
				tls := tlsOf(t, in)
				reality, _ := tls["reality"].(map[string]any)
				if reality == nil || reality["enabled"] != true || reality["private_key"] != "pk" {
					t.Errorf("Reality 段不符: %v", tls)
				}
				if hs, _ := reality["handshake"].(map[string]any); hs["server"] != "www.bing.com" || hs["server_port"] != 443 {
					t.Errorf("Reality 握手目标不符: %v", reality)
				}
				if ids, _ := reality["short_id"].([]string); len(ids) != 1 || ids[0] != "0123abcd" {
					t.Errorf("short_id 不符: %v", reality)
				}
				if _, hasCert := tls["certificate_path"]; hasCert {
					t.Errorf("Reality 模式不应输出证书路径: %v", tls)
				}
			},
		},
		{
			name:     "Hysteria2",
			protocol: ProtoHysteria2,
			config:   `{"password":"pw","obfsPassword":"obfs","upMbps":100,"downMbps":200,"tls":{"enabled":true,"certMode":"managed","certId":7}}`,
			assert: func(t *testing.T, in map[string]any) {
				if users(t, in)["password"] != "pw" {
					t.Errorf("Hysteria2 密码不符: %v", in["users"])
				}
				obfs, _ := in["obfs"].(map[string]any)
				if obfs["type"] != "salamander" || obfs["password"] != "obfs" {
					t.Errorf("obfs 段不符: %v", in["obfs"])
				}
				if in["up_mbps"] != 100 || in["down_mbps"] != 200 {
					t.Errorf("带宽字段不符: %v %v", in["up_mbps"], in["down_mbps"])
				}
				tls := tlsOf(t, in)
				if tls["certificate_path"] != "/data/certs/c.pem" || tls["key_path"] != "/data/certs/k.pem" {
					t.Errorf("证书路径未解析: %v", tls)
				}
			},
		},
		{
			name:     "TUIC v5",
			protocol: ProtoTUIC,
			config:   `{"uuid":"bf000d23-0752-40b4-affe-68f7707a9661","password":"pw","congestionControl":"bbr","tls":{"enabled":true,"certMode":"path","certPath":"/etc/c.pem","keyPath":"/etc/k.pem"}}`,
			assert: func(t *testing.T, in map[string]any) {
				u := users(t, in)
				if u["uuid"] != "bf000d23-0752-40b4-affe-68f7707a9661" || u["password"] != "pw" {
					t.Errorf("TUIC 用户字段不符: %v", u)
				}
				if in["congestion_control"] != "bbr" {
					t.Errorf("拥塞控制不符: %v", in["congestion_control"])
				}
				tls := tlsOf(t, in)
				if tls["certificate_path"] != "/etc/c.pem" || tls["key_path"] != "/etc/k.pem" {
					t.Errorf("path 模式证书路径不符: %v", tls)
				}
			},
		},
		{
			name:     "AnyTLS",
			protocol: ProtoAnyTLS,
			config:   `{"password":"pw","tls":{"enabled":true,"certMode":"managed","certId":7}}`,
			assert: func(t *testing.T, in map[string]any) {
				if users(t, in)["password"] != "pw" {
					t.Errorf("AnyTLS 密码不符: %v", in["users"])
				}
				tls := tlsOf(t, in)
				if tls["key_path"] != "/data/certs/k.pem" {
					t.Errorf("AnyTLS 证书路径不符: %v", tls)
				}
			},
		},
		{
			name:     "Trojan",
			protocol: ProtoTrojan,
			config:   `{"password":"pw","tls":{"enabled":true,"certMode":"managed","certId":7}}`,
			assert: func(t *testing.T, in map[string]any) {
				if users(t, in)["password"] != "pw" {
					t.Errorf("Trojan 密码不符: %v", in["users"])
				}
				tls := tlsOf(t, in)
				if _, has := tls["server_name"]; has {
					t.Errorf("未设置时不应输出 server_name: %v", tls)
				}
			},
		},
		{
			name:     "VMess+WS+TLS",
			protocol: ProtoVMess,
			config:   `{"uuid":"bf000d23-0752-40b4-affe-68f7707a9661","wsPath":"/ws","tls":{"enabled":true,"certMode":"managed","certId":7,"serverName":"a.com","alpn":["h2","http/1.1"]}}`,
			assert: func(t *testing.T, in map[string]any) {
				if users(t, in)["uuid"] != "bf000d23-0752-40b4-affe-68f7707a9661" {
					t.Errorf("VMess 用户字段不符: %v", in["users"])
				}
				ws, _ := in["transport"].(map[string]any)
				if ws["type"] != "ws" || ws["path"] != "/ws" {
					t.Errorf("WS transport 不符: %v", in["transport"])
				}
				tls := tlsOf(t, in)
				if tls["server_name"] != "a.com" {
					t.Errorf("server_name 不符: %v", tls)
				}
				if alpn, _ := tls["alpn"].([]string); len(alpn) != 2 || alpn[0] != "h2" {
					t.Errorf("alpn 不符: %v", tls)
				}
			},
		},
		{
			name:     "Shadowsocks-2022",
			protocol: ProtoShadowsocks,
			config:   `{"method":"2022-blake3-aes-256-gcm","password":"` + ssKey + `"}`,
			assert: func(t *testing.T, in map[string]any) {
				if in["method"] != "2022-blake3-aes-256-gcm" || in["password"] != ssKey {
					t.Errorf("Shadowsocks 字段不符: %v %v", in["method"], in["password"])
				}
				if _, has := in["users"]; has {
					t.Errorf("单用户 SS 不应输出 users: %v", in)
				}
			},
		},
		{
			name:     "Direct",
			protocol: ProtoDirect,
			config:   `{}`,
			assert: func(t *testing.T, in map[string]any) {
				for _, key := range []string{"users", "tls", "transport", "method"} {
					if _, has := in[key]; has {
						t.Errorf("Direct 入站不应有 %s: %v", key, in)
					}
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := buildOne(t, tt.protocol, tt.config)
			if in["type"] != tt.protocol || in["tag"] != "test-in" || in["listen"] != "::" || in["listen_port"] != 8443 {
				t.Errorf("基础字段不符: %v", in)
			}
			tt.assert(t, in)
		})
	}
}

// TestVLESSRealityWithoutTLS 回归 B1：Reality 开启且 TLS 开关关闭时，
// Reality 隐含 TLS，应产出 enabled 的 tls 段（而非 nil map panic），且无需证书。
func TestVLESSRealityWithoutTLS(t *testing.T) {
	in := buildOne(t, ProtoVLESS,
		`{"uuid":"bf000d23-0752-40b4-affe-68f7707a9661","flow":"xtls-rprx-vision",`+
			`"reality":{"enabled":true,"privateKey":"pk","shortId":"0123abcd","handshakeServer":"www.bing.com","handshakePort":443},`+
			`"tls":{"enabled":false}}`)
	tls := tlsOf(t, in)
	if tls["enabled"] != true {
		t.Errorf("Reality 开启时 TLS 应强制启用: %v", tls)
	}
	reality, _ := tls["reality"].(map[string]any)
	if reality == nil || reality["enabled"] != true || reality["private_key"] != "pk" {
		t.Errorf("Reality 段不符: %v", tls)
	}
	if _, hasCert := tls["certificate_path"]; hasCert {
		t.Errorf("Reality 模式不应输出证书路径: %v", tls)
	}
}

func TestDefaultInboundConfig(t *testing.T) {
	for _, protocol := range SupportedProtocols {
		cfg, err := DefaultInboundConfig(protocol)
		if err != nil {
			t.Fatalf("%s 默认配置生成失败: %v", protocol, err)
		}
		switch c := cfg.(type) {
		case *VLESSConfig:
			if c.UUID == "" || c.Reality.PrivateKey == "" || c.Reality.PublicKey == "" || c.Reality.ShortID == "" {
				t.Errorf("VLESS 默认凭据不完整: %+v", c)
			}
			if priv, _ := base64.RawURLEncoding.DecodeString(c.Reality.PrivateKey); len(priv) != 32 {
				t.Errorf("Reality 私钥应为 32 字节 x25519: %q", c.Reality.PrivateKey)
			}
			if pub, _ := base64.RawURLEncoding.DecodeString(c.Reality.PublicKey); len(pub) != 32 {
				t.Errorf("Reality 公钥应为 32 字节: %q", c.Reality.PublicKey)
			}
		case *Hysteria2Config:
			if c.Password == "" || !c.TLS.Enabled || c.TLS.CertMode != CertModeManaged {
				t.Errorf("Hysteria2 默认值不符: %+v", c)
			}
		case *TUICConfig:
			if c.UUID == "" || c.Password == "" || len(c.TLS.ALPN) != 1 || c.TLS.ALPN[0] != "h3" {
				t.Errorf("TUIC 默认值不符: %+v", c)
			}
		case *AnyTLSConfig:
			if c.Password == "" {
				t.Errorf("AnyTLS 默认密码为空")
			}
		case *TrojanConfig:
			if c.Password == "" {
				t.Errorf("Trojan 默认密码为空")
			}
		case *VMessConfig:
			if c.UUID == "" || c.WSPath != "/" {
				t.Errorf("VMess 默认值不符: %+v", c)
			}
		case *ShadowsocksConfig:
			raw, err := base64.StdEncoding.DecodeString(c.Password)
			if c.Method != "2022-blake3-aes-128-gcm" || err != nil || len(raw) != 16 {
				t.Errorf("SS2022 默认密钥不符: %+v", c)
			}
		}
	}
	if _, err := DefaultInboundConfig("unknown"); err == nil {
		t.Error("未知协议应报错")
	}
}

func TestInboundWarnings(t *testing.T) {
	weakSS := base64.StdEncoding.EncodeToString([]byte("short"))
	tests := []struct {
		name     string
		protocol string
		config   string
		contains string // 期望至少包含该关键词的警示；空表示应无警示
	}{
		{name: "Trojan 未启用 TLS", protocol: ProtoTrojan,
			config: `{"password":"pw","tls":{"enabled":false}}`, contains: "明文"},
		{name: "Hysteria2 空密码", protocol: ProtoHysteria2,
			config: `{"password":"","tls":{"enabled":true,"certMode":"managed","certId":1}}`, contains: "密码为空"},
		{name: "Hysteria2 关闭 TLS", protocol: ProtoHysteria2,
			config: `{"password":"pw","tls":{"enabled":false}}`, contains: "必须启用 TLS"},
		{name: "SS2022 弱密钥", protocol: ProtoShadowsocks,
			config: `{"method":"2022-blake3-aes-128-gcm","password":"` + weakSS + `"}`, contains: "强度不足"},
		{name: "SS2022 非法密钥", protocol: ProtoShadowsocks,
			config: `{"method":"2022-blake3-aes-128-gcm","password":"!!!"}`, contains: "强度不足"},
		{name: "SS2022 弱方法", protocol: ProtoShadowsocks,
			config: `{"method":"aes-128-gcm","password":"pw"}`, contains: "2022"},
		{name: "VLESS 未启用 Reality/TLS", protocol: ProtoVLESS,
			config: `{"uuid":"u","reality":{"enabled":false},"tls":{"enabled":false}}`, contains: "明文"},
		{name: "VLESS Reality 开启 TLS 关闭", protocol: ProtoVLESS,
			config: `{"uuid":"u","reality":{"enabled":true,"privateKey":"pk"},"tls":{"enabled":false}}`, contains: "强制启用 TLS"},
		{name: "TUIC 凭据为空", protocol: ProtoTUIC,
			config: `{"uuid":"","password":"","tls":{"enabled":true,"certMode":"managed","certId":1}}`, contains: "为空"},
		{name: "TLS 启用但缺证书", protocol: ProtoTrojan,
			config: `{"password":"pw","tls":{"enabled":true,"certMode":"managed","certId":0}}`, contains: "未配置证书"},
		{name: "path 模式路径不全", protocol: ProtoTrojan,
			config: `{"password":"pw","tls":{"enabled":true,"certMode":"path","certPath":"/c.pem"}}`, contains: "未配置证书"},
		{name: "配置完备无警示", protocol: ProtoHysteria2,
			config: `{"password":"pw","tls":{"enabled":true,"certMode":"managed","certId":3}}`, contains: ""},
		{name: "Direct 无警示", protocol: ProtoDirect, config: `{}`, contains: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseInboundConfig(tt.protocol, tt.config)
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			warnings := InboundWarnings(tt.protocol, cfg)
			if tt.contains == "" {
				if len(warnings) > 0 {
					t.Errorf("不应有警示, got %v", warnings)
				}
				return
			}
			for _, w := range warnings {
				if strings.Contains(w, tt.contains) {
					return
				}
			}
			t.Errorf("警示 %v 未包含 %q", warnings, tt.contains)
		})
	}
}

func TestParseInboundConfig(t *testing.T) {
	if _, err := ParseInboundConfig("socks", "{}"); err == nil {
		t.Error("不支持协议应报错")
	}
	if _, err := ParseInboundConfig(ProtoTrojan, "not-json"); err == nil {
		t.Error("非法 JSON 应报错")
	}
	cfg, err := ParseInboundConfig(ProtoHysteria2, "")
	if err != nil {
		t.Fatalf("空配置应返回默认值: %v", err)
	}
	if c, ok := cfg.(*Hysteria2Config); !ok || c.Password == "" {
		t.Errorf("空配置默认值不符: %+v", cfg)
	}
}
