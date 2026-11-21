package singbox

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// stubResolver 面板存储证书解析桩：任意 ID 均解析为固定路径。
func stubResolver(uint) (string, string, error) { return "/data/certs/c.pem", "/data/certs/k.pem", nil }

// configJSON 把连接层配置序列化为 JSON 文本（聚合入参用）。
func configJSON(t *testing.T, cfg any) string {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("序列化配置失败: %v", err)
	}
	return string(raw)
}

// templateWithCert 取协议默认配置并补上面板证书 ID（managed 模式必须指向已存在证书）。
func templateWithCert(t *testing.T, protocol string) any {
	t.Helper()
	cfg, err := NewTemplateConfig(protocol)
	if err != nil {
		t.Fatalf("生成 %s 默认配置失败: %v", protocol, err)
	}
	if tls := InboundTLS(cfg); tls != nil && tls.CertMode == CertModeManaged {
		tls.CertID = 1
	}
	return cfg
}

// configFields 把连接层配置解析为字段映射（断言"不含用户凭据"用）。
func configFields(t *testing.T, cfg any) map[string]any {
	t.Helper()
	fields := map[string]any{}
	if err := json.Unmarshal([]byte(configJSON(t, cfg)), &fields); err != nil {
		t.Fatalf("解析配置字段失败: %v", err)
	}
	return fields
}

// usersOf 取出 inbound JSON 的 users 数组。
func usersOf(t *testing.T, in map[string]any) []map[string]any {
	t.Helper()
	list, ok := in["users"].([]any)
	if !ok {
		t.Fatalf("缺少 users 数组: %v", in)
	}
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		user, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("users 元素类型不符: %T", item)
		}
		out = append(out, user)
	}
	return out
}

// TestTemplateConfigNoCredential 默认连接层配置不得夹带任何用户凭据。
func TestTemplateConfigNoCredential(t *testing.T) {
	for _, protocol := range SupportedProtocols {
		t.Run(protocol, func(t *testing.T) {
			cfg, err := NewTemplateConfig(protocol)
			if err != nil {
				t.Fatalf("生成默认配置失败: %v", err)
			}
			fields := configFields(t, cfg)
			for _, key := range []string{"uuid", "password"} {
				if _, ok := fields[key]; ok {
					t.Errorf("%s 默认配置不应包含用户凭据字段 %s: %v", protocol, key, fields)
				}
			}
		})
	}
	// 不支持的协议报错而非返回空配置
	if _, err := NewTemplateConfig("wireguard"); err == nil {
		t.Error("不支持的协议应报错")
	}
}

// TestTemplateConfigQUICDefaults QUIC 类协议必须默认带上 h3 与非默认算法档位。
func TestTemplateConfigQUICDefaults(t *testing.T) {
	raw, err := NewTemplateConfig(ProtoHysteria2)
	if err != nil {
		t.Fatalf("生成 Hysteria2 默认配置失败: %v", err)
	}
	hy2, ok := raw.(*Hysteria2Config)
	if !ok {
		t.Fatalf("Hysteria2 配置类型不符: %T", raw)
	}
	if !hy2.TLS.Enabled {
		t.Error("Hysteria2 必须默认启用 TLS")
	}
	if !contains(hy2.TLS.ALPN, "h3") {
		t.Errorf("Hysteria2 默认 ALPN 应含 h3: %v", hy2.TLS.ALPN)
	}
	if hy2.BBRProfile != BBRProfileStandard {
		t.Errorf("Hysteria2 默认 BBR 档位 = %q, want %q", hy2.BBRProfile, BBRProfileStandard)
	}

	raw, err = NewTemplateConfig(ProtoTUIC)
	if err != nil {
		t.Fatalf("生成 TUIC 默认配置失败: %v", err)
	}
	tuic, ok := raw.(*TUICConfig)
	if !ok {
		t.Fatalf("TUIC 配置类型不符: %T", raw)
	}
	if !tuic.TLS.Enabled {
		t.Error("TUIC 必须默认启用 TLS")
	}
	if !contains(tuic.TLS.ALPN, "h3") {
		t.Errorf("TUIC 默认 ALPN 应含 h3: %v", tuic.TLS.ALPN)
	}
	if tuic.CongestionControl != CongestionBBR {
		t.Errorf("TUIC 默认拥塞控制 = %q, want %q", tuic.CongestionControl, CongestionBBR)
	}

	if !QUICProtocol(ProtoHysteria2) || !QUICProtocol(ProtoTUIC) {
		t.Error("Hysteria2/TUIC 应判为 QUIC 类协议")
	}
	for _, protocol := range []string{ProtoVLESS, ProtoAnyTLS, ProtoTrojan, ProtoVMess, ProtoShadowsocks} {
		if QUICProtocol(protocol) {
			t.Errorf("%s 不应判为 QUIC 类协议", protocol)
		}
	}
}

// TestTemplateConfigShadowsocksKey Shadowsocks 默认 PSK 长度必须与内核要求一致。
func TestTemplateConfigShadowsocksKey(t *testing.T) {
	raw, err := NewTemplateConfig(ProtoShadowsocks)
	if err != nil {
		t.Fatalf("生成 Shadowsocks 默认配置失败: %v", err)
	}
	ss, ok := raw.(*ShadowsocksConfig)
	if !ok {
		t.Fatalf("Shadowsocks 配置类型不符: %T", raw)
	}
	if ss.PSK == "" {
		t.Fatal("Shadowsocks-2022 默认配置必须带 PSK（入站级身份，不能每次随机）")
	}
	n, ok := SSKeyLength(ss.Method)
	if !ok {
		t.Fatalf("默认方法不受支持: %q", ss.Method)
	}
	key, err := base64.StdEncoding.DecodeString(ss.PSK)
	if err != nil {
		t.Fatalf("PSK 不是合法 base64: %v", err)
	}
	if len(key) != n {
		t.Errorf("PSK 字节数 = %d, want %d（方法 %s）", len(key), n, ss.Method)
	}
	// 非 2022 方法不受支持，NewSSKey 应报错
	if _, err := NewSSKey("aes-256-gcm"); err == nil {
		t.Error("非 Shadowsocks-2022 方法应报错")
	}
	if _, ok := SSKeyLength("aes-256-gcm"); ok {
		t.Error("非 Shadowsocks-2022 方法不应有密钥长度")
	}
}

