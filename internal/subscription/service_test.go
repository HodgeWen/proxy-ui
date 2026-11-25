package subscription

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
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
	return New(database, t.TempDir()), database
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

// TestSubscriptionHeaders 响应头订阅名称（不含 config.yaml/config.json）、Profile-Title 与 Subscription-Userinfo。
func TestSubscriptionHeaders(t *testing.T) {
	s, database := newTestService(t)
	cfg, _ := singbox.NewTemplateConfig(singbox.ProtoVLESS)
	rawCfg, _ := json.Marshal(cfg)
	inbound := db.Inbound{Name: "v-in", Protocol: singbox.ProtoVLESS, ListenPort: 12401, Enabled: true, Config: string(rawCfg)}
	if err := database.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	cred, _ := singbox.NewCredential(singbox.ProtoVLESS)
	rawCred, _ := json.Marshal(cred)

	mkToken := func(tok, name string, limit int64, exp *time.Time) db.SubscriptionToken {
		t.Helper()
		rec := db.SubscriptionToken{Token: tok, Name: name, TrafficLimitBytes: limit, ExpiresAt: exp}
		if err := database.Create(&rec).Error; err != nil {
			t.Fatal(err)
		}
		u := db.InboundUser{InboundID: inbound.ID, Name: "u-" + tok, Credential: string(rawCred), Enabled: true}
		if err := database.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Create(&db.SubscriptionNode{TokenID: rec.ID, InboundID: inbound.ID, InboundUserID: u.ID}).Error; err != nil {
			t.Fatal(err)
		}
		return rec
	}

	future := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	asciiTok := mkToken("tok-ascii", "alice-phone", 1024, &future)
	if err := database.Create(&db.TokenTraffic{TokenID: asciiTok.ID, UploadBytes: 100, DownloadBytes: 200}).Error; err != nil {
		t.Fatal(err)
	}

	rec := doRequest(t, s, "tok-ascii", "clash", "clash-verge-rev")
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d: %s", rec.Code, rec.Body.String())
	}
	cd := rec.Header().Get("Content-Disposition")
	if cd != "attachment; filename*=UTF-8''alice-phone" {
		t.Errorf("ASCII Content-Disposition = %q, want %q", cd, "attachment; filename*=UTF-8''alice-phone")
	}
	if strings.Contains(cd, "config.yaml") || strings.Contains(cd, "config.json") {
		t.Errorf("Content-Disposition 不得包含 config.yaml/config.json: %q", cd)
	}
	wantPT := "base64:" + base64.StdEncoding.EncodeToString([]byte("alice-phone"))
	if got := rec.Header().Get("Profile-Title"); got != wantPT {
		t.Errorf("Profile-Title = %q, want %q", got, wantPT)
	}
	ui := rec.Header().Get("Subscription-Userinfo")
	for _, part := range []string{"upload=100", "download=200", "total=1024", "expire="} {
		if !strings.Contains(ui, part) {
			t.Errorf("Subscription-Userinfo 缺少 %q: %q", part, ui)
		}
	}

	// 中文名使用 RFC 5987 filename*=UTF-8''...，且不带 config.*
	mkToken("tok-zh", "我的节点", 0, nil)
	rec = doRequest(t, s, "tok-zh", "sing-box", "SFM")
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d: %s", rec.Code, rec.Body.String())
	}
	cd = rec.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment; filename*=UTF-8''") || strings.Contains(cd, "config.json") {
		t.Errorf("中文 Content-Disposition 不符: %q", cd)
	}
	wantPT = "base64:" + base64.StdEncoding.EncodeToString([]byte("我的节点"))
	if got := rec.Header().Get("Profile-Title"); got != wantPT {
		t.Errorf("中文 Profile-Title = %q, want %q", got, wantPT)
	}
	ui = rec.Header().Get("Subscription-Userinfo")
	if ui != "upload=0; download=0" {
		t.Errorf("不限流量无过期时 Subscription-Userinfo = %q, want %q", ui, "upload=0; download=0")
	}

	// 空备注名回退请求 Host
	mkToken("tok-empty", "   ", 0, nil)
	rec = doRequest(t, s, "tok-empty", "clash", "")
	if cd = rec.Header().Get("Content-Disposition"); cd != "attachment; filename*=UTF-8''example.com" {
		t.Errorf("空备注应回退到 Host: %q", cd)
	}
}

