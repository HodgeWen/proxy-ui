// Package singbox 管理 sing-box 内核：配置文件生成、子进程启停与状态、日志采集。
package singbox

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 进程控制参数。
const (
	startGrace  = 500 * time.Millisecond // 启动后观察窗口：进程立即退出视为启动失败
	stopTimeout = 5 * time.Second        // 优雅停止等待，超时强杀
	logCapacity = 1000                   // 日志环形缓冲行数

	trafficFirstLine = 3 * time.Second // Clash API /traffic 首行（每秒一拍）最长等待
)

// LogLine 内核日志行。
type LogLine struct {
	Time   time.Time `json:"time"`   // 采集时间
	Stream string    `json:"stream"` // 来源：stdout / stderr / panel（面板自身记录）
	Line   string    `json:"line"`   // 日志内容
}

// Status 内核运行状态。
type Status struct {
	Installed bool   `json:"installed"`               // 内核是否已安装
	Running   bool   `json:"running"`                 // 是否运行中
	PID       int    `json:"pid,omitempty"`           // 进程 PID（运行中时有值）
	Version   string `json:"version,omitempty"`       // 内核版本号（已安装时有值）
	Uptime    int64  `json:"uptimeSeconds,omitempty"` // 运行时长（秒，运行中时有值）
	CorePath  string `json:"corePath,omitempty"`      // 内核可执行文件路径
}

// Manager sing-box 内核子进程管理。
type Manager struct {
	dir     string        // 内核工作目录（可执行文件与 config.json 所在）
	mu      sync.Mutex    // 保护以下字段
	cmd     *exec.Cmd     // 当前内核进程
	exited  chan struct{} // 当前进程退出信号
	started time.Time     // 当前进程启动时间
	version string        // 已安装内核版本缓存（sing-box version 解析）
	logs    logBuffer     // 日志环形缓冲
}

// New 构造内核管理器；dir 为内核目录（<data>/core）。
func New(dir string) *Manager {
	return &Manager{dir: dir}
}

// CorePath 内核可执行文件路径。
func (m *Manager) CorePath() string { return filepath.Join(m.dir, "sing-box") }

// ConfigPath 内核配置文件路径。
func (m *Manager) ConfigPath() string { return filepath.Join(m.dir, "config.json") }

// Installed 内核是否已安装。
func (m *Manager) Installed() bool {
	_, err := os.Stat(m.CorePath())
	return err == nil
}

// WriteConfig 将配置对象序列化为 JSON（缩进两格）写入 config.json。
func (m *Manager) WriteConfig(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化内核配置失败: %w", err)
	}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fmt.Errorf("创建内核目录失败: %w", err)
	}
	if err := os.WriteFile(m.ConfigPath(), data, 0o600); err != nil {
		return fmt.Errorf("写入内核配置失败: %w", err)
	}
	return nil
}

// EnsureConfig 确保 config.json 存在；不存在时生成最小可运行配置。
func (m *Manager) EnsureConfig() error {
	if _, err := os.Stat(m.ConfigPath()); err == nil {
		return nil
	}
	return m.WriteConfig(MinConfig())
}

// CheckConfig 以 `sing-box check` 校验当前 config.json；未安装内核时跳过（返回 nil）。
func (m *Manager) CheckConfig(ctx context.Context) error {
	if !m.Installed() {
		return nil
	}
	// -c 传相对 -D 的文件名：sing-box 会把相对 -c 路径按 -D 目录解析，传完整路径会拼错位置
	out, err := exec.CommandContext(ctx, m.CorePath(), "check", "-D", m.dir, "-c", "config.json").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("内核配置校验失败: %s", msg)
	}
	return nil
}

// Start 拉起内核子进程；已在运行时幂等返回，未安装内核或进程立即退出（多为配置错误）时报错。
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil {
		return nil // 已在运行
	}
	if !m.Installed() {
		return errors.New("sing-box 内核未安装，请先在设置页下载安装")
	}
	if err := m.EnsureConfig(); err != nil {
		return err
	}
	// 注意不用请求 ctx：内核生命周期独立于触发启动的 HTTP 请求；-c 相对 -D 解析，只传文件名
	cmd := exec.Command(m.CorePath(), "run", "-D", m.dir, "-c", "config.json")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("接管内核输出失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("接管内核输出失败: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动内核进程失败: %w", err)
	}
	exited := make(chan struct{})
	m.cmd, m.started, m.exited = cmd, time.Now(), exited
	go m.pipeLogs("stdout", stdout)
	go m.pipeLogs("stderr", stderr)
	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		m.cmd = nil
		m.mu.Unlock()
		if err != nil {
			m.logs.add("panel", "内核进程退出: "+err.Error())
		} else {
			m.logs.add("panel", "内核进程已退出")
		}
		close(exited)
	}()
	// 观察窗口内进程即退出视为启动失败，日志里已有原因
	select {
	case <-exited:
		return errors.New("内核启动失败，进程已退出，请查看内核日志")
	case <-time.After(startGrace):
		return nil
	}
}

// Stop 停止内核：先发中断信号，超时未退出则强杀；未运行时幂等返回。
func (m *Manager) Stop() error {
	m.mu.Lock()
	cmd, exited := m.cmd, m.exited
	m.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
	}
	select {
	case <-exited:
	case <-time.After(stopTimeout):
		_ = cmd.Process.Kill()
		<-exited
	}
	return nil
}

// Restart 重启内核（先停再起，配置沿用现有 config.json）。
func (m *Manager) Restart() error {
	if err := m.Stop(); err != nil {
		return err
	}
	return m.Start()
}

// Status 采集内核状态；Version 由调用方经 Version(ctx) 补充（内部有缓存）。
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := Status{Installed: m.Installed(), Version: m.version, CorePath: m.CorePath()}
	if m.cmd != nil {
		st.Running = true
		if m.cmd.Process != nil {
			st.PID = m.cmd.Process.Pid
		}
		st.Uptime = int64(time.Since(m.started).Seconds())
	}
	return st
}

// Logs 返回最近 limit 条日志（时间升序）；limit<=0 返回全部缓冲。
func (m *Manager) Logs(limit int) []LogLine { return m.logs.snapshot(limit) }

// Version 返回已安装内核版本号（执行 sing-box version 解析并缓存；未安装返回空）。
func (m *Manager) Version(ctx context.Context) string {
	m.mu.Lock()
	cached := m.version
	m.mu.Unlock()
	if cached != "" {
		return cached
	}
	if !m.Installed() {
		return ""
	}
	out, err := exec.CommandContext(ctx, m.CorePath(), "version").Output()
	if err != nil {
		return ""
	}
	v := parseVersionOutput(string(out))
	if v != "" {
		m.mu.Lock()
		m.version = v
		m.mu.Unlock()
	}
	return v
}

// InvalidateVersion 清空版本缓存（内核重装后由调用方触发重新解析）。
func (m *Manager) InvalidateVersion() {
	m.mu.Lock()
	m.version = ""
	m.mu.Unlock()
}

// parseVersionOutput 从 `sing-box version` 输出解析版本号（第一行第三个字段）。
func parseVersionOutput(out string) string {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 && fields[0] == "sing-box" && fields[1] == "version" {
			return fields[2]
		}
	}
	return ""
}

// pipeLogs 逐行读取子进程输出写入日志缓冲，直至管道关闭。
func (m *Manager) pipeLogs(stream string, r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 512*1024)
	for scanner.Scan() {
		m.logs.add(stream, scanner.Text())
	}
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
