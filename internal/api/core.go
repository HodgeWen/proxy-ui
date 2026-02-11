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
