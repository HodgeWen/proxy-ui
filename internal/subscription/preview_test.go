package subscription

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// seedPreviewNode 建一个启用入站与启用用户并绑定到该订阅用户（预览测试种子数据）。
func seedPreviewNode(t *testing.T, database *gorm.DB, tokenID uint) {
	t.Helper()
	cfg, err := singbox.NewTemplateConfig(singbox.ProtoVLESS)
	if err != nil {
		t.Fatal(err)
	}
	rawCfg, _ := json.Marshal(cfg)
	inbound := db.Inbound{
		Name: "preview-in", Protocol: singbox.ProtoVLESS,
		ListenPort: 12500 + int(tokenID), Enabled: true, Config: string(rawCfg),
	}
	if err := database.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	cred, err := singbox.NewCredential(singbox.ProtoVLESS)
	if err != nil {
		t.Fatal(err)
	}
	rawCred, _ := json.Marshal(cred)
	user := db.InboundUser{
		InboundID: inbound.ID, Name: "preview-user-" + string(rune('0'+tokenID)),
		Credential: string(rawCred), Enabled: true,
	}
	if err := database.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&db.SubscriptionNode{
		TokenID: tokenID, InboundID: inbound.ID, InboundUserID: user.ID,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

// TestPreviewIgnoresQuotaAndExpiry 预览跳过停用/配额/到期校验：耗尽、到期且已停用的
// 用户仍渲染出节点；两种格式都能解析；与匿名端点在同一节点集合下输出一致。
func TestPreviewIgnoresQuotaAndExpiry(t *testing.T) {
	s, database := newTestService(t)

	past := time.Now().Add(-time.Hour)
	broken := db.SubscriptionToken{
		Token: "preview-broken", Name: "bob", Revoked: true,
		TrafficLimitBytes: 100, ExpiresAt: &past,
	}
	if err := database.Create(&broken).Error; err != nil {
		t.Fatal(err)
	}
	seedPreviewNode(t, database, broken.ID)
	if err := database.Create(&db.TokenTraffic{TokenID: broken.ID, UploadBytes: 60, DownloadBytes: 60}).Error; err != nil {
		t.Fatal(err)
	}
	// 该状态下匿名端点（Fetch）会被拒绝，预览不受影响
	if _, err := s.Fetch(broken.Token, "example.com"); err == nil {
		t.Fatal("停用/到期/超额令牌走 Fetch 应被拒绝")
	}

	// sing-box：合法 JSON 且服务器地址取请求 Host
	content, err := s.Preview(broken.ID, "example.com:8443", FormatSingBox)
	if err != nil {
		t.Fatalf("预览应忽略停用/配额/到期: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(content, &doc); err != nil {
		t.Fatalf("sing-box 预览应为合法 JSON: %v\n%s", err, content)
	}
	if body := string(content); !strings.Contains(body, `"server": "example.com"`) {
		t.Errorf("sing-box 预览应包含服务器地址:\n%s", body)
	}

	// mihomo：合法 YAML
	content, err = s.Preview(broken.ID, "example.com:8443", FormatClash)
	if err != nil {
		t.Fatalf("mihomo 预览失败: %v", err)
	}
	parseClash(t, content)

	// 不存在的订阅用户
	if _, err := s.Preview(9999, "example.com", FormatSingBox); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("不存在的订阅用户应返回 ErrInvalidToken, got %v", err)
	}
}

// TestPreviewMatchesEndpoint 同一节点集合下，预览输出与匿名订阅端点一致（差异只在校验）。
func TestPreviewMatchesEndpoint(t *testing.T) {
	s, database := newTestService(t)

	valid := db.SubscriptionToken{Token: "preview-valid", Name: "carol"}
	if err := database.Create(&valid).Error; err != nil {
		t.Fatal(err)
	}
	seedPreviewNode(t, database, valid.ID)

	want := doRequest(t, s, "preview-valid", "sing-box", "").Body.String()
	got, err := s.Preview(valid.ID, "example.com:8443", FormatSingBox)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("预览与订阅端点输出不一致:\n预览:\n%s\n端点:\n%s", got, want)
	}
}

// TestPreviewNoNodes 无可用节点时返回带明确原因的 ErrNoNodes。
func TestPreviewNoNodes(t *testing.T) {
	s, database := newTestService(t)

	// 未勾选任何入站
	empty := db.SubscriptionToken{Token: "preview-empty", Name: "eve"}
	if err := database.Create(&empty).Error; err != nil {
		t.Fatal(err)
	}
	_, err := s.Preview(empty.ID, "example.com", FormatSingBox)
	if !errors.Is(err, ErrNoNodes) {
		t.Fatalf("未勾选入站应返回 ErrNoNodes, got %v", err)
	}
	if msg := err.Error(); !strings.Contains(msg, "未勾选") {
		t.Errorf("无节点错误应说明原因: %s", msg)
	}

	// 勾了入站但入站停用、用户被暂停
	cfg, err := singbox.NewTemplateConfig(singbox.ProtoVLESS)
	if err != nil {
		t.Fatal(err)
	}
	rawCfg, _ := json.Marshal(cfg)
	inbound := db.Inbound{Name: "preview-off", Protocol: singbox.ProtoVLESS, ListenPort: 12600, Enabled: true, Config: string(rawCfg)}
	if err := database.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	// enabled 带 default:true 标签，零值插入会被默认值覆盖，停用状态需显式更新
	if err := database.Model(&inbound).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	cred, err := singbox.NewCredential(singbox.ProtoVLESS)
	if err != nil {
		t.Fatal(err)
	}
	rawCred, _ := json.Marshal(cred)
	user := db.InboundUser{InboundID: inbound.ID, Name: "preview-suspended", Credential: string(rawCred), Enabled: true, Suspended: true}
	if err := database.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	stopped := db.SubscriptionToken{Token: "preview-stopped", Name: "mallory"}
	if err := database.Create(&stopped).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&db.SubscriptionNode{TokenID: stopped.ID, InboundID: inbound.ID, InboundUserID: user.ID}).Error; err != nil {
		t.Fatal(err)
	}
	_, err = s.Preview(stopped.ID, "example.com", FormatSingBox)
	if !errors.Is(err, ErrNoNodes) {
		t.Fatalf("入站停用且用户被暂停应返回 ErrNoNodes, got %v", err)
	}
	if msg := err.Error(); !strings.Contains(msg, "停用") || !strings.Contains(msg, "暂停") {
		t.Errorf("无节点错误应说明停用/暂停原因: %s", msg)
	}
}
