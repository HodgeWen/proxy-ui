package subscription

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"gorm.io/gorm"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// inboundSpecOf 以协议默认配置构造订阅组装入站条目。
func inboundSpecOf(t *testing.T, tag, protocol string, port int) InboundSpec {
	t.Helper()
	cfg, err := singbox.DefaultInboundConfig(protocol)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return InboundSpec{Tag: tag, Protocol: protocol, Port: port, Config: string(raw)}
}

// sampleData 构造含推荐/经典协议入站、模板规则与策略组的订阅数据。
func sampleData(t *testing.T, server string) Data {
	t.Helper()
	inbounds := []InboundSpec{
		inboundSpecOf(t, "vless-main", singbox.ProtoVLESS, 443),
		inboundSpecOf(t, "hy2-main", singbox.ProtoHysteria2, 8443),
		inboundSpecOf(t, "ss-main", singbox.ProtoShadowsocks, 8388),
		inboundSpecOf(t, "direct-in", singbox.ProtoDirect, 1080),
	}
	groups := []singbox.GroupEntry{
		// 成员为服务端自定义出站 tag，客户端不存在 → 回退全部节点
		{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"hk-upstream"}},
	}
	rules := []singbox.RuleEntry{
		{Type: singbox.RuleTypeRuleSet, Value: singbox.AntiADTag, Outbound: singbox.OutboundBlock},
		{Type: singbox.RuleTypeGeosite, Value: "cn", Outbound: "direct"},
		{Type: singbox.RuleTypeGeoIP, Value: "cn", Outbound: "direct"},
	}
	data, err := Assemble(server, inbounds, groups, rules, "proxy", "")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAssemble(t *testing.T) {
	data := sampleData(t, "sub.example.com")
	// Direct 入站无客户端凭据，不产生节点
	if len(data.Nodes) != 3 {
		t.Fatalf("节点数 = %d, want 3（Direct 不导出）: %v", len(data.Nodes), data.Nodes)
	}
	// 策略组成员服务端出站被剔除后回退为全部节点
	if len(data.Groups) != 1 || data.Groups[0].Name != "proxy" {
		t.Fatalf("策略组映射异常: %v", data.Groups)
	}
	if len(data.Groups[0].Members) != 3 || data.Groups[0].Members[0] != "vless-main" {
		t.Fatalf("策略组成员应回退全部节点: %v", data.Groups[0].Members)
	}
	// block/direct 保留，兜底为 proxy
	if data.Rules[0].Outbound != singbox.OutboundBlock || data.Rules[1].Outbound != "direct" {
		t.Fatalf("规则出站映射异常: %v", data.Rules)
	}
	if data.Final != "proxy" {
		t.Fatalf("兜底出站 = %q, want proxy", data.Final)
	}
}

func TestAssembleNoGroupSynthesizesFallback(t *testing.T) {
	data, err := Assemble("s.example.com",
		[]InboundSpec{inboundSpecOf(t, "hy2", singbox.ProtoHysteria2, 8443)},
		nil, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Groups) != 1 || data.Groups[0].Name != FallbackGroup {
		t.Fatalf("无策略组时应合成默认组: %v", data.Groups)
	}
	if data.Final != FallbackGroup {
		t.Fatalf("兜底出站 = %q, want %s", data.Final, FallbackGroup)
	}
}