// TestVLESSRealityTemplate VLESS 默认开启 Reality 且密钥对可用。
func TestVLESSRealityTemplate(t *testing.T) {
	raw, err := NewTemplateConfig(ProtoVLESS)
	if err != nil {
		t.Fatalf("生成 VLESS 默认配置失败: %v", err)
	}
	cfg := raw.(*VLESSConfig)
	if !cfg.Reality.Enabled {
		t.Error("VLESS 默认应开启 Reality")
	}
	if cfg.Reality.PrivateKey == "" || cfg.Reality.PublicKey == "" {
		t.Errorf("Reality 密钥对不应为空: %+v", cfg.Reality)
	}
	if cfg.Reality.PrivateKey == cfg.Reality.PublicKey {
		t.Error("Reality 私钥与公钥不应相同")
	}
	if cfg.Reality.HandshakeServer == "" || cfg.Reality.HandshakePort == 0 {
		t.Errorf("Reality 握手目标应有默认值: %+v", cfg.Reality)
	}
	if len(cfg.Reality.ShortID) > 16 {
		t.Errorf("short_id 长度不应超过 16 位 hex: %q", cfg.Reality.ShortID)
	}
}

// TestParseInboundConfig 空白配置回落默认值，非法 JSON 报错，未知协议报错。
func TestParseInboundConfig(t *testing.T) {
	for _, protocol := range SupportedProtocols {
		cfg, err := ParseInboundConfig(protocol, "")
		if err != nil {
			t.Errorf("%s 空白配置应回落默认值: %v", protocol, err)
			continue
		}
		if cfg == nil {
			t.Errorf("%s 空白配置不应返回 nil", protocol)
		}
	}
	if _, err := ParseInboundConfig(ProtoTrojan, "{not-json"); err == nil {
		t.Error("非法 JSON 应报错")
	}
	if _, err := ParseInboundConfig("nope", "{}"); err == nil {
		t.Error("未知协议应报错")
	}
}

// TestCredentialRoundTrip NewCredential 与 ParseCredential 各协议往返一致。
func TestCredentialRoundTrip(t *testing.T) {
	tests := []struct {
		protocol     string
		wantUUID     bool
		wantPassword bool
	}{
		{ProtoVLESS, true, false},
		{ProtoVMess, true, false},
		{ProtoHysteria2, false, true},
		{ProtoAnyTLS, false, true},
		{ProtoTrojan, false, true},
		{ProtoTUIC, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			cred, err := NewCredential(tt.protocol)
			if err != nil {
				t.Fatalf("生成凭据失败: %v", err)
			}
			if got := cred.UUID != ""; got != tt.wantUUID {
				t.Errorf("UUID 有无 = %v, want %v: %+v", got, tt.wantUUID, cred)
			}
			if got := cred.Password != ""; got != tt.wantPassword {
				t.Errorf("密码有无 = %v, want %v: %+v", got, tt.wantPassword, cred)
			}
			if tt.wantUUID {
				if _, err := uuid.Parse(cred.UUID); err != nil {
					t.Errorf("UUID 格式不合法: %q", cred.UUID)
				}
			}
			// 两次生成不应重复凭据
			again, err := NewCredential(tt.protocol)
			if err != nil {
				t.Fatalf("再次生成凭据失败: %v", err)
			}
			if again == cred {
				t.Errorf("两次生成的凭据不应完全相同: %+v", cred)
			}
			// JSON 往返
			parsed, err := ParseCredential(tt.protocol, configJSON(t, cred))
			if err != nil {
				t.Fatalf("解析凭据失败: %v", err)
			}
			if parsed != cred {
				t.Errorf("凭据往返不一致: got %+v, want %+v", parsed, cred)
			}
		})
	}

	// 空白凭据按协议默认值补齐
	cred, err := ParseCredential(ProtoVLESS, "  ")
	if err != nil || cred.UUID == "" {
		t.Errorf("空白凭据应按协议生成: %+v, err=%v", cred, err)
	}
	// 非法 JSON 报错
	if _, err := ParseCredential(ProtoVLESS, "{"); err == nil {
		t.Error("非法凭据 JSON 应报错")
	}
	// Shadowsocks 由调用方按方法补齐密钥，这里校验显式密钥往返
	key, err := NewSSKey("2022-blake3-aes-256-gcm")
	if err != nil {
		t.Fatalf("生成 SS 密钥失败: %v", err)
	}
	ssCred := Credential{Password: key}
	parsed, err := ParseCredential(ProtoShadowsocks, configJSON(t, ssCred))
	if err != nil || parsed.Password != key {
		t.Errorf("SS 密钥往返不一致: %+v, err=%v", parsed, err)
	}
	// 未带密钥的 SS 默认凭据为空（由调用方补齐）
	empty, err := NewCredential(ProtoShadowsocks)
	if err != nil || empty.Password != "" {
		t.Errorf("SS 默认凭据应为空密码: %+v, err=%v", empty, err)
	}
}

