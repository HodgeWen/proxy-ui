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
)

// SupportedProtocols 面板支持的入站协议（推荐组排前）。
var SupportedProtocols = []string{
	ProtoVLESS, ProtoHysteria2, ProtoTUIC, ProtoAnyTLS,
	ProtoTrojan, ProtoVMess, ProtoShadowsocks,
}

// 监听地址语义常量。
const (
	ListenAll      = "::"        // 双栈监听（IPv4 + IPv6）
	ListenIPv4     = "0.0.0.0"   // 仅 IPv4
	ListenLoopback = "127.0.0.1" // 仅本机（走反向代理）
	DefaultListen  = ListenAll   // 默认双栈

	// 随机端口段：避开常见服务端口段与 443，降低与既有服务冲突的概率。
	RandomPortMin = 10000
	RandomPortMax = 60000
)

// 证书引用方式常量。
const (
	CertModeManaged = "managed" // 面板上传/粘贴存储（落数据目录）
	CertModePath    = "path"    // 服务器本地路径引用
)

// Hysteria2 BBR 配置档位（sing-box 1.14+）。
const (
	BBRProfileConservative = "conservative" // 保守：低带宽/弱机器
	BBRProfileStandard     = "standard"     // 标准（默认）
	BBRProfileAggressive   = "aggressive"   // 激进：高带宽/低丢包
)

// SupportedBBRProfiles Hysteria2 BBR 配置档位列表。
var SupportedBBRProfiles = []string{BBRProfileConservative, BBRProfileStandard, BBRProfileAggressive}

// 拥塞控制算法常量（TUIC）。
const (
	CongestionBBR     = "bbr" // BBR（默认，高丢包链路表现最好）
	CongestionCubic   = "cubic"
	CongestionNewReno = "new_reno"
)

// SupportedCongestionControls TUIC 拥塞控制算法列表。
var SupportedCongestionControls = []string{CongestionBBR, CongestionCubic, CongestionNewReno}

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

// RealityConfig Reality 配置（VLESS 专用，无需面板证书；入站级共享）。
type RealityConfig struct {
	Enabled         bool   `json:"enabled"`                   // 是否启用 Reality
	PrivateKey      string `json:"privateKey,omitempty"`      // x25519 私钥 base64(raw url)
	PublicKey       string `json:"publicKey,omitempty"`       // 对应公钥（客户端订阅使用，内核不需要）
	ShortID         string `json:"shortId,omitempty"`         // hex 短 ID（0-8 位）
	HandshakeServer string `json:"handshakeServer,omitempty"` // 握手目标站点，默认 www.microsoft.com
	HandshakePort   int    `json:"handshakePort,omitempty"`   // 握手目标端口，默认 443
}

// VLESSConfig VLESS 入站连接层配置（用户凭据在 InboundUser.Credential）。
type VLESSConfig struct {
	Flow    string        `json:"flow,omitempty"` // 流控，默认 xtls-rprx-vision
	Reality RealityConfig `json:"reality"`        // Reality 配置
	TLS     TLSConfig     `json:"tls"`            // 普通 TLS（Reality 关闭时使用）
}

// Hysteria2Config Hysteria2 入站连接层配置。
type Hysteria2Config struct {
	ObfsEnabled           bool      `json:"obfsEnabled,omitempty"`           // 是否启用 salamander 混淆
	ObfsPassword          string    `json:"obfsPassword,omitempty"`          // 混淆密码（入站级共享）
	UpMbps                int       `json:"upMbps"`                          // 上行带宽上限（Mbps），0 为不限
	DownMbps              int       `json:"downMbps"`                        // 下行带宽上限（Mbps），0 为不限
	IgnoreClientBandwidth bool      `json:"ignoreClientBandwidth,omitempty"` // 忽略客户端带宽设置（要求客户端用 BBR）
	BBRProfile            string    `json:"bbrProfile,omitempty"`            // BBR 档位，默认 standard
	TLS                   TLSConfig `json:"tls"`                             // TLS（必须启用，ALPN 默认 h3）
}

// TUICConfig TUIC v5 入站连接层配置。
type TUICConfig struct {
	CongestionControl string    `json:"congestionControl,omitempty"` // 拥塞控制：bbr/cubic/new_reno，默认 bbr
	TLS               TLSConfig `json:"tls"`                         // TLS（必须启用，ALPN 默认 h3）
}

// AnyTLSConfig AnyTLS 入站连接层配置。
type AnyTLSConfig struct {
	TLS TLSConfig `json:"tls"` // TLS（必须启用）
}