func TestRenderSingBox(t *testing.T) {
	cfg, err := RenderSingBox(sampleData(t, "sub.example.com"))
	if err != nil {
		t.Fatal(err)
	}
	// 出站 = 3 节点 + 1 策略组 + direct 兜底
	if len(cfg.Outbounds) != 5 {
		t.Fatalf("出站数 = %d, want 5: %v", len(cfg.Outbounds), cfg.Outbounds)
	}
	nodes := map[string]map[string]any{}
	for _, ob := range cfg.Outbounds[:4] {
		nodes[ob["tag"].(string)] = ob
	}
	// VLESS+Reality 节点：uuid/flow/utls/reality 公钥
	vless := nodes["vless-main"]
	if vless["type"] != singbox.ProtoVLESS || vless["server"] != "sub.example.com" || vless["server_port"] != 443 {
		t.Fatalf("vless 节点基础字段异常: %v", vless)
	}
	tls := vless["tls"].(map[string]any)
	reality := tls["reality"].(map[string]any)
	if reality["public_key"] == "" || tls["utls"] == nil {
		t.Fatalf("vless Reality 客户端字段缺失: %v", vless)
	}
	// Reality SNI 必须取握手目标站点，而非节点服务器地址
	if tls["server_name"] != "www.microsoft.com" {
		t.Fatalf("vless Reality server_name = %v, want www.microsoft.com", tls["server_name"])
	}
	// 策略组：自动选择 → urltest
	group := nodes["proxy"]
	if group["type"] != "urltest" {
		t.Fatalf("策略组类型 = %v, want urltest", group["type"])
	}
	// direct 兜底出站收尾
	if last := cfg.Outbounds[len(cfg.Outbounds)-1]; last["tag"] != "direct" || last["type"] != "direct" {
		t.Fatalf("末尾出站应为 direct 兜底: %v", last)
	}
	// 路由：anti-AD reject、geosite-cn 直连、final=proxy
	if cfg.Route == nil || cfg.Route.Final != "proxy" {
		t.Fatalf("路由兜底异常: %v", cfg.Route)
	}
	if len(cfg.Route.Rules) != 3 {
		t.Fatalf("路由规则数 = %d, want 3", len(cfg.Route.Rules))
	}
	if cfg.Route.Rules[0]["action"] != "reject" {
		t.Fatalf("anti-AD 规则应为 reject: %v", cfg.Route.Rules[0])
	}
	if cfg.Route.Rules[1]["outbound"] != "direct" {
		t.Fatalf("geosite-cn 规则应直连: %v", cfg.Route.Rules[1])
	}
	// 规则集：anti-AD 官方源 + direct 下载
	var antiAD *singbox.RuleSetConfig
	for i, rs := range cfg.Route.RuleSet {
		if rs.Tag == singbox.AntiADTag {
			antiAD = &cfg.Route.RuleSet[i]
		}
	}
	if antiAD == nil || antiAD.URL != singbox.DefaultAntiADURL || antiAD.DownloadDetour != "direct" {
		t.Fatalf("anti-AD 规则集异常: %+v", antiAD)
	}
	// 输出可 JSON 序列化（订阅下发格式）
	if _, err := marshalJSON(cfg); err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
}

func TestRenderSingBoxRealitySNIEmptyFallback(t *testing.T) {
	// HandshakeServer 为空时兜底 www.microsoft.com，与 clash 渲染及服务端默认一致
	data := Data{
		Server: "sub.example.com",
		Final:  "direct",
		Nodes: []Node{{
			Tag: "vless-empty", Protocol: singbox.ProtoVLESS, Port: 443,
			Config: &singbox.VLESSConfig{UUID: "u", Reality: singbox.RealityConfig{Enabled: true, PublicKey: "pbk", ShortID: "sid"}},
		}},
	}
	cfg, err := RenderSingBox(data)
	if err != nil {
		t.Fatal(err)
	}
	tls := cfg.Outbounds[0]["tls"].(map[string]any)
	if tls["server_name"] != "www.microsoft.com" {
		t.Fatalf("Reality server_name 兜底 = %v, want www.microsoft.com", tls["server_name"])
	}
}

func TestRenderClash(t *testing.T) {
	out, err := RenderClash(sampleData(t, "sub.example.com"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	// 关键结构：proxies、proxy-groups、rules、anti-AD rule-providers
	for _, key := range []string{"proxies:", "proxy-groups:", "rules:", "rule-providers:", "anti-ad:"} {
		if !strings.Contains(text, key) {
			t.Errorf("clash 配置缺少关键结构 %s:\n%s", key, text)
		}
	}
	// 节点、策略组与规则行
	for _, want := range []string{
		"name: vless-main", "type: vless", "public-key:", "client-fingerprint: chrome",
		"name: hy2-main", "type: hysteria2", "name: ss-main", "cipher: 2022-blake3-aes-128-gcm",
		"name: proxy", "type: url-test",
		"RULE-SET,anti-ad,REJECT", "GEOSITE,cn,DIRECT", "GEOIP,cn,DIRECT", "MATCH,proxy",
		DefaultAntiADClashURL,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("clash 配置缺少 %q:\n%s", want, text)
		}
	}
	// Direct 入站不应出现在节点里
	if strings.Contains(text, "direct-in") {
		t.Errorf("Direct 入站不应导出节点:\n%s", text)
	}
	// 结构可被 YAML 解析且 proxies 数为 3
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("clash 配置 YAML 解析失败: %v\n%s", err, text)
	}
	if proxies := doc["proxies"].([]any); len(proxies) != 3 {
		t.Fatalf("clash proxies 数 = %d, want 3", len(proxies))
	}
	if providers := doc["rule-providers"].(map[string]any); providers["anti-ad"] == nil {
		t.Fatalf("clash rule-providers 缺少 anti-ad: %v", providers)
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name  string
		param string
		ua    string
		want  Format
	}{
		{"参数 clash", "clash", "sing-box/1.11", FormatClash},
		{"参数 mihomo", "mihomo", "", FormatClash},
		{"参数 singbox 覆盖 UA", "singbox", "ClashMeta/1.19", FormatSingBox},
		{"UA clash", "", "clash-verge/v2.0", FormatClash},
		{"UA mihomo", "", "Mihomo/1.19", FormatClash},
		{"UA sing-box", "", "SFM/0.5 sing-box", FormatSingBox},
		{"UA SFI", "", "SFI/1.11", FormatSingBox},
		{"未知 UA 默认 sing-box", "", "Mozilla/5.0", FormatSingBox},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectFormat(tt.param, tt.ua); got != tt.want {
				t.Fatalf("DetectFormat(%q, %q) = %s, want %s", tt.param, tt.ua, got, tt.want)
			}
		})
	}
}