// TestInboundJSONMultiUser 多用户入站每个用户一条 users 记录，凭据各自独立。
func TestInboundJSONMultiUser(t *testing.T) {
	entry := InboundEntry{
		Tag: "vless-in", Protocol: ProtoVLESS, Listen: ListenLoopback, Port: 10001,
		Users: []InboundUserEntry{
			{Name: "alice", Credential: Credential{UUID: uuid.NewString()}},
			{Name: "bob", Credential: Credential{UUID: uuid.NewString()}},
			{Name: "carol", Credential: Credential{UUID: uuid.NewString()}},
		},
		Cert: stubResolver,
	}
	cfg, err := ParseInboundConfig(ProtoVLESS, configJSON(t, &VLESSConfig{
		Flow:    "xtls-rprx-vision",
		Reality: RealityConfig{Enabled: true, PrivateKey: "pk", ShortID: "0123abcd"},
	}))
	if err != nil {
		t.Fatalf("解析配置失败: %v", err)
	}
	in, err := inboundJSON(entry, cfg)
	if err != nil {
		t.Fatalf("生成入站 JSON 失败: %v", err)
	}
	if in["tag"] != "vless-in" || in["type"] != ProtoVLESS || in["listen_port"] != 10001 {
		t.Errorf("入站基础字段不符: %v", in)
	}
	users := usersOf(t, in)
	if len(users) != 3 {
		t.Fatalf("users 应有 3 条, got %d", len(users))
	}
	for i, want := range []string{"alice", "bob", "carol"} {
		if users[i]["name"] != want {
			t.Errorf("第 %d 个用户名 = %v, want %s", i, users[i]["name"], want)
		}
		if users[i]["uuid"] != entry.Users[i].Credential.UUID {
			t.Errorf("用户 %s 的 UUID 与凭据不一致: %v", want, users[i])
		}
		if users[i]["flow"] != "xtls-rprx-vision" {
			t.Errorf("用户 %s 应继承入站级 flow: %v", want, users[i])
		}
	}
	// 用户级 flow 优先于入站级
	entry.Users[0].Credential.Flow = "custom-flow"
	in, err = inboundJSON(entry, cfg)
	if err != nil {
		t.Fatalf("生成入站 JSON 失败: %v", err)
	}
	if got := usersOf(t, in)[0]["flow"]; got != "custom-flow" {
		t.Errorf("用户级 flow 应优先, got %v", got)
	}
}

// TestInboundJSONErrors 空用户与缺失凭据必须报错。
func TestInboundJSONErrors(t *testing.T) {
	// 空用户
	if _, err := inboundJSON(InboundEntry{Tag: "empty", Protocol: ProtoVLESS}, &VLESSConfig{}); err == nil {
		t.Error("无用户的入站应报错")
	}
	// VLESS 缺 UUID
	_, err := inboundJSON(InboundEntry{
		Tag: "vless-in", Protocol: ProtoVLESS,
		Users: []InboundUserEntry{{Name: "alice"}}, Cert: stubResolver,
	}, &VLESSConfig{})
	if err == nil || !strings.Contains(err.Error(), "UUID") {
		t.Errorf("VLESS 缺 UUID 应报错: %v", err)
	}
	// Hysteria2 缺密码
	_, err = inboundJSON(InboundEntry{
		Tag: "hy2-in", Protocol: ProtoHysteria2,
		Users: []InboundUserEntry{{Name: "alice"}}, Cert: stubResolver,
	}, &Hysteria2Config{TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1}})
	if err == nil || !strings.Contains(err.Error(), "密码") {
		t.Errorf("Hysteria2 缺密码应报错: %v", err)
	}
	// TUIC 缺密码
	_, err = inboundJSON(InboundEntry{
		Tag: "tuic-in", Protocol: ProtoTUIC,
		Users: []InboundUserEntry{{Name: "alice", Credential: Credential{UUID: uuid.NewString()}}},
		Cert:  stubResolver,
	}, &TUICConfig{TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1}})
	if err == nil || !strings.Contains(err.Error(), "密码") {
		t.Errorf("TUIC 缺密码应报错: %v", err)
	}
	// 未提供证书解析器时 managed 证书无法解析
	_, err = inboundJSON(InboundEntry{
		Tag: "trojan-in", Protocol: ProtoTrojan,
		Users: []InboundUserEntry{{Name: "alice", Credential: Credential{Password: "pw"}}},
	}, &TrojanConfig{TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1}})
	if err == nil {
		t.Error("managed 证书缺少解析器应报错")
	}
}

// TestInboundJSONShadowsocks2022 Shadowsocks-2022 入站级 password 为 PSK，用户各自密钥。
func TestInboundJSONShadowsocks2022(t *testing.T) {
	const method = "2022-blake3-aes-128-gcm"
	psk, err := NewSSKey(method)
	if err != nil {
		t.Fatalf("生成 PSK 失败: %v", err)
	}
	key1, _ := NewSSKey(method)
	key2, _ := NewSSKey(method)
	entry := InboundEntry{
		Tag: "ss-in", Protocol: ProtoShadowsocks, Port: 18388,
		Users: []InboundUserEntry{
			{Name: "u1", Credential: Credential{Password: key1}},
			{Name: "u2", Credential: Credential{Password: key2}},
		},
	}
	in, err := inboundJSON(entry, &ShadowsocksConfig{Method: method, PSK: psk})
	if err != nil {
		t.Fatalf("生成 Shadowsocks 入站失败: %v", err)
	}
	if in["method"] != method {
		t.Errorf("method = %v, want %s", in["method"], method)
	}
	if in["password"] != psk {
		t.Errorf("入站 password 应为入站 PSK: got %v, want %s", in["password"], psk)
	}
	users := usersOf(t, in)
	if len(users) != 2 {
		t.Fatalf("users 应有 2 条, got %d", len(users))
	}
	if users[0]["password"] != key1 || users[1]["password"] != key2 {
		t.Errorf("用户应各自持有自己的密钥: %v", users)
	}
	if users[0]["password"] == users[1]["password"] {
		t.Error("不同用户的密钥不应相同")
	}

	// 缺 PSK 必须报错（否则客户端无法拼出「PSK:用户密钥」）
	_, err = inboundJSON(entry, &ShadowsocksConfig{Method: method})
	if err == nil || !strings.Contains(err.Error(), "PSK") {
		t.Errorf("缺 PSK 应报错: %v", err)
	}

	// 非 2022 方法：单用户，入站 password 取首个用户密钥
	single := InboundEntry{
		Tag: "ss-legacy", Protocol: ProtoShadowsocks,
		Users: []InboundUserEntry{{Name: "u1", Credential: Credential{Password: "legacy-key"}}},
	}
	in, err = inboundJSON(single, &ShadowsocksConfig{Method: "aes-128-gcm"})
	if err != nil {
		t.Fatalf("生成非 2022 入站失败: %v", err)
	}
	if in["password"] != "legacy-key" {
		t.Errorf("非 2022 入站 password 应取首个用户密钥: %v", in["password"])
	}
	if _, hasUsers := in["users"]; hasUsers {
		t.Error("非 2022 单用户入站不应输出 users 数组")
	}
}

