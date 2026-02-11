package core

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ProcessManager manages the sing-box process lifecycle.
type ProcessManager struct {
	configPath string
}

// NewProcessManager creates a ProcessManager with config path from env or default.
func NewProcessManager() *ProcessManager {
	configPath := os.Getenv("SINGBOX_CONFIG_PATH")
	if configPath == "" {
		configPath = "./config.json"
	}
	return &ProcessManager{configPath: configPath}
}

// ConfigPath returns the configured sing-box config path.
func (p *ProcessManager) ConfigPath() string {
	return p.configPath
}

// Version runs "sing-box version -n" (no color) and returns the trimmed stdout.
// Returns empty string and error if sing-box is not found or fails.
func (p *ProcessManager) Version() (string, error) {
	cmd := exec.Command("sing-box", "version", "-n")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// Check runs "sing-box check -c path" and returns combined output.
// On failure, returns error containing stderr output for frontend display.
func (p *ProcessManager) Check(configPath string) (output string, err error) {
	cmd := exec.Command("sing-box", "check", "-c", configPath)
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

// Restart stops any existing sing-box process and starts "sing-box run -c path" in background.
func (p *ProcessManager) Restart(configPath string) error {
	// Stop existing process
	_ = exec.Command("pkill", "-x", "sing-box").Run()

	// Start sing-box in background
	cmd := exec.Command("sing-box", "run", "-c", configPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start sing-box: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}
