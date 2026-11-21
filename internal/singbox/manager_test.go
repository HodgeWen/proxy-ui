package singbox

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestCheckConfigMinConfig 最小配置必须能通过内核解析（不启动实例）。
func TestCheckConfigMinConfig(t *testing.T) {
	m := New(t.TempDir())
	if err := m.CheckConfig(MinConfig()); err != nil {
		t.Fatalf("最小配置应通过校验: %v", err)
	}
	st := m.Status()
	if st.Running {
		t.Errorf("校验不应启动内核: %+v", st)
	}
	if !st.Installed || !st.Embedded {
		t.Error("内置内核应恒为已安装/内置")
	}
	if st.Version != EmbeddedVersion {
		t.Errorf("内核版本 = %q, want %q", st.Version, EmbeddedVersion)
	}
}

// TestCheckConfigInvalidConfig 非法配置应校验失败并带回错误信息。
func TestCheckConfigInvalidConfig(t *testing.T) {
	m := New(t.TempDir())
	// 未知入站类型（内核无法解析）
	if err := m.CheckConfig(Config{Inbounds: []any{map[string]any{"type": "not-a-protocol", "tag": "x"}}}); err == nil {
		t.Error("未知入站类型应校验失败")
	}
	// 字段类型错误：端口给了字符串
	badPort := Config{Inbounds: []any{map[string]any{
		"type": "vless", "tag": "vless-in", "listen": "127.0.0.1", "listen_port": "not-a-port",
	}}}
	if err := m.CheckConfig(badPort); err == nil {
		t.Error("端口类型错误的入站应校验失败")
	}
	// 未知出站类型
	badOutbound := Config{Outbounds: []map[string]any{{"type": "not-a-protocol", "tag": "x"}}}
	if err := m.CheckConfig(badOutbound); err == nil {
		t.Error("未知出站类型应校验失败")
	}
	if st := m.Status(); st.Running {
		t.Errorf("校验失败不应启动内核: %+v", st)
	}
}

// TestManagerLifecycle 启动/停止生命周期（最小配置无入站，只绑定本机统计端口）。
func TestManagerLifecycle(t *testing.T) {
	m := New(t.TempDir())
	if err := m.Start(MinConfig()); err != nil {
		t.Skipf("本机统计端口不可用，跳过启动生命周期用例: %v", err)
	}
	if st := m.Status(); !st.Running {
		t.Fatalf("启动后应处于运行中: %+v", st)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("停止内核失败: %v", err)
	}
	if st := m.Status(); st.Running {
		t.Fatalf("停止后不应运行: %+v", st)
	}
	// 重复停止幂等
	if err := m.Stop(); err != nil {
		t.Errorf("重复停止应幂等: %v", err)
	}
}

// TestManagerRestart 以 Start 重建运行中实例后仍处于运行中且代号递增。
func TestManagerRestart(t *testing.T) {
	m := New(t.TempDir())
	if err := m.Start(MinConfig()); err != nil {
		t.Skipf("本机统计端口不可用，跳过重启用例: %v", err)
	}
	defer func() { _ = m.Stop() }()
	gen := m.Generation()
	if err := m.Start(MinConfig()); err != nil {
		t.Fatalf("重建内核失败: %v", err)
	}
	if m.Generation() <= gen {
		t.Errorf("重建后实例代号应递增: %d -> %d", gen, m.Generation())
	}
	if st := m.Status(); !st.Running {
		t.Fatalf("重启后应处于运行中: %+v", st)
	}
}

