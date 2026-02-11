package core

import (
	"fmt"
	"os"
	"path/filepath"
)

// ApplyConfig writes configJSON to a temp file, runs sing-box check, and atomically
// renames to configPath on success. On check failure, original config is preserved
// and error includes check output for frontend Modal display.
func ApplyConfig(configPath string, configJSON []byte) error {
	dir := filepath.Dir(configPath)
	tmpPath := filepath.Join(dir, filepath.Base(configPath)+".tmp")

	if err := os.WriteFile(tmpPath, configJSON, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	defer os.Remove(tmpPath)

	pm := NewProcessManager()
	_, err := pm.Check(tmpPath)
	if err != nil {
		return err
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		return fmt.Errorf("atomic rename: %w", err)
	}
	return nil
}