// TestInboundJSONProtocolFields 各协议的关键连接层字段。
func TestInboundJSONProtocolFields(t *testing.T) {
	// Hysteria2：混淆、带宽、BBR 档位、TLS ALPN
	hy2, err := inboundJSON(InboundEntry{
		Tag: "hy2-in", Protocol: ProtoHysteria2, Port: 8443,
		Users: []InboundUserEntry{{Name: "u1", Credential: Credential{Password: "pw"}}},
		Cert:  stubResolver,
	}, &Hysteria2Config{
		ObfsEnabled: true, ObfsPassword: "obfs", UpMbps: 100, DownMbps: 200,
		BBRProfile: BBRProfileAggressive,
		TLS:        TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1},
	})
	if err != nil {
		t.Fatalf("生成 Hysteria2 入站失败: %v", err)
	}
	obfs, _ := hy2["obfs"].(map[string]any)
	if obfs["type"] != "salamander" || obfs["password"] != "obfs" {
		t.Errorf("obfs 段不符: %v", hy2["obfs"])
	}
	if hy2["up_mbps"] != 100 || hy2["down_mbps"] != 200 {
		t.Errorf("带宽字段不符: %v / %v", hy2["up_mbps"], hy2["down_mbps"])
	}
	if hy2["bbr_profile"] != BBRProfileAggressive {
		t.Errorf("bbr_profile = %v, want %s", hy2["bbr_profile"], BBRProfileAggressive)
	}
	tls, _ := hy2["tls"].(map[string]any)
	if alpn, _ := tls["alpn"].([]string); len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("QUIC 协议 TLS 必须含 h3: %v", tls)
	}
	if usersOf(t, hy2)[0]["password"] != "pw" {
		t.Errorf("Hysteria2 用户密码不符: %v", hy2["users"])
	}

	// TUIC：拥塞控制透传 + ALPN 兜底 h3
	tuic, err := inboundJSON(InboundEntry{
		Tag: "tuic-in", Protocol: ProtoTUIC, Port: 8444,
		Users: []InboundUserEntry{{Name: "u1", Credential: Credential{UUID: uuid.NewString(), Password: "pw"}}},
		Cert:  stubResolver,
	}, &TUICConfig{
		CongestionControl: CongestionCubic,
		TLS:               TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1},
	})
	if err != nil {
		t.Fatalf("生成 TUIC 入站失败: %v", err)
	}
	if tuic["congestion_control"] != CongestionCubic {
		t.Errorf("congestion_control = %v, want %s", tuic["congestion_control"], CongestionCubic)
	}
	tls, _ = tuic["tls"].(map[string]any)
	if alpn, _ := tls["alpn"].([]string); len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("TUIC TLS 必须含 h3: %v", tls)
	}
	tuicUser := usersOf(t, tuic)[0]
	if tuicUser["uuid"] == "" || tuicUser["password"] != "pw" {
		t.Errorf("TUIC 用户应同时带 UUID 与密码: %v", tuicUser)
	}

	// VMess：WS 传输 + TLS
	vmess, err := inboundJSON(InboundEntry{
		Tag: "vmess-in", Protocol: ProtoVMess, Port: 8080,
		Users: []InboundUserEntry{{Name: "u1", Credential: Credential{UUID: uuid.NewString()}}},
		Cert:  stubResolver,
	}, &VMessConfig{WSPath: "/ws", TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1}})
	if err != nil {
		t.Fatalf("生成 VMess 入站失败: %v", err)
	}
	transport, _ := vmess["transport"].(map[string]any)
	if transport["type"] != "ws" || transport["path"] != "/ws" {
		t.Errorf("VMess 传输段不符: %v", vmess["transport"])
	}
	if _, ok := vmess["tls"].(map[string]any); !ok {
		t.Errorf("VMess 应输出 TLS 段: %v", vmess)
	}

	// AnyTLS / Trojan：密码用户
	for _, tc := range []struct {
		protocol string
		cfg      any
	}{
		{ProtoAnyTLS, &AnyTLSConfig{TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1}}},
		{ProtoTrojan, &TrojanConfig{TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1}}},
	} {
		in, err := inboundJSON(InboundEntry{
			Tag: tc.protocol + "-in", Protocol: tc.protocol, Port: 9443,
			Users: []InboundUserEntry{{Name: "u1", Credential: Credential{Password: "pw"}}},
			Cert:  stubResolver,
		}, tc.cfg)
		if err != nil {
			t.Fatalf("生成 %s 入站失败: %v", tc.protocol, err)
		}
		if usersOf(t, in)[0]["password"] != "pw" {
			t.Errorf("%s 用户密码不符: %v", tc.protocol, in["users"])
		}
		if _, ok := in["tls"].(map[string]any); !ok {
			t.Errorf("%s 应输出 TLS 段: %v", tc.protocol, in)
		}
	}
}