// TrojanConfig Trojan 入站连接层配置。
type TrojanConfig struct {
	TLS TLSConfig `json:"tls"` // TLS
}

// VMessConfig VMess（WS + TLS）入站连接层配置。
type VMessConfig struct {
	WSPath string    `json:"wsPath,omitempty"` // WebSocket 路径，默认 /
	TLS    TLSConfig `json:"tls"`              // TLS
}

// ShadowsocksConfig Shadowsocks-2022 入站连接层配置。
type ShadowsocksConfig struct {
	Method string `json:"method,omitempty"` // 加密方法，默认 2022-blake3-aes-128-gcm
	// PSK 入站级预共享密钥（服务端身份）。Shadowsocks-2022 多用户模式下客户端密码为
	// 「PSK:用户密钥」，因此它必须在入站上固定下来，而不能每次生成。
	PSK string `json:"psk,omitempty"`
}

// Credential 入站内单用户凭据（按协议只填相关字段）。
type Credential struct {
	UUID     string `json:"uuid,omitempty"`     // VLESS / TUIC / VMess
	Password string `json:"password,omitempty"` // Hysteria2 / TUIC / AnyTLS / Trojan / Shadowsocks
	Flow     string `json:"flow,omitempty"`     // VLESS 用户级流控（缺省用入站级）
}

