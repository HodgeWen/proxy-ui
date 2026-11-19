package api

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
)

// tokenView 订阅令牌响应视图。
type tokenView struct {
	ID         uint       `json:"id"`         // 主键
	Token      string     `json:"token"`      // 匿名令牌
	Name       string     `json:"name"`       // 备注
	Revoked    bool       `json:"revoked"`    // 是否已吊销
	CreatedAt  time.Time  `json:"createdAt"`  // 创建时间
	LastUsedAt *time.Time `json:"lastUsedAt"` // 最近一次使用时间
}

// tokenNameReq 创建令牌请求。
type tokenNameReq struct {
	Name string `json:"name"` // 备注
}

// serverReq 订阅节点服务器地址设置请求；空表示按请求 Host 推断。
type serverReq struct {
	Server string `json:"server"` // 服务器地址（域名或 IP）
}

// HandleSubscriptionGet GET /api/subscription 订阅设置与令牌列表。
func (a *API) HandleSubscriptionGet(c *gin.Context) {
	var records []db.SubscriptionToken
	if err := a.db.Order("id").Find(&records).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "查询订阅令牌失败: "+err.Error())
		return
	}
	views := make([]tokenView, len(records))
	for i, r := range records {
		views[i] = tokenView(r)
	}
	OK(c, gin.H{"server": a.getSetting(db.SettingSubServer), "tokens": views})
}

// HandleSubscriptionServerPut PUT /api/subscription/server 保存订阅节点服务器地址。
func (a *API) HandleSubscriptionServerPut(c *gin.Context) {
	var req serverReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	server := strings.TrimSpace(req.Server)
	if err := a.setSetting(db.SettingSubServer, server); err != nil {
		Fail(c, http.StatusInternalServerError, "保存服务器地址失败: "+err.Error())
		return
	}
	OK(c, gin.H{"server": server})
}

// HandleTokenCreate POST /api/subscription/tokens 生成订阅令牌。
func (a *API) HandleTokenCreate(c *gin.Context) {
	var req tokenNameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	record := db.SubscriptionToken{Token: randToken(), Name: strings.TrimSpace(req.Name)}
	if err := a.db.Create(&record).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "创建订阅令牌失败: "+err.Error())
		return
	}
	OK(c, tokenView(record))
}

// HandleTokenRevoke PATCH /api/subscription/tokens/:id/revoked 吊销/恢复订阅令牌。
func (a *API) HandleTokenRevoke(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req enabledReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var record db.SubscriptionToken
	if err := a.db.First(&record, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "订阅令牌不存在")
		return
	}
	if err := a.db.Model(&record).Update("revoked", req.Enabled).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "更新订阅令牌失败: "+err.Error())
		return
	}
	record.Revoked = req.Enabled
	OK(c, tokenView(record))
}

// HandleTokenReset POST /api/subscription/tokens/:id/reset 重置订阅令牌（换新随机值并恢复可用）。
func (a *API) HandleTokenReset(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var record db.SubscriptionToken
	if err := a.db.First(&record, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "订阅令牌不存在")
		return
	}
	updates := map[string]any{"token": randToken(), "revoked": false}
	if err := a.db.Model(&record).Updates(updates).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "重置订阅令牌失败: "+err.Error())
		return
	}
	record.Token = updates["token"].(string)
	record.Revoked = false
	OK(c, tokenView(record))
}

// randToken 生成 32 字符随机订阅令牌。
func randToken() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		panic(err) // 系统随机源不可用属致命错误
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