// writeTestCertFile 生成自签或 CA 签发证书文件（测试证书检测用）。
func writeTestCertFile(t *testing.T, path string, selfSigned bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "node.example.com"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"node.example.com"},
	}
	var parentTpl *x509.Certificate
	var parentKey any
	if selfSigned {
		parentTpl = leafTpl
		parentKey = leafKey
	} else {
		caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		parentTpl = &x509.Certificate{
			SerialNumber:          big.NewInt(1),
			Subject:               pkix.Name{CommonName: "Test Root CA"},
			NotBefore:             time.Now().Add(-time.Hour),
			NotAfter:              time.Now().Add(48 * time.Hour),
			KeyUsage:              x509.KeyUsageCertSign,
			BasicConstraintsValid: true,
			IsCA:                  true,
		}
		parentKey = caKey
	}
	der, err := x509.CreateCertificate(rand.Reader, leafTpl, parentTpl, &leafKey.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestSelfSignedCertAutoInsecure 托管/本地自签证书自动下发 insecure / skip-cert-verify，CA 签发证书不下发。
func TestSelfSignedCertAutoInsecure(t *testing.T) {
	s, database := newTestService(t)
	selfFile := "self-cert.pem"
	caFile := "ca-cert.pem"
	writeTestCertFile(t, filepath.Join(s.dataDir, "certs", selfFile), true)
	writeTestCertFile(t, filepath.Join(s.dataDir, "certs", caFile), false)
	pathSelfFile := filepath.Join(t.TempDir(), "path-self.pem")
	writeTestCertFile(t, pathSelfFile, true)

	selfCert := db.Certificate{Name: "self", Mode: singbox.CertModeManaged, CertPath: selfFile, KeyPath: "k1.pem"}
	caCert := db.Certificate{Name: "ca", Mode: singbox.CertModeManaged, CertPath: caFile, KeyPath: "k2.pem"}
	for _, c := range []*db.Certificate{&selfCert, &caCert} {
		if err := database.Create(c).Error; err != nil {
			t.Fatal(err)
		}
	}

	hySelfCfg, _ := json.Marshal(&singbox.Hysteria2Config{
		TLS: singbox.TLSConfig{Enabled: true, CertMode: singbox.CertModeManaged, CertID: selfCert.ID, ServerName: "node.example.com"},
	})
	hyCACfg, _ := json.Marshal(&singbox.Hysteria2Config{
		TLS: singbox.TLSConfig{Enabled: true, CertMode: singbox.CertModeManaged, CertID: caCert.ID, ServerName: "node.example.com"},
	})
	trojanPathCfg, _ := json.Marshal(&singbox.TrojanConfig{
		TLS: singbox.TLSConfig{Enabled: true, CertMode: singbox.CertModePath, CertPath: pathSelfFile, KeyPath: "/tmp/k.pem"},
	})
	inSelf := db.Inbound{Name: "hy-self", Protocol: singbox.ProtoHysteria2, ListenPort: 12501, Enabled: true, Config: string(hySelfCfg)}
	inCA := db.Inbound{Name: "hy-ca", Protocol: singbox.ProtoHysteria2, ListenPort: 12502, Enabled: true, Config: string(hyCACfg)}
	inPath := db.Inbound{Name: "tr-path", Protocol: singbox.ProtoTrojan, ListenPort: 12503, Enabled: true, Config: string(trojanPathCfg)}
	for _, in := range []*db.Inbound{&inSelf, &inCA, &inPath} {
		if err := database.Create(in).Error; err != nil {
			t.Fatal(err)
		}
	}

	tok := db.SubscriptionToken{Token: "cert-tok", Name: "cert-user"}
	if err := database.Create(&tok).Error; err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct {
		inID uint
		name string
		pw   string
	}{
		{inSelf.ID, "n-self", "pw1"},
		{inCA.ID, "n-ca", "pw2"},
		{inPath.ID, "n-path", "pw3"},
	} {
		cred, _ := json.Marshal(singbox.Credential{Password: pair.pw})
		u := db.InboundUser{InboundID: pair.inID, Name: pair.name, Credential: string(cred), Enabled: true}
		if err := database.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Create(&db.SubscriptionNode{TokenID: tok.ID, InboundID: pair.inID, InboundUserID: u.ID}).Error; err != nil {
			t.Fatal(err)
		}
	}

	// sing-box 订阅
	rec := doRequest(t, s, "cert-tok", "sing-box", "SFM")
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d: %s", rec.Code, rec.Body.String())
	}
	var sbDoc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &sbDoc); err != nil {
		t.Fatal(err)
	}
	obs := map[string]map[string]any{}
	for _, item := range sbDoc["outbounds"].([]any) {
		ob := item.(map[string]any)
		if tag, ok := ob["tag"].(string); ok {
			obs[tag] = ob
		}
	}
	if tls, _ := obs["n-self"]["tls"].(map[string]any); tls["insecure"] != true {
		t.Errorf("managed 自签证书节点应带 tls.insecure=true: %v", tls)
	}
	if tls, _ := obs["n-path"]["tls"].(map[string]any); tls["insecure"] != true {
		t.Errorf("path 自签证书节点应带 tls.insecure=true: %v", tls)
	}
	if tls, _ := obs["n-ca"]["tls"].(map[string]any); tls["insecure"] != nil {
		t.Errorf("CA 签发证书节点不应带 tls.insecure: %v", tls)
	}

	// clash 订阅
	rec = doRequest(t, s, "cert-tok", "clash", "Mihomo")
	if rec.Code != http.StatusOK {
		t.Fatalf("clash 状态码 = %d: %s", rec.Code, rec.Body.String())
	}
	doc := parseClash(t, rec.Body.Bytes())
	if p := findProxy(t, doc, "n-self"); p["skip-cert-verify"] != true {
		t.Errorf("clash 自签节点应带 skip-cert-verify=true: %v", p)
	}
	if p := findProxy(t, doc, "n-path"); p["skip-cert-verify"] != true {
		t.Errorf("clash path 自签节点应带 skip-cert-verify=true: %v", p)
	}
	if p := findProxy(t, doc, "n-ca"); p["skip-cert-verify"] != nil {
		t.Errorf("clash CA 证书节点不应带 skip-cert-verify: %v", p)
	}
}