// TestInboundWarnings 弱配置警示（不阻断保存）。
func TestInboundWarnings(t *testing.T) {
	// 明文 VLESS
	if list := InboundWarnings(ProtoVLESS, &VLESSConfig{}); len(list) == 0 {
		t.Error("未启用 Reality/TLS 的 VLESS 应给出警示")
	}
	// Reality + TLS 均启用时无警示
	ok := &VLESSConfig{Reality: RealityConfig{Enabled: true}, TLS: TLSConfig{Enabled: true}}
	if list := InboundWarnings(ProtoVLESS, ok); len(list) != 0 {
		t.Errorf("Reality 与 TLS 均启用不应有警示: %v", list)
	}
	// Reality 启用但 TLS 关闭：提示实际会强制启用 TLS，且不得报缺少证书
	if list := InboundWarnings(ProtoVLESS, &VLESSConfig{Reality: RealityConfig{Enabled: true}}); !containsSubstring(list, "强制启用 TLS") {
		t.Errorf("Reality 启用而 TLS 关闭应提示强制启用 TLS: %v", list)
	}
	// QUIC 协议 ALPN 不含 h3
	hy2 := &Hysteria2Config{TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1, ALPN: []string{"h2"}}}
	if list := InboundWarnings(ProtoHysteria2, hy2); !containsSubstring(list, "h3") {
		t.Errorf("ALPN 不含 h3 应给出警示: %v", list)
	}
	// TLS 开启但未配置证书
	found := containsSubstring(InboundWarnings(ProtoTUIC, &TUICConfig{TLS: TLSConfig{Enabled: true}}), "证书")
	if !found {
		t.Error("TLS 已启用但缺证书应给出警示")
	}
}

// containsSubstring 判断警示列表某项包含子串。
func containsSubstring(list []string, sub string) bool {
	for _, item := range list {
		if strings.Contains(item, sub) {
			return true
		}
	}
	return false
}

// inboundWith 用给定配置构造单用户入站条目（渲染断言用）。
func inboundWith(protocol string, port int, cfg any) InboundEntry {
	cred := Credential{Password: "pw"}
	switch protocol {
	case ProtoVLESS, ProtoVMess:
		cred = Credential{UUID: uuid.NewString()}
	case ProtoTUIC:
		cred = Credential{UUID: uuid.NewString(), Password: "pw"}
	}
	return InboundEntry{
		Tag: protocol + "-in", Protocol: protocol, Port: port, Cert: stubResolver,
		Users: []InboundUserEntry{{Name: "u1", Credential: cred}},
	}
}

// connectConfig 各协议的监听层选项（新字段渲染断言用）。
func connectConfig() ConnectConfig {
	return ConnectConfig{TCPFastOpen: true, Multiplex: MultiplexConfig{Enabled: true, Padding: true}}
}

// TestInboundJSONConnectFields 监听层选项（多路复用 / TCP Fast Open）按协议支持情况写入。
func TestInboundJSONConnectFields(t *testing.T) {
	// 支持 multiplex 的四个协议都要写出 multiplex 与 tcp_fast_open
	for _, protocol := range []string{ProtoVLESS, ProtoTrojan, ProtoVMess, ProtoShadowsocks} {
		var cfg any
		switch protocol {
		case ProtoVLESS:
			cfg = &VLESSConfig{
				Reality:       RealityConfig{Enabled: true, PrivateKey: "pk"},
				ConnectConfig: connectConfig(),
			}
		case ProtoTrojan:
			cfg = &TrojanConfig{TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1}, ConnectConfig: connectConfig()}
		case ProtoVMess:
			cfg = &VMessConfig{TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1}, ConnectConfig: connectConfig()}
		default:
			psk, err := NewSSKey("2022-blake3-aes-128-gcm")
			if err != nil {
				t.Fatalf("生成 PSK 失败: %v", err)
			}
			cfg = &ShadowsocksConfig{Method: "2022-blake3-aes-128-gcm", PSK: psk, ConnectConfig: connectConfig()}
		}
		in, err := inboundJSON(inboundWith(protocol, 10001, cfg), cfg)
		if err != nil {
			t.Fatalf("%s 生成入站失败: %v", protocol, err)
		}
		multiplex, ok := in["multiplex"].(map[string]any)
		if !ok {
			t.Fatalf("%s 应输出 multiplex 段: %v", protocol, in)
		}
		if multiplex["enabled"] != true || multiplex["padding"] != true {
			t.Errorf("%s multiplex 段不符: %v", protocol, multiplex)
		}
		if in["tcp_fast_open"] != true {
			t.Errorf("%s 应输出 tcp_fast_open: %v", protocol, in)
		}
	}

	// 内核没有 multiplex 字段的协议：多路复用不写入，TFO 仍透传
	for _, protocol := range []string{ProtoHysteria2, ProtoTUIC, ProtoAnyTLS} {
		var cfg any
		tlsCfg := TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1}
		switch protocol {
		case ProtoHysteria2:
			cfg = &Hysteria2Config{TLS: tlsCfg, ConnectConfig: connectConfig()}
		case ProtoTUIC:
			cfg = &TUICConfig{TLS: tlsCfg, ConnectConfig: connectConfig()}
		default:
			cfg = &AnyTLSConfig{TLS: tlsCfg, ConnectConfig: connectConfig()}
		}
		in, err := inboundJSON(inboundWith(protocol, 10002, cfg), cfg)
		if err != nil {
			t.Fatalf("%s 生成入站失败: %v", protocol, err)
		}
		if _, ok := in["multiplex"]; ok {
			t.Errorf("%s 不应输出 multiplex 段: %v", protocol, in)
		}
		if in["tcp_fast_open"] != true {
			t.Errorf("%s 应输出 tcp_fast_open: %v", protocol, in)
		}
	}
}

