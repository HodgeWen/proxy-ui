package singbox

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// 入站协议常量。
const (
	ProtoVLESS       = "vless"       // VLESS（推荐搭配 Reality）
	ProtoHysteria2   = "hysteria2"   // Hysteria2
	ProtoTUIC        = "tuic"        // TUIC v5
	ProtoAnyTLS      = "anytls"      // AnyTLS
	ProtoTrojan      = "trojan"      // Trojan
	ProtoVMess       = "vmess"       // VMess（WS + TLS）
	ProtoShadowsocks = "shadowsocks" // Shadowsocks-2022
	ProtoDirect      = "direct"      // Direct
)

// 证书引用方式常量。
const (
	CertModeManaged = "managed" // 面板上传/粘贴存储（落数据目录）
	CertModePath    = "path"    // 服务器本地路径引用
)

// SupportedProtocols 面板支持的入站协议（推荐组排前）。
var SupportedProtocols = []string{
	ProtoVLESS, ProtoHysteria2, ProtoTUIC, ProtoAnyTLS,
	ProtoTrojan, ProtoVMess, ProtoShadowsocks, ProtoDirect,
}

// TLSConfig TLS 类入站通用配置；证书二选一：面板存储证书（CertID）或服务器本地路径。
type TLSConfig struct {
	Enabled    bool     `json:"enabled"`              // 是否启用 TLS
	ServerName string   `json:"serverName,omitempty"` // 服务器名称（SNI）
	ALPN       []string `json:"alpn,omitempty"`       // ALPN 协议列表
	CertMode   string   `json:"certMode,omitempty"`   // 证书引用方式：managed / path
	CertID     uint     `json:"certId,omitempty"`     // managed：面板存储证书 ID
	CertPath   string   `json:"certPath,omitempty"`   // path：证书文件路径
	KeyPath    string   `json:"keyPath,omitempty"`    // path：私钥文件路径
}

// RealityConfig Reality 配置（VLESS 专用，无需面板证书）。
type RealityConfig struct {
	Enabled         bool   `json:"enabled"`                   // 是否启用 Reality
	PrivateKey      string `json:"privateKey,omitempty"`      // x25519 私钥 base64(raw url)
	PublicKey       string `json:"publicKey,omitempty"`       // 对应公钥（客户端/订阅使用，内核不需要）
	ShortID         string `json:"shortId,omitempty"`         // hex 短 ID（0-8 位）
	HandshakeServer string `json:"handshakeServer,omitempty"` // 握手目标站点，默认 www.microsoft.com
	HandshakePort   int    `json:"handshakePort,omitempty"`   // 握手目标端口，默认 443
}

// VLESSConfig VLESS 入站字段模型。
type VLESSConfig struct {
	UUID    string        `json:"uuid"`           // 用户 UUID
	Flow    string        `json:"flow,omitempty"` // 流控，默认 xtls-rprx-vision
	Reality RealityConfig `json:"reality"`        // Reality 配置
	TLS     TLSConfig     `json:"tls"`            // 普通TLS（Reality 关闭时使用）
}

// Hysteria2Config Hysteria2 入站字段模型。
type Hysteria2Config struct {
	Password     string    `json:"password"`               // 认证密码
	ObfsPassword string    `json:"obfsPassword,omitempty"` // salamander 混淆密码，空为不启用
	UpMbps       int       `json:"upMbps,omitempty"`       // 上行带宽上限（Mbps），0 为不限
	DownMbps     int       `json:"downMbps,omitempty"`     // 下行带宽上限（Mbps），0 为不限
	TLS          TLSConfig `json:"tls"`                    // TLS（必须启用）
}

// TUICConfig TUIC v5 入站字段模型。
type TUICConfig struct {
	UUID              string    `json:"uuid"`                        // 用户 UUID
	Password          string    `json:"password"`                    // 认证密码
	CongestionControl string    `json:"congestionControl,omitempty"` // 拥塞控制：cubic/new_reno/bbr，默认 bbr
	TLS               TLSConfig `json:"tls"`                         // TLS（必须启用，ALPN 默认 h3）
}

// AnyTLSConfig AnyTLS 入站字段模型。
type AnyTLSConfig struct {
	Password string    `json:"password"` // 认证密码
	TLS      TLSConfig `json:"tls"`      // TLS（必须启用）
}

// TrojanConfig Trojan 入站字段模型。
type TrojanConfig struct {
	Password string    `json:"password"` // 认证密码
	TLS      TLSConfig `json:"tls"`      // TLS
}

