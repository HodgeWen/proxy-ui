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

// HandleKernelStatus GET /api/kernel/status 内核运行状态（只读）。
func (a *API) HandleKernelStatus(c *gin.Context) {
	OK(c, a.kernelStatusOf(c.Request.Context()))
}

// HandleKernelLogs GET /api/kernel/logs?limit=200 内核日志（只读）。
func (a *API) HandleKernelLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	OK(c, a.core.Logs(limit))
}

// StartKernel 面板启动即拉起内核（无可用入站时以最小配置运行）。
// 失败只告警：面板保持可用，用户修正配置后任意一次保存都会重新热应用。
func (a *API) StartKernel(ctx context.Context) {
	if err := a.applyKernelConfig(ctx); err != nil {
		slog.Warn("启动时自动拉起内核失败，面板仍可用；修正配置后再保存任一配置即可重试", "err", err)
		return
	}
	slog.Info("内核已随面板启动", "version", singbox.EmbeddedVersion)
}

// applyKernelConfig 聚合当前配置并热应用：校验通过后重建内核实例。
//
// 所有写操作落库成功后都调用它，因此「保存即生效」；重建失败时 Manager 会自动回滚到
// 上一份可用配置（代理不中断），失败原因经返回值带回本次请求。调用方的数据库改动一律
// 保留，用户修正配置后再保存即可重试。
//
// 整个过程串行化：并发保存若各自聚合再各自重建，后完成的一方可能把先落库的改动覆盖掉。
func (a *API) applyKernelConfig(ctx context.Context) error {
	a.applyMu.Lock()
	defer a.applyMu.Unlock()
	cfg, err := a.buildKernelConfig()
	if err != nil {
		return err
	}
	if err := a.core.CheckConfig(cfg); err != nil {
		return err
	}
	// 统计服务监听地址由 Manager 在应用时按实例端口写入
	return a.core.Apply(cfg)
}

// applyOrFail 保存成功后热应用内核；失败时写出响应并返回 false（数据已保存，带上失败原因）。
func (a *API) applyOrFail(c *gin.Context) bool {
	if err := a.applyKernelConfig(c.Request.Context()); err != nil {
		Fail(c, http.StatusBadRequest, "已保存，但内核按新配置重建失败: "+err.Error())
		return false
	}
	return true
}

// QuotaLoop 周期核对配额与到期：用户生效状态有变化时热应用，
// 否则超限用户会一直可用到下次管理员操作。
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
			if err := a.applyKernelConfig(ctx); err != nil {
				slog.Warn("配额核对后热应用内核配置失败", "err", err)
			}
		}
	}
}
