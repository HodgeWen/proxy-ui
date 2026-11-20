// Package api 提供 REST handlers；路由统一由 internal/server 装配。
package api

import (
	"path/filepath"
	"sync"
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
}

// API 面板 REST handlers，持有数据库、会话管理与内核相关模块。
type API struct {
	db          *gorm.DB
	sessions    SessionManager
	core        *singbox.Manager      // 内置内核实例管理
	selfUpdates *update.SelfUpdater   // 面板自身版本查询、替换与重启
	stats       *singbox.StatsMonitor // 内核统计采集（实时速率与按用户累计流量）
	ai          *ai.Service           // AI 助手（供应商配置、对话代理、工具注册表）
	subs        *subscription.Service // 订阅组装与渲染（面板内预览；匿名端点由 server 单独装配）
	dataDir     string                // 数据目录（证书存储等落盘位置）
	applyMu     sync.Mutex            // 串行化「聚合配置 + 重建内核」，保证并发写操作按落库顺序生效
}

// New 构造 API；dataDir 为面板数据目录。
func New(database *gorm.DB, sessions SessionManager, core *singbox.Manager, selfUpdates *update.SelfUpdater, stats *singbox.StatsMonitor, dataDir string) *API {
	a := &API{db: database, sessions: sessions, core: core, selfUpdates: selfUpdates, stats: stats, dataDir: dataDir, subs: subscription.New(database)}
	a.ai = ai.New(database)
	a.registerAITools()
	return a
}

// certDir 面板存储证书目录。
func (a *API) certDir() string { return filepath.Join(a.dataDir, "certs") }

// Initialized 面板是否已完成初始化（存在管理员账号）。
func (a *API) Initialized() bool {
	var count int64
	a.db.Model(&db.Admin{}).Count(&count)
	return count > 0
}

// setSession 创建会话并写入 Cookie。
func (a *API) setSession(c *gin.Context) {
	token := a.sessions.Create()
	c.SetCookie(SessionCookie, token, int(SessionTTL.Seconds()), "/", "", false, true)
}