// VMessConfig VMess（WS + TLS）入站字段模型。
type VMessConfig struct {
	UUID   string    `json:"uuid"`             // 用户 UUID
	WSPath string    `json:"wsPath,omitempty"` // WebSocket 路径，默认 /
	TLS    TLSConfig `json:"tls"`              // TLS
}

// ShadowsocksConfig Shadowsocks-2022 入站字段模型。
type ShadowsocksConfig struct {
	Method   string `json:"method,omitempty"` // 加密方法，默认 2022-blake3-aes-128-gcm
	Password string `json:"password"`         // 密钥（2022 方法为 base64 编码的定长密钥）
}

// DirectConfig Direct 入站字段模型（无协议特定字段）。
type DirectConfig struct{}

// DefaultInboundConfig 生成带随机凭据的协议默认配置（证书引用默认 managed 且待选择，保存时按弱配置警示提示）。
func DefaultInboundConfig(protocol string) (any, error) {
	managedTLS := TLSConfig{Enabled: true, CertMode: CertModeManaged}
	switch protocol {
	case ProtoVLESS:
		priv, pub, err := generateRealityKeypair()
		if err != nil {
			return nil, err
		}
		return &VLESSConfig{
			UUID: uuid.NewString(),
			Flow: "xtls-rprx-vision",
			Reality: RealityConfig{
				Enabled:         true,
				PrivateKey:      priv,
				PublicKey:       pub,
				ShortID:         randHex(4),
				HandshakeServer: "www.microsoft.com",
				HandshakePort:   443,
			},
			TLS: TLSConfig{Enabled: true, ServerName: "www.microsoft.com"},
		}, nil
	case ProtoHysteria2:
		return &Hysteria2Config{Password: randBase64(16), TLS: managedTLS}, nil
	case ProtoTUIC:
		return &TUICConfig{
			UUID:              uuid.NewString(),
			Password:          randBase64(16),
			CongestionControl: "bbr",
			TLS:               TLSConfig{Enabled: true, CertMode: CertModeManaged, ALPN: []string{"h3"}},
		}, nil
	case ProtoAnyTLS:
		return &AnyTLSConfig{Password: randBase64(16), TLS: managedTLS}, nil
	case ProtoTrojan:
		return &TrojanConfig{Password: randBase64(16), TLS: managedTLS}, nil
	case ProtoVMess:
		return &VMessConfig{UUID: uuid.NewString(), WSPath: "/", TLS: managedTLS}, nil
	case ProtoShadowsocks:
		return &ShadowsocksConfig{Method: "2022-blake3-aes-128-gcm", Password: randBase64(16)}, nil
	case ProtoDirect:
		return &DirectConfig{}, nil
	}
	return nil, fmt.Errorf("不支持的入站协议: %s", protocol)
}

// ParseInboundConfig 按协议解析入站配置 JSON；空白配置返回该协议默认配置。
func ParseInboundConfig(protocol, raw string) (any, error) {
	var dst any
	switch protocol {
	case ProtoVLESS:
		dst = &VLESSConfig{}
	case ProtoHysteria2:
		dst = &Hysteria2Config{}
	case ProtoTUIC:
		dst = &TUICConfig{}
	case ProtoAnyTLS:
		dst = &AnyTLSConfig{}
	case ProtoTrojan:
		dst = &TrojanConfig{}
	case ProtoVMess:
		dst = &VMessConfig{}
	case ProtoShadowsocks:
		dst = &ShadowsocksConfig{}
	case ProtoDirect:
		dst = &DirectConfig{}
	default:
		return nil, fmt.Errorf("不支持的入站协议: %s", protocol)
	}
	if strings.TrimSpace(raw) == "" {
		return DefaultInboundConfig(protocol)
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		return nil, fmt.Errorf("入站配置 JSON 不合法: %w", err)
	}
	return dst, nil
}

// InboundTLS 返回入站配置中的 TLS 段（Direct/Shadowsocks 等无 TLS 概念的协议返回 nil）。
func InboundTLS(cfg any) *TLSConfig {
	switch c := cfg.(type) {
	case *VLESSConfig:
		return &c.TLS
	case *Hysteria2Config:
		return &c.TLS
	case *TUICConfig:
		return &c.TLS
	case *AnyTLSConfig:
		return &c.TLS
	case *TrojanConfig:
		return &c.TLS
	case *VMessConfig:
		return &c.TLS
	}
	return nil
}

