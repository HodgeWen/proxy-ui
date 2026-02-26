package core

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/s-ui/s-ui/internal/config"
)

// ProcessManager manages the sing-box process lifecycle.
type ProcessManager struct {
	configPath string
	binaryPath string // explicit path; empty = use LookPath
}

type ProcessErrorCode string

const (
	ProcessErrorNotInstalled   ProcessErrorCode = "CORE_NOT_INSTALLED"
	ProcessErrorAlreadyRunning ProcessErrorCode = "CORE_ALREADY_RUNNING"
	ProcessErrorAlreadyStopped ProcessErrorCode = "CORE_ALREADY_STOPPED"
	ProcessErrorStartFailed    ProcessErrorCode = "CORE_START_FAILED"
	ProcessErrorStopFailed     ProcessErrorCode = "CORE_STOP_FAILED"
	ProcessErrorRestartFailed  ProcessErrorCode = "CORE_RESTART_FAILED"
)

const (
	startupHealthWindow = 500 * time.Millisecond
	stopGraceWindow    = 2 * time.Second
	stopForceWindow    = 1 * time.Second
	stopPollInterval   = 100 * time.Millisecond
)

// ProcessError carries semantic lifecycle errors for API mapping.
type ProcessError struct {
	Code    ProcessErrorCode
	Message string
	Detail  string
}

func (e *ProcessError) Error() string {
	if e.Detail == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Message, e.Detail)
}

func newProcessError(code ProcessErrorCode, message, detail string) error {
	return &ProcessError{
		Code:    code,
		Message: message,
		Detail:  detail,
	}
}

// IsProcessErrorCode checks whether err is a ProcessError with the given code.
func IsProcessErrorCode(err error, code ProcessErrorCode) bool {
	var processErr *ProcessError
	if !errors.As(err, &processErr) {
		return false
	}
	return processErr.Code == code
}

// NewProcessManagerFromConfig creates a ProcessManager from panel config.
func NewProcessManagerFromConfig(cfg *config.Config) *ProcessManager {
	configPath := cfg.SingboxConfigPath
	binaryPath := cfg.SingboxBinaryPath
	if binaryPath == "" {
		binaryPath = filepath.Join(cfg.DataDir, "bin", "sing-box")
	}
	if binaryPath != "" {
		dir := filepath.Dir(binaryPath)
		_ = os.MkdirAll(dir, 0755)
	}
	return &ProcessManager{configPath: configPath, binaryPath: binaryPath}
}

// NewProcessManagerWithBinary creates a ProcessManager with explicit binary path.
func NewProcessManagerWithBinary(configPath, binaryPath string) *ProcessManager {
	if binaryPath != "" {
		dir := filepath.Dir(binaryPath)
		_ = os.MkdirAll(dir, 0755)
	}
	return &ProcessManager{configPath: configPath, binaryPath: binaryPath}
}

// ConfigPath returns the configured sing-box config path.
func (p *ProcessManager) ConfigPath() string {
	return p.configPath
}

// Available returns true if the sing-box binary is found.
func (p *ProcessManager) Available() bool {
	if p.binaryPath != "" {
		_, err := os.Stat(p.binaryPath)
		return err == nil
	}
	_, err := exec.LookPath("sing-box")
	return err == nil
}

// BinaryPath returns the configured binary path (may be empty).
func (p *ProcessManager) BinaryPath() string {
	return p.binaryPath
}

func (p *ProcessManager) binary() string {
	if p.binaryPath != "" {
		return p.binaryPath
	}
	return "sing-box"
}

