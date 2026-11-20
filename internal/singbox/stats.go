package singbox

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	v2 "github.com/sagernet/sing-box/experimental/v2rayapi"

	"proxy-ui/internal/db"
)

// 统计采集参数。
const (
	statsInterval   = time.Second     // 采集周期
	statsRPCTimeout = 3 * time.Second // 单轮 RPC 超时：内核卡死时不能拖住采集循环
)

// StatsMonitor 内核统计采集：周期读取 v2ray 统计服务的**按用户**计数器，
// 差分后累加落库（面板重启后累计值保留）。
//
// 只统计 user 计数器的原因：
//   - 共享入站下只有 user 维度能区分不同订阅用户，配额依赖它；
//   - inbound 计数器与 user 计数器是同一份字节的两种视角，同时累加会导致速率与累计重复计算；
//   - 两类计数器共用 tag 名空间，入站名与用户名相同时会互相覆盖。
type StatsMonitor struct {
	mgr *Manager
	db  *gorm.DB

	mu      sync.Mutex
	conn    *grpc.ClientConn      // v2ray 统计 gRPC 连接
	gen     uint64                // 连接对应的内核实例代号
	last    map[string]CounterSet // 计数器 tag -> 上次读数（差分基线）
	lastRtc TrafficSample         // 最近一次全局速率样本
	lastAt  time.Time             // 速率样本时间
}

// CounterSet 单个 tag 的上/下行读数。
type CounterSet struct {
	Up   int64
	Down int64
}

// TrafficSample 全局速率样本（字节/秒）。
type TrafficSample struct {
	Up   int64
	Down int64
}

// NewStatsMonitor 构造统计采集器；database 为面板数据库。
func NewStatsMonitor(mgr *Manager, database *gorm.DB) *StatsMonitor {
	return &StatsMonitor{mgr: mgr, db: database, last: map[string]CounterSet{}}
}

// Run 常驻采集直至 ctx 取消。
func (s *StatsMonitor) Run(ctx context.Context) {
	tick := time.NewTicker(statsInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			s.reset()
			return
		case <-tick.C:
			s.collect(ctx)
		}
	}
}

// Rates 返回最近一次全局速率样本及其采集时间。
func (s *StatsMonitor) Rates() (TrafficSample, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRtc, s.lastAt
}

// collect 单轮采集：读取按用户计数器并按 tag 差分累加落库。
func (s *StatsMonitor) collect(ctx context.Context) {
	// 内核实例每次重建计数器都会归零，基线必须同步清空，否则会把新读数算成增量或漏记
	if gen := s.mgr.Generation(); gen != s.currentGen() {
		s.reset()
	}
	conn, err := s.clientConn()
	if err != nil {
		return // 内核未运行或统计服务未就绪，下轮重试
	}
	rpcCtx, cancel := context.WithTimeout(ctx, statsRPCTimeout)
	defer cancel()
	values, err := s.queryCounters(rpcCtx, conn)
	if err != nil {
		// 只是一次查询失败（超时/瞬断）：内核实例没变、计数器还在原值上，
		// 此时绝不能清空基线——否则下轮会把「自内核启动以来的总量」当成增量重复累加。
		if gen := s.mgr.Generation(); gen != s.currentGen() {
			s.reset()
		}
		return
	}
	s.accumulate(values)
}

// currentGen 返回当前连接对应的实例代号。
func (s *StatsMonitor) currentGen() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gen
}

// clientConn 返回统计服务连接（实例代号变化时重建连接并清空基线）。
func (s *StatsMonitor) clientConn() (*grpc.ClientConn, error) {
	addr := s.mgr.StatsAddr()
	if addr == "" {
		return nil, errNoStatsAddr
	}
	gen := s.mgr.Generation()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil && s.gen == gen {
		return s.conn, nil
	}
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
	conn, err := grpc.NewClient("passthrough:///"+addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	s.conn, s.gen = conn, gen
	s.last = map[string]CounterSet{} // 新实例计数器从零开始
	return conn, nil
}

// reset 关闭连接并清空基线与速率样本。
func (s *StatsMonitor) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
	s.gen = 0
	s.last = map[string]CounterSet{}
	s.lastRtc = TrafficSample{}
}

