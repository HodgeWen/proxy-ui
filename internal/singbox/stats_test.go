package singbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"proxy-ui/internal/db"
)

// statsTestDB 建内存 SQLite 并迁移入站与累计流量表。
func statsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	g, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := g.AutoMigrate(&db.Inbound{}, &db.InboundTraffic{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return g
}

func TestStatsMonitorAccumulates(t *testing.T) {
	g := statsTestDB(t)
	if err := g.Create(&db.Inbound{Name: "hy-in", Protocol: "hysteria2", ListenPort: 443, Enabled: true}).Error; err != nil {
		t.Fatalf("建入站失败: %v", err)
	}

	// 可变状态的 Clash API 模拟：连接计数可调整，供差分累计断言
	var up, down atomic.Int64
	var fail atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/traffic", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fmt.Sprintf(`{"up":%d,"down":%d}`, up.Load(), down.Load())))
	})
	mux.HandleFunc("/connections", func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"uploadTotal": up.Load(), "downloadTotal": down.Load(), "memory": 2048,
			"connections": []map[string]any{
				{"id": "conn-1", "metadata": map[string]any{"network": "tcp", "type": "hysteria2/hy-in", "sourceIP": "1.1.1.1", "destinationIP": "2.2.2.2", "sourcePort": "1", "destinationPort": "443", "host": "a.com"}, "upload": up.Load(), "download": down.Load(), "chains": []string{"direct"}},
				{"id": "conn-orphan", "metadata": map[string]any{"network": "tcp", "type": "vless/unknown-in", "sourceIP": "1.1.1.1", "destinationIP": "3.3.3.3", "sourcePort": "2", "destinationPort": "80"}, "upload": int64(999999), "download": int64(999999), "chains": []string{"direct"}},
			},
		})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	m := managerWithClash(t, server.Listener.Addr().String(), "")
	monitor := NewStatsMonitor(m, g)

	// 第 1 轮：只记基线，不累计
	up.Store(1000)
	down.Store(5000)
	monitor.collect(context.Background())
	var row db.InboundTraffic
	if err := g.First(&row).Error; err == nil {
		t.Fatalf("首见连接只应记基线，不应落库: %+v", row)
	}

	// 第 2 轮：增量 400/1000 落库
	up.Store(1400)
	down.Store(6000)
	monitor.collect(context.Background())
	if err := g.First(&row, "inbound_id = ?", 1).Error; err != nil {
		t.Fatalf("应有累计记录: %v", err)
	}
	if row.UploadBytes != 400 || row.DownloadBytes != 1000 {
		t.Errorf("累计增量不符: %+v", row)
	}

	// 第 3 轮：再增 100/500，累计应叠加
	up.Store(1500)
	down.Store(6500)
	monitor.collect(context.Background())
	if err := g.First(&row, "inbound_id = ?", 1).Error; err != nil {
		t.Fatalf("查询累计记录失败: %v", err)
	}
	if row.UploadBytes != 500 || row.DownloadBytes != 1500 {
		t.Errorf("累计应叠加: %+v", row)
	}

	// 快照与速率缓存可用
	rates, at := monitor.Rates()
	if rates.Up != 1500 || rates.Down != 6500 || at.IsZero() {
		t.Errorf("速率缓存不符: %+v %v", rates, at)
	}
	snap := monitor.Snapshot()
	if snap.Memory != 2048 || len(snap.Connections) != 2 {
		t.Errorf("快照缓存不符: %+v", snap)
	}

	// 内核不可达：缓存清空，基线重置
	fail.Store(true)
	monitor.collect(context.Background())
	if rates, at = monitor.Rates(); !at.IsZero() {
		t.Errorf("不可达时应清空速率缓存: %+v %v", rates, at)
	}
	if snap = monitor.Snapshot(); len(snap.Connections) != 0 {
		t.Errorf("不可达时应清空快照: %+v", snap)
	}

	// 恢复后首轮回基线，不重复累计
	fail.Store(false)
	monitor.collect(context.Background())
	if err := g.First(&row, "inbound_id = ?", 1).Error; err != nil {
		t.Fatalf("查询累计记录失败: %v", err)
	}
	if row.UploadBytes != 500 || row.DownloadBytes != 1500 {
		t.Errorf("恢复后首轮不应累计: %+v", row)
	}

	// 恢复后第 2 轮恢复差分累计
	up.Store(2000)
	down.Store(8000)
	monitor.collect(context.Background())
	if err := g.First(&row, "inbound_id = ?", 1).Error; err != nil {
		t.Fatalf("查询累计记录失败: %v", err)
	}
	if row.UploadBytes != 1000 || row.DownloadBytes != 3000 {
		t.Errorf("恢复后累计不符: %+v", row)
	}
}