// TestInboundJSONTLSAdvanced TLS 版本与 ECH 渲染为内核字段名。
func TestInboundJSONTLSAdvanced(t *testing.T) {
	cfg := &TrojanConfig{TLS: TLSConfig{
		Enabled: true, CertMode: CertModeManaged, CertID: 1,
		MinVersion: TLSVersion12, MaxVersion: TLSVersion13,
		ECH: ECHConfig{Enabled: true, Key: "-----BEGIN ECH KEYS-----"},
	}}
	in, err := inboundJSON(inboundWith(ProtoTrojan, 10003, cfg), cfg)
	if err != nil {
		t.Fatalf("生成 Trojan 入站失败: %v", err)
	}
	tls, _ := in["tls"].(map[string]any)
	if tls["min_version"] != TLSVersion12 || tls["max_version"] != TLSVersion13 {
		t.Errorf("TLS 版本字段不符: %v", tls)
	}
	ech, _ := tls["ech"].(map[string]any)
	if ech["enabled"] != true {
		t.Errorf("ECH 段不符: %v", tls["ech"])
	}
	if key, _ := ech["key"].([]string); len(key) != 1 || key[0] != "-----BEGIN ECH KEYS-----" {
		t.Errorf("ECH 密钥应渲染为字符串数组: %v", ech["key"])
	}

	// 密钥路径写法
	cfg.TLS.ECH = ECHConfig{Enabled: true, KeyPath: "/data/ech.pem"}
	in, err = inboundJSON(inboundWith(ProtoTrojan, 10004, cfg), cfg)
	if err != nil {
		t.Fatalf("生成 Trojan 入站失败: %v", err)
	}
	ech, _ = in["tls"].(map[string]any)["ech"].(map[string]any)
	if ech["key_path"] != "/data/ech.pem" {
		t.Errorf("ECH 密钥路径不符: %v", ech)
	}

	// 启用但既无密钥也无路径：不写 ech 段（内核会拒绝启动），由警示提示
	cfg.TLS.ECH = ECHConfig{Enabled: true}
	in, err = inboundJSON(inboundWith(ProtoTrojan, 10005, cfg), cfg)
	if err != nil {
		t.Fatalf("生成 Trojan 入站失败: %v", err)
	}
	if _, ok := in["tls"].(map[string]any)["ech"]; ok {
		t.Errorf("缺密钥时不应写入 ech 段: %v", in["tls"])
	}
}

// TestInboundJSONTUICAdvanced TUIC zeroRTT / 心跳渲染。
func TestInboundJSONTUICAdvanced(t *testing.T) {
	cfg := &TUICConfig{
		ZeroRTTHandshake: true, Heartbeat: "10s",
		TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1},
	}
	in, err := inboundJSON(inboundWith(ProtoTUIC, 10006, cfg), cfg)
	if err != nil {
		t.Fatalf("生成 TUIC 入站失败: %v", err)
	}
	if in["zero_rtt_handshake"] != true || in["heartbeat"] != "10s" {
		t.Errorf("TUIC 新字段不符: %v / %v", in["zero_rtt_handshake"], in["heartbeat"])
	}
}

// TestInboundJSONMasquerade Hysteria2 masquerade 三种类型渲染。
func TestInboundJSONMasquerade(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name string
		cfg  MasqueradeConfig
		want map[string]any
	}{
		{"file", MasqueradeConfig{Type: MasqueradeFile, Directory: dir},
			map[string]any{"type": MasqueradeFile, "directory": dir}},
		{"proxy", MasqueradeConfig{Type: MasqueradeProxy, ProxyURL: "https://example.com"},
			map[string]any{"type": MasqueradeProxy, "url": "https://example.com"}},
		{"string", MasqueradeConfig{Type: MasqueradeString, Content: "<h1>hi</h1>"},
			map[string]any{"type": MasqueradeString, "status_code": 200, "content": "<h1>hi</h1>"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Hysteria2Config{
				Masquerade: tt.cfg,
				TLS:        TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1},
			}
			in, err := inboundJSON(inboundWith(ProtoHysteria2, 10007, cfg), cfg)
			if err != nil {
				t.Fatalf("生成 Hysteria2 入站失败: %v", err)
			}
			got, _ := in["masquerade"].(map[string]any)
			for key, want := range tt.want {
				if got[key] != want {
					t.Errorf("masquerade[%s] = %v, want %v", key, got[key], want)
				}
			}
		})
	}

	// 未配置时不写 masquerade 段
	cfg := &Hysteria2Config{TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1}}
	in, err := inboundJSON(inboundWith(ProtoHysteria2, 10008, cfg), cfg)
	if err != nil {
		t.Fatalf("生成 Hysteria2 入站失败: %v", err)
	}
	if _, ok := in["masquerade"]; ok {
		t.Errorf("未配置 masquerade 时不应写入: %v", in)
	}
}

// TestInboundJSONShadowsocksNetwork Shadowsocks network 字段透传。
func TestInboundJSONShadowsocksNetwork(t *testing.T) {
	psk, err := NewSSKey("2022-blake3-aes-128-gcm")
	if err != nil {
		t.Fatalf("生成 PSK 失败: %v", err)
	}
	cfg := &ShadowsocksConfig{Method: "2022-blake3-aes-128-gcm", PSK: psk, Network: "tcp"}
	in, err := inboundJSON(inboundWith(ProtoShadowsocks, 10009, cfg), cfg)
	if err != nil {
		t.Fatalf("生成 Shadowsocks 入站失败: %v", err)
	}
	if in["network"] != "tcp" {
		t.Errorf("network = %v, want tcp", in["network"])
	}
	// 留空不写（内核默认 TCP + UDP）
	cfg.Network = ""
	in, err = inboundJSON(inboundWith(ProtoShadowsocks, 10010, cfg), cfg)
	if err != nil {
		t.Fatalf("生成 Shadowsocks 入站失败: %v", err)
	}
	if _, ok := in["network"]; ok {
		t.Errorf("network 留空时不应写入: %v", in)
	}
}

