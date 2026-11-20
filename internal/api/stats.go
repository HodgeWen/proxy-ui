package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
)

// HandleStatsRealtime GET /api/stats/realtime 实时速率概览
// （缓存最近一次采集样本，不直接触发内核请求）。
func (a *API) HandleStatsRealtime(c *gin.Context) {
	rates, at := a.stats.Rates()
	OK(c, gin.H{"up": rates.Up, "down": rates.Down, "at": at})
}

// trafficRow 按用户累计流量视图。
type trafficRow struct {
	Tag           string `json:"tag"`           // 用户名
	Subscriber    string `json:"subscriber"`    // 归属订阅用户备注（未绑定为空）
	InboundName   string `json:"inboundName"`   // 所属入站名
	Protocol      string `json:"protocol"`      // 协议
	UploadBytes   int64  `json:"uploadBytes"`   // 上行累计字节数
	DownloadBytes int64  `json:"downloadBytes"` // 下行累计字节数
}

// HandleStatsUserTraffic GET /api/stats/user-traffic 按用户累计流量（含未产生流量的用户）。
func (a *API) HandleStatsUserTraffic(c *gin.Context) {
	rows, err := a.trafficRows()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询累计流量失败: "+err.Error())
		return
	}
	OK(c, rows)
}

// trafficRows 组装按用户累计流量视图。
func (a *API) trafficRows() ([]trafficRow, error) {
	var users []db.InboundUser
	if err := a.db.Order("inbound_id, id").Find(&users).Error; err != nil {
		return nil, err
	}
	var inbounds []db.Inbound
	if err := a.db.Find(&inbounds).Error; err != nil {
		return nil, err
	}
	inMeta := make(map[uint]db.Inbound, len(inbounds))
	for _, in := range inbounds {
		inMeta[in.ID] = in
	}
	bindings, names, err := a.bindingMap()
	if err != nil {
		return nil, err
	}
	traffic, err := a.trafficByTag()
	if err != nil {
		return nil, err
	}
	rows := make([]trafficRow, 0, len(users))
	for _, u := range users {
		in := inMeta[u.InboundID]
		t := traffic[u.Name]
		row := trafficRow{
			Tag: u.Name, InboundName: in.Name, Protocol: in.Protocol,
			UploadBytes: t.UploadBytes, DownloadBytes: t.DownloadBytes,
		}
		if b, ok := bindings[u.ID]; ok {
			row.Subscriber = names[b.TokenID]
		}
		rows = append(rows, row)
	}
	return rows, nil
}