// InboundWarnings 弱配置判定：不阻断保存，但以明确警示返回（明文传输、空密码、弱密钥、缺证书等）。
func InboundWarnings(protocol string, cfg any) []string {
	var warnings []string
	add := func(format string, args ...any) { warnings = append(warnings, fmt.Sprintf(format, args...)) }
	switch c := cfg.(type) {
	case *VLESSConfig:
		switch {
		case !c.Reality.Enabled && !c.TLS.Enabled:
			add("未启用 Reality 或 TLS，流量将明文传输")
		case !c.Reality.Enabled:
			add("未启用 Reality（推荐）")
			c.TLS.certWarnings(add)
		case !c.TLS.Enabled:
			add("Reality 已启用，将忽略 TLS 开关强制启用 TLS（无需证书）")
		}
	case *Hysteria2Config:
		if c.Password == "" {
			add("Hysteria2 密码为空")
		}
		if !c.TLS.Enabled {
			add("Hysteria2 必须启用 TLS，当前已关闭")
		} else {
			c.TLS.certWarnings(add)
		}
	case *TUICConfig:
		if c.UUID == "" || c.Password == "" {
			add("UUID 或密码为空")
		}
		if !c.TLS.Enabled {
			add("TUIC 必须启用 TLS，当前已关闭")
		} else {
			c.TLS.certWarnings(add)
		}
	case *AnyTLSConfig:
		if c.Password == "" {
			add("AnyTLS 密码为空")
		}
		if !c.TLS.Enabled {
			add("AnyTLS 必须启用 TLS，当前已关闭")
		} else {
			c.TLS.certWarnings(add)
		}
	case *TrojanConfig:
		if c.Password == "" {
			add("Trojan 密码为空")
		}
		if !c.TLS.Enabled {
			add("Trojan 未启用 TLS，密码将明文传输")
		} else {
			c.TLS.certWarnings(add)
		}
	case *VMessConfig:
		if c.UUID == "" {
			add("UUID 为空")
		}
		if !c.TLS.Enabled {
			add("VMess 未启用 TLS，流量将明文传输")
		} else {
			c.TLS.certWarnings(add)
		}
	case *ShadowsocksConfig:
		switch {
		case c.Password == "":
			add("密钥为空")
		case !strings.HasPrefix(c.Method, "2022-"):
			add("非 Shadowsocks-2022 加密方法，强度较弱")
		default:
			n, _ := ss2022KeyLength(c.Method) // 2022- 前缀方法均在表内
			if keyMismatch(c.Password, n) {
				add("SS2022 密钥强度不足，需 base64 编码的 %d 字节密钥", n)
			}
		}
	}
	return warnings
}

// certWarnings 已启用 TLS 时校验证书引用是否齐备。
func (t *TLSConfig) certWarnings(add func(string, ...any)) {
	missing := t.CertMode == "" ||
		(t.CertMode == CertModeManaged && t.CertID == 0) ||
		(t.CertMode == CertModePath && (t.CertPath == "" || t.KeyPath == ""))
	if missing {
		add("TLS 已启用但未配置证书，应用配置将失败")
	}
}

// keyMismatch 判断 base64 密钥解码后长度是否与方法要求不符（解码失败视为不合规）。
func keyMismatch(password string, want int) bool {
	raw, err := base64.StdEncoding.DecodeString(password)
	return err != nil || len(raw) != want
}

// ss2022KeyLength 返回 Shadowsocks-2022 方法所需密钥字节数。
func ss2022KeyLength(method string) (int, bool) {
	switch method {
	case "2022-blake3-aes-128-gcm":
		return 16, true
	case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		return 32, true
	}
	return 0, false
}

