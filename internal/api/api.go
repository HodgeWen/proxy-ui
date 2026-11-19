// Package api 提供 REST handlers；路由统一由 internal/server 装配。
package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/db"
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

// API 面板 REST handlers，持有数据库与会话管理。
type API struct {
	db       *gorm.DB
	sessions SessionManager
}

// New 构造 API。
func New(database *gorm.DB, sessions SessionManager) *API {
	return &API{db: database, sessions: sessions}
}

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