// TestManagerStartRollback 新配置不合法或端口冲突时自动回滚到上一个可用配置，代理不中断。
func TestManagerStartRollback(t *testing.T) {
	m := New(t.TempDir())
	if err := m.Start(MinConfig()); err != nil {
		t.Skipf("本机统计端口不可用，跳过回滚用例: %v", err)
	}
	defer func() { _ = m.Stop() }()

	// 配置不合法（build 阶段失败）应回滚
	bad := Config{Inbounds: []any{map[string]any{"type": "not-a-protocol", "tag": "x"}}}
	if err := m.Start(bad); err == nil || !strings.Contains(err.Error(), "已回滚") {
		t.Fatalf("非法配置失败应返回含回滚提示的错误, got %v", err)
	}
	if st := m.Status(); !st.Running {
		t.Fatalf("回滚后内核应仍处于运行中: %+v", st)
	}

	// 端口冲突（instance.Start 阶段失败）应回滚
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("占用临时端口失败: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	conflict := MinConfig()
	conflict.Inbounds = []any{map[string]any{
		"type": "vless", "tag": "conflict-in",
		"listen": "127.0.0.1", "listen_port": port,
		"users": []map[string]any{{"uuid": "0b3f6e8c-0000-4000-8000-000000000000"}},
	}}
	if err := m.Start(conflict); err == nil || !strings.Contains(err.Error(), "已回滚") {
		t.Fatalf("端口冲突失败应返回含回滚提示的错误, got %v", err)
	}
	if st := m.Status(); !st.Running {
		t.Fatalf("端口冲突回滚后内核应仍处于运行中: %+v", st)
	}
}

// TestWithStatsAddrDoesNotMutateCaller withStatsAddr 不得原地改写调用方的指针字段。
func TestWithStatsAddrDoesNotMutateCaller(t *testing.T) {
	m := New(t.TempDir())
	cfg := MinConfig()
	origExp := cfg.Experimental
	origV2 := cfg.Experimental.V2RayAPI
	origListen := origV2.Listen

	copied := cfg
	m.withStatsAddr(&copied)
	if origV2.Listen != origListen {
		t.Errorf("调用方 V2RayAPI.Listen 被原地修改: %q -> %q", origListen, origV2.Listen)
	}
	if copied.Experimental == origExp || copied.Experimental.V2RayAPI == origV2 {
		t.Error("withStatsAddr 应先复制指针再写入 statsAddr")
	}
	if copied.Experimental.V2RayAPI.Listen != m.StatsAddr() {
		t.Errorf("副本监听地址 = %q, want %q", copied.Experimental.V2RayAPI.Listen, m.StatsAddr())
	}
}

// TestManagerStartInvalid 非法配置启动失败且不残留运行状态。
func TestManagerStartInvalid(t *testing.T) {
	m := New(t.TempDir())
	bad := Config{Inbounds: []any{map[string]any{"type": "not-a-protocol", "tag": "x"}}}
	err := m.Start(bad)
	if err == nil {
		t.Fatal("非法配置启动应报错")
	}
	if st := m.Status(); st.Running {
		t.Errorf("启动失败后不应处于运行中: %+v", st)
	}
	// 未运行时应带回失败原因（面板据此提示修复方向）
	if st := m.Status(); st.Error == "" {
		t.Error("启动失败后状态应携带失败原因")
	} else if !strings.Contains(st.Error, "not-a-protocol") && !strings.Contains(st.Error, "内核") {
		t.Errorf("失败原因缺少上下文: %s", st.Error)
	}
	// 错误信息应可定位到原因
	if !strings.Contains(err.Error(), "配置") && !strings.Contains(err.Error(), "内核") {
		t.Errorf("错误信息缺少上下文: %v", err)
	}
}

// TestManagerApplySkipsIdenticalConfig 配置未变时不重建实例（写操作频繁保存不该反复重启代理）。
func TestManagerApplySkipsIdenticalConfig(t *testing.T) {
	m := New(t.TempDir())
	if err := m.Start(MinConfig()); err != nil {
		t.Skipf("本机统计端口不可用，跳过重建跳过用例: %v", err)
	}
	defer func() { _ = m.Stop() }()
	gen := m.Generation()
	if err := m.Apply(MinConfig()); err != nil {
		t.Fatalf("应用相同配置失败: %v", err)
	}
	if got := m.Generation(); got != gen {
		t.Errorf("相同配置不应重建实例: generation %d -> %d", gen, got)
	}
	// 配置变化后必须重建
	changed := MinConfig()
	changed.Log = &LogConfig{Level: "debug", Timestamp: true}
	if err := m.Apply(changed); err != nil {
		t.Fatalf("应用新配置失败: %v", err)
	}
	if got := m.Generation(); got == gen {
		t.Errorf("配置变化后应重建实例: generation 仍为 %d", got)
	}
	if st := m.Status(); !st.Running {
		t.Errorf("重建后应仍在运行: %+v", st)
	}
}

// TestLogBuffer 日志环形缓冲：容量上限与 limit 语义。

// TestManagerStatsAddrIsDynamic 每个管理器使用各自的回环端口，避免多实例争用同一端口。
func TestManagerStatsAddrIsDynamic(t *testing.T) {
	first, second := New(t.TempDir()), New(t.TempDir())
	if first.StatsAddr() == second.StatsAddr() {
		t.Errorf("两个管理器的统计端口不应相同: %s", first.StatsAddr())
	}
	for _, m := range []*Manager{first, second} {
		host, port, err := net.SplitHostPort(m.StatsAddr())
		if err != nil {
			t.Fatalf("统计地址不合法 %q: %v", m.StatsAddr(), err)
		}
		if host != "127.0.0.1" || port == "" {
			t.Errorf("统计地址应为本机回环: %s", m.StatsAddr())
		}
	}
}

func TestLogBuffer(t *testing.T) {
	var b logBuffer
	for range logCapacity + 10 {
		b.add("core", "line")
	}
	all := b.snapshot(0)
	if len(all) != logCapacity {
		t.Errorf("缓冲应截断到 %d 条, got %d", logCapacity, len(all))
	}
	if got := b.snapshot(5); len(got) != 5 {
		t.Errorf("limit=5 应返回 5 条, got %d", len(got))
	}
	if got := b.snapshot(-1); len(got) != logCapacity {
		t.Errorf("负 limit 应返回全部, got %d", len(got))
	}
	if got := b.snapshot(logCapacity * 2); len(got) != logCapacity {
		t.Errorf("超量 limit 应返回全部, got %d", len(got))
	}
	if all[0].Stream != "core" || all[0].Line != "line" {
		t.Errorf("日志行内容不符: %+v", all[0])
	}
}

// TestManagerStartWithConcurrentStatus 重建内核期间并发读状态不应产生数据竞争
// （-race 下回归：修复前 Start 调 stopLocked 未持 m.mu，与 Status() 的 m.box 读竞争）。
func TestManagerStartWithConcurrentStatus(t *testing.T) {
	m := New(t.TempDir())
	if err := m.Start(MinConfig()); err != nil {
		t.Skipf("本机统计端口不可用，跳过并发重建用例: %v", err)
	}
	defer func() { _ = m.Stop() }()

	done := make(chan struct{})
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			for {
				select {
				case <-done:
					return
				default:
					_ = m.Status()
					_ = m.Generation()
				}
			}
		})
	}
	changed := MinConfig()
	changed.Log = &LogConfig{Level: "debug", Timestamp: true}
	for range 3 {
		if err := m.Start(changed); err != nil {
			t.Errorf("并发读状态下重建内核失败: %v", err)
			break
		}
	}
	close(done)
	wg.Wait()
}

// TestManagerDirIsolation 内核落盘文件（cache.db）必须写在管理器工作目录，而非进程工作目录。
func TestManagerDirIsolation(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if err := m.Start(MinConfig()); err != nil {
		t.Skipf("本机统计端口不可用，跳过落盘隔离用例: %v", err)
	}
	// 给关闭时的落盘留出时间
	time.Sleep(150 * time.Millisecond)
	if err := m.Stop(); err != nil {
		t.Fatalf("停止内核失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cache.db")); err != nil {
		t.Errorf("cache.db 应写入内核工作目录 %s: %v", dir, err)
	}
	// 仓库/进程工作目录不应出现 cache.db
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cwd, "cache.db")); err == nil {
		t.Errorf("cache.db 不应写在进程工作目录 %s", cwd)
	}
}

// TestManagerLogs 面板日志写入环形缓冲并回调。
func TestManagerLogs(t *testing.T) {
	m := New(t.TempDir())
	m.log("panel", "面板启动")
	logs := m.Logs(0)
	if len(logs) != 1 || logs[0].Stream != "panel" || logs[0].Line != "面板启动" {
		t.Fatalf("日志缓冲内容不符: %+v", logs)
	}
}
