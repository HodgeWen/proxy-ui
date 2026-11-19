// Package server 装配 Gin 路由、会话中间件、统一守卫与 SPA 静态资源服务。
package server

import (
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/api"
	"proxy-ui/internal/singbox"
	"proxy-ui/internal/update"
)

// Options 服务装配参数。
type Options struct {
	DB      *gorm.DB // 数据库
	Dist    fs.FS    // 前端构建产物（web/dist 子目录）
	Version string   // 面板版本号
	DataDir string   // 数据目录（内核、配置等落盘位置）
}

// New 装配面板 HTTP 服务。
func New(opt Options) *gin.Engine {
	if gin.Mode() == gin.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}
	store := NewSessionStore()
	coreDir := filepath.Join(opt.DataDir, "core")
	core := singbox.New(coreDir)
	updates := update.New(opt.DB, coreDir)
	a := api.New(opt.DB, store, core, updates, opt.DataDir)

	engine := gin.New()
	engine.Use(gin.Recovery(), requestLog())
	_ = engine.SetTrustedProxies(nil)

	g := engine.Group("/api")
	g.Use(guard(a, store))
	g.GET("/health", func(c *gin.Context) {
		api.OK(c, gin.H{"status": "ok", "initialized": a.Initialized(), "version": opt.Version})
	})
	g.POST("/setup", a.HandleSetup)
	g.POST("/login", a.HandleLogin)
	g.POST("/logout", a.HandleLogout)
	g.GET("/me", a.HandleMe)
	g.PUT("/password", a.HandleChangePassword)
	g.GET("/kernel/status", a.HandleKernelStatus)
	g.GET("/kernel/logs", a.HandleKernelLogs)
	g.POST("/kernel/start", a.HandleKernelStart)
	g.POST("/kernel/stop", a.HandleKernelStop)
	g.POST("/kernel/restart", a.HandleKernelRestart)
	g.GET("/kernel/versions", a.HandleKernelVersions)
	g.POST("/kernel/install", a.HandleKernelInstall)
	g.GET("/kernel/mirror", a.HandleMirrorGet)
	g.PUT("/kernel/mirror", a.HandleMirrorSet)
	g.POST("/kernel/apply", a.HandleKernelApply)
	g.GET("/inbounds", a.HandleInboundsList)
	g.POST("/inbounds", a.HandleInboundCreate)
	g.GET("/inbounds/:id", a.HandleInboundGet)
	g.PUT("/inbounds/:id", a.HandleInboundUpdate)
	g.PATCH("/inbounds/:id/enabled", a.HandleInboundSetEnabled)
	g.DELETE("/inbounds/:id", a.HandleInboundDelete)
	g.GET("/outbounds", a.HandleOutboundsList)
	g.POST("/outbounds", a.HandleOutboundCreate)
	g.GET("/outbounds/:id", a.HandleOutboundGet)
	g.PUT("/outbounds/:id", a.HandleOutboundUpdate)
	g.PATCH("/outbounds/:id/enabled", a.HandleOutboundSetEnabled)
	g.DELETE("/outbounds/:id", a.HandleOutboundDelete)
	g.GET("/certs", a.HandleCertsList)
	g.POST("/certs", a.HandleCertCreate)
	g.PUT("/certs/:id", a.HandleCertUpdate)
	g.DELETE("/certs/:id", a.HandleCertDelete)
	g.GET("/rule-templates", a.HandleRuleTemplates)
	g.POST("/rule-templates/:id/apply", a.HandleRuleTemplateApply)
	g.GET("/rules", a.HandleRulesList)
	g.POST("/rules", a.HandleRuleCreate)
	g.PUT("/rules/order", a.HandleRulesOrder)
	g.GET("/rules/final", a.HandleRouteFinalGet)
	g.PUT("/rules/final", a.HandleRouteFinalSet)
	g.PUT("/rules/:id", a.HandleRuleUpdate)
	g.PATCH("/rules/:id/enabled", a.HandleRuleSetEnabled)
	g.DELETE("/rules/:id", a.HandleRuleDelete)
	g.GET("/policy-groups", a.HandleGroupsList)
	g.POST("/policy-groups", a.HandleGroupCreate)
	g.PUT("/policy-groups/:id", a.HandleGroupUpdate)
	g.PATCH("/policy-groups/:id/enabled", a.HandleGroupSetEnabled)
	g.DELETE("/policy-groups/:id", a.HandleGroupDelete)
	g.GET("/anti-ad", a.HandleAntiADGet)
	g.PUT("/anti-ad", a.HandleAntiADSet)

	engine.NoRoute(spa(opt.Dist))
	return engine
}

// guard /api 统一守卫：健康检查与 setup 放行（setup 内部拒绝重复初始化）；
// 其余接口未初始化或未登录一律 401。
func guard(a *api.API, store *SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.URL.Path {
		case "/api/health", "/api/setup":
			c.Next()
			return
		case "/api/login":
			if !a.Initialized() {
				api.Fail(c, http.StatusUnauthorized, "面板未初始化")
				c.Abort()
				return
			}
			c.Next()
			return
		}
		if token, err := c.Cookie(api.SessionCookie); err == nil && store.Valid(token) {
			c.Next()
			return
		}
		api.Fail(c, http.StatusUnauthorized, "未登录或会话已过期")
		c.Abort()
	}
}

// spa SPA 静态资源回退：命中嵌入文件直接返回，否则回退 index.html 由前端路由接管。
func spa(dist fs.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			api.Fail(c, http.StatusNotFound, "接口不存在")
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			api.Fail(c, http.StatusMethodNotAllowed, "方法不允许")
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+c.Request.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(dist, name); err != nil {
			name = "index.html"
		}
		// 不走 http.FileServer：其对 *\/index.html 会重定向到 "./" 造成回退循环
		http.ServeFileFS(c.Writer, c.Request, dist, name)
	}
}

// requestLog 以 slog 记录访问日志。
func requestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("http_request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}
