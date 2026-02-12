package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/alexedwards/scs/v2"
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

func configPath() string {
	if p := os.Getenv("SINGBOX_CONFIG_PATH"); p != "" {
		return p
	}
	return "./config.json"
}

func binaryPath() string {
	if p := os.Getenv("SINGBOX_BINARY_PATH"); p != "" {
		return p
	}
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	return filepath.Join(dataDir, "bin", "sing-box")
}

// StatusHandler returns sing-box running state and version.
func StatusHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pm := core.NewProcessManager()
		version, err := pm.Version()
		if err != nil {
			version = ""
		}
		running := pm.IsRunning()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"running": running,
			"version": version,
		})
	}
}

// RestartHandler restarts sing-box if config file exists.
func RestartHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := configPath()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			http.Error(w, "config file not found", http.StatusNotFound)
			return
		}
		pm := core.NewProcessManager()
		if err := pm.Restart(path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}
}

// ConfigHandler applies JSON config body via ApplyConfig.
func ConfigHandler(sm *scs.SessionManager) http.HandlerFunc {
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
		path := configPath()
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			http.Error(w, "failed to create config dir", http.StatusInternalServerError)
			return
		}
		if err := core.ApplyConfig(path, body); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}
}

// VersionsHandler returns GitHub releases for sing-box.
func VersionsHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := core.NewCoreUpdater(binaryPath())
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
func UpdateHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		u := core.NewCoreUpdater(binaryPath())
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
func RollbackHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		u := core.NewCoreUpdater(binaryPath())
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
