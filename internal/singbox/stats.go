package singbox

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"proxy-ui/internal/db"
)

// 统计采集参数。
const (
	statsInterval = time.Second // 采集周期：与 Clash API /traffic 每秒一拍对齐
)

// StatsMonitor 内核统计采集：周期采样实时速率与连接快照供面板查询，
// 并按连接差分把各入站累计流量落库（面板重启后累计值保留）。
type StatsMonitor struct {
	mgr *Manager
	db  *gorm.DB

	mu       sync.Mutex
	rates    TrafficSample        // 最近一次速率样本
	ratesAt  time.Time            // 速率样本时间
	snapshot *ConnectionsSnapshot // 最近一次连接快照

	last map[string]TrafficSample // 连接 ID -> 上次累计上/下行（差分基线）
}

// NewStatsMonitor 构造统计采集器；database 为面板数据库（累计流量落库）。
func NewStatsMonitor(mgr *Manager, database *gorm.DB) *StatsMonitor {
	return &StatsMonitor{mgr: mgr, db: database, last: map[string]TrafficSample{}}
}

// Run 常驻采集直至 ctx 取消；随面板进程生命周期运行。
func (s *StatsMonitor) Run(ctx context.Context) {
	tick := time.NewTicker(statsInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.collect(ctx)
		}
	}
}

// Rates 返回最近一次速率样本及其采集时间。
func (s *StatsMonitor) Rates() (TrafficSample, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rates, s.ratesAt
}

// Snapshot 返回最近一次连接快照副本；内核不可用时为空快照。
func (s *StatsMonitor) Snapshot() ConnectionsSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snapshot == nil {
		return ConnectionsSnapshot{Connections: []Connection{}}
	}
	return *s.snapshot
}

// collect 单轮采集：速率样本与连接快照任一失败视为内核不可用，清空缓存；
// 连接计数同时驱动按入站累计流量的差分与落库。
func (s *StatsMonitor) collect(ctx context.Context) {
	client := s.mgr.ClashClient()
	if client == nil {
		s.reset()
		return
	}
	cctx, cancel := context.WithTimeout(ctx, statsInterval+2*time.Second)
	defer cancel()
	sample, err := client.Traffic(cctx)
	if err != nil {
		s.reset()
		return
	}
	snap, err := client.Connections(cctx)
	if err != nil {
		s.reset()
		return
	}
	s.mu.Lock()
	s.rates, s.ratesAt, s.snapshot = sample, time.Now(), snap
	s.mu.Unlock()
	s.accumulate(snap)
}

// reset 清空缓存与差分基线（内核停止或 Clash API 不可达）。
func (s *StatsMonitor) reset() {
	s.mu.Lock()
	s.rates, s.ratesAt, s.snapshot = TrafficSample{}, time.Time{}, nil
	s.mu.Unlock()
	s.last = map[string]TrafficSample{}
}

// accumulate 按连接 ID 差分累计各入站流量：新连接只记基线不累计，
// 已见连接把增量累加到对应入站；连接消失后从基线剔除。
func (s *StatsMonitor) accumulate(snap *ConnectionsSnapshot) {
	deltas := map[string]TrafficSample{}
	next := make(map[string]TrafficSample, len(snap.Connections))
	for _, conn := range snap.Connections {
		counters := TrafficSample{Up: conn.Upload, Down: conn.Download}
		next[conn.ID] = counters
		prev, seen := s.last[conn.ID]
		if !seen {
			continue
		}
		d := deltas[conn.Inbound]
		d.Up += max(counters.Up-prev.Up, 0)
		d.Down += max(counters.Down-prev.Down, 0)
		deltas[conn.Inbound] = d
	}
	s.last = next
	if len(deltas) == 0 {
		return
	}
	if err := s.flush(deltas); err != nil {
		slog.Warn("按入站累计流量落库失败", "err", err)
	}
}

// flush 把各入站流量增量累加落库；面板中已不存在的入站 tag 跳过。
func (s *StatsMonitor) flush(deltas map[string]TrafficSample) error {
	var inbounds []db.Inbound
	if err := s.db.Find(&inbounds).Error; err != nil {
		return err
	}
	ids := make(map[string]uint, len(inbounds))
	for _, in := range inbounds {
		ids[in.Name] = in.ID
	}
	now := time.Now()
	for tag, d := range deltas {
		id, ok := ids[tag]
		if !ok {
			continue
		}
		row := db.InboundTraffic{InboundID: id, UploadBytes: d.Up, DownloadBytes: d.Down, UpdatedAt: now}
		if err := s.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "inbound_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"upload_bytes":   gorm.Expr("upload_bytes + ?", d.Up),
				"download_bytes": gorm.Expr("download_bytes + ?", d.Down),
				"updated_at":     now,
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
