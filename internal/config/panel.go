package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds panel configuration (file + env overrides).
type Config struct {
	Addr               string `json:"addr"`
	SessionSecret      string `json:"session_secret"`
	DataDir            string `json:"data_dir"`
	SingboxConfigPath  string `json:"singbox_config_path"`
	SingboxBinaryPath  string `json:"singbox_binary_path"`
}

// defaultConfig returns Config with sensible defaults.
// Caller should fill SessionSecret before writing to file.
func defaultConfig() *Config {
	return &Config{
		Addr:              ":8080",
		SessionSecret:     "", // must be set before write
		DataDir:           "./data",
		SingboxConfigPath: "",
		SingboxBinaryPath: "",
	}
}

// LoadConfig loads panel config. If CONFIG_PATH env is set, uses file mode;
// otherwise env-only mode (no file, build from env for backward compat).
// When CONFIG_PATH set: if file missing, writes defaultConfig with random
// session_secret and returns; if exists, reads and unmarshals.
// Env overrides apply in both modes: ADDR, DATA_DIR, SINGBOX_CONFIG_PATH, SINGBOX_BINARY_PATH.
func LoadConfig() (*Config, error) {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		// Env-only mode: no file, build from env
		cfg := defaultConfig()
		cfg.Addr = envOr("ADDR", cfg.Addr)
		cfg.DataDir = envOr("DATA_DIR", cfg.DataDir)
		cfg.SingboxConfigPath = envOr("SINGBOX_CONFIG_PATH", filepath.Join(cfg.DataDir, "sing-box.json"))
		cfg.SingboxBinaryPath = envOr("SINGBOX_BINARY_PATH", "")
		if cfg.SingboxBinaryPath == "" {
			cfg.SingboxBinaryPath = filepath.Join(cfg.DataDir, "bin", "sing-box")
		}
		return cfg, nil
	}

	// File mode
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// First run: create default with random session secret
		cfg := defaultConfig()
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, err
		}
		cfg.SessionSecret = hex.EncodeToString(secret)
		cfg.Addr = envOr("ADDR", cfg.Addr)
		cfg.DataDir = envOr("DATA_DIR", cfg.DataDir)
		cfg.SingboxConfigPath = envOr("SINGBOX_CONFIG_PATH", filepath.Join(cfg.DataDir, "sing-box.json"))
		cfg.SingboxBinaryPath = envOr("SINGBOX_BINARY_PATH", "")
		if cfg.SingboxBinaryPath == "" {
			cfg.SingboxBinaryPath = filepath.Join(cfg.DataDir, "bin", "sing-box")
		}
		dir := filepath.Dir(configPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
		b, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(configPath, b, 0644); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	// File exists: read and unmarshal
	b, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, err
	}
	// Apply env overrides
	cfg.Addr = envOr("ADDR", cfg.Addr)
	cfg.DataDir = envOr("DATA_DIR", cfg.DataDir)
	cfg.SingboxConfigPath = envOr("SINGBOX_CONFIG_PATH", cfg.SingboxConfigPath)
	cfg.SingboxBinaryPath = envOr("SINGBOX_BINARY_PATH", cfg.SingboxBinaryPath)
	if cfg.SingboxConfigPath == "" {
		cfg.SingboxConfigPath = filepath.Join(cfg.DataDir, "sing-box.json")
	}
	if cfg.SingboxBinaryPath == "" {
		cfg.SingboxBinaryPath = filepath.Join(cfg.DataDir, "bin", "sing-box")
	}
	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// EnsureDir creates the directory if it does not exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// DBPath returns the s-ui database path for the given data dir.
func DBPath(dataDir string) string {
	return filepath.Join(dataDir, "s-ui.db")
}
