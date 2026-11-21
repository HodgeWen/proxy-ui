// Package singbox 管理内置 sing-box 内核：配置生成、进程内实例生命周期与统计采集。
//
// 内核以库形式编进面板（见 embedded.go），不再下载外部二进制；这样能启用
// v2ray_api，从而按「用户」统计流量——这是共享入站下实现配额的前提。
package singbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/service/filemanager"
)

// 进程控制参数。
const (
	stopTimeout = 5 * time.Second // 优雅停止等待，超时视为失败
	logCapacity = 1000            // 日志环形缓冲行数
)

// LogLine 内核日志行。
type LogLine struct {
	Time   time.Time `json:"time"`   // 采集时间
	Stream string    `json:"stream"` // 来源：stdout / stderr / panel（面板自身记录）
	Line   string    `json:"line"`   // 日志内容
}

// Status 内核运行状态。
type Status struct {
	Installed bool   `json:"installed"`               // 内核是否可用（内置内核实为恒真）
	Running   bool   `json:"running"`                 // 是否运行中
	Version   string `json:"version,omitempty"`       // 内核版本号
	Uptime    int64  `json:"uptimeSeconds,omitempty"` // 运行时长（秒）
	Embedded  bool   `json:"embedded"`                // 是否内置内核（恒真）
	Error     string `json:"error,omitempty"`         // 上次启动失败原因（未运行时给出，便于面板提示修复方向）
}

// Manager sing-box 内核实例管理：进程内构建、启动、停止与重建。
type Manager struct {
	dir       string // 工作目录（cache.db 等内核落盘文件）
	statsAddr string // 本实例的 v2ray 统计监听地址（构造时选定的空闲回环端口）

	startMu    sync.Mutex // 串行化「停旧-起新」全过程（并发应用会互相踩踏独占资源）
	mu         sync.Mutex
	box        *box.Box
	started    time.Time
	generation uint64  // 实例代号：每次成功重建递增，统计采集据此重置基线
	lastGood   *Config // 上一个成功启动的配置（新配置启动失败时回滚）
	lastErr    string  // 最近一次启动失败原因（成功启动后清空）
	logs       logBuffer
}

// New 构造内核管理器；dir 为工作目录（<data>/core）。
func New(dir string) *Manager {
	return &Manager{dir: dir, statsAddr: freeStatsAddr()}
}

// statsAddrFallback 回环端口探测失败时的兜底地址（仅本机）。
const statsAddrFallback = "127.0.0.1:19091"

// freeStatsAddr 选一个本机回环空闲端口承载 v2ray 统计服务。
//
// 不写死端口：同一台机器上可能跑多个面板实例（或测试里多个内核管理器），
// 固定端口会让第二个实例直接启动失败。
func freeStatsAddr() string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return statsAddrFallback
	}
	defer ln.Close()
	return ln.Addr().String()
}

// StatsAddr 返回本实例内核的统计服务监听地址。
func (m *Manager) StatsAddr() string { return m.statsAddr }

// StatsListen 统计服务的默认监听地址（供不持管理器实例的配置生成使用）。
func StatsListen() string { return statsAddrFallback }

// Status 采集内核运行状态。
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := Status{Installed: true, Embedded: true, Version: EmbeddedVersion}
	if m.box != nil {
		st.Running = true
		st.Uptime = int64(time.Since(m.started).Seconds())
	} else {
		st.Error = m.lastErr
	}
	return st
}

// Logs 返回最近 limit 条日志（时间升序）；limit<=0 返回全部缓冲。
func (m *Manager) Logs(limit int) []LogLine { return m.logs.snapshot(limit) }

// Start 以给定配置启动内核。
//
// 内核的缓存文件与统计监听端口是独占资源，两个实例无法共存，因此必须先停旧实例。
// 为避免并发应用（配额核对与写操作同时触发）互相踩踏，整个「停旧-起新」过程串行化；
// 为免一次错误配置把代理整个停掉，新配置启动失败时自动回滚到上一个可用配置。
func (m *Manager) Start(cfg Config) error {
	m.withStatsAddr(&cfg)
	m.startMu.Lock()
	defer m.startMu.Unlock()
	return m.startLocked(cfg)
}

// Apply 按新配置重建内核；新配置与当前运行的配置完全一致时直接返回（不必白白重启代理）。
func (m *Manager) Apply(cfg Config) error {
	m.withStatsAddr(&cfg)
	m.startMu.Lock()
	defer m.startMu.Unlock()
	m.mu.Lock()
	last, running := m.lastGood, m.box != nil
	m.mu.Unlock()
	if running && last != nil && configEqual(*last, cfg) {
		return nil
	}
	return m.startLocked(cfg)
}

// startLocked 停旧实例并启动新实例，新配置失败时自动回滚到上一个可用配置（调用方持 startMu）。
func (m *Manager) startLocked(cfg Config) error {
	m.mu.Lock()
	last := m.lastGood
	m.mu.Unlock()
	if err := m.stopInstance(); err != nil {
		return err
	}
	if err := m.start(cfg); err != nil {
		m.mu.Lock()
		m.lastErr = err.Error()
		m.mu.Unlock()
		if last != nil {
			if rollbackErr := m.start(*last); rollbackErr == nil {
				return fmt.Errorf("%w（已回滚到上一个可用配置，代理未中断）", err)
			}
		}
		return err
	}
	return nil
}

// configEqual 判断两份内核配置的序列化结果是否一致。
func configEqual(a, b Config) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}

