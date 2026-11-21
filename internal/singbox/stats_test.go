package singbox

import (
	"path/filepath"
	"testing"
	"time"

	"proxy-ui/internal/db"
)

// newStatsMonitor 以临时数据库构造统计采集器（不启动内核）。
func newStatsMonitor(t *testing.T) *StatsMonitor {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	return NewStatsMonitor(New(t.TempDir()), database)
}

// trafficOf 读取某 tag 的累计流量。
func trafficOf(t *testing.T, s *StatsMonitor, tag string) db.UserTraffic {
	t.Helper()
	var row db.UserTraffic
	if err := s.db.First(&row, "tag = ?", tag).Error; err != nil {
		t.Fatalf("查询流量 %s 失败: %v", tag, err)
	}
	return row
}

// tokenTrafficOf 读取某订阅用户的配额计数行。
func tokenTrafficOf(t *testing.T, s *StatsMonitor, tokenID uint) db.TokenTraffic {
	t.Helper()
	var row db.TokenTraffic
	if err := s.db.First(&row, "token_id = ?", tokenID).Error; err != nil {
		t.Fatalf("查询订阅流量 %d 失败: %v", tokenID, err)
	}
	return row
}

// bindUser 在某入站下建一个用户；tokenID > 0 时同时建立订阅绑定（绑定按 token+入站唯一）。
func bindUser(t *testing.T, s *StatsMonitor, name string, inboundID, tokenID uint) {
	t.Helper()
	u := db.InboundUser{InboundID: inboundID, Name: name, Credential: "{}", Enabled: true}
	if err := s.db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	if tokenID > 0 {
		binding := db.SubscriptionNode{TokenID: tokenID, InboundID: inboundID, InboundUserID: u.ID}
		if err := s.db.Create(&binding).Error; err != nil {
			t.Fatal(err)
		}
	}
}

// TestAccumulateFirstRound 首轮采集：计数器绝对值全部计入累计（落库窗口到期批量落库）。
func TestAccumulateFirstRound(t *testing.T) {
	s := newStatsMonitor(t)
	s.accumulate(map[string]CounterSet{
		"alice": {Up: 100, Down: 200},
		"bob":   {Up: 50, Down: 60},
	})
	s.flushPending()

	if got := trafficOf(t, s, "alice"); got.UploadBytes != 100 || got.DownloadBytes != 200 {
		t.Errorf("alice 累计流量不符: %+v", got)
	}
	if got := trafficOf(t, s, "bob"); got.UploadBytes != 50 || got.DownloadBytes != 60 {
		t.Errorf("bob 累计流量不符: %+v", got)
	}

	// 速率样本只累计用户维度增量（入站维度不参与，避免同一份字节被算两遍）
	rates, at := s.Rates()
	if rates.Up != 150 || rates.Down != 260 {
		t.Errorf("速率样本不符: %+v", rates)
	}
	if at.IsZero() {
		t.Error("速率样本应记录时间")
	}
}

// TestAccumulateDiff 第二轮采集只累加增量，计数器回绕不会写出负数；
// 同一落库窗口内的多轮差分合并为一次落库。
func TestAccumulateDiff(t *testing.T) {
	s := newStatsMonitor(t)
	s.accumulate(map[string]CounterSet{"alice": {Up: 100, Down: 200}})
	// 同一窗口内第二次读数增长：两轮 delta 合并，一次落库
	s.accumulate(map[string]CounterSet{"alice": {Up: 130, Down: 250}})
	s.flushPending()
	got := trafficOf(t, s, "alice")
	if got.UploadBytes != 130 || got.DownloadBytes != 250 {
		t.Errorf("两轮差分合并落库后累计流量不符: %+v", got)
	}

	// 计数器重置（实例重建）时增量按 0 计，不产生负数
	s.reset() // 清空差分基线（待落库增量是已测得的流量，不受影响）
	if _, at := s.Rates(); !at.IsZero() {
		t.Errorf("reset 后应清空速率样本时间: %v", at)
	}
	s.accumulate(map[string]CounterSet{"alice": {Up: 5, Down: 7}})
	s.flushPending()
	got = trafficOf(t, s, "alice")
	if got.UploadBytes != 135 || got.DownloadBytes != 257 {
		t.Errorf("基线清空后应按新绝对值累加: %+v", got)
	}
}