// inboundJSON 把解析后的协议配置映射为 sing-box inbound JSON 对象。
func inboundJSON(e InboundEntry, cfg any) (map[string]any, error) {
	in := map[string]any{"type": e.Protocol, "tag": e.Tag, "listen": "::", "listen_port": e.ListenPort}
	switch c := cfg.(type) {
	case *VLESSConfig:
		user := map[string]any{"name": e.Tag, "uuid": c.UUID}
		if c.Flow != "" {
			user["flow"] = c.Flow
		}
		in["users"] = []any{user}
		// Reality 隐含 TLS：开启时即使 TLS 开关关闭也强制产出 tls 段，且无需面板证书。
		tls, err := c.TLS.tlsJSON(c.Reality.Enabled, !c.Reality.Enabled, e.Cert)
		if err != nil {
			return nil, err
		}
		if c.Reality.Enabled {
			tls["reality"] = c.Reality.json()
		}
		if tls != nil {
			in["tls"] = tls
		}
	case *Hysteria2Config:
		in["users"] = []any{map[string]any{"name": e.Tag, "password": c.Password}}
		if c.ObfsPassword != "" {
			in["obfs"] = map[string]any{"type": "salamander", "password": c.ObfsPassword}
		}
		if c.UpMbps > 0 {
			in["up_mbps"] = c.UpMbps
		}
		if c.DownMbps > 0 {
			in["down_mbps"] = c.DownMbps
		}
		tls, err := c.TLS.tlsJSON(true, true, e.Cert)
		if err != nil {
			return nil, err
		}
		in["tls"] = tls
	case *TUICConfig:
		in["users"] = []any{map[string]any{"name": e.Tag, "uuid": c.UUID, "password": c.Password}}
		if c.CongestionControl != "" {
			in["congestion_control"] = c.CongestionControl
		}
		tls, err := c.TLS.tlsJSON(true, true, e.Cert)
		if err != nil {
			return nil, err
		}
		if _, ok := tls["alpn"]; !ok {
			tls["alpn"] = []string{"h3"}
		}
		in["tls"] = tls
	case *AnyTLSConfig:
		in["users"] = []any{map[string]any{"name": e.Tag, "password": c.Password}}
		tls, err := c.TLS.tlsJSON(true, true, e.Cert)
		if err != nil {
			return nil, err
		}
		in["tls"] = tls
	case *TrojanConfig:
		in["users"] = []any{map[string]any{"name": e.Tag, "password": c.Password}}
		tls, err := c.TLS.tlsJSON(false, true, e.Cert)
		if err != nil {
			return nil, err
		}
		if tls != nil {
			in["tls"] = tls
		}
	case *VMessConfig:
		in["users"] = []any{map[string]any{"name": e.Tag, "uuid": c.UUID}}
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
	case *ShadowsocksConfig:
		method := c.Method
		if method == "" {
			method = "2022-blake3-aes-128-gcm"
		}
		in["method"] = method
		in["password"] = c.Password
	case *DirectConfig:
		// 无协议特定字段
	default:
		return nil, fmt.Errorf("入站 %s: 配置类型不符: %T", e.Tag, cfg)
	}
	return in, nil
}

// json 渲染 sing-box Reality 配置段。
func (r RealityConfig) json() map[string]any {
	server, port := r.HandshakeServer, r.HandshakePort
	if server == "" {
		server, port = "www.microsoft.com", 443
	}
	reality := map[string]any{
		"enabled":     true,
		"handshake":   map[string]any{"server": server, "server_port": port},
		"private_key": r.PrivateKey,
	}
	if r.ShortID != "" {
		reality["short_id"] = []string{r.ShortID}
	}
	return reality
}

// tlsJSON 渲染 sing-box TLS 对象；forceEnabled 用于必须启用 TLS 的协议，
// requireCert 为 false 时（Reality 场景）不解析证书。未启用返回 nil。
func (t *TLSConfig) tlsJSON(forceEnabled, requireCert bool, resolve CertResolver) (map[string]any, error) {
	if !t.Enabled && !forceEnabled {
		return nil, nil
	}
	tls := map[string]any{"enabled": true}
	if t.ServerName != "" {
		tls["server_name"] = t.ServerName
	}
	if len(t.ALPN) > 0 {
		tls["alpn"] = t.ALPN
	}
	if requireCert {
		certPath, keyPath, err := t.resolveCert(resolve)
		if err != nil {
			return nil, err
		}
		tls["certificate_path"] = certPath
		tls["key_path"] = keyPath
	}
	return tls, nil
}

// resolveCert 解析证书引用为证书/私钥文件路径。
func (t *TLSConfig) resolveCert(resolve CertResolver) (string, string, error) {
	switch t.CertMode {
	case CertModeManaged:
		if resolve == nil {
			return "", "", fmt.Errorf("未提供面板证书解析器")
		}
		if t.CertID == 0 {
			return "", "", fmt.Errorf("未选择面板存储证书")
		}
		return resolve(t.CertID)
	case CertModePath:
		if t.CertPath == "" || t.KeyPath == "" {
			return "", "", fmt.Errorf("本地证书路径不完整")
		}
		return t.CertPath, t.KeyPath, nil
	}
	return "", "", fmt.Errorf("未配置证书")
}

// generateRealityKeypair 生成 Reality x25519 密钥对（base64 raw url 编码，与 sing-box 一致）。
func generateRealityKeypair() (private, public string, err error) {
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("生成 Reality 密钥对失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(key.Bytes()),
		base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), nil
}

// randBase64 生成 n 字节安全随机数的 base64 字符串。
func randBase64(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(err) // 系统随机源不可用属致命错误
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// randHex 生成 n 字节安全随机数的 hex 字符串。
func randHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(err) // 系统随机源不可用属致命错误
	}
	return hex.EncodeToString(buf)
}