// TestNormalizeIPv6Host IPv6 入口规范化：支持纯 IPv6（含方括号）/ 纯 AAAA 域名 / 空串，拒绝 IPv4 与带端口。
func TestNormalizeIPv6Host(t *testing.T) {
	valid := map[string]string{
		"":                   "",
		"  ":                 "",
		"2001:0db8::1":       "2001:db8::1",
		"[2001:0db8::1]":     "2001:db8::1",
		"V6.Example.COM":     "v6.example.com",
		"ipv6-only.test.org": "ipv6-only.test.org",
	}
	for in, want := range valid {
		got, err := NormalizeIPv6Host(in)
		if err != nil {
			t.Errorf("NormalizeIPv6Host(%q) unexpected err: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeIPv6Host(%q) = %q, want %q", in, got, want)
		}
	}

	invalid := []string{
		"1.2.3.4",
		"::ffff:192.0.2.1",
		"[2001:db8::1]:443",
		"[2001:db8::1",
		"v6.example.com:443",
		"https://v6.example.com",
		"v6.example.com/path",
		"2001:db8::1/64",
		"fe80::1%eth0",
		"bad host",
		"-bad.example.com",
		"bad..example.com",
	}
	for _, in := range invalid {
		if got, err := NormalizeIPv6Host(in); err == nil {
			t.Errorf("NormalizeIPv6Host(%q) 应报错, got %q", in, got)
		}
	}
}

