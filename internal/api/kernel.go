package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/singbox"
)

// kernelStatusOf 组合内核状态与内置版本。
func (a *API) kernelStatusOf(ctx context.Context) singbox.Status {
	st := a.core.Status()
	st.Version = a.core.Version(ctx)
	return st
}

// HandleKernelStatus GET /api/kernel/status 内核运行状态。
func (a *API) HandleKernelStatus(c *gin.Context) {
	OK(c, a.kernelStatusOf(c.Request.Context()))
}

// HandleKernelLogs GET /api/kernel/logs?limit=200 内核日志。
func (a *API) HandleKernelLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	OK(c, a.core.Logs(limit))
}

// HandleKernelStart POST /api/kernel/start 启动内核（用已聚合的配置，无则现聚合）。
func (a *API) HandleKernelStart(c *gin.Context) {
	cfg := a.currentKernelConfig()
	if err := a.core.Start(cfg); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, a.kernelStatusOf(c.Request.Context()))
}

// HandleKernelStop POST /api/kernel/stop 停止内核。
func (a *API) HandleKernelStop(c *gin.Context) {
	if err := a.core.Stop(); err != nil {
		Fail(c, http.StatusInternalServerError, "停止内核失败: "+err.Error())
		return
	}
	OK(c, a.kernelStatusOf(c.Request.Context()))
}

// HandleKernelRestart POST /api/kernel/restart 重启内核（按最新配置重建）。
func (a *API) HandleKernelRestart(c *gin.Context) {
	cfg := a.currentKernelConfig()
	if err := a.core.Restart(cfg); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, a.kernelStatusOf(c.Request.Context()))
}

// HandleKernelVersions GET /api/kernel/versions 内置内核版本信息（不再有可选的下载版本）。
func (a *API) HandleKernelVersions(c *gin.Context) {
	OK(c, gin.H{"version": singbox.EmbeddedVersion, "embedded": true})
}

// currentKernelConfig 取待启动配置：优先使用上次应用时缓存的配置，否则现聚合一份。
func (a *API) currentKernelConfig() singbox.Config {
	a.configMu.Lock()
	cached := a.pendingConfig
	a.configMu.Unlock()
	if cached.Inbounds != nil || cached.Outbounds != nil {
		return cached
	}
	cfg, err := a.buildKernelConfig()
	if err != nil {
		return singbox.MinConfig()
	}
	return cfg
}

// QuotaLoop 周期核对配额与到期（否则只有管理员操作时才暂停超限用户，
// 客户端会一直可用到下次手动「应用配置」）。有变化则重建内核。
func (a *API) QuotaLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			changed, err := a.ReconcileQuota()
			if err != nil || !changed {
				continue
			}
			if _, _, err := a.applyKernel(ctx); err != nil {
				slog.Warn("配额核对后应用配置失败", "err", err)
			}
		}
	}
}
