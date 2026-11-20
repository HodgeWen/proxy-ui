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

// tokenView 订阅用户响应视图（底层为匿名订阅令牌）。
type tokenView struct {
	ID         uint       `json:"id"`         // 主键
	Token      string     `json:"token"`      // 匿名令牌
	Name       string     `json:"name"`       // 名称（设备/使用者备注）
	Revoked    bool       `json:"revoked"`    // 是否已吊销
	CreatedAt  time.Time  `json:"createdAt"`  // 创建时间
	LastUsedAt *time.Time `json:"lastUsedAt"` // 最近一次使用时间
}

// tokenNameReq 创建令牌请求。
type tokenNameReq struct {
	Name string `json:"name"` // 备注
}

// HandleSubscriptionGet GET /api/subscription 订阅用户列表。
func (a *API) HandleSubscriptionGet(c *gin.Context) {
	var records []db.SubscriptionToken
	if err := a.db.Order("id").Find(&records).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "查询订阅用户失败: "+err.Error())
		return
	}
	views := make([]tokenView, len(records))
	for i, r := range records {
		views[i] = tokenView(r)
	}
	OK(c, views)
}

// HandleTokenDelete DELETE /api/subscription/tokens/:id 删除订阅用户。
func (a *API) HandleTokenDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var record db.SubscriptionToken
	if err := a.db.First(&record, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "订阅用户不存在")
		return
	}
	if err := a.db.Delete(&record).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "删除订阅用户失败: "+err.Error())
		return
	}
	OK(c, gin.H{"deleted": id})
}

// HandleTokenCreate POST /api/subscription/tokens 新建订阅用户。
func (a *API) HandleTokenCreate(c *gin.Context) {
	var req tokenNameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	record := db.SubscriptionToken{Token: randToken(), Name: strings.TrimSpace(req.Name)}
	if err := a.db.Create(&record).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "创建订阅用户失败: "+err.Error())
		return
	}
	OK(c, tokenView(record))
}

// HandleTokenRevoke PATCH /api/subscription/tokens/:id/revoked 停用/启用订阅用户。
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
		Fail(c, http.StatusNotFound, "订阅用户不存在")
		return
	}
	if err := a.db.Model(&record).Update("revoked", req.Enabled).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "更新订阅用户失败: "+err.Error())
		return
	}
	record.Revoked = req.Enabled
	OK(c, tokenView(record))
}

// HandleTokenReset POST /api/subscription/tokens/:id/reset 重置订阅链接（换新随机令牌并恢复可用）。
func (a *API) HandleTokenReset(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var record db.SubscriptionToken
	if err := a.db.First(&record, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "订阅用户不存在")
		return
	}
	updates := map[string]any{"token": randToken(), "revoked": false}
	if err := a.db.Model(&record).Updates(updates).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "重置订阅链接失败: "+err.Error())
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
