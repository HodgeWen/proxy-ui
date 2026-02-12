package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/s-ui/s-ui/internal/core"
	"github.com/s-ui/s-ui/internal/db"
)

// SubscriptionHandler handles GET /sub/{token}. No auth required.
// Returns Base64 or Clash YAML per format detection; 403 for disabled/expired/over-limit.
func SubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	user, err := db.GetUserBySubscriptionToken(token)
	if err != nil || user == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !user.Enabled {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if user.ExpireAt != nil && !user.ExpireAt.After(time.Now().UTC()) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if user.TrafficLimit > 0 && user.TrafficUsed >= user.TrafficLimit {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Extract hostname from request Host header as fallback for inbounds without tls.server_name
	fallbackHost := r.Host
	if idx := strings.LastIndex(fallbackHost, ":"); idx > 0 {
		fallbackHost = fallbackHost[:idx]
	}

	wantClash := r.URL.Query().Get("format") == "clash" ||
		strings.Contains(strings.ToLower(r.Header.Get("User-Agent")), "clash")

	var body []byte
	if wantClash {
		body, err = core.GenerateClash(user, fallbackHost)
	} else {
		body, err = core.GenerateBase64(user, fallbackHost)
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(body) == 0 {
		body = []byte{}
	}

	w.Header().Set("subscription-userinfo", core.BuildUserinfoHeader(user))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(body)
}