// TestTemplateConfigDefaults 新建默认值口径：多路复用默认关闭、Hysteria2 混淆默认开、
// 拥塞控制与 BBR 档位按协议给默认。
func TestTemplateConfigDefaults(t *testing.T) {
	for _, protocol := range SupportedProtocols {
		cfg, err := NewTemplateConfig(protocol)
		if err != nil {
			t.Fatalf("%s 生成默认配置失败: %v", protocol, err)
		}
		var connect ConnectConfig
		switch c := cfg.(type) {
		case *VLESSConfig:
			connect = c.ConnectConfig
		case *Hysteria2Config:
			connect = c.ConnectConfig
			if !c.ObfsEnabled {
				t.Error("Hysteria2 新建默认应开启混淆")
			}
			if c.ObfsPassword == "" {
				t.Error("Hysteria2 新建默认应有非空混淆密码")
			}
			if c.BBRProfile != BBRProfileStandard {
				t.Errorf("Hysteria2 默认 BBR 档位 = %q, want %q", c.BBRProfile, BBRProfileStandard)
			}
		case *TUICConfig:
			connect = c.ConnectConfig
			if c.CongestionControl != CongestionBBR {
				t.Errorf("TUIC 默认拥塞控制 = %q, want %q", c.CongestionControl, CongestionBBR)
			}
		case *AnyTLSConfig:
			connect = c.ConnectConfig
		case *TrojanConfig:
			connect = c.ConnectConfig
		case *VMessConfig:
			connect = c.ConnectConfig
		case *ShadowsocksConfig:
			connect = c.ConnectConfig
		default:
			t.Fatalf("%s 配置类型不符: %T", protocol, cfg)
		}
		if connect.Multiplex.Enabled {
			t.Errorf("%s 新建默认不应开启多路复用", protocol)
		}
	}
}

// TestParseInboundConfigPreservesStored 已有记录反序列化不受新默认值影响：
// 旧配置 JSON 里没有的字段保持零值（不套用 NewTemplateConfig 的默认）。
func TestParseInboundConfigPreservesStored(t *testing.T) {
	// Hysteria2 旧记录：混淆关闭且密码为空，不得被"默认开启混淆"覆盖
	legacy := `{"obfsEnabled":false,"upMbps":0,"downMbps":0,"bbrProfile":"standard","tls":{"enabled":true,"certMode":"managed","certId":7}}`
	cfg, err := ParseInboundConfig(ProtoHysteria2, legacy)
	if err != nil {
		t.Fatalf("解析旧 Hysteria2 配置失败: %v", err)
	}
	hy2 := cfg.(*Hysteria2Config)
	if hy2.ObfsEnabled || hy2.ObfsPassword != "" {
		t.Errorf("已有记录不应套用新默认混淆: %+v", hy2)
	}
	if hy2.ConnectConfig.Multiplex.Enabled {
		t.Error("已有记录不应自动开启多路复用")
	}

	// TUIC 旧记录：未指定拥塞控制时保持空（内核按自身默认），不套用 bbr
	tuicCfg, err := ParseInboundConfig(ProtoTUIC, `{"tls":{"enabled":true,"certMode":"managed","certId":7}}`)
	if err != nil {
		t.Fatalf("解析旧 TUIC 配置失败: %v", err)
	}
	if got := tuicCfg.(*TUICConfig).CongestionControl; got != "" {
		t.Errorf("已有记录不应套用默认拥塞控制, got %q", got)
	}

	// 往返：新字段落库后再解析保持不变
	src := &Hysteria2Config{
		ObfsEnabled: true, ObfsPassword: "secret",
		Masquerade:    MasqueradeConfig{Type: MasqueradeString, Content: "hi"},
		ConnectConfig: connectConfig(),
		TLS:           TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 7, MinVersion: TLSVersion12},
	}
	round, err := ParseInboundConfig(ProtoHysteria2, configJSON(t, src))
	if err != nil {
		t.Fatalf("解析配置失败: %v", err)
	}
	if got, want := configJSON(t, round), configJSON(t, src); got != want {
		t.Errorf("配置往返不一致:\n got %s\nwant %s", got, want)
	}
}

