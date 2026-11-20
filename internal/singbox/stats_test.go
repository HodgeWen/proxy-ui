package singbox

import (
	"path/filepath"
	"testing"

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

// TestAccumulateFirstRound 首轮采集：计数器绝对值全部计入累计。
func TestAccumulateFirstRound(t *testing.T) {
	s := newStatsMonitor(t)
	s.accumulate(map[string]CounterSet{
		"alice": {Up: 100, Down: 200},
		"bob":   {Up: 50, Down: 60},
	})

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

// TestAccumulateDiff 第二轮采集只累加增量，计数器回绕不会写出负数。
func TestAccumulateDiff(t *testing.T) {
	s := newStatsMonitor(t)
	s.accumulate(map[string]CounterSet{"alice": {Up: 100, Down: 200}})
	// 第二次读数增长
	s.accumulate(map[string]CounterSet{"alice": {Up: 130, Down: 250}})
	got := trafficOf(t, s, "alice")
	if got.UploadBytes != 130 || got.DownloadBytes != 250 {
		t.Errorf("增量累加后累计流量不符: %+v", got)
	}

	// 计数器重置（实例重建）时增量按 0 计，不产生负数
	s.reset() // 清空差分基线
	s.accumulate(map[string]CounterSet{"alice": {Up: 5, Down: 7}})
	got = trafficOf(t, s, "alice")
	if got.UploadBytes != 135 || got.DownloadBytes != 257 {
		t.Errorf("基线清空后应按新绝对值累加: %+v", got)
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
	var count int64
	if err := s.db.Model(&db.UserTraffic{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("空输入不应落库: %d 行", count)
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
