package api

import (
	"encoding/json"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/s-ui/s-ui/internal/db"
)

func StatsSummaryHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var inboundCount, userCount, activeUserCount int64
		db.DB.Model(&db.Inbound{}).Count(&inboundCount)
		db.DB.Model(&db.User{}).Count(&userCount)
		db.DB.Model(&db.User{}).Where("enabled = ?", true).Count(&activeUserCount)

		var uplinkResult, downlinkResult struct{ Total int64 }
		db.DB.Model(&db.Inbound{}).Select("COALESCE(SUM(traffic_uplink), 0) as total").Scan(&uplinkResult)
		db.DB.Model(&db.Inbound{}).Select("COALESCE(SUM(traffic_downlink), 0) as total").Scan(&downlinkResult)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"inbound_count":     inboundCount,
			"user_count":        userCount,
			"active_user_count": activeUserCount,
			"total_uplink":      uplinkResult.Total,
			"total_downlink":    downlinkResult.Total,
		})
	}
}
