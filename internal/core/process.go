package core

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/s-ui/s-ui/internal/config"
)

// ProcessManager manages the sing-box process lifecycle.
type ProcessManager struct {
	configPath string
	binaryPath string // explicit path; empty = use LookPath
}

// NewProcessManager creates a ProcessManager with config path from env or default.
func NewProcessManager() *ProcessManager {
	configPath := os.Getenv("SINGBOX_CONFIG_PATH")
	if configPath == "" {
		configPath = "./config.json"
	}
	binaryPath := os.Getenv("SINGBOX_BINARY_PATH")
	if binaryPath != "" {
		dir := filepath.Dir(binaryPath)
		_ = os.MkdirAll(dir, 0755)
	}
	return &ProcessManager{configPath: configPath, binaryPath: binaryPath}
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

// Version runs "sing-box version -n" (no color) and returns the trimmed stdout.
// Returns empty string and error if sing-box is not found or fails.
func (p *ProcessManager) Version() (string, error) {
	bin := "sing-box"
	if p.binaryPath != "" {
		bin = p.binaryPath
	}
	cmd := exec.Command(bin, "version", "-n")
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
	bin := "sing-box"
	if p.binaryPath != "" {
		bin = p.binaryPath
	}
	cmd := exec.Command(bin, "check", "-c", configPath)
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

// Restart stops any existing sing-box process and starts sing-box in background.
// If sing-box is not installed, logs a warning and returns nil (no-op).
func (p *ProcessManager) Restart(configPath string) error {
	if !p.Available() {
		log.Println("[warn] sing-box not found in PATH, skipping restart")
		return nil
	}

	// Stop existing process
	_ = exec.Command("pkill", "-x", "sing-box").Run()

	bin := "sing-box"
	if p.binaryPath != "" {
		bin = p.binaryPath
	}
	cmd := exec.Command(bin, "run", "-c", configPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start sing-box: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}
