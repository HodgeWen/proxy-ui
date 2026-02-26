package core

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

const startupHealthWindow = 500 * time.Millisecond

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
// Uses pgrep-style check via exec.
func (p *ProcessManager) IsRunning() bool {
	cmd := exec.Command("pgrep", "-x", "sing-box")
	err := cmd.Run()
	return err == nil
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
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	return nil
}

// Stop stops the running sing-box process.
func (p *ProcessManager) Stop() error {
	if !p.IsRunning() {
		return newProcessError(ProcessErrorAlreadyStopped, "sing-box is already stopped", "")
	}

	if err := exec.Command("pkill", "-x", "sing-box").Run(); err != nil {
		// Re-check to handle short race where process exits before pkill.
		if !p.IsRunning() {
			return newProcessError(ProcessErrorAlreadyStopped, "sing-box is already stopped", "")
		}
		return newProcessError(ProcessErrorStopFailed, "failed to stop sing-box", err.Error())
	}

	time.Sleep(100 * time.Millisecond)
	if p.IsRunning() {
		return newProcessError(ProcessErrorStopFailed, "failed to stop sing-box", "process is still running")
	}

	return nil
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
