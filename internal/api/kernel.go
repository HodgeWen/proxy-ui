package api

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/singbox"
)

// kernelStatusView 内核状态视图：singbox.Status 附加热应用失败标记（内嵌结构体在 JSON 中扁平展开）。
type kernelStatusView struct {
	singbox.Status
	// 有已保存配置未生效：上次热应用失败，内核在回滚配置上运行，重新保存相关配置可重试
	PendingChanges bool `json:"pendingChanges"`
}

// kernelStatusOf 组合内核状态、内置版本与热应用失败标记。
func (a *API) kernelStatusOf(ctx context.Context) kernelStatusView {
	st := a.core.Status()
	st.Version = a.core.Version(ctx)
	return kernelStatusView{Status: st, PendingChanges: a.pendingKernel.Load()}
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
	// 用户↔订阅归属只在写路径变化，而所有写 handler 落库后都收口到这里：无论接下来热应用
	// 成败，库内归属已变，在此失效统计采集的归属缓存即覆盖全部写路径，且天然与写路径串行
	if a.stats != nil {
		a.stats.InvalidateOwners()
	}
	err := a.applyKernelConfigLocked()
	// 失败置位「有配置未生效」、成功清除：内核在回滚配置上运行时仪表盘常驻黄条提示，
	// 否则数据已保存但内核跑旧配置的状态毫无指示。读写都在 applyMu 临界区内
	a.pendingKernel.Store(err != nil)
	return err
}

// applyKernelConfigLocked 聚合当前配置并热应用（调用方持 applyMu）。
func (a *API) applyKernelConfigLocked() error {
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

// applyThenOK 落库成功后的统一收尾：热应用内核并写出成功响应。
// 热应用失败时数据已保存，仍写成功响应（msg 携带失败原因供前端提示），
// 内核由 Manager 回滚到上一份可用配置，用户修正配置后再保存即可重试。
func (a *API) applyThenOK(c *gin.Context, data any) {
	if err := a.applyKernelConfig(c.Request.Context()); err != nil {
		OKWarn(c, data, "已保存，但内核按新配置重建失败: "+err.Error())
		return
	}
	OK(c, data)
}

// QuotaLoop 周期核对配额与到期：用户生效状态有变化时热应用，
// 否则超限用户会一直可用到下次管理员操作。
// 循环保留作出期兜底；即时核对回调也在此装配——配额核对逻辑归本包，
// singbox 反向 import api 会成环，故以方法值注入统计采集（落库成功后只对本轮有流量的 token 触发）。
func (a *API) QuotaLoop(ctx context.Context, interval time.Duration) {
	if a.stats != nil {
		a.stats.SetQuotaReconciler(a.ReconcileQuotaTokens)
	}
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