// queryCounters 一次查询取回全部按用户计数器的当前值（键为用户名）。
//
// QueryStats 的返回已带各计数器数值，无需再逐个 GetStats，避免每秒 N 次 RPC。
func (s *StatsMonitor) queryCounters(ctx context.Context, conn *grpc.ClientConn) (map[string]CounterSet, error) {
	var resp v2.QueryStatsResponse
	if err := invokeStats(ctx, conn, "QueryStats", &v2.QueryStatsRequest{Patterns: []string{"user>>>"}}, &resp); err != nil {
		return nil, err
	}
	values := make(map[string]CounterSet, len(resp.GetStat())/2)
	for _, stat := range resp.GetStat() {
		user, ok := userCounter(stat.GetName())
		if !ok {
			continue
		}
		set := values[user]
		if isUplink(stat.GetName()) {
			set.Up = stat.GetValue()
		} else {
			set.Down = stat.GetValue()
		}
		values[user] = set
	}
	return values, nil
}

// accumulate 把读数差分累加到各用户，并计算全局速率。
func (s *StatsMonitor) accumulate(values map[string]CounterSet) {
	s.mu.Lock()
	prev := s.last
	next := make(map[string]CounterSet, len(values))
	deltas := map[string]CounterSet{}
	var totalUp, totalDown int64
	for user, cur := range values {
		next[user] = cur
		old := prev[user]
		up := max(cur.Up-old.Up, 0)
		down := max(cur.Down-old.Down, 0)
		totalUp += up
		totalDown += down
		if up > 0 || down > 0 {
			deltas[user] = CounterSet{Up: up, Down: down}
		}
	}
	s.last = next
	s.lastRtc = TrafficSample{Up: totalUp, Down: totalDown}
	s.lastAt = time.Now()
	s.mu.Unlock()
	if len(deltas) == 0 {
		return
	}
	if err := s.flush(deltas); err != nil {
		slog.Warn("按用户累计流量落库失败", "err", err)
	}
}

// flush 把增量累加落库：一份到 UserTraffic（按用户名，供列表展示），
// 一份按订阅用户汇总到 TokenTraffic（配额与到期的权威计数）。
func (s *StatsMonitor) flush(deltas map[string]CounterSet) error {
	owners, err := s.userOwners()
	if err != nil {
		return err
	}
	tokenDeltas := map[uint]CounterSet{}
	now := time.Now()
	for tag, d := range deltas {
		tokenID := owners[tag]
		row := db.UserTraffic{Tag: tag, TokenID: tokenID, UploadBytes: d.Up, DownloadBytes: d.Down, UpdatedAt: now}
		if err := s.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "tag"}},
			DoUpdates: clause.Assignments(map[string]any{
				"token_id":       tokenID, // 用户重建后归属可能变化，随最新绑定更新
				"upload_bytes":   gorm.Expr("upload_bytes + ?", d.Up),
				"download_bytes": gorm.Expr("download_bytes + ?", d.Down),
				"updated_at":     now,
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
		if tokenID == 0 {
			continue // 无订阅的手动用户不计入任何配额
		}
		sum := tokenDeltas[tokenID]
		sum.Up += d.Up
		sum.Down += d.Down
		tokenDeltas[tokenID] = sum
	}
	for tokenID, d := range tokenDeltas {
		row := db.TokenTraffic{TokenID: tokenID, UploadBytes: d.Up, DownloadBytes: d.Down, UpdatedAt: now}
		if err := s.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "token_id"}},
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

// userOwners 返回 用户名 -> 所属订阅用户 ID（未绑定订阅的手动用户为 0）。
func (s *StatsMonitor) userOwners() (map[string]uint, error) {
	var users []db.InboundUser
	if err := s.db.Find(&users).Error; err != nil {
		return nil, err
	}
	var bindings []db.SubscriptionNode
	if err := s.db.Find(&bindings).Error; err != nil {
		return nil, err
	}
	byUser := make(map[uint]uint, len(bindings))
	for _, b := range bindings {
		byUser[b.InboundUserID] = b.TokenID
	}
	owners := make(map[string]uint, len(users))
	for _, u := range users {
		owners[u.Name] = byUser[u.ID]
	}
	return owners, nil
}