// TestInboundWarningsAdvanced 新字段的弱配置判定。
func TestInboundWarningsAdvanced(t *testing.T) {
	// ECH 启用但无密钥
	ech := &TrojanConfig{TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1, ECH: ECHConfig{Enabled: true}}}
	if list := InboundWarnings(ProtoTrojan, ech); !containsSubstring(list, "ECH") {
		t.Errorf("ECH 缺密钥应给出警示: %v", list)
	}
	// TLS 版本非法 / 倒挂
	bad := &TrojanConfig{TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1, MinVersion: "1.3", MaxVersion: "1.2"}}
	if list := InboundWarnings(ProtoTrojan, bad); !containsSubstring(list, "最低版本高于最高版本") {
		t.Errorf("TLS 版本倒挂应给出警示: %v", list)
	}
	invalid := &TrojanConfig{TLS: TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1, MinVersion: "2.0"}}
	if list := InboundWarnings(ProtoTrojan, invalid); !containsSubstring(list, "合法值") {
		t.Errorf("非法 TLS 版本应给出警示: %v", list)
	}
	// 协议不支持多路复用却开启了
	unsupported := &AnyTLSConfig{
		TLS:           TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1},
		ConnectConfig: ConnectConfig{Multiplex: MultiplexConfig{Enabled: true}},
	}
	if list := InboundWarnings(ProtoAnyTLS, unsupported); !containsSubstring(list, "多路复用") {
		t.Errorf("不支持多路复用的协议开启多路复用应给出警示: %v", list)
	}
	// 支持多路复用的协议不警示
	supported := &TrojanConfig{
		TLS:           TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1},
		ConnectConfig: ConnectConfig{Multiplex: MultiplexConfig{Enabled: true}},
	}
	if list := InboundWarnings(ProtoTrojan, supported); len(list) != 0 {
		t.Errorf("支持多路复用的协议不应警示: %v", list)
	}
	// masquerade 目录不存在
	hy2 := &Hysteria2Config{
		Masquerade: MasqueradeConfig{Type: MasqueradeFile, Directory: "/nonexistent-masquerade-dir"},
		TLS:        TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1, ALPN: []string{"h3"}},
	}
	if list := InboundWarnings(ProtoHysteria2, hy2); !containsSubstring(list, "masquerade 目录") {
		t.Errorf("masquerade 目录不存在应给出警示: %v", list)
	}
	hy2.Masquerade = MasqueradeConfig{Type: MasqueradeProxy, ProxyURL: "ftp://example.com"}
	if list := InboundWarnings(ProtoHysteria2, hy2); !containsSubstring(list, "http") {
		t.Errorf("masquerade 反代 URL scheme 非法应给出警示: %v", list)
	}
	// Shadowsocks network 非法
	ss := &ShadowsocksConfig{Method: "2022-blake3-aes-128-gcm", Network: "tcp,udp"}
	if list := InboundWarnings(ProtoShadowsocks, ss); !containsSubstring(list, "network") && !containsSubstring(list, "网络") {
		t.Errorf("非法 network 应给出警示: %v", list)
	}
	// TUIC 心跳间隔非法
	tuicBeat := &TUICConfig{
		Heartbeat: "10 秒",
		TLS:       TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 1, ALPN: []string{"h3"}},
	}
	if list := InboundWarnings(ProtoTUIC, tuicBeat); !containsSubstring(list, "心跳间隔") {
		t.Errorf("非法心跳间隔应给出警示: %v", list)
	}
}

// TestVLESSRealityECH Reality 与 ECH 并存时以 Reality 优先（内核不允许两者并存）。
func TestVLESSRealityECH(t *testing.T) {
	cfg := &VLESSConfig{
		Reality: RealityConfig{Enabled: true, PrivateKey: "pk"},
		TLS: TLSConfig{
			Enabled: true, MinVersion: TLSVersion12,
			ECH: ECHConfig{Enabled: true, Key: "-----BEGIN ECH KEYS-----"},
		},
	}
	in, err := inboundJSON(inboundWith(ProtoVLESS, 10011, cfg), cfg)
	if err != nil {
		t.Fatalf("生成 VLESS 入站失败: %v", err)
	}
	tls, _ := in["tls"].(map[string]any)
	if _, ok := tls["ech"]; ok {
		t.Errorf("Reality 启用时不应写入 ech 段: %v", tls)
	}
	if _, ok := tls["reality"]; !ok {
		t.Errorf("Reality 段应存在: %v", tls)
	}
	if list := InboundWarnings(ProtoVLESS, cfg); !containsSubstring(list, "ECH") {
		t.Errorf("Reality + ECH 应给出警示: %v", list)
	}
}

// TestCompleteRealityKeys 补齐启用中的 Reality 缺失密钥材料；已有密钥与其余情形不受影响。
func TestCompleteRealityKeys(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		raw      string
		check    func(t *testing.T, cfg any)
	}{
		{
			name:     "缺私钥时补齐密钥对与短 ID",
			protocol: ProtoVLESS,
			raw:      `{"reality":{"enabled":true,"handshakeServer":"www.microsoft.com","handshakePort":443}}`,
			check: func(t *testing.T, cfg any) {
				r := cfg.(*VLESSConfig).Reality
				if r.PrivateKey == "" || r.PublicKey == "" || r.ShortID == "" {
					t.Errorf("密钥材料未补齐: %+v", r)
				}
			},
		},
		{
			name:     "已有密钥保持原值",
			protocol: ProtoVLESS,
			raw:      `{"reality":{"enabled":true,"privateKey":"priv-keep","publicKey":"pub-keep","shortId":"a1b2"}}`,
			check: func(t *testing.T, cfg any) {
				r := cfg.(*VLESSConfig).Reality
				if r.PrivateKey != "priv-keep" || r.PublicKey != "pub-keep" || r.ShortID != "a1b2" {
					t.Errorf("已有密钥不应被改动: %+v", r)
				}
			},
		},
		{
			name:     "Reality 关闭时不补",
			protocol: ProtoVLESS,
			raw:      `{"reality":{"enabled":false}}`,
			check: func(t *testing.T, cfg any) {
				if r := cfg.(*VLESSConfig).Reality; r.PrivateKey != "" {
					t.Errorf("Reality 关闭不应生成密钥: %+v", r)
				}
			},
		},
		{
			name:     "非 VLESS 协议无操作",
			protocol: ProtoTrojan,
			raw:      `{"tls":{"enabled":false}}`,
			check: func(t *testing.T, cfg any) {
				if _, ok := cfg.(*VLESSConfig); ok {
					t.Error("trojan 配置不应被当作 VLESS 处理")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseInboundConfig(tt.protocol, tt.raw)
			if err != nil {
				t.Fatalf("解析配置失败: %v", err)
			}
			if err := CompleteRealityKeys(cfg); err != nil {
				t.Fatalf("补齐 Reality 密钥失败: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}
