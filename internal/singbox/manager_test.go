package singbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeFakeCore 写一个模拟 sing-box 的 shell 脚本：version/check 子命令正常返回，run 时打印日志后常驻。
func writeFakeCore(t *testing.T, m *Manager) {
	t.Helper()
	script := `#!/bin/sh
if [ "$1" = "version" ]; then
  echo "sing-box version 9.9.9 test"
  echo "Environment: go1.26 test"
  exit 0
fi
if [ "$1" = "check" ]; then
  echo "configuration is valid"
  exit 0
fi
echo "stdout line 1"
echo "stderr line 1" >&2
sleep 60
`
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		t.Fatalf("创建内核目录失败: %v", err)
	}
	if err := os.WriteFile(m.CorePath(), []byte(script), 0o755); err != nil {
		t.Fatalf("写假内核失败: %v", err)
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(msg)
}

func TestStartWithoutCore(t *testing.T) {
	m := New(t.TempDir())
	if err := m.Start(); err == nil || !strings.Contains(err.Error(), "未安装") {
		t.Errorf("未安装内核时 Start 应报错, got %v", err)
	}
}

func TestVersionParses(t *testing.T) {
	m := New(t.TempDir())
	writeFakeCore(t, m)
	if v := m.Version(context.Background()); v != "9.9.9" {
		t.Errorf("版本解析错误: %q", v)
	}
	if got := parseVersionOutput("sing-box version 1.14.4\nEnvironment: x\n"); got != "1.14.4" {
		t.Errorf("parseVersionOutput = %q, want 1.14.4", got)
	}
	if got := parseVersionOutput("garbage"); got != "" {
		t.Errorf("无法解析的输出应返回空, got %q", got)
	}
}

func TestCheckConfig(t *testing.T) {
	// 未安装内核时跳过校验
	if err := New(t.TempDir()).CheckConfig(context.Background()); err != nil {
		t.Errorf("未安装内核应跳过校验: %v", err)
	}
	m := New(t.TempDir())
	writeFakeCore(t, m)
	if err := m.EnsureConfig(); err != nil {
		t.Fatalf("EnsureConfig 失败: %v", err)
	}
	if err := m.CheckConfig(context.Background()); err != nil {
		t.Errorf("合法配置不应报错: %v", err)
	}
	// check 退出的假内核：错误信息应带回输出
	script := "#!/bin/sh\necho 'FATAL parse config failed' >&2\nexit 1\n"
	if err := os.WriteFile(m.CorePath(), []byte(script), 0o755); err != nil {
		t.Fatalf("改写假内核失败: %v", err)
	}
	err := m.CheckConfig(context.Background())
	if err == nil || !strings.Contains(err.Error(), "parse config failed") {
		t.Errorf("校验失败应带回内核输出, got %v", err)
	}
}

func TestLifecycleAndLogs(t *testing.T) {
	m := New(t.TempDir())
	writeFakeCore(t, m)

	if err := m.EnsureConfig(); err != nil {
		t.Fatalf("EnsureConfig 失败: %v", err)
	}
	data, err := os.ReadFile(m.ConfigPath())
	if err != nil || !strings.Contains(string(data), `"level": "info"`) {
		t.Fatalf("config.json 未按最小配置生成: %v %s", err, data)
	}

	// 幂等：重复 EnsureConfig 不覆盖已有配置
	if err := os.WriteFile(m.ConfigPath(), []byte(`{"log":{"level":"debug"}}`), 0o600); err != nil {
		t.Fatalf("改写配置失败: %v", err)
	}
	_ = m.EnsureConfig()
	data, _ = os.ReadFile(m.ConfigPath())
	if strings.Contains(string(data), `"level": "info"`) {
		t.Error("EnsureConfig 不应覆盖已存在的配置")
	}

	if err := m.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	st := m.Status()
	if !st.Running || st.PID == 0 || !st.Installed {
		t.Errorf("启动后状态不符: %+v", st)
	}
	if err := m.Start(); err != nil {
		t.Errorf("重复 Start 应幂等: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		for _, l := range m.Logs(0) {
			if l.Stream == "stdout" && l.Line == "stdout line 1" {
				return true
			}
		}
		return false
	}, "应采集到 stdout 日志")
	foundStderr := false
	for _, l := range m.Logs(0) {
		if l.Stream == "stderr" && l.Line == "stderr line 1" {
			foundStderr = true
		}
	}
	if !foundStderr {
		t.Errorf("应采集到 stderr 日志: %+v", m.Logs(0))
	}

	if err := m.Restart(); err != nil {
		t.Fatalf("Restart 失败: %v", err)
	}
	if st := m.Status(); !st.Running {
		t.Errorf("重启后应运行中: %+v", st)
	}

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop 失败: %v", err)
	}
	if st := m.Status(); st.Running {
		t.Errorf("停止后不应运行: %+v", st)
	}
	if err := m.Stop(); err != nil {
		t.Errorf("重复 Stop 应幂等: %v", err)
	}
}

func TestWriteConfig(t *testing.T) {
	dir := t.TempDir()
	m := New(filepath.Join(dir, "core"))
	if err := m.WriteConfig(map[string]any{"log": map[string]any{"level": "debug"}}); err != nil {
		t.Fatalf("WriteConfig 失败: %v", err)
	}
	data, err := os.ReadFile(m.ConfigPath())
	if err != nil || !strings.Contains(string(data), `"level": "debug"`) {
		t.Fatalf("配置未写入或格式不符: %v %s", err, data)
	}
}

// TestStartWithoutConfig 回归：config.json 缺失时 Start 内部经 EnsureConfig 写配置，
// 与 Manager 锁交互不得死锁（曾在 WriteConfig 内加锁导致）。
func TestStartWithoutConfig(t *testing.T) {
	m := New(t.TempDir())
	writeFakeCore(t, m)
	done := make(chan error, 1)
	go func() { done <- m.Start() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start 失败: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start 疑似死锁：EnsureConfig 与锁路径冲突")
	}
	if _, err := os.Stat(m.ConfigPath()); err != nil {
		t.Errorf("Start 应生成 config.json: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Errorf("Stop 失败: %v", err)
	}
}