// TestAccumulateNormalizesLongInterval 跨周期采样（如 RPC 超时 4s 后）按实际经过秒数归一化速率，
// 避免出现 4 倍速率尖峰；累计落库流量仍按真实增量保存。
func TestAccumulateNormalizesLongInterval(t *testing.T) {
	s := newStatsMonitor(t)
	s.accumulate(map[string]CounterSet{"alice": {Up: 100, Down: 200}})
	s.mu.Lock()
	s.lastAt = time.Now().Add(-4 * time.Second)
	s.mu.Unlock()

	s.accumulate(map[string]CounterSet{"alice": {Up: 4100, Down: 8200}})
	rates, at := s.Rates()
	if at.IsZero() {
		t.Fatal("速率样本应记录时间")
	}
	if rates.Up < 900 || rates.Up > 1050 {
		t.Errorf("上行速率应按约 4 秒归一化到 ~1000 B/s, got %d", rates.Up)
	}
	if rates.Down < 1800 || rates.Down > 2100 {
		t.Errorf("下行速率应按约 4 秒归一化到 ~2000 B/s, got %d", rates.Down)
	}

	s.flushPending()
	got := trafficOf(t, s, "alice")
	if got.UploadBytes != 4100 || got.DownloadBytes != 8200 {
		t.Errorf("累计落库流量不应受速率归一化影响: %+v", got)
	}
}