// Version runs "sing-box version -n" (no color) and returns the trimmed stdout.
// Returns empty string and error if sing-box is not found or fails.
func (p *ProcessManager) Version() (string, error) {
	cmd := exec.Command(p.binary(), "version", "-n")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// Check runs "sing-box check -c path" and returns combined output.
// If sing-box is not installed, logs a warning and returns success (skip check).
// On failure, returns error containing stderr output for frontend display.
func (p *ProcessManager) Check(configPath string) (output string, err error) {
	if !p.Available() {
		log.Println("[warn] sing-box not found in PATH, skipping config check")
		return "", nil
	}
	cmd := exec.Command(p.binary(), "check", "-c", configPath)
	out, e := cmd.CombinedOutput()
	if e != nil {
		return string(out), fmt.Errorf("check failed: %w\n%s", e, out)
	}
	return string(out), nil
}

// IsRunning returns true if sing-box process exists.
// When configPath is set, only counts processes launched with that config path.
func (p *ProcessManager) IsRunning() bool {
	pids, err := p.runningPIDs()
	if err != nil {
		return legacyIsRunning()
	}
	return len(pids) > 0
}

// Start starts sing-box in background and validates startup health.
func (p *ProcessManager) Start(configPath string) error {
	if !p.Available() {
		return newProcessError(ProcessErrorNotInstalled, "sing-box binary is not installed", p.BinaryPath())
	}
	if p.IsRunning() {
		return newProcessError(ProcessErrorAlreadyRunning, "sing-box is already running", "")
	}

	cmd := exec.Command(p.binary(), "run", "-c", configPath)
	if err := cmd.Start(); err != nil {
		setLastFailure(newFailureContext("failed to start sing-box", "start", err.Error()))
		return newProcessError(ProcessErrorStartFailed, "failed to start sing-box", err.Error())
	}

	time.Sleep(startupHealthWindow)
	if !p.IsRunning() {
		waitErr := cmd.Wait()
		detail := "process exited during startup health check"
		if waitErr != nil {
			detail = waitErr.Error()
		}
		setLastFailure(newFailureContext("sing-box startup failed", "start", detail))
		return newProcessError(ProcessErrorStartFailed, "sing-box exited during startup", detail)
	}

	clearLastFailure()
	// Keep a background waiter to reap process exit and avoid defunct children.
	go reapProcess(cmd)
	return nil
}

// Stop stops the running sing-box process.
func (p *ProcessManager) Stop() error {
	pids, err := p.runningPIDs()
	if err != nil {
		return newProcessError(ProcessErrorStopFailed, "failed to stop sing-box", err.Error())
	}
	if len(pids) == 0 {
		return newProcessError(ProcessErrorAlreadyStopped, "sing-box is already stopped", "")
	}

	if err := signalPIDs("TERM", pids); err != nil {
		// Re-check to handle races where process exits before signal delivery.
		if !p.IsRunning() {
			return nil
		}
		return newProcessError(ProcessErrorStopFailed, "failed to stop sing-box", err.Error())
	}

	if waitForProcessStop(p.IsRunning, stopGraceWindow) {
		return nil
	}

	remaining, err := p.runningPIDs()
	if err != nil {
		return newProcessError(ProcessErrorStopFailed, "failed to stop sing-box", err.Error())
	}

	// Escalate to SIGKILL when graceful stop times out.
	if err := signalPIDs("KILL", remaining); err != nil {
		if !p.IsRunning() {
			return nil
		}
		return newProcessError(ProcessErrorStopFailed, "failed to stop sing-box", err.Error())
	}
	if waitForProcessStop(p.IsRunning, stopForceWindow) {
		return nil
	}

	remaining, err = p.runningPIDs()
	if err != nil {
		return newProcessError(ProcessErrorStopFailed, "failed to stop sing-box", err.Error())
	}
	return newProcessError(ProcessErrorStopFailed, "failed to stop sing-box", fmt.Sprintf("%d managed process(es) still running after SIGTERM and SIGKILL", len(remaining)))
}

// Restart is a strict Stop + Start sequence.
func (p *ProcessManager) Restart(configPath string) error {
	stopErr := p.Stop()
	if stopErr != nil && !IsProcessErrorCode(stopErr, ProcessErrorAlreadyStopped) {
		return newProcessError(ProcessErrorRestartFailed, "failed to restart sing-box", stopErr.Error())
	}

	if err := p.Start(configPath); err != nil {
		return err
	}
	return nil
}

func (p *ProcessManager) runningPIDs() ([]int, error) {
	pids, err := listSingBoxPIDs()
	if err != nil {
		return nil, err
	}

	// Backward compatibility: when config path is unknown, keep legacy behavior.
	if strings.TrimSpace(p.configPath) == "" {
		return pids, nil
	}

	targetConfig := normalizePath(p.configPath)
	matched := make([]int, 0, len(pids))
	for _, pid := range pids {
		ok, err := processUsesConfig(pid, targetConfig)
		if err != nil {
			continue
		}
		if ok {
			matched = append(matched, pid)
		}
	}
	return matched, nil
}

func listSingBoxPIDs() ([]int, error) {
	out, err := exec.Command("pgrep", "-x", "sing-box").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return []int{}, nil
		}
		return nil, err
	}

	fields := strings.Fields(string(out))
	pids := make([]int, 0, len(fields))
	for _, field := range fields {
		pid, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		if pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func processUsesConfig(pid int, targetConfig string) (bool, error) {
	cmdlinePath := fmt.Sprintf("/proc/%d/cmdline", pid)
	data, err := os.ReadFile(cmdlinePath)
	if err != nil {
		return false, err
	}

	parts := strings.Split(string(data), "\x00")
	for idx, part := range parts {
		part = strings.TrimSpace(part)
		switch {
		case part == "-c" || part == "--config":
			if idx+1 < len(parts) && samePath(parts[idx+1], targetConfig) {
				return true, nil
			}
		case strings.HasPrefix(part, "-c="):
			if samePath(strings.TrimPrefix(part, "-c="), targetConfig) {
				return true, nil
			}
		case strings.HasPrefix(part, "--config="):
			if samePath(strings.TrimPrefix(part, "--config="), targetConfig) {
				return true, nil
			}
		}
	}
	return false, nil
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func samePath(left, right string) bool {
	return normalizePath(left) == normalizePath(right)
}

func signalPIDs(signal string, pids []int) error {
	if len(pids) == 0 {
		return nil
	}

	args := []string{"-" + signal}
	for _, pid := range pids {
		args = append(args, strconv.Itoa(pid))
	}

	out, err := exec.Command("kill", args...).CombinedOutput()
	if err == nil {
		return nil
	}

	detail := strings.TrimSpace(string(out))
	if detail == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, detail)
}

func legacyIsRunning() bool {
	cmd := exec.Command("pgrep", "-x", "sing-box")
	err := cmd.Run()
	return err == nil
}

func waitForProcessStop(isRunning func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !isRunning() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(stopPollInterval)
	}
}

func reapProcess(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	_ = cmd.Wait()
}
