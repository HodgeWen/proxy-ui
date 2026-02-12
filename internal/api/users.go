package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/s-ui/s-ui/internal/core"
	"github.com/s-ui/s-ui/internal/db"
)

// userItem is the API response shape for list/get.
type userItem struct {
	ID           uint     `json:"id"`
	Name         string   `json:"name"`
	Remark       string   `json:"remark"`
	UUID         string   `json:"uuid"`
	Password     string   `json:"password"`
	TrafficLimit int64    `json:"traffic_limit"`
	TrafficUsed  int64    `json:"traffic_used"`
	ExpireAt     *string  `json:"expire_at"` // ISO date or null
	Enabled      bool     `json:"enabled"`
	CreatedAt    string   `json:"created_at"`
	InboundIDs   []uint   `json:"inbound_ids"`
	InboundTags  []string `json:"inbound_tags"`
}

func userFromDB(u *db.User) userItem {
	item := userItem{
		ID:           u.ID,
		Name:         u.Name,
		Remark:       u.Remark,
		UUID:         u.UUID,
		Password:     u.Password,
		TrafficLimit: u.TrafficLimit,
		TrafficUsed:  u.TrafficUsed,
		Enabled:      u.Enabled,
		CreatedAt:    u.CreatedAt.Format(time.RFC3339),
		InboundIDs:   make([]uint, 0, len(u.Inbounds)),
		InboundTags:  make([]string, 0, len(u.Inbounds)),
	}
	if u.ExpireAt != nil {
		s := u.ExpireAt.Format(time.RFC3339)
		item.ExpireAt = &s
	}
	for _, ib := range u.Inbounds {
		item.InboundIDs = append(item.InboundIDs, ib.ID)
		item.InboundTags = append(item.InboundTags, ib.Tag)
	}
	return item
}

// ListUsersHandler returns GET /api/users handler.
func ListUsersHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyword := r.URL.Query().Get("q")
		users, err := db.ListUsers(keyword)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items := make([]userItem, len(users))
		for i := range users {
			items[i] = userFromDB(&users[i])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": items})
	}
}

// GetUserHandler returns GET /api/users/:id handler.
func GetUserHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id64, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		id := uint(id64)
		u, err := db.GetUserByID(id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(userFromDB(u))
	}
}

// userCreateRequest is the POST body for create.
type userCreateRequest struct {
	Name         string   `json:"name"`
	Remark       string   `json:"remark"`
	InboundIDs   []uint   `json:"inbound_ids"`
	TrafficLimit *int64   `json:"traffic_limit"`
	ExpireAt     *string  `json:"expire_at"` // ISO date string
}

// userUpdateRequest is the PUT body for update.
type userUpdateRequest struct {
	Name         string   `json:"name"`
	Remark       string   `json:"remark"`
	InboundIDs   []uint   `json:"inbound_ids"`
	TrafficLimit *int64   `json:"traffic_limit"`
	ExpireAt     *string  `json:"expire_at"`
}

func parseExpireAt(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateUserHandler handles POST /api/users.
func CreateUserHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req userCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		expireAt, err := parseExpireAt(req.ExpireAt)
		if err != nil {
			http.Error(w, "invalid expire_at format", http.StatusBadRequest)
			return
		}
		var trafficLimit int64
		if req.TrafficLimit != nil {
			trafficLimit = *req.TrafficLimit
		}
		u := &db.User{
			Name:         req.Name,
			Remark:       req.Remark,
			TrafficLimit: trafficLimit,
			ExpireAt:     expireAt,
		}
		if len(req.InboundIDs) > 0 {
			inbounds, err := db.GetInboundsByIDs(req.InboundIDs)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			u.Inbounds = inbounds
		}
		if err := db.CreateUser(u); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := configPath()
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			db.DeleteUser(u.ID)
			http.Error(w, "failed to create config dir", http.StatusInternalServerError)
			return
		}
		gen := &core.ConfigGenerator{}
		cfg, err := gen.Generate()
		if err != nil {
			db.DeleteUser(u.ID)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := core.ApplyConfig(path, cfg); err != nil {
			db.DeleteUser(u.ID)
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
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(userFromDB(u))
	}
}

