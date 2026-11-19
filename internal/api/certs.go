package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/s-ui/s-ui/internal/config"
	"github.com/s-ui/s-ui/internal/core"
	"github.com/s-ui/s-ui/internal/db"
)

// certItem is the API response shape for list/get.
type certItem struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	FullchainPath string `json:"fullchain_path"`
	PrivkeyPath   string `json:"privkey_path"`
	CreatedAt     string `json:"created_at"`
}

func certFromDB(c *db.Certificate) certItem {
	return certItem{
		ID:            c.ID,
		Name:          c.Name,
		FullchainPath: c.FullchainPath,
		PrivkeyPath:   c.PrivkeyPath,
		CreatedAt:     c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// ListCertificatesHandler returns GET /api/certs handler.
func ListCertificatesHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		certs, err := db.ListCertificates()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items := make([]certItem, len(certs))
		for i := range certs {
			items[i] = certFromDB(&certs[i])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": items})
	}
}

// GetCertificateHandler returns GET /api/certs/:id handler.
func GetCertificateHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id64, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		id := uint(id64)
		c, err := db.GetCertificateByID(id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(certFromDB(c))
	}
}

// certCreateRequest is the POST body for create.
type certCreateRequest struct {
	Name          string `json:"name"`
	FullchainPath string `json:"fullchain_path"`
	PrivkeyPath   string `json:"privkey_path"`
}

// certUpdateRequest is the PUT body for update.
type certUpdateRequest struct {
	Name          string `json:"name"`
	FullchainPath string `json:"fullchain_path"`
	PrivkeyPath   string `json:"privkey_path"`
}

// CreateCertificateHandler handles POST /api/certs.
func CreateCertificateHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req certCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.FullchainPath) == "" || strings.TrimSpace(req.PrivkeyPath) == "" {
			http.Error(w, "fullchain_path and privkey_path required", http.StatusBadRequest)
			return
		}
		c := &db.Certificate{
			Name:          strings.TrimSpace(req.Name),
			FullchainPath: strings.TrimSpace(req.FullchainPath),
			PrivkeyPath:   strings.TrimSpace(req.PrivkeyPath),
		}
		if err := db.CreateCertificate(c); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(certFromDB(c))
	}
}

// UpdateCertificateHandler handles PUT /api/certs/:id.
func UpdateCertificateHandler(sm *scs.SessionManager, panelCfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id64, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		id := uint(id64)
		old, err := db.GetCertificateByID(id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req certUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.FullchainPath) == "" || strings.TrimSpace(req.PrivkeyPath) == "" {
			http.Error(w, "fullchain_path and privkey_path required", http.StatusBadRequest)
			return
		}
		c := &db.Certificate{
			ID:            id,
			Name:          strings.TrimSpace(req.Name),
			FullchainPath: strings.TrimSpace(req.FullchainPath),
			PrivkeyPath:   strings.TrimSpace(req.PrivkeyPath),
		}
		if err := db.UpdateCertificate(c); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := configPath(panelCfg)
		gen := &core.ConfigGenerator{}
		cfg, err := gen.Generate()
		if err != nil {
			db.UpdateCertificate(old)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pm := core.NewProcessManagerFromConfig(panelCfg)
		if err := core.ApplyConfig(path, cfg, pm); err != nil {
			db.UpdateCertificate(old)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if err := pm.Restart(path); err != nil {
			// Config applied; restart failure is best-effort
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(certFromDB(c))
	}
}

// DeleteCertificateHandler handles DELETE /api/certs/:id.
func DeleteCertificateHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id64, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		id := uint(id64)
		_, err = db.GetCertificateByID(id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		tags, err := db.InboundsReferencingCert(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(tags) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			msg := "证书正在被以下入站使用: " + strings.Join(tags, ", ")
			json.NewEncoder(w).Encode(map[string]string{"error": msg})
			return
		}
		if err := db.DeleteCertificate(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