// TestFetchDualStackIPv6Nodes 配置 server_ipv6 后为双栈(::)入站生成 -ipv6 节点，跳过 0.0.0.0 与 127.0.0.1，
// 且当请求 Host 已是同一个 IPv6 入口时不重复生成。
func TestFetchDualStackIPv6Nodes(t *testing.T) {
	s, database := newTestService(t)
	cfg, _ := singbox.NewTemplateConfig(singbox.ProtoVLESS)
	rawCfg, _ := json.Marshal(cfg)
	cred, _ := singbox.NewCredential(singbox.ProtoVLESS)
	rawCred, _ := json.Marshal(cred)

	inDual := db.Inbound{Name: "vless-dual", Protocol: singbox.ProtoVLESS, Listen: singbox.ListenAll, ListenPort: 12601, Enabled: true, Config: string(rawCfg)}
	inV4 := db.Inbound{Name: "vless-v4", Protocol: singbox.ProtoVLESS, Listen: singbox.ListenIPv4, ListenPort: 12602, Enabled: true, Config: string(rawCfg)}
	inLocal := db.Inbound{Name: "vless-local", Protocol: singbox.ProtoVLESS, Listen: singbox.ListenLoopback, ListenPort: 12603, Enabled: true, Config: string(rawCfg)}
	for _, in := range []*db.Inbound{&inDual, &inV4, &inLocal} {
		if err := database.Create(in).Error; err != nil {
			t.Fatal(err)
		}
	}

	tok := db.SubscriptionToken{Token: "dual-tok", Name: "alice"}
	if err := database.Create(&tok).Error; err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		inID uint
		name string
	}{
		{inDual.ID, "alice-dual"},
		{inV4.ID, "alice-v4"},
		{inLocal.ID, "alice-local"},
	} {
		u := db.InboundUser{InboundID: item.inID, Name: item.name, Credential: string(rawCred), Enabled: true}
		if err := database.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Create(&db.SubscriptionNode{TokenID: tok.ID, InboundID: item.inID, InboundUserID: u.ID}).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := database.Save(&db.Setting{Key: db.SettingServerIPv6, Value: "2001:db8::1"}).Error; err != nil {
		t.Fatal(err)
	}

	data, err := s.Fetch("dual-tok", "example.com:8443")
	if err != nil {
		t.Fatalf("Fetch 失败: %v", err)
	}
	if len(data.Nodes) != 4 {
		t.Fatalf("应生成 3 个主节点 + 1 个双栈 -ipv6 节点 = 4 个, got %d: %+v", len(data.Nodes), data.Nodes)
	}
	if data.Nodes[0].Tag != "alice-dual" || data.Nodes[1].Tag != "alice-dual-ipv6" {
		t.Fatalf("双栈 -ipv6 节点应紧随主节点之后: %+v", data.Nodes)
	}
	if !data.Nodes[1].IPv6 || data.Nodes[1].Server != "2001:db8::1" {
		t.Errorf("IPv6 节点属性不符: %+v", data.Nodes[1])
	}
	if data.Nodes[2].Tag != "alice-v4" || data.Nodes[3].Tag != "alice-local" {
		t.Errorf("仅 IPv4 与本机反代入站不应生成 -ipv6 节点: %+v", data.Nodes)
	}

	// 请求 Host 与 server_ipv6 指向同一 IPv6 地址（含未压缩形式与端口）时，不重复生成
	sameData, err := s.Fetch("dual-tok", "[2001:0db8::1]:8443")
	if err != nil {
		t.Fatalf("同 IPv6 Host Fetch 失败: %v", err)
	}
	if len(sameData.Nodes) != 3 {
		t.Errorf("请求 Host 与 server_ipv6 相同时不应重复追加 -ipv6 节点, got %d", len(sameData.Nodes))
	}
}