// UpdateUserHandler handles PUT /api/users/:id.
func UpdateUserHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id64, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		id := uint(id64)
		old, err := db.GetUserByID(id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req userUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		expireAt, err := parseExpireAt(req.ExpireAt)
		if err != nil {
			http.Error(w, "invalid expire_at format", http.StatusBadRequest)
			return
		}
		u := &db.User{
			ID:           id,
			Name:         req.Name,
			Remark:       req.Remark,
			UUID:         old.UUID,
			Password:     old.Password,
			TrafficLimit: old.TrafficLimit,
			TrafficUsed:  old.TrafficUsed,
			ExpireAt:     expireAt,
			Enabled:      old.Enabled,
		}
		if req.TrafficLimit != nil {
			u.TrafficLimit = *req.TrafficLimit
		}
		if err := db.ReplaceUserInbounds(id, req.InboundIDs); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := db.UpdateUser(u); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := configPath()
		gen := &core.ConfigGenerator{}
		cfg, err := gen.Generate()
		if err != nil {
			db.UpdateUser(old)
			db.ReplaceUserInbounds(id, inboundIDsFromUsers(old.Inbounds))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := core.ApplyConfig(path, cfg); err != nil {
			db.UpdateUser(old)
			db.ReplaceUserInbounds(id, inboundIDsFromUsers(old.Inbounds))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		pm := core.NewProcessManager()
		if err := pm.Restart(path); err != nil {
			// Config applied; restart failure is best-effort
		}
		u, _ = db.GetUserByID(id)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(userFromDB(u))
	}
}

func inboundIDsFromUsers(inbounds []db.Inbound) []uint {
	ids := make([]uint, len(inbounds))
	for i := range inbounds {
		ids[i] = inbounds[i].ID
	}
	return ids
}

// DeleteUserHandler handles DELETE /api/users/:id.
func DeleteUserHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id64, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		id := uint(id64)
		u, err := db.GetUserByID(id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := db.DeleteUser(id); err != nil {
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
			u.ID = 0
			db.CreateUser(u)
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

// batchRequest is the POST body for batch operations.
type batchRequest struct {
	Action string `json:"action"`
	IDs    []uint `json:"ids"`
}

// BatchUsersHandler handles POST /api/users/batch.
func BatchUsersHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req batchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Action == "" {
			http.Error(w, "action required", http.StatusBadRequest)
			return
		}
		if len(req.IDs) == 0 {
			http.Error(w, "ids must be non-empty", http.StatusBadRequest)
			return
		}
		switch req.Action {
		case "delete", "enable", "disable", "reset_traffic":
			// valid
		default:
			http.Error(w, "invalid action", http.StatusBadRequest)
			return
		}

		// Store state for rollback; apply mutations
		type rollbackFn func()
		var rollbacks []rollbackFn

		if req.Action == "delete" {
			// Store users before deletion
			var toRestore []*db.User
			for _, id := range req.IDs {
				u, err := db.GetUserByID(id)
				if err != nil {
					continue
				}
				toRestore = append(toRestore, u)
			}
			for _, id := range req.IDs {
				if err := db.DeleteUser(id); err != nil {
					for _, u := range toRestore {
						u.ID = 0
						db.CreateUser(u)
					}
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			rollbacks = append(rollbacks, func() {
				for _, u := range toRestore {
					u.ID = 0
					db.CreateUser(u)
				}
			})
		} else {
			// enable, disable, reset_traffic
			var snapshots []*db.User
			for _, id := range req.IDs {
				u, err := db.GetUserByID(id)
				if err != nil {
					continue
				}
				snapshots = append(snapshots, u)
				updated := *u
				switch req.Action {
				case "enable":
					updated.Enabled = true
				case "disable":
					updated.Enabled = false
				case "reset_traffic":
					updated.TrafficUsed = 0
				}
				if err := db.UpdateUser(&updated); err != nil {
					for _, s := range snapshots {
						db.UpdateUser(s)
					}
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			snapshotsCopy := make([]*db.User, len(snapshots))
			copy(snapshotsCopy, snapshots)
			rollbacks = []rollbackFn{func() {
				for _, s := range snapshotsCopy {
					db.UpdateUser(s)
				}
			}}
		}

		path := configPath()
		gen := &core.ConfigGenerator{}
		cfg, err := gen.Generate()
		if err != nil {
			for _, rb := range rollbacks {
				rb()
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := core.ApplyConfig(path, cfg); err != nil {
			for _, rb := range rollbacks {
				rb()
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		pm := core.NewProcessManager()
		if err := pm.Restart(path); err != nil {
			// Config applied; restart failure is best-effort
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}
}