// openTestDB 打开临时数据库并写入订阅样例数据（入站/规则/策略组/令牌）。
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	seed := []db.Inbound{
		{Name: "vless-main", Protocol: singbox.ProtoVLESS, ListenPort: 443, Enabled: true,
			Config: inboundSpecOf(t, "vless-main", singbox.ProtoVLESS, 443).Config},
		{Name: "hy2-main", Protocol: singbox.ProtoHysteria2, ListenPort: 8443, Enabled: true,
			Config: inboundSpecOf(t, "hy2-main", singbox.ProtoHysteria2, 8443).Config},
		{Name: "off-in", Protocol: singbox.ProtoTrojan, ListenPort: 9443, Enabled: false,
			Config: inboundSpecOf(t, "off-in", singbox.ProtoTrojan, 9443).Config},
	}
	for i := range seed {
		if err := database.Create(&seed[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Enabled 零值会被 gorm default:true 吞掉，停用须显式 Update
	if err := database.Model(&db.Inbound{}).Where("name = ?", "off-in").Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	rows := []any{
		&db.Rule{Type: singbox.RuleTypeRuleSet, Value: singbox.AntiADTag, Outbound: singbox.OutboundBlock, Order: 1, Enabled: true},
		&db.Rule{Type: singbox.RuleTypeGeosite, Value: "cn", Outbound: "direct", Order: 2, Enabled: true},
		&db.PolicyGroup{Name: "proxy", Mode: singbox.GroupModeAuto, Members: `["hk-upstream"]`, Enabled: true},
		&db.Setting{Key: db.SettingRouteFinal, Value: "proxy"},
		&db.SubscriptionToken{Token: "tok-active", Name: "手机"},
		&db.SubscriptionToken{Token: "tok-revoked", Name: "旧设备", Revoked: true},
	}
	for _, r := range rows {
		if err := database.Create(r).Error; err != nil {
			t.Fatal(err)
		}
	}
	return database
}

func TestFetch(t *testing.T) {
	s := New(openTestDB(t))

	// 错误/吊销令牌
	if _, err := s.Fetch("no-such-token", "h.example.com"); err != ErrInvalidToken {
		t.Fatalf("错误令牌应返回 ErrInvalidToken, got %v", err)
	}
	if _, err := s.Fetch("tok-revoked", "h.example.com"); err != ErrInvalidToken {
		t.Fatalf("吊销令牌应返回 ErrInvalidToken, got %v", err)
	}

	// 未设置服务器地址：取请求 Host（含端口时去端口）
	data, err := s.Fetch("tok-active", "panel.example.com:8080")
	if err != nil {
		t.Fatal(err)
	}
	if data.Server != "panel.example.com" {
		t.Fatalf("服务器地址 = %q, want panel.example.com", data.Server)
	}
	if len(data.Nodes) != 2 {
		t.Fatalf("节点数 = %d, want 2（停用入站不计）", len(data.Nodes))
	}

	// 设置服务器地址后优先使用
	if err := s.db.Save(&db.Setting{Key: db.SettingSubServer, Value: "vpn.example.com"}).Error; err != nil {
		t.Fatal(err)
	}
	data, err = s.Fetch("tok-active", "panel.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if data.Server != "vpn.example.com" {
		t.Fatalf("服务器地址 = %q, want vpn.example.com", data.Server)
	}

	// 最近使用时间被更新
	var record db.SubscriptionToken
	if err := s.db.First(&record, "token = ?", "tok-active").Error; err != nil {
		t.Fatal(err)
	}
	if record.LastUsedAt == nil {
		t.Fatal("最近使用时间未更新")
	}

	// 两格式均可渲染
	if _, err := Render(data, FormatSingBox); err != nil {
		t.Fatalf("sing-box 渲染失败: %v", err)
	}
	if _, err := Render(data, FormatClash); err != nil {
		t.Fatalf("clash 渲染失败: %v", err)
	}
}
