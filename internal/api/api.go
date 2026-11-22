// Package api 提供 REST handlers；路由统一由 internal/server 装配。
package api

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/ai"
	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
	"proxy-ui/internal/subscription"
	"proxy-ui/internal/update"
)

// SessionCookie 会话 Cookie 名。
const SessionCookie = "proxy_ui_session"

// SessionTTL 会话有效期。
const SessionTTL = 7 * 24 * time.Hour

// SessionManager 会话管理抽象，由 server 包提供实现。
type SessionManager interface {
	Create() string      // 创建会话，返回随机令牌
	Remove(token string) // 注销会话
	Clear()              // 清空全部会话（改密后吊销所有已登录端）
}

// API 面板 REST handlers，持有数据库、会话管理与内核相关模块。
type API struct {
	db            *gorm.DB
	sessions      SessionManager
	core          *singbox.Manager      // 内置内核实例管理
	selfUpdates   *update.SelfUpdater   // 面板自身版本查询、替换与重启
	stats         *singbox.StatsMonitor // 内核统计采集（实时速率与按用户累计流量）
	ai            *ai.Service           // AI 助手（供应商配置、对话代理、工具注册表）
	subs          *subscription.Service // 订阅组装与渲染（面板内预览；匿名端点由 server 单独装配）
	dataDir       string                // 数据目录（证书存储等落盘位置）
	applyMu       sync.Mutex            // 串行化「聚合配置 + 重建内核 + 停机」，保证并发写操作与关闭顺序一致
	stopped       bool                  // 停机标记（applyMu 保护）：为 true 后拒绝任何后续内核启动或重建
	pendingKernel atomic.Bool           // 热应用失败标记：true＝有已保存配置未生效，内核跑在回滚配置上（applyMu 临界区内写，状态接口原子读）
	limiter       *failLimiter          // 登录与首次设置共用的按 IP 失败限流（内存）
	setupMu       sync.Mutex            // 保护 setupCode 与 setupClaimed
	setupCode     string                // 一次性设置码（空表示已消费或面板已初始化）
	setupClaimed  bool                  // 首次建管理员的占位标记，串行化并发 setup
}

// New 构造 API；dataDir 为面板数据目录。
func New(database *gorm.DB, sessions SessionManager, core *singbox.Manager, selfUpdates *update.SelfUpdater, stats *singbox.StatsMonitor, dataDir string) *API {
	a := &API{db: database, sessions: sessions, core: core, selfUpdates: selfUpdates, stats: stats, dataDir: dataDir, subs: subscription.New(database, dataDir)}
	a.limiter = newFailLimiter()
	a.initSetupCode()
	a.ai = ai.New(database)
	a.registerAITools()
	return a
}

// initSetupCode 面板未初始化时生成一次性设置码并打印到启动日志（公网抢注防护）；
// 已初始化则直接占位，使后续 setup 一律按已初始化拒绝。
func (a *API) initSetupCode() {
	if a.Initialized() {
		a.setupClaimed = true
		return
	}
	a.setupCode = newSetupCode()
	slog.Info("面板未初始化，已生成一次性设置码；请在服务器控制台查看启动日志获取，完成首次设置后失效", "setup_code", a.setupCode)
}

// certDir 面板存储证书目录。
func (a *API) certDir() string { return filepath.Join(a.dataDir, "certs") }

// Initialized 面板是否已完成初始化（存在管理员账号）。
func (a *API) Initialized() bool {
	var count int64
	a.db.Model(&db.Admin{}).Count(&count)
	return count > 0
}

// cookieSecure 按请求判断 Cookie 是否限定 HTTPS：直连 TLS 或反代声明 https 均视为安全信道。
func cookieSecure(c *gin.Context) bool {
	return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
}

// setSession 创建会话并写入 Cookie（SameSite=Lax；HTTPS 信道下附加 Secure）。
func (a *API) setSession(c *gin.Context) {
	token := a.sessions.Create()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookie, token, int(SessionTTL.Seconds()), "/", "", cookieSecure(c), true)
}

// clearSessionCookie 注销会话 Cookie，属性与写入时一致以保证同信道下可正确覆盖。
func clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookie, "", -1, "/", "", cookieSecure(c), true)
}
