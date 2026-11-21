package api

import (
	"testing"
	"time"
)

// TestFailLimiterLocksAtThreshold 达到失败阈值锁定、期满放行、再失败立即重锁且时长翻倍。
func TestFailLimiterLocksAtThreshold(t *testing.T) {
	l := newFailLimiter()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }

	const ip = "203.0.113.9"
	for i := 0; i < failThreshold; i++ {
		if !l.allow(ip) {
			t.Fatalf("第 %d 次失败前不应锁定", i+1)
		}
		l.recordFail(ip)
	}
	if l.allow(ip) {
		t.Fatal("达到阈值后应锁定")
	}
	if e := l.entries[ip]; e.lock != 2*failBaseLock {
		t.Fatalf("下次锁定时长 = %v, want %v", e.lock, 2*failBaseLock)
	}

	now = now.Add(failBaseLock + time.Second)
	if !l.allow(ip) {
		t.Fatal("锁定期满后应放行")
	}
	l.recordFail(ip)
	if l.allow(ip) {
		t.Fatal("期满后再失败应立即重新锁定")
	}
	if e := l.entries[ip]; !e.lockedUntil.Equal(now.Add(2 * failBaseLock)) {
		t.Fatalf("重锁时长 = %v, want %v", e.lockedUntil.Sub(now), 2*failBaseLock)
	}

	// 其它 IP 不受影响
	l.recordFail("198.51.100.1")
	if !l.allow("198.51.100.1") {
		t.Fatal("无关 IP 不应被连带锁定")
	}
}

// TestFailLimiterEscalationCap 锁定时长 10m→20m→40m 递增并封顶 1 小时。
func TestFailLimiterEscalationCap(t *testing.T) {
	l := newFailLimiter()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }

	const ip = "203.0.113.9"
	for i := 0; i < failThreshold; i++ {
		l.recordFail(ip)
	}
	e := l.entries[ip]
	want := []time.Duration{failBaseLock, 2 * failBaseLock, 4 * failBaseLock, failMaxLock, failMaxLock}
	for i, w := range want {
		if got := e.lockedUntil.Sub(now); got != w {
			t.Fatalf("第 %d 轮锁定时长 = %v, want %v", i+1, got, w)
		}
		now = e.lockedUntil.Add(time.Second)
		l.recordFail(ip)
	}
	if e.lock != failMaxLock {
		t.Fatalf("锁定梯度应封顶 %v, got %v", failMaxLock, e.lock)
	}
}

// TestFailLimiterReset 成功后失败计数与锁定梯度全部清零。
func TestFailLimiterReset(t *testing.T) {
	l := newFailLimiter()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }

	const ip = "203.0.113.9"
	for i := 0; i < failThreshold-1; i++ {
		l.recordFail(ip)
	}
	l.reset(ip)
	// 若计数未清零，累计第 5 次会触发锁定
	for i := 0; i < failThreshold-1; i++ {
		l.recordFail(ip)
	}
	if !l.allow(ip) {
		t.Fatal("成功清零后未达阈值不应锁定")
	}
	if e := l.entries[ip]; e.lock != failBaseLock {
		t.Fatalf("清零后锁定梯度 = %v, want %v", e.lock, failBaseLock)
	}
}

// TestFailLimiterSweep 周期清理已解锁且长期无失败的条目。
func TestFailLimiterSweep(t *testing.T) {
	l := newFailLimiter()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }

	l.recordFail("198.51.100.1")
	l.recordFail("198.51.100.2")
	now = now.Add(idleExpiry + sweepPeriod)
	l.recordFail("198.51.100.3") // 触发周期清理
	if len(l.entries) != 1 {
		t.Fatalf("空闲条目应被清理, 剩余: %v", l.entries)
	}
	if _, ok := l.entries["198.51.100.3"]; !ok {
		t.Fatal("新写入的条目不应被清理")
	}
}
