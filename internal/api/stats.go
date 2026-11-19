package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
)

// HandleStatsRealtime GET /api/stats/realtime 实时速率、内存与连接概览
// （缓存最近一秒采集样本，不直接触发 Clash API 请求）。
func (a *API) HandleStatsRealtime(c *gin.Context) {
	rates, at := a.stats.Rates()
	snap := a.stats.Snapshot()
	OK(c, gin.H{
		"up":                rates.Up,
		"down":              rates.Down,
		"memory":            snap.Memory,
		"uploadTotal":       snap.UploadTotal,
		"downloadTotal":     snap.DownloadTotal,
		"activeConnections": len(snap.Connections),
		"at":                at,
	})
}

// HandleStatsConnections GET /api/stats/connections 当前连接列表。
func (a *API) HandleStatsConnections(c *gin.Context) {
	OK(c, a.stats.Snapshot())
}

// inboundTrafficView 按入站累计流量视图。
type inboundTrafficView struct {
	InboundID     uint   `json:"inboundId"`     // 入站 ID
	Name          string `json:"name"`          // 入站名称
	Protocol      string `json:"protocol"`      // 协议
	Enabled       bool   `json:"enabled"`       // 是否启用
	UploadBytes   int64  `json:"uploadBytes"`   // 上行累计字节数
	DownloadBytes int64  `json:"downloadBytes"` // 下行累计字节数
}

// HandleStatsInboundTraffic GET /api/stats/inbound-traffic 按入站累计流量（含未产生流量的入站）。
func (a *API) HandleStatsInboundTraffic(c *gin.Context) {
	views, err := a.inboundTrafficViews()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询累计流量失败: "+err.Error())
		return
	}
	OK(c, views)
}

// inboundTrafficViews 查询按入站累计流量视图（含未产生流量的入站）。
func (a *API) inboundTrafficViews() ([]inboundTrafficView, error) {
	var inbounds []db.Inbound
	if err := a.db.Order("id").Find(&inbounds).Error; err != nil {
		return nil, err
	}
	var traffics []db.InboundTraffic
	if err := a.db.Find(&traffics).Error; err != nil {
		return nil, err
	}
	byInbound := make(map[uint]db.InboundTraffic, len(traffics))
	for _, t := range traffics {
		byInbound[t.InboundID] = t
	}
	views := make([]inboundTrafficView, 0, len(inbounds))
	for _, in := range inbounds {
		t := byInbound[in.ID]
		views = append(views, inboundTrafficView{
			InboundID:     in.ID,
			Name:          in.Name,
			Protocol:      in.Protocol,
			Enabled:       in.Enabled,
			UploadBytes:   t.UploadBytes,
			DownloadBytes: t.DownloadBytes,
		})
	}
	return views, nil
}