// withStatsAddr 把统计服务监听地址指向本管理器的端口，避免多实例争用同一端口（不原地修改调用方指针）。
func (m *Manager) withStatsAddr(cfg *Config) {
	if cfg.Experimental == nil {
		cfg.Experimental = &Experimental{}
	} else {
		exp := *cfg.Experimental
		cfg.Experimental = &exp
	}
	if cfg.Experimental.V2RayAPI == nil {
		cfg.Experimental.V2RayAPI = &V2RayAPI{Stats: &V2RayStatsConf{Enabled: true}}
	} else {
		v2 := *cfg.Experimental.V2RayAPI
		cfg.Experimental.V2RayAPI = &v2
	}
	cfg.Experimental.V2RayAPI.Listen = m.statsAddr
}

// start 构建并启动实例，成功后记为「上一个可用配置」（调用方持 startMu）。
func (m *Manager) start(cfg Config) error {
	instance, err := m.build(cfg)
	if err != nil {
		return err
	}
	if err := instance.Start(); err != nil {
		_ = instance.Close()
		return fmt.Errorf("内核启动失败: %w", err)
	}
	m.mu.Lock()
	m.box, m.started, m.generation, m.lastErr = instance, time.Now(), m.generation+1, ""
	m.lastGood = &cfg
	m.mu.Unlock()
	m.log("panel", "内核已启动")
	return nil
}

// Generation 返回内核实例代号（每次成功重建递增）；统计采集据此重置差分基线。
func (m *Manager) Generation() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generation
}

// Stop 停止内核；未运行时幂等返回。
func (m *Manager) Stop() error {
	m.startMu.Lock()
	defer m.startMu.Unlock()
	err := m.stopInstance()
	if err == nil {
		m.log("panel", "内核已停止")
	}
	return err
}

// stopInstance 关闭当前实例（调用方持 startMu）；等待 Close 期间释放 m.mu 以免阻塞只读状态查询。
func (m *Manager) stopInstance() error {
	m.mu.Lock()
	if m.box == nil {
		m.mu.Unlock()
		return nil
	}
	instance := m.box
	m.box = nil
	m.mu.Unlock()

	done := make(chan error, 1)
	go func() { done <- instance.Close() }()
	select {
	case err := <-done:
		if err != nil {
			m.log("panel", "内核关闭异常: "+err.Error())
		}
		return nil
	case <-time.After(stopTimeout):
		// 超时也继续持有引用，便于下次关闭重试，避免端口/缓存文件被泄漏占用
		m.mu.Lock()
		m.box = instance
		m.mu.Unlock()
		m.log("panel", "内核停止超时")
		return errors.New("内核停止超时")
	}
}

// build 把配置 JSON 解析为内核选项并构建实例（不持锁：box.New 可能耗时）。
func (m *Manager) build(cfg Config) (*box.Box, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("序列化内核配置失败: %w", err)
	}
	ctx := m.boxContext()
	var opts option.Options
	if err := opts.UnmarshalJSONContext(ctx, raw); err != nil {
		return nil, fmt.Errorf("内核配置不合法: %w", err)
	}
	instance, err := box.New(box.Options{
		Context:           ctx,
		Options:           opts,
		PlatformLogWriter: &panelPlatformWriter{mgr: m},
	})
	if err != nil {
		return nil, fmt.Errorf("构建内核失败: %w", err)
	}
	return instance, nil
}

// boxContext 构建带全部注册表、日志写入与工作目录的 sing-box 上下文。
func (m *Manager) boxContext() context.Context {
	ctx := box.Context(context.Background(),
		include.InboundRegistry(), include.OutboundRegistry(), include.EndpointRegistry(),
		include.DNSTransportRegistry(), include.ServiceRegistry(), include.CertificateProviderRegistry())
	// 内核用 filemanager 的 basePath 解析相对路径（cache.db 等）；不设置会落到进程工作目录，
	// 与数据目录分离且可能因不可写导致缓存失效，因此固定为内核工作目录。
	dir := m.dir
	if dir == "" {
		dir = "."
	}
	_ = os.MkdirAll(dir, 0o755)
	return filemanager.WithDefault(ctx, dir, "", os.Getuid(), os.Getgid())
}

// CheckConfig 校验配置能否被内核解析（不启动），未通过返回错误。
func (m *Manager) CheckConfig(cfg Config) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化内核配置失败: %w", err)
	}
	var opts option.Options
	if err := opts.UnmarshalJSONContext(m.boxContext(), raw); err != nil {
		return fmt.Errorf("内核配置校验失败: %w", err)
	}
	return nil
}

// panelPlatformWriter 把内核日志写入面板环形缓冲（替代旧子进程 stdout 采集）。
type panelPlatformWriter struct{ mgr *Manager }

// WriteMessage 实现 log.PlatformWriter。
func (w *panelPlatformWriter) WriteMessage(_ log.Level, message string) {
	w.mgr.log("core", message)
}

// log 追加一行日志（不持管理器锁，可从内核回调任意线程安全调用）。
func (m *Manager) log(stream, line string) {
	m.logs.add(stream, line)
}

// logBuffer 固定容量日志环形缓冲。
type logBuffer struct {
	mu    sync.Mutex
	lines []LogLine
}

// add 追加一行日志，超容量丢弃最旧。
func (b *logBuffer) add(stream, line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, LogLine{Time: time.Now(), Stream: stream, Line: line})
	if n := len(b.lines) - logCapacity; n > 0 {
		b.lines = b.lines[n:]
	}
}

// snapshot 返回最近 limit 条日志副本（时间升序）；limit<=0 返回全部。
func (b *logBuffer) snapshot(limit int) []LogLine {
	b.mu.Lock()
	defer b.mu.Unlock()
	if limit <= 0 || limit > len(b.lines) {
		limit = len(b.lines)
	}
	out := make([]LogLine, limit)
	copy(out, b.lines[len(b.lines)-limit:])
	return out
}
