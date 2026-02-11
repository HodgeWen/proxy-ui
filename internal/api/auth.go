package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/s-ui/s-ui/internal/db"
	"golang.org/x/crypto/bcrypt"
)

type SetupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Confirm  string `json:"confirm"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

func SetupHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req SetupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		has, err := db.HasAdmin()
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		if has {
			http.Error(w, "admin already exists", http.StatusBadRequest)
			return
		}
		username := strings.TrimSpace(req.Username)
		if len(username) < 3 || len(username) > 50 {
			http.Error(w, "username must be 3-50 characters", http.StatusBadRequest)
			return
		}
		if len(req.Password) < 8 {
			http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
			return
		}
		if req.Password != req.Confirm {
			http.Error(w, "password and confirm must match", http.StatusBadRequest)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		if err := db.CreateAdmin(username, string(hash)); err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		if err := sm.RenewToken(r.Context()); err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		sm.Put(r.Context(), "user_id", int64(1))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}
}

func LoginHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		admin, err := db.GetAdminByUsername(strings.TrimSpace(req.Username))
		if err != nil || admin == nil {
			http.Error(w, "用户名或密码错误", http.StatusUnauthorized)
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
			http.Error(w, "用户名或密码错误", http.StatusUnauthorized)
			return
		}
		if err := sm.RenewToken(r.Context()); err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		sm.Put(r.Context(), "user_id", int64(admin.ID))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}
}

func MeHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !sm.Exists(r.Context(), "user_id") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}
}

func LogoutHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sm.Destroy(r.Context())
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}
}

func RequireSetupMiddleware(sm *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if path == "/api/setup" {
				next.ServeHTTP(w, r)
				return
			}
			has, err := db.HasAdmin()
			if err != nil {
				http.Error(w, "server error", http.StatusInternalServerError)
				return
			}
			if !has {
				if path != "/setup" && !strings.HasPrefix(path, "/setup/") {
					http.Redirect(w, r, "/setup", http.StatusFound)
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			if path == "/setup" || strings.HasPrefix(path, "/setup/") {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
			if (path == "/login" || strings.HasPrefix(path, "/login/")) && sm.Exists(r.Context(), "user_id") {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