// NewTemplateConfig 生成协议默认连接层配置（TLS/Reality/混淆/算法），不含任何用户凭据。
func NewTemplateConfig(protocol string) (any, error) {
	managedTLS := TLSConfig{Enabled: true, CertMode: CertModeManaged}
	switch protocol {
	case ProtoVLESS:
		priv, pub, err := generateRealityKeypair()
		if err != nil {
			return nil, err
		}
		return &VLESSConfig{
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
		// QUIC 类协议 ALPN 必须含 h3，默认即给上，避免保存后握手失败
		return &Hysteria2Config{
			ObfsEnabled:  true,
			ObfsPassword: randBase64(16),
			BBRProfile:   BBRProfileStandard,
			TLS:          TLSConfig{Enabled: true, CertMode: CertModeManaged, ALPN: []string{"h3"}},
		}, nil
	case ProtoTUIC:
		return &TUICConfig{
			CongestionControl: CongestionBBR,
			TLS:               TLSConfig{Enabled: true, CertMode: CertModeManaged, ALPN: []string{"h3"}},
		}, nil
	case ProtoAnyTLS:
		return &AnyTLSConfig{TLS: managedTLS}, nil
	case ProtoTrojan:
		return &TrojanConfig{TLS: managedTLS}, nil
	case ProtoVMess:
		return &VMessConfig{WSPath: "/", TLS: managedTLS}, nil
	case ProtoShadowsocks:
		const method = "2022-blake3-aes-128-gcm"
		psk, err := NewSSKey(method)
		if err != nil {
			return nil, err
		}
		return &ShadowsocksConfig{Method: method, PSK: psk}, nil
	}
	return nil, fmt.Errorf("不支持的入站协议: %s", protocol)
}

// ParseInboundConfig 按协议解析连接层配置 JSON；空白配置返回协议默认值。
func ParseInboundConfig(protocol, raw string) (any, error) {
	dst := newConfigOf(protocol)
	if dst == nil {
		return nil, fmt.Errorf("不支持的入站协议: %s", protocol)
	}
	if strings.TrimSpace(raw) == "" {
		return NewTemplateConfig(protocol)
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		return nil, fmt.Errorf("入站配置 JSON 不合法: %w", err)
	}
	return dst, nil
}

// newConfigOf 按协议返回空连接层配置指针；不支持的协议返回 nil。
func newConfigOf(protocol string) any {
	switch protocol {
	case ProtoVLESS:
		return &VLESSConfig{}
	case ProtoHysteria2:
		return &Hysteria2Config{}
	case ProtoTUIC:
		return &TUICConfig{}
	case ProtoAnyTLS:
		return &AnyTLSConfig{}
	case ProtoTrojan:
		return &TrojanConfig{}
	case ProtoVMess:
		return &VMessConfig{}
	case ProtoShadowsocks:
		return &ShadowsocksConfig{}
	}
	return nil
}

// NewCredential 为某用户在指定入站下生成独立凭据。
func NewCredential(protocol string) (Credential, error) {
	switch protocol {
	case ProtoVLESS, ProtoVMess:
		return Credential{UUID: uuid.NewString()}, nil
	case ProtoTUIC:
		return Credential{UUID: uuid.NewString(), Password: randBase64(16)}, nil
	case ProtoHysteria2, ProtoAnyTLS, ProtoTrojan:
		return Credential{Password: randBase64(16)}, nil
	case ProtoShadowsocks:
		// 密钥长度取决于入站方法，由调用方按方法补齐（见 SSKeyLength）
		return Credential{Password: ""}, nil
	}
	return Credential{}, fmt.Errorf("不支持的入站协议: %s", protocol)
}

// ParseCredential 解析用户凭据 JSON；空白返回该协议的默认凭据。
func ParseCredential(protocol, raw string) (Credential, error) {
	if strings.TrimSpace(raw) == "" {
		return NewCredential(protocol)
	}
	var cred Credential
	if err := json.Unmarshal([]byte(raw), &cred); err != nil {
		return Credential{}, fmt.Errorf("用户凭据 JSON 不合法: %w", err)
	}
	return cred, nil
}

// SSMethodMultiUser 判断 Shadowsocks 方法是否支持多用户（sing-box 的多用户服务只支持两种 AES 方法）。
func SSMethodMultiUser(method string) bool {
	switch method {
	case "2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm":
		return true
	}
	return false
}

// SSKeyLength 返回 Shadowsocks-2022 方法所需密钥字节数。
func SSKeyLength(method string) (int, bool) {
	switch method {
	case "2022-blake3-aes-128-gcm":
		return 16, true
	case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		return 32, true
	}
	return 0, false
}

// NewSSKey 生成 Shadowsocks 指定方法所需长度的 base64 密钥。
func NewSSKey(method string) (string, error) {
	n, ok := SSKeyLength(method)
	if !ok {
		return "", fmt.Errorf("不支持的 Shadowsocks 方法: %s", method)
	}
	return randBytesBase64(n), nil
}

// InboundTLS 返回入站配置中的 TLS 段（Shadowsocks 等无 TLS 概念的协议返回 nil）。
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

// QUICProtocol 是否为 QUIC 类协议（ALPN 必须含 h3）。
func QUICProtocol(protocol string) bool {
	return protocol == ProtoHysteria2 || protocol == ProtoTUIC
}

// InboundWarnings 弱配置判定：不阻断保存，但以明确警示返回（明文传输、缺证书、ALPN 缺 h3 等）。
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
		if !c.TLS.Enabled {
			add("Hysteria2 必须启用 TLS，当前已关闭")
		} else {
			c.TLS.certWarnings(add)
			c.TLS.alpnWarn(add)
		}
		if c.ObfsEnabled && c.ObfsPassword == "" {
			add("已启用混淆但混淆密码为空")
		}
		if c.UpMbps > 0 && c.DownMbps > 0 {
			add("已设置带宽上限，客户端将无法使用 BBR（如需 BBR 请保持带宽不限）")
		}
	case *TUICConfig:
		if !c.TLS.Enabled {
			add("TUIC 必须启用 TLS，当前已关闭")
		} else {
			c.TLS.certWarnings(add)
			c.TLS.alpnWarn(add)
		}
	case *AnyTLSConfig:
		if !c.TLS.Enabled {
			add("AnyTLS 必须启用 TLS，当前已关闭")
		} else {
			c.TLS.certWarnings(add)
		}
	case *TrojanConfig:
		if !c.TLS.Enabled {
			add("Trojan 未启用 TLS，密码将明文传输")
		} else {
			c.TLS.certWarnings(add)
		}
	case *VMessConfig:
		if !c.TLS.Enabled {
			add("VMess 未启用 TLS，流量将明文传输")
		} else {
			c.TLS.certWarnings(add)
		}
	case *ShadowsocksConfig:
		if !strings.HasPrefix(c.Method, "2022-") {
			add("非 Shadowsocks-2022 加密方法，强度较弱")
		}
	}
	return warnings
}

// alpnWarn QUIC 类协议配置了 ALPN 但不含 h3 时提示（内核会补默认值，但客户端未必对齐）。
func (t *TLSConfig) alpnWarn(add func(string, ...any)) {
	if len(t.ALPN) == 0 {
		return
	}
	for _, a := range t.ALPN {
		if a == "h3" {
			return
		}
	}
	add("QUIC 类协议 ALPN 需含 h3，当前配置不含 h3")
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
func randBase64(n int) string { return randBytesBase64(n) }

// randBytesBase64 生成 n 字节安全随机数的 base64 字符串。
func randBytesBase64(n int) string {
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