// TestAccumulateIgnoresUnknown 非 traffic 计数器被忽略。
func TestAccumulateIgnoresUnknown(t *testing.T) {
	s := newStatsMonitor(t)
	// 无法解析的名字由 userCounter 过滤；accumulate 只接受已解析结果
	s.accumulate(map[string]CounterSet{})
	var count int64
	if err := s.db.Model(&db.UserTraffic{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("无法解析的计数器不应落库, got %d 行", count)
	}
}

// TestAccumulateEmpty 空输入不落库、不清基线。
func TestAccumulateEmpty(t *testing.T) {
	s := newStatsMonitor(t)
	s.accumulate(nil)
	s.flushPending()
	var count int64
	if err := s.db.Model(&db.UserTraffic{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("空输入不应落库: %d 行", count)
	}
}

// TestFlushRetriesWithoutLoss 落库失败时增量并回内存，故障期间继续累计，库恢复后一轮补齐，不丢数。
func TestFlushRetriesWithoutLoss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	s := NewStatsMonitor(New(t.TempDir()), database)
	s.accumulate(map[string]CounterSet{"alice": {Up: 10, Down: 20}})
	s.accumulate(map[string]CounterSet{"alice": {Up: 15, Down: 25}}) // 窗口内累计 15/25

	// 库不可用：本轮落库失败，增量并回内存
	sqlDB, _ := database.DB()
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	s.flushPending()
	s.mu.Lock()
	pending := s.pending["alice"]
	s.mu.Unlock()
	if pending != (CounterSet{Up: 15, Down: 25}) {
		t.Errorf("落库失败后增量应并回内存: %+v", pending)
	}

	// 故障期间采集继续，与未落库增量在内存合并（再累计 15/25）
	s.accumulate(map[string]CounterSet{"alice": {Up: 30, Down: 50}})
	reopened, err := db.Open(path)
	if err != nil {
		t.Fatalf("重开数据库失败: %v", err)
	}
	s.db = reopened
	s.flushPending()
	got := trafficOf(t, s, "alice")
	if got.UploadBytes != 30 || got.DownloadBytes != 50 {
		t.Errorf("失败重试应不丢数: %+v", got)
	}
}

// TestFlushTokenAggregationAndQuotaHook 订阅配额计数按 token 汇总（同 token 多用户合并），
// 无订阅的手动用户不计入；落库成功后只对本轮有流量的 token 触发即时配额核对回调。
func TestFlushTokenAggregationAndQuotaHook(t *testing.T) {
	s := newStatsMonitor(t)
	bindUser(t, s, "alice", 1, 7)
	bindUser(t, s, "bob", 2, 7)
	bindUser(t, s, "manual", 3, 0)

	var gotIDs []uint // 未回调时保持 nil
	s.SetQuotaReconciler(func(tokenIDs []uint) { gotIDs = tokenIDs })
	s.accumulate(map[string]CounterSet{
		"alice":  {Up: 10, Down: 0},
		"bob":    {Up: 0, Down: 5},
		"manual": {Up: 3, Down: 0},
	})
	s.flushPending()

	if got := trafficOf(t, s, "alice"); got.UploadBytes != 10 || got.TokenID != 7 {
		t.Errorf("alice 用户维度累计不符: %+v", got)
	}
	if got := tokenTrafficOf(t, s, 7); got.UploadBytes != 10 || got.DownloadBytes != 5 {
		t.Errorf("同 token 多用户应汇总到配额计数: %+v", got)
	}
	var tokenRows int64
	if err := s.db.Model(&db.TokenTraffic{}).Count(&tokenRows).Error; err != nil {
		t.Fatal(err)
	}
	if tokenRows != 1 {
		t.Errorf("无订阅的手动用户不应计入配额: %d 行", tokenRows)
	}
	if len(gotIDs) != 1 || gotIDs[0] != 7 {
		t.Errorf("落库成功后应只对本轮有流量的 token 触发回调: ids=%v", gotIDs)
	}
}

// TestOwnersCacheInvalidation 归属缓存随写路径失效：绑定变化后未失效时沿用缓存，
// InvalidateOwners 后按最新归属落库。
func TestOwnersCacheInvalidation(t *testing.T) {
	s := newStatsMonitor(t)
	bindUser(t, s, "carol", 1, 3)

	s.accumulate(map[string]CounterSet{"carol": {Up: 5, Down: 5}})
	s.flushPending()
	if got := trafficOf(t, s, "carol"); got.TokenID != 3 {
		t.Errorf("初始归属不符: %+v", got)
	}

	// 换绑到 token 4：未失效时仍按缓存里的旧归属（证明缓存生效，不每轮全表扫描）
	if err := s.db.Model(&db.SubscriptionNode{}).Where("token_id = ?", 3).
		Update("token_id", 4).Error; err != nil {
		t.Fatal(err)
	}
	s.accumulate(map[string]CounterSet{"carol": {Up: 6, Down: 6}}) // 计数器读数（绝对值）
	s.flushPending()
	if got := trafficOf(t, s, "carol"); got.TokenID != 3 {
		t.Errorf("未失效时不应重查归属: %+v", got)
	}

	// 写路径调用失效后：按最新归属落库，且历史计数分账到各自 token
	s.InvalidateOwners()
	s.accumulate(map[string]CounterSet{"carol": {Up: 7, Down: 7}})
	s.flushPending()
	if got := trafficOf(t, s, "carol"); got.TokenID != 4 {
		t.Errorf("失效后应按最新归属落库: %+v", got)
	}
	if got := tokenTrafficOf(t, s, 3); got.UploadBytes != 6 || got.DownloadBytes != 6 {
		t.Errorf("token 3 应含失效前全部增量: %+v", got)
	}
	if got := tokenTrafficOf(t, s, 4); got.UploadBytes != 1 || got.DownloadBytes != 1 {
		t.Errorf("token 4 应只含失效后的增量: %+v", got)
	}
}

// TestResolveCert 证书引用解析：managed / path / 缺配置各分支。
func TestResolveCert(t *testing.T) {
	// managed：调用面板解析器
	tls := &TLSConfig{CertMode: CertModeManaged, CertID: 7}
	certPath, keyPath, err := tls.resolveCert(stubResolver)
	if err != nil {
		t.Fatalf("managed 解析失败: %v", err)
	}
	if certPath != "/data/certs/c.pem" || keyPath != "/data/certs/k.pem" {
		t.Errorf("managed 解析结果不符: %s %s", certPath, keyPath)
	}
	// managed 缺解析器 / 缺 ID
	if _, _, err := tls.resolveCert(nil); err == nil {
		t.Error("managed 缺解析器应报错")
	}
	if _, _, err := (&TLSConfig{CertMode: CertModeManaged}).resolveCert(stubResolver); err == nil {
		t.Error("managed 未选证书应报错")
	}
	// path：直接返回本地路径
	pathTLS := &TLSConfig{CertMode: CertModePath, CertPath: "/etc/c.pem", KeyPath: "/etc/k.pem"}
	certPath, keyPath, err = pathTLS.resolveCert(nil)
	if err != nil || certPath != "/etc/c.pem" || keyPath != "/etc/k.pem" {
		t.Errorf("path 解析结果不符: %s %s err=%v", certPath, keyPath, err)
	}
	if _, _, err := (&TLSConfig{CertMode: CertModePath, CertPath: "/etc/c.pem"}).resolveCert(nil); err == nil {
		t.Error("path 缺私钥应报错")
	}
	// 未配置模式
	if _, _, err := (&TLSConfig{}).resolveCert(nil); err == nil {
		t.Error("未配置证书模式应报错")
	}
}

// TestTLSJSON tlsJSON 渲染：未启用、强制启用、证书要求。
func TestTLSJSON(t *testing.T) {
	// 未启用且不强制 → nil
	if got, err := (&TLSConfig{}).tlsJSON(false, true, stubResolver); err != nil || got != nil {
		t.Errorf("未启用且不强制时应返回 nil: %v err=%v", got, err)
	}
	// 强制启用（Reality）：写 enabled，但 requireCert=false 时不解析证书
	got, err := (&TLSConfig{ServerName: "www.microsoft.com", ALPN: []string{"h3"}}).tlsJSON(true, false, nil)
	if err != nil {
		t.Fatalf("强制启用渲染失败: %v", err)
	}
	if got["enabled"] != true || got["server_name"] != "www.microsoft.com" {
		t.Errorf("强制启用 TLS 字段不符: %v", got)
	}
	if _, ok := got["certificate_path"]; ok {
		t.Errorf("requireCert=false 时不应写证书路径: %v", got)
	}
	// 要求证书：managed 经解析器
	got, err = (&TLSConfig{Enabled: true, CertMode: CertModeManaged, CertID: 3}).tlsJSON(false, true, stubResolver)
	if err != nil {
		t.Fatalf("证书渲染失败: %v", err)
	}
	if got["certificate_path"] != "/data/certs/c.pem" || got["key_path"] != "/data/certs/k.pem" {
		t.Errorf("证书路径字段不符: %v", got)
	}
}

// TestInboundTLS 各协议 TLS 段提取；Shadowsocks 无 TLS 概念。
func TestInboundTLS(t *testing.T) {
	tls := &TLSConfig{Enabled: true, ServerName: "sni.example.com"}
	cases := map[string]any{
		ProtoVLESS:     &VLESSConfig{TLS: *tls},
		ProtoHysteria2: &Hysteria2Config{TLS: *tls},
		ProtoTUIC:      &TUICConfig{TLS: *tls},
		ProtoAnyTLS:    &AnyTLSConfig{TLS: *tls},
		ProtoTrojan:    &TrojanConfig{TLS: *tls},
		ProtoVMess:     &VMessConfig{TLS: *tls},
	}
	for protocol, cfg := range cases {
		got := InboundTLS(cfg)
		if got == nil || got.ServerName != "sni.example.com" {
			t.Errorf("%s 的 TLS 段不符: %+v", protocol, got)
		}
	}
	if got := InboundTLS(&ShadowsocksConfig{}); got != nil {
		t.Errorf("Shadowsocks 不应有 TLS 段: %+v", got)
	}
	if got := InboundTLS(nil); got != nil {
		t.Errorf("nil 配置应返回 nil: %+v", got)
	}
}

// TestRealityJSON Reality 渲染（默认握手目标、short_id 可选）。
func TestRealityJSON(t *testing.T) {
	// 显式握手目标
	got := RealityConfig{PrivateKey: "pk", ShortID: "0123abcd", HandshakeServer: "www.bing.com", HandshakePort: 8443}.json()
	if got["enabled"] != true || got["private_key"] != "pk" {
		t.Errorf("Reality 基础字段不符: %v", got)
	}
	hs, _ := got["handshake"].(map[string]any)
	if hs["server"] != "www.bing.com" || hs["server_port"] != 8443 {
		t.Errorf("Reality 握手目标不符: %v", hs)
	}
	if ids, _ := got["short_id"].([]string); len(ids) != 1 || ids[0] != "0123abcd" {
		t.Errorf("Reality short_id 不符: %v", got["short_id"])
	}
	// 未指定握手目标时取默认值；short_id 为空不输出
	got = RealityConfig{PrivateKey: "pk"}.json()
	hs, _ = got["handshake"].(map[string]any)
	if hs["server"] != "www.microsoft.com" || hs["server_port"] != 443 {
		t.Errorf("Reality 默认握手目标不符: %v", hs)
	}
	if _, ok := got["short_id"]; ok {
		t.Errorf("short_id 为空时不应输出: %v", got)
	}
}
