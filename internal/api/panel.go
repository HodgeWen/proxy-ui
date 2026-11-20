package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// versionReq 面板更新的目标版本请求。
type versionReq struct {
	Version string `json:"version"` // 目标版本 tag；空表示最新稳定版
}

// HandlePanelVersions GET /api/panel/versions 面板当前版本与可用版本列表。
func (a *API) HandlePanelVersions(c *gin.Context) {
	versions, err := a.selfUpdates.ListVersions(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusBadGateway, err.Error())
		return
	}
	OK(c, gin.H{"currentVersion": a.selfUpdates.CurrentVersion(), "versions": versions})
}

// HandlePanelUpdate POST /api/panel/update 下载替换面板二进制并自动重启进程（内核随二进制一起更新）。
func (a *API) HandlePanelUpdate(c *gin.Context) {
	var req versionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	tag, err := a.selfUpdates.Apply(c.Request.Context(), req.Version)
	if err != nil {
		Fail(c, http.StatusBadGateway, err.Error())
		return
	}
	OK(c, gin.H{"version": tag})
	go a.restartPanel()
}

// restartPanel 等响应送达后停止内置内核并换入新二进制；
// 重启失败仅记录日志：二进制已替换，下次手动重启生效。
func (a *API) restartPanel() {
	time.Sleep(time.Second)
	if err := a.core.Stop(); err != nil {
		slog.Warn("面板更新重启前停止内核失败", "err", err)
	}
	if err := a.selfUpdates.Restart(); err != nil {
		slog.Error("面板重启失败，请手动重启（二进制已替换为新版本）", "err", err)
	}
}
