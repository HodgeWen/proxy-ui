package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/s-ui/s-ui/internal/config"
	"github.com/s-ui/s-ui/internal/core"
)

// RequireAuth returns 401 if user is not logged in.
func RequireAuth(sm *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !sm.Exists(r.Context(), "user_id") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func configPath(cfg *config.Config) string {
	return cfg.SingboxConfigPath
}

func binaryPath(cfg *config.Config) string {
	if cfg.SingboxBinaryPath != "" {
		return cfg.SingboxBinaryPath
	}
	return filepath.Join(cfg.DataDir, "bin", "sing-box")
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeCoreError(w http.ResponseWriter, status int, code, message, detail string) {
	writeJSON(w, status, map[string]string{
		"code":    code,
		"message": message,
		"detail":  detail,
	})
}

func writeProcessError(w http.ResponseWriter, err error) {
	var processErr *core.ProcessError
	if errors.As(err, &processErr) {
		status := http.StatusInternalServerError
		switch processErr.Code {
		case core.ProcessErrorNotInstalled:
			status = http.StatusBadRequest
		case core.ProcessErrorAlreadyRunning, core.ProcessErrorAlreadyStopped:
			status = http.StatusConflict
		}
		writeCoreError(w, status, string(processErr.Code), processErr.Message, processErr.Detail)
		return
	}

	writeCoreError(w, http.StatusInternalServerError, "CORE_INTERNAL_ERROR", "unexpected core error", err.Error())
}

func parseLogLines(v string, defaultLines int) (int, error) {
	if v == "" {
		return defaultLines, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, errors.New("lines must be a positive integer")
	}
	if n > 2000 {
		n = 2000
	}
	return n, nil
}

func tailLogLines(content string, n int) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if n <= 0 || n >= len(lines) {
		return lines
	}
	return lines[len(lines)-n:]
}

// StatusHandler returns sing-box lifecycle state, version, and path info.
func StatusHandler(sm *scs.SessionManager, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pm := core.NewProcessManagerFromConfig(cfg)
		snapshot := core.ResolveCoreState(pm)
		version, err := pm.Version()
		if err != nil {
			version = ""
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"running":    snapshot.Running, // compatibility field for existing callers
			"state":      snapshot.State,
			"actions":    snapshot.Actions,
			"lastError":  snapshot.LastError,
			"version":    version,
			"binaryPath": binaryPath(cfg),
			"configPath": configPath(cfg),
		})
	}
}

// StartHandler starts sing-box with semantic lifecycle error responses.
func StartHandler(sm *scs.SessionManager, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := configPath(cfg)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			writeCoreError(w, http.StatusNotFound, "CORE_CONFIG_NOT_FOUND", "config file not found", path)
			return
		}

		pm := core.NewProcessManagerFromConfig(cfg)
		if err := pm.Start(path); err != nil {
			writeProcessError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// StopHandler stops sing-box with semantic lifecycle error responses.
func StopHandler(sm *scs.SessionManager, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		pm := core.NewProcessManagerFromConfig(cfg)
		if err := pm.Stop(); err != nil {
			writeProcessError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// RestartHandler restarts sing-box with semantic lifecycle error responses.
func RestartHandler(sm *scs.SessionManager, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := configPath(cfg)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			writeCoreError(w, http.StatusNotFound, "CORE_CONFIG_NOT_FOUND", "config file not found", path)
			return
		}
		pm := core.NewProcessManagerFromConfig(cfg)
		if err := pm.Restart(path); err != nil {
			writeProcessError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// LogsHandler returns the latest N lines from sing-box log file.
func LogsHandler(sm *scs.SessionManager, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		n, err := parseLogLines(r.URL.Query().Get("lines"), 200)
		if err != nil {
			writeCoreError(w, http.StatusBadRequest, "CORE_INVALID_LOG_LINES", "invalid lines parameter", err.Error())
			return
		}

		logPath := filepath.Join(cfg.DataDir, "sing-box.log")
		content, err := os.ReadFile(logPath)
		if err != nil {
			if os.IsNotExist(err) {
				writeCoreError(w, http.StatusNotFound, "CORE_LOG_NOT_FOUND", "sing-box log file not found", logPath)
				return
			}
			writeCoreError(w, http.StatusInternalServerError, "CORE_LOG_READ_FAILED", "failed to read sing-box log file", err.Error())
			return
		}

		lines := tailLogLines(string(content), n)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"path":    logPath,
			"count":   len(lines),
			"entries": lines,
		})
	}
}

// ConfigHandler applies JSON config body via ApplyConfig.
func ConfigHandler(sm *scs.SessionManager, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		path := configPath(cfg)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			http.Error(w, "failed to create config dir", http.StatusInternalServerError)
			return
		}
		pm := core.NewProcessManagerFromConfig(cfg)
		if err := core.ApplyConfig(path, body, pm); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}
}

// ConfigFileHandler returns the raw sing-box config file as JSON.
func ConfigFileHandler(sm *scs.SessionManager, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := configPath(cfg)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "config file not found"})
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

// VersionsHandler returns GitHub releases for sing-box.
func VersionsHandler(sm *scs.SessionManager, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := core.NewCoreUpdater(configPath(cfg), binaryPath(cfg))
		releases, err := u.ListReleases()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"releases": releases,
		})
	}
}

// UpdateHandler updates sing-box to latest version.
func UpdateHandler(sm *scs.SessionManager, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		u := core.NewCoreUpdater(configPath(cfg), binaryPath(cfg))
		if err := u.Update(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			if err.Error() == "请设置 SINGBOX_BINARY_PATH 以启用核心更新" {
				w.WriteHeader(http.StatusBadRequest)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}
}

// RollbackHandler restores sing-box from backup.
func RollbackHandler(sm *scs.SessionManager, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		u := core.NewCoreUpdater(configPath(cfg), binaryPath(cfg))
		if err := u.Rollback(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			status := http.StatusInternalServerError
			if err.Error() == "暂无备份可回滚" || err.Error() == "请设置 SINGBOX_BINARY_PATH 以启用核心更新" {
				status = http.StatusBadRequest
			}
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}
}
