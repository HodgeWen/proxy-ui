package singbox

import (
	"context"
	"log/slog"
	"slices"
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
	statsFlushEvery = 5 * time.Second // 落库窗口：窗口内差分在内存聚合，到期合并为单事务批量落库
	statsRPCTimeout = 3 * time.Second // 单轮 RPC 超时：内核卡死时不能拖住采集循环
)

// StatsMonitor 内核统计采集：周期读取 v2ray 统计服务的**按用户**计数器，
// 差分在内存按用户累计，每个落库窗口合并为单事务批量落库（面板重启后累计值保留）。
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
	pending map[string]CounterSet // 落库窗口内已累计未落库的增量（已测得的流量，内核实例重建也不清）
	onQuota func(tokenIDs []uint) // 落库成功后的即时配额核对回调（api 侧注入，见 SetQuotaReconciler）
	lastRtc TrafficSample         // 最近一次全局速率样本
	lastAt  time.Time             // 速率样本时间

	ownersMu sync.Mutex
	owners   map[string]uint // 用户归属缓存：用户名 -> 订阅用户 ID（nil＝未加载或已失效）
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
	return &StatsMonitor{
		mgr: mgr, db: database,
		last:    map[string]CounterSet{},
		pending: map[string]CounterSet{},
	}
}

// Run 常驻采集直至 ctx 取消：每秒差分，按落库窗口批量落库。
func (s *StatsMonitor) Run(ctx context.Context) {
	tick := time.NewTicker(statsInterval)
	defer tick.Stop()
	flush := time.NewTicker(statsFlushEvery)
	defer flush.Stop()
	for {
		select {
		case <-ctx.Done():
			s.flushPending() // 退出前把窗口内已测得的增量落库，尽量不丢数
			s.reset()
			return
		case <-tick.C:
			s.collect(ctx)
		case <-flush.C:
			s.flushPending()
		}
	}
}

// Rates 返回最近一次全局速率样本及其采集时间。
func (s *StatsMonitor) Rates() (TrafficSample, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRtc, s.lastAt
}

// SetQuotaReconciler 注入「落库成功后即时核对配额」回调（参数为本轮有流量的订阅用户 ID）。
// 配额核对逻辑在 api 包，singbox 直接 import api 会成环，故由 api 侧装配时传入；缺省不核对。
// 回调在采集 goroutine 同步执行（核对顺序与落库一致；期间采集暂停，计数器是绝对值，恢复后差分不丢）。
func (s *StatsMonitor) SetQuotaReconciler(fn func(tokenIDs []uint)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onQuota = fn
}

// InvalidateOwners 失效用户归属缓存：凡会改用户或订阅绑定的写操作落库后由 api 侧调用
// （挂点在 applyKernelConfig 入口，即所有写 handler 的收口处），下轮落库按最新归属重建。
func (s *StatsMonitor) InvalidateOwners() {
	s.ownersMu.Lock()
	defer s.ownersMu.Unlock()
	s.owners = nil
}

// collect 单轮采集：读取按用户计数器并按 tag 差分累加进内存窗口累计。
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

// reset 关闭连接并清空基线与速率样本；不清待落库增量——那是已测得的流量，与实例无关。
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
	s.lastAt = time.Time{}
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

// accumulate 把读数差分累加进内存窗口累计，并计算全局速率；落库由 flushPending 按窗口批量完成。
func (s *StatsMonitor) accumulate(values map[string]CounterSet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.last
	next := make(map[string]CounterSet, len(values))
	var totalUp, totalDown int64
	for user, cur := range values {
		next[user] = cur
		old := prev[user]
		up := max(cur.Up-old.Up, 0)
		down := max(cur.Down-old.Down, 0)
		totalUp += up
		totalDown += down
		if up > 0 || down > 0 {
			d := s.pending[user]
			d.Up += up
			d.Down += down
			s.pending[user] = d
		}
	}
	now := time.Now()
	// 跨周期采样（如 RPC 超时）按实际经过秒数归一化，避免下轮出现多倍速率尖峰
	if !s.lastAt.IsZero() {
		if dt := now.Sub(s.lastAt); dt > statsInterval+500*time.Millisecond {
			sec := dt.Seconds()
			totalUp = int64(float64(totalUp) / sec)
			totalDown = int64(float64(totalDown) / sec)
		}
	}
	s.last = next
	s.lastRtc = TrafficSample{Up: totalUp, Down: totalDown}
	s.lastAt = now
}

