package subscription

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// newTestService 以临时数据库构造订阅服务。
func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "sub.db"))
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	return New(database), database
}

// doRequest 直接调用匿名订阅 handler（httptest，不监听端口）。
func doRequest(t *testing.T, s *Service, token, format, ua string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/sub/:token", s.Handle)

	target := "/sub/" + token
	if format != "" {
		target += "?format=" + format
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = "example.com:8443"
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// TestHostOnly 取 Host 头主机名（去端口、IPv6 兼容）。
func TestHostOnly(t *testing.T) {
	tests := map[string]string{
		"example.com:8080": "example.com",
		"example.com":      "example.com",
		"127.0.0.1:9000":   "127.0.0.1",
		"[::1]:8080":       "::1",
		"[::1]":            "::1",
	}
	for in, want := range tests {
		if got := hostOnly(in); got != want {
			t.Errorf("hostOnly(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFetchInvalidToken 不存在或已停用的令牌。
func TestFetchInvalidToken(t *testing.T) {
	s, database := newTestService(t)

	if _, err := s.Fetch("missing", "example.com"); err != ErrInvalidToken {
		t.Errorf("未知令牌应返回 ErrInvalidToken, got %v", err)
	}

	token := db.SubscriptionToken{Token: "revoked-token", Name: "u", Revoked: true}
	if err := database.Create(&token).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Fetch("revoked-token", "example.com"); err != ErrInvalidToken {
		t.Errorf("已停用令牌应返回 ErrInvalidToken, got %v", err)
	}
}

// TestFetchExpiredAndOverQuota 到期与超额都返回 ErrUnavailable。
func TestFetchExpiredAndOverQuota(t *testing.T) {
	s, database := newTestService(t)

	past := time.Now().Add(-time.Hour)
	expired := db.SubscriptionToken{Token: "expired", Name: "u", ExpiresAt: &past}
	if err := database.Create(&expired).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Fetch("expired", "example.com"); err != ErrUnavailable {
		t.Errorf("到期令牌应返回 ErrUnavailable, got %v", err)
	}

	// 超额：令牌 + 绑定 + 用户 + 流量
	inbound := db.Inbound{Name: "vless-x", Protocol: singbox.ProtoVLESS, ListenPort: 12345, Enabled: true, Config: `{}`}
	if err := database.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	cred, err := singbox.NewCredential(singbox.ProtoVLESS)
	if err != nil {
		t.Fatal(err)
	}
	rawCred, _ := json.Marshal(cred)
	user := db.InboundUser{InboundID: inbound.ID, Name: "over-user", Credential: string(rawCred), Enabled: true}
	if err := database.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	over := db.SubscriptionToken{Token: "over", Name: "u", TrafficLimitBytes: 100}
	if err := database.Create(&over).Error; err != nil {
		t.Fatal(err)
	}
	binding := db.SubscriptionNode{TokenID: over.ID, InboundID: inbound.ID, InboundUserID: user.ID}
	if err := database.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}
	// 配额计数在 TokenTraffic（按订阅用户），与用户身份无关
	if err := database.Create(&db.TokenTraffic{TokenID: over.ID, UploadBytes: 60, DownloadBytes: 60}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Fetch("over", "example.com"); err != ErrUnavailable {
		t.Errorf("超额令牌应返回 ErrUnavailable, got %v", err)
	}
}

// TestFetchAssemblesNodes 正常令牌：节点按生效条件过滤，服务器地址取 Host。
func TestFetchAssemblesNodes(t *testing.T) {
	s, database := newTestService(t)

	cfg, err := singbox.NewTemplateConfig(singbox.ProtoVLESS)
	if err != nil {
		t.Fatal(err)
	}
	rawCfg, _ := json.Marshal(cfg)
	disabled := db.Inbound{Name: "off-in", Protocol: singbox.ProtoVLESS, ListenPort: 12346, Enabled: false, Config: string(rawCfg)}
	active := db.Inbound{Name: "on-in", Protocol: singbox.ProtoVLESS, ListenPort: 12347, Enabled: true, Config: string(rawCfg)}
	active2 := db.Inbound{Name: "on-in-2", Protocol: singbox.ProtoVLESS, ListenPort: 12348, Enabled: true, Config: string(rawCfg)}
	active3 := db.Inbound{Name: "on-in-3", Protocol: singbox.ProtoVLESS, ListenPort: 12349, Enabled: true, Config: string(rawCfg)}
	for _, in := range []*db.Inbound{&disabled, &active, &active2, &active3} {
		if err := database.Create(in).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Enabled 带 default:true 标签，零值插入会被默认值覆盖，停用状态需显式更新
	if err := database.Model(&disabled).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}

	cred, err := singbox.NewCredential(singbox.ProtoVLESS)
	if err != nil {
		t.Fatal(err)
	}
	rawCred, _ := json.Marshal(cred)

	token := db.SubscriptionToken{Token: "ok-token", Name: "alice"}
	if err := database.Create(&token).Error; err != nil {
		t.Fatal(err)
	}

	// 四个用户分布在不同的入站上（同一令牌在同一入站只能绑定一个用户）：
	// 停用入站下的用户、被暂停的用户、被停用的用户都应被过滤，只留启用入站下的正常用户
	mkUser := func(inboundID uint, name string, enabled, suspended bool) db.InboundUser {
		t.Helper()
		u := db.InboundUser{InboundID: inboundID, Name: name, Credential: string(rawCred), Suspended: suspended}
		if err := database.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		// enabled 带 default:true 标签，插入零值会被默认值覆盖，需显式更新
		if err := database.Model(&u).Update("enabled", enabled).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Create(&db.SubscriptionNode{TokenID: token.ID, InboundID: inboundID, InboundUserID: u.ID}).Error; err != nil {
			t.Fatal(err)
		}
		return u
	}
	suspended := mkUser(active3.ID, "alice-suspended", true, true)
	mkUser(disabled.ID, "alice-disabled-inbound", true, false)
	mkUser(active2.ID, "alice-disabled-user", false, false)
	ok := mkUser(active.ID, "alice-ok", true, false)

	data, err := s.Fetch("ok-token", "example.com:8443")
	if err != nil {
		t.Fatalf("拉取订阅失败: %v", err)
	}
	if data.Server != "example.com" {
		t.Errorf("服务器地址 = %q, want example.com（去端口）", data.Server)
	}
	if len(data.Nodes) != 1 {
		t.Fatalf("只应包含启用入站下未暂停的用户: %+v", data.Nodes)
	}
	if data.Nodes[0].Tag != ok.Name || data.Nodes[0].Port != active.ListenPort {
		t.Errorf("节点字段不符: %+v", data.Nodes[0])
	}
	if data.Nodes[0].Tag == suspended.Name {
		t.Error("被暂停用户不应出现在订阅中")
	}
	// DNS 未设置时用默认国内外分流配置
	if data.DNS == nil || !data.DNS.Enabled {
		t.Errorf("未设置 DNS 时应使用默认配置: %+v", data.DNS)
	}
	if len(data.Groups) != 1 || data.Groups[0].Name != FallbackGroup {
		t.Errorf("无策略组时应合成兜底组: %+v", data.Groups)
	}

	// 最近使用时间被更新
	var reloaded db.SubscriptionToken
	if err := database.First(&reloaded, token.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.LastUsedAt == nil {
		t.Error("成功拉取后应更新最近使用时间")
	}
}

// TestFetchRuleEntries 规则聚合：规则集规则解析下载源，缺启用项时跳过。
func TestFetchRuleEntries(t *testing.T) {
	s, database := newTestService(t)

	withItem := db.RuleSet{Name: "set-a"}
	noItem := db.RuleSet{Name: "set-b"}
	for _, set := range []*db.RuleSet{&withItem, &noItem} {
		if err := database.Create(set).Error; err != nil {
			t.Fatal(err)
		}
	}
	item := db.RuleSetItem{
		RuleSetID: withItem.ID, SingBoxURL: "https://example.com/a.srs",
		MihomoURL: "https://example.com/a.mrs", MihomoBehavior: "domain", Enabled: true,
	}
	if err := database.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	rules := []db.Rule{
		{MatchType: db.RuleMatchRuleSet, RuleSetID: withItem.ID, Outbound: "block", Enabled: true},
		{MatchType: db.RuleMatchRuleSet, RuleSetID: noItem.ID, Outbound: "block", Enabled: true},
		{MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodSuffix, Value: "cn", Outbound: "direct", Enabled: true},
	}

	entries, err := s.ruleEntries(rules)
	if err != nil {
		t.Fatalf("聚合规则失败: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("无启用项的规则集规则应被跳过, got %d 条: %+v", len(entries), entries)
	}
	if entries[0].RuleSet == nil || entries[0].RuleSet.Tag != "set-a" {
		t.Errorf("规则集下载源不符: %+v", entries[0].RuleSet)
	}
	if entries[0].RuleSet.MihomoURL != item.MihomoURL || entries[0].RuleSet.MihomoBehavior != "domain" {
		t.Errorf("规则集 mihomo 源不符: %+v", entries[0].RuleSet)
	}
	if entries[1].MatchType != db.RuleMatchDomain {
		t.Errorf("普通规则应保留: %+v", entries[1])
	}
}

// TestDNSSettingRoundTrip 面板 DNS 设置被订阅读取，非法 JSON 报错。
func TestDNSSettingRoundTrip(t *testing.T) {
	s, database := newTestService(t)

	// 未设置 → 默认配置
	cfg, err := s.dnsConfig()
	if err != nil || cfg == nil || !cfg.Enabled {
		t.Fatalf("未设置时应返回默认配置: %+v, err=%v", cfg, err)
	}

	custom := singbox.DNSConfig{
		Enabled: true,
		Servers: []singbox.DNSServer{{Tag: "local", Type: singbox.DNSTypeLocal}},
		Final:   "local",
	}
	raw, _ := json.Marshal(custom)
	if err := database.Save(&db.Setting{Key: db.SettingDNS, Value: string(raw)}).Error; err != nil {
		t.Fatal(err)
	}
	cfg, err = s.dnsConfig()
	if err != nil {
		t.Fatalf("读取自定义 DNS 配置失败: %v", err)
	}
	if len(cfg.Servers) != 1 || cfg.Servers[0].Tag != "local" {
		t.Errorf("自定义 DNS 配置未生效: %+v", cfg)
	}

	// 非法 JSON → 报错
	if err := database.Save(&db.Setting{Key: db.SettingDNS, Value: "{not-json"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.dnsConfig(); err == nil {
		t.Error("非法 DNS 配置应报错")
	}
}

// TestHandleSubscription 匿名订阅端点：状态码与响应头。
func TestHandleSubscription(t *testing.T) {
	s, database := newTestService(t)

	cfg, err := singbox.NewTemplateConfig(singbox.ProtoTrojan)
	if err != nil {
		t.Fatal(err)
	}
	rawCfg, _ := json.Marshal(cfg)
	inbound := db.Inbound{Name: "trojan-in", Protocol: singbox.ProtoTrojan, ListenPort: 12400, Enabled: true, Config: string(rawCfg)}
	if err := database.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	cred, err := singbox.NewCredential(singbox.ProtoTrojan)
	if err != nil {
		t.Fatal(err)
	}
	rawCred, _ := json.Marshal(cred)
	user := db.InboundUser{InboundID: inbound.ID, Name: "u1", Credential: string(rawCred), Enabled: true}
	if err := database.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	token := db.SubscriptionToken{Token: "handle-token", Name: "u1"}
	if err := database.Create(&token).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&db.SubscriptionNode{TokenID: token.ID, InboundID: inbound.ID, InboundUserID: user.ID}).Error; err != nil {
		t.Fatal(err)
	}

	// sing-box 格式
	rec := doRequest(t, s, "handle-token", "", "SFM/1.0 sing-box/1.14.4")
	if rec.Code != http.StatusOK {
		t.Fatalf("订阅状态码 = %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Error("应设置 Content-Type")
	}
	if cd := rec.Header().Get("Content-Disposition"); cd == "" {
		t.Error("应设置 Content-Disposition 附件名")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("应设置 no-store 缓存头: %q", rec.Header().Get("Cache-Control"))
	}
	// 节点服务器地址取请求 Host（去端口）
	if body := rec.Body.String(); !strings.Contains(body, `"server": "example.com"`) {
		t.Errorf("节点服务器地址应取请求 Host:\n%s", body)
	}

	// clash 格式
	rec = doRequest(t, s, "handle-token", "clash", "SFM")
	if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("clash 订阅失败: code=%d", rec.Code)
	}

	// 未知 token → 404
	rec = doRequest(t, s, "missing", "", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("未知 token 状态码 = %d, want 404", rec.Code)
	}

	// 超额 → 403
	if err := database.Create(&db.TokenTraffic{TokenID: token.ID, UploadBytes: 10}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&token).Update("traffic_limit_bytes", 5).Error; err != nil {
		t.Fatal(err)
	}
	rec = doRequest(t, s, "handle-token", "", "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("超额订阅状态码 = %d, want 403", rec.Code)
	}
}
