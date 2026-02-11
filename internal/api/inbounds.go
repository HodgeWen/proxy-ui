package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/s-ui/s-ui/internal/db"
)

// inboundItem is the API response shape for list/get.
type inboundItem struct {
	ID            uint   `json:"id"`
	Tag           string `json:"tag"`
	Protocol      string `json:"protocol"`
	Listen        string `json:"listen"`
	ListenPort    uint   `json:"listen_port"`
	TLSType       string `json:"tls_type"`
	TransportType string `json:"transport_type"`
	UserCount     int    `json:"user_count"`
	CreatedAt     string `json:"created_at"`
}

// deriveTLSAndTransport extracts tls_type and transport_type from config_json.
func deriveTLSAndTransport(configJSON []byte) (tlsType, transportType string) {
	tlsType = "none"
	transportType = "tcp"
	if len(configJSON) == 0 {
		return
	}
	var cfg map[string]any
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return
	}
	if tls, ok := cfg["tls"].(map[string]any); ok && tls != nil {
		if r, ok := tls["reality"].(map[string]any); ok && r != nil {
			tlsType = "reality"
		} else if _, ok := tls["enabled"]; ok {
			tlsType = "tls"
		}
	}
	if tr, ok := cfg["transport"].(map[string]any); ok && tr != nil {
		if t, ok := tr["type"].(string); ok && t != "" {
			transportType = t
		}
	}
	return
}

// inboundFromDB converts db.Inbound to API inboundItem.
func inboundFromDB(ib *db.Inbound) inboundItem {
	tlsType, transportType := deriveTLSAndTransport(ib.ConfigJSON)
	return inboundItem{
		ID:            ib.ID,
		Tag:           ib.Tag,
		Protocol:      ib.Protocol,
		Listen:        ib.Listen,
		ListenPort:    ib.ListenPort,
		TLSType:       tlsType,
		TransportType: transportType,
		UserCount:     0,
		CreatedAt:     ib.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// ListInboundsHandler returns GET /api/inbounds handler.
func ListInboundsHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inbounds, err := db.ListInbounds()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items := make([]inboundItem, len(inbounds))
		for i := range inbounds {
			items[i] = inboundFromDB(&inbounds[i])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": items})
	}
}

// GetInboundHandler returns GET /api/inbounds/:id handler.
func GetInboundHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id64, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		id := uint(id64)
		ib, err := db.GetInboundByID(id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(inboundFromDB(ib))
	}
}