// flushPending 落库窗口到期：取出窗口内累计的增量，单事务批量落库。
// 落库失败把增量并回内存累计，下轮窗口重试（不丢数）；成功后对本轮有流量的订阅用户
// 即时核对配额，超限/到期的暂停不必再等周期核对循环。
func (s *StatsMonitor) flushPending() {
	s.mu.Lock()
	if len(s.pending) == 0 {
		s.mu.Unlock()
		return
	}
	deltas := s.pending
	s.pending = make(map[string]CounterSet, len(deltas))
	hook := s.onQuota
	s.mu.Unlock()

	tokenIDs, err := s.flush(deltas)
	if err != nil {
		slog.Warn("按用户流量批量落库失败，增量并回内存待下轮重试", "err", err)
		s.requeue(deltas)
		return
	}
	if hook != nil && len(tokenIDs) > 0 {
		hook(tokenIDs)
	}
}

// requeue 把未落库的增量并回内存累计（期间新采到的增量在另一份 map 上，两者相加即全量）。
func (s *StatsMonitor) requeue(deltas map[string]CounterSet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for tag, d := range deltas {
		p := s.pending[tag]
		p.Up += d.Up
		p.Down += d.Down
		s.pending[tag] = p
	}
}

// flush 把窗口增量合并落库：单事务内先批量 upsert UserTraffic（按用户名，供列表展示），
// 再按订阅用户汇总批量 upsert TokenTraffic（配额与到期的权威计数），返回本轮有流量的
// 订阅用户 ID。单事务让两张表同窗口一致、每窗口只 fsync 一次；任一步失败整体回滚，
// 由调用方把增量并回内存重试。
func (s *StatsMonitor) flush(deltas map[string]CounterSet) ([]uint, error) {
	owners, err := s.userOwners()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	userRows := make([]db.UserTraffic, 0, len(deltas))
	tokenDeltas := map[uint]CounterSet{}
	for tag, d := range deltas {
		tokenID := owners[tag]
		userRows = append(userRows, db.UserTraffic{
			Tag: tag, TokenID: tokenID, UploadBytes: d.Up, DownloadBytes: d.Down, UpdatedAt: now,
		})
		if tokenID == 0 {
			continue // 无订阅的手动用户不计入任何配额
		}
		sum := tokenDeltas[tokenID]
		sum.Up += d.Up
		sum.Down += d.Down
		tokenDeltas[tokenID] = sum
	}
	tokenRows := make([]db.TokenTraffic, 0, len(tokenDeltas))
	for tokenID, d := range tokenDeltas {
		tokenRows = append(tokenRows, db.TokenTraffic{
			TokenID: tokenID, UploadBytes: d.Up, DownloadBytes: d.Down, UpdatedAt: now,
		})
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if len(userRows) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "tag"}},
				DoUpdates: trafficIncrementSet("token_id", "updated_at"), // token_id 随最新绑定更新（用户重建后归属可能变化）
			}).Create(&userRows).Error; err != nil {
				return err
			}
		}
		if len(tokenRows) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "token_id"}},
				DoUpdates: trafficIncrementSet("updated_at"),
			}).Create(&tokenRows).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	tokenIDs := make([]uint, 0, len(tokenDeltas))
	for id := range tokenDeltas {
		tokenIDs = append(tokenIDs, id)
	}
	slices.Sort(tokenIDs)
	return tokenIDs, nil
}

// trafficIncrementSet 生成流量表批量 upsert 的更新集：累计列做增量加法（当前行 + 本行新值），
// 其余给定列直接取本行新值。SQLite 的 excluded 指代本行拟写入的值。
func trafficIncrementSet(plainColumns ...string) clause.Set {
	set := clause.Set{
		{Column: clause.Column{Name: "upload_bytes"}, Value: gorm.Expr("upload_bytes + excluded.upload_bytes")},
		{Column: clause.Column{Name: "download_bytes"}, Value: gorm.Expr("download_bytes + excluded.download_bytes")},
	}
	for _, name := range plainColumns {
		set = append(set, clause.Assignment{Column: clause.Column{Name: name}, Value: gorm.Expr("excluded." + name)})
	}
	return set
}

// userOwners 返回 用户名 -> 所属订阅用户 ID（未绑定订阅的手动用户为 0）。
// 结果缓存：全表扫描只在首轮落库与失效后发生，查询失败不写缓存。
func (s *StatsMonitor) userOwners() (map[string]uint, error) {
	s.ownersMu.Lock()
	defer s.ownersMu.Unlock()
	if s.owners != nil {
		return s.owners, nil
	}
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
	s.owners = owners
	return owners, nil
}
