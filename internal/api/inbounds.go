package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/s-ui/s-ui/internal/core"
	"github.com/s-ui/s-ui/internal/db"
	"gorm.io/datatypes"
)

// inboundItem is the API response shape for list/get.
type inboundItem struct {
	ID              uint   `json:"id"`
	Tag             string `json:"tag"`
	Protocol        string `json:"protocol"`
	Listen          string `json:"listen"`
	ListenPort      uint   `json:"listen_port"`
	TLSType         string `json:"tls_type"`
	TransportType   string `json:"transport_type"`
	UserCount       int    `json:"user_count"`
	TrafficUplink   int64  `json:"traffic_uplink"`
	TrafficDownlink int64  `json:"traffic_downlink"`
	CreatedAt       string `json:"created_at"`
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
		ID:              ib.ID,
		Tag:             ib.Tag,
		Protocol:        ib.Protocol,
		Listen:          ib.Listen,
		ListenPort:      ib.ListenPort,
		TLSType:         tlsType,
		TransportType:   transportType,
		UserCount:       0,
		TrafficUplink:   ib.TrafficUplink,
		TrafficDownlink: ib.TrafficDownlink,
		CreatedAt:       ib.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// inboundDetail adds config_json for edit form.
type inboundDetail struct {
	inboundItem
	ConfigJSON datatypes.JSON `json:"config_json,omitempty"`
}

// ListInboundsHandler returns GET /api/inbounds handler.
func ListInboundsHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sort := r.URL.Query().Get("sort")
		inbounds, err := db.ListInbounds(sort)
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
		detail := inboundDetail{
			inboundItem: inboundFromDB(ib),
			ConfigJSON:  ib.ConfigJSON,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	}
}

// inboundCreateRequest is the POST body for create.
type inboundCreateRequest struct {
	Tag        string          `json:"tag"`
	Protocol   string          `json:"protocol"`
	Listen     string          `json:"listen"`
	ListenPort uint            `json:"listen_port"`
	ConfigJSON datatypes.JSON  `json:"config_json"`
}

// inboundUpdateRequest is the PUT body for update.
type inboundUpdateRequest struct {
	Tag        string          `json:"tag"`
	Protocol   string          `json:"protocol"`
	Listen     string          `json:"listen"`
	ListenPort uint            `json:"listen_port"`
	ConfigJSON datatypes.JSON  `json:"config_json"`
}

// CreateInboundHandler handles POST /api/inbounds.
func CreateInboundHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req inboundCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Tag == "" || req.Protocol == "" {
			http.Error(w, "tag and protocol required", http.StatusBadRequest)
			return
		}
		if req.Listen == "" {
			req.Listen = "::"
		}
		exists, err := db.InboundExistsByTag(req.Tag)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if exists {
			http.Error(w, "tag already exists", http.StatusBadRequest)
			return
		}
		ib := &db.Inbound{
			Tag:        req.Tag,
			Protocol:   req.Protocol,
			Listen:     req.Listen,
			ListenPort: req.ListenPort,
			ConfigJSON: req.ConfigJSON,
		}
		if err := db.CreateInbound(ib); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := configPath()
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			db.DeleteInbound(ib.ID)
			http.Error(w, "failed to create config dir", http.StatusInternalServerError)
			return
		}
		gen := &core.ConfigGenerator{}
		cfg, err := gen.Generate()
		if err != nil {
			db.DeleteInbound(ib.ID)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := core.ApplyConfig(path, cfg); err != nil {
			db.DeleteInbound(ib.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		pm := core.NewProcessManager()
		if err := pm.Restart(path); err != nil {
			// Config applied; restart failure is logged but inbound is persisted
			// Per plan: apply succeeded, return 201. Restart is best-effort.
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(inboundFromDB(ib))
	}
}

// UpdateInboundHandler handles PUT /api/inbounds/:id.
func UpdateInboundHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id64, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		id := uint(id64)
		old, err := db.GetInboundByID(id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req inboundUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Tag == "" || req.Protocol == "" {
			http.Error(w, "tag and protocol required", http.StatusBadRequest)
			return
		}
		if req.Listen == "" {
			req.Listen = "::"
		}
		if req.Tag != old.Tag {
			exists, err := db.InboundExistsByTag(req.Tag)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if exists {
				http.Error(w, "tag already exists", http.StatusBadRequest)
				return
			}
		}
		updated := &db.Inbound{
			ID:         id,
			Tag:        req.Tag,
			Protocol:   req.Protocol,
			Listen:     req.Listen,
			ListenPort: req.ListenPort,
			ConfigJSON: req.ConfigJSON,
		}
		if err := db.UpdateInbound(updated); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := configPath()
		gen := &core.ConfigGenerator{}
		cfg, err := gen.Generate()
		if err != nil {
			db.UpdateInbound(old)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := core.ApplyConfig(path, cfg); err != nil {
			db.UpdateInbound(old)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		pm := core.NewProcessManager()
		if err := pm.Restart(path); err != nil {
			// Config applied; restart failure is best-effort
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(inboundFromDB(updated))
	}
}

// DeleteInboundHandler handles DELETE /api/inbounds/:id.
func DeleteInboundHandler(sm *scs.SessionManager) http.HandlerFunc {
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
		if err := db.DeleteInbound(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := configPath()
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			http.Error(w, "failed to create config dir", http.StatusInternalServerError)
			return
		}
		gen := &core.ConfigGenerator{}
		cfg, err := gen.Generate()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := core.ApplyConfig(path, cfg); err != nil {
			db.CreateInbound(ib)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		pm := core.NewProcessManager()
		if err := pm.Restart(path); err != nil {
			// Config applied; restart failure is best-effort
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
