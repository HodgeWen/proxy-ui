package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/ai"
	"proxy-ui/internal/db"
)

// realtimeStatsResp 实时速率视图。
type realtimeStatsResp struct {
	Up   int64     `json:"up"`   // 上行速率（字节/秒）
	Down int64     `json:"down"` // 下行速率（字节/秒）
	At   time.Time `json:"at"`   // 采样时间
}

// HandleStatsRealtime GET /api/stats/realtime 实时速率概览
// （缓存最近一次采集样本，不直接触发内核请求）。
func (a *API) HandleStatsRealtime(c *gin.Context) {
	rates, at := a.stats.Rates()
	OK(c, realtimeStatsResp{Up: rates.Up, Down: rates.Down, At: at})
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

// ---------- 统计 AI 工具 ----------

// registerStatsAITools 注册内核状态与流量统计相关的查询 AI 工具。
func (a *API) registerStatsAITools() {
	a.ai.Register(ai.Tool{
		Name:        "get_kernel_status",
		Description: "查询 sing-box 内核运行状态（内置内核、是否运行、版本、运行时长）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.kernelStatusOf(), nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_kernel_logs",
		Description: "查询内核最近日志",
		Parameters: toolParams(map[string]any{
			"limit": map[string]any{"type": "integer", "description": "返回的最近行数，缺省 100"},
		}),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg struct {
				Limit int `json:"limit"`
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if arg.Limit <= 0 {
				arg.Limit = 100
			}
			return a.core.Logs(arg.Limit), nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_realtime_stats",
		Description: "查询实时统计：当前上传/下载速率、内存占用、连接总数与累计流量",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			rates, at := a.stats.Rates()
			return realtimeStatsResp{Up: rates.Up, Down: rates.Down, At: at}, nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_user_traffic",
		Description: "查询按用户名/入站持久化累计的上下行流量（面板重启不丢失）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.trafficByTag()
		},
	})
}
