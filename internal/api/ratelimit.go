package api

import (
	"sync"
	"time"
)

// 限流参数：连续 failThreshold 次失败锁定 failBaseLock，锁定期满后再失败则时长翻倍，上限 failMaxLock；成功即清零。
const (
	failThreshold = 5
	failBaseLock  = 10 * time.Minute
	failMaxLock   = time.Hour
	sweepPeriod   = 10 * time.Minute // 条目清理周期
	idleExpiry    = time.Hour        // 已解锁条目的空闲保留时长
)

// failEntry 单 IP 的失败计数与锁定状态。
type failEntry struct {
	fails       int           // 连续失败次数
	lock        time.Duration // 本次锁定后的下次锁定时长（逐次翻倍）
	lockedUntil time.Time     // 锁定截止时间
	lastFail    time.Time     // 最近一次失败时间（清理依据）
}

// failLimiter 登录与首次设置共用的按 IP 失败限流器（纯内存，防公网在线爆破）。
type failLimiter struct {
	mu        sync.Mutex
	entries   map[string]*failEntry
	now       func() time.Time // 可注入时钟（测试用）
	lastSweep time.Time
}

// newFailLimiter 构造限流器。
func newFailLimiter() *failLimiter {
	return &failLimiter{entries: make(map[string]*failEntry), now: time.Now}
}

// allow IP 是否允许尝试（未处于锁定期）。
func (l *failLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[ip]
	return !ok || l.now().After(e.lockedUntil)
}

// recordFail 记一次失败：达到阈值即锁定；锁定期满后再失败按翻倍后的时长重新锁定。
func (l *failLimiter) recordFail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.entries[ip]
	if !ok {
		e = &failEntry{lock: failBaseLock}
		l.entries[ip] = e
	}
	e.fails++
	e.lastFail = now
	if e.fails >= failThreshold {
		e.lockedUntil = now.Add(e.lock)
		e.lock = min(2*e.lock, failMaxLock)
	}
	l.sweep(now)
}

// reset 成功后清零该 IP 的失败记录与锁定梯度。
func (l *failLimiter) reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}

// sweep 周期清理已解锁且长期无失败的条目，防止条目数随来源 IP 无限增长（调用方需持锁）。
func (l *failLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < sweepPeriod {
		return
	}
	l.lastSweep = now
	for ip, e := range l.entries {
		if now.After(e.lockedUntil) && now.Sub(e.lastFail) > idleExpiry {
			delete(l.entries, ip)
		}
	}
}
