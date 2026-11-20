package api

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/db"
	"proxy-ui/internal/subscription"
)

// tokenView 订阅用户响应视图（含配额用量与名下节点）。
type tokenView struct {
	ID                uint            `json:"id"`                // 主键
	Token             string          `json:"token"`             // 匿名令牌
	Name              string          `json:"name"`              // 备注
	Revoked           bool            `json:"revoked"`           // 是否手动停用
	TrafficLimitBytes int64           `json:"trafficLimitBytes"` // 流量配额字节，0 不限
	UsedBytes         int64           `json:"usedBytes"`         // 已用流量（上+下行）
	ExpiresAt         *time.Time      `json:"expiresAt"`         // 到期时间，空为永久
	Expired           bool            `json:"expired"`           // 是否已到期
	OverQuota         bool            `json:"overQuota"`         // 是否超出配额
	CreatedAt         time.Time       `json:"createdAt"`         // 创建时间
	LastUsedAt        *time.Time      `json:"lastUsedAt"`        // 最近一次使用时间
	Nodes             []tokenNodeView `json:"nodes"`             // 该用户在各入站下的节点
}

// tokenNodeView 订阅用户的一个节点（= 在某入站下的一个用户身份）。
type tokenNodeView struct {
	UserID        uint   `json:"userId"`        // 入站用户 ID
	InboundID     uint   `json:"inboundId"`     // 入站 ID
	InboundName   string `json:"inboundName"`   // 入站名（节点 tag 前缀）
	Protocol      string `json:"protocol"`      // 协议
	ListenPort    int    `json:"listenPort"`    // 入站端口
	Name          string `json:"name"`          // 用户名（客户端节点名）
	Enabled       bool   `json:"enabled"`       // 生效状态
	Disabled      bool   `json:"disabled"`      // 管理员手动停用
	Suspended     bool   `json:"suspended"`     // 是否被系统暂停
	UploadBytes   int64  `json:"uploadBytes"`   // 上行累计
	DownloadBytes int64  `json:"downloadBytes"` // 下行累计
}

// tokenReq 创建/更新订阅用户请求；指针字段缺省表示不修改。
type tokenReq struct {
	Name              *string    `json:"name"`              // 备注
	TrafficLimitBytes *int64     `json:"trafficLimitBytes"` // 流量配额字节，0 不限
	ExpiresAt         *time.Time `json:"expiresAt"`         // 到期时间
	ClearExpiry       bool       `json:"clearExpiry"`       // 清除到期时间
	NodeInbounds      *[]uint    `json:"nodeInbounds"`      // 勾选的入站 ID 列表（缺省不改）
}

// HandleSubscriptionGet GET /api/subscription 订阅用户列表。
func (a *API) HandleSubscriptionGet(c *gin.Context) {
	views, err := a.tokenViews()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询订阅用户失败: "+err.Error())
		return
	}
	OK(c, views)
}

// HandleTokenPreview GET /api/subscription/tokens/:id/preview?format=sing-box|mihomo
// 预览该用户的订阅配置：忽略停用、配额与到期，只要有启用中的节点就按格式渲染真实下发内容；
// 无可用节点时返回 400 与明确原因。
func (a *API) HandleTokenPreview(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	format := subscription.FormatSingBox
	if raw := strings.TrimSpace(c.Query("format")); raw != "" {
		parsed, valid := subscription.ParseFormat(raw)
		if !valid {
			Fail(c, http.StatusBadRequest, "不支持的格式：仅支持 sing-box 或 mihomo")
			return
		}
		format = parsed
	}
	content, err := a.subs.Preview(id, c.Request.Host, format)
	if err != nil {
		switch {
		case errors.Is(err, subscription.ErrInvalidToken):
			Fail(c, http.StatusNotFound, "订阅用户不存在")
		case errors.Is(err, subscription.ErrNoNodes):
			Fail(c, http.StatusBadRequest, err.Error())
		default:
			Fail(c, http.StatusInternalServerError, "生成预览失败: "+err.Error())
		}
		return
	}
	OK(c, gin.H{"format": previewFormatLabel(format), "content": string(content)})
}

// previewFormatLabel 预览响应的格式标识：对外统一称 mihomo（即 clash 格式）。
func previewFormatLabel(format subscription.Format) string {
	if format == subscription.FormatClash {
		return "mihomo"
	}
	return "sing-box"
}

// tokenViews 组装全部订阅用户视图。
func (a *API) tokenViews() ([]tokenView, error) {
	var records []db.SubscriptionToken
	if err := a.db.Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	views := make([]tokenView, 0, len(records))
	for _, r := range records {
		view, err := a.tokenViewOf(r)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// HandleTokenCreate POST /api/subscription/tokens 新建订阅用户（可同时勾选入站）。
func (a *API) HandleTokenCreate(c *gin.Context) {
	var req tokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	record := db.SubscriptionToken{Token: randToken()}
	if req.Name != nil {
		record.Name = strings.TrimSpace(*req.Name)
	}
	if req.TrafficLimitBytes != nil && *req.TrafficLimitBytes > 0 {
		record.TrafficLimitBytes = *req.TrafficLimitBytes
	}
	if req.ExpiresAt != nil {
		record.ExpiresAt = req.ExpiresAt
	}
	if err := a.db.Create(&record).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "创建订阅用户失败: "+err.Error())
		return
	}
	if req.NodeInbounds != nil {
		if err := a.syncTokenNodes(&record, dedupeIDs(*req.NodeInbounds)); err != nil {
			_ = a.deleteToken(record.ID)
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
	}
	if err := a.syncQuota(c); err != nil {
		return
	}
	view, err := a.tokenView(record.ID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, view)
}

// HandleTokenUpdate PUT /api/subscription/tokens/:id 更新订阅用户。
func (a *API) HandleTokenUpdate(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var record db.SubscriptionToken
	if err := a.db.First(&record, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "订阅用户不存在")
		return
	}
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.TrafficLimitBytes != nil {
		limit := *req.TrafficLimitBytes
		if limit < 0 {
			limit = 0
		}
		updates["traffic_limit_bytes"] = limit
	}
	switch {
	case req.ClearExpiry:
		updates["expires_at"] = nil
	case req.ExpiresAt != nil:
		updates["expires_at"] = *req.ExpiresAt
	}
	if len(updates) > 0 {
		if err := a.db.Model(&record).Updates(updates).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "保存订阅用户失败: "+err.Error())
			return
		}
		// 同步内存中的名称，否则新增节点会沿用改名前的旧前缀
		record = mustReloadToken(a, record.ID)
	}
	if req.NodeInbounds != nil {
		if err := a.syncTokenNodes(&record, dedupeIDs(*req.NodeInbounds)); err != nil {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
	}
	if err := a.syncQuota(c); err != nil {
		return
	}
	view, err := a.tokenView(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, view)
}

// HandleTokenDelete DELETE /api/subscription/tokens/:id 删除订阅用户及其全部节点用户。
func (a *API) HandleTokenDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.deleteToken(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	OK(c, gin.H{"deleted": id})
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
	if err := a.syncQuota(c); err != nil {
		return
	}
	view, err := a.tokenView(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, view)
}

// HandleTokenReset POST /api/subscription/tokens/:id/reset 重置订阅链接（换新令牌并恢复可用）。
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
	if err := a.db.Model(&record).Updates(map[string]any{"token": randToken(), "revoked": false}).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "重置订阅链接失败: "+err.Error())
		return
	}
	if err := a.syncQuota(c); err != nil {
		return
	}
	view, err := a.tokenView(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, view)
}

// HandleTokenResetTraffic POST /api/subscription/tokens/:id/reset-traffic 清零该用户全部节点累计流量。
func (a *API) HandleTokenResetTraffic(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var record db.SubscriptionToken
	if err := a.db.First(&record, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "订阅用户不存在")
		return
	}
	// 配额计数在 TokenTraffic 上，与用户的入站身份无关：
	// 先清零再对账，被暂停的用户就能立刻恢复。
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("token_id = ?", id).Delete(&db.TokenTraffic{}).Error; err != nil {
			return err
		}
		// 节点级的展示用量一并清零，列表里不会留着旧数字
		var users []db.InboundUser
		if err := tx.Where("id IN (?)", tx.Model(&db.SubscriptionNode{}).
			Select("inbound_user_id").Where("token_id = ?", id)).Find(&users).Error; err != nil {
			return err
		}
		for _, u := range users {
			if err := tx.Where("tag = ?", u.Name).Delete(&db.UserTraffic{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		Fail(c, http.StatusInternalServerError, "清零流量失败: "+err.Error())
		return
	}
	if err := a.syncQuota(c); err != nil {
		return
	}
	view, err := a.tokenView(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, view)
}

// mustReloadToken 重新读取订阅用户记录（改名后需要新名称给节点命名）。
func mustReloadToken(a *API, id uint) db.SubscriptionToken {
	var record db.SubscriptionToken
	if err := a.db.First(&record, id).Error; err != nil {
		return db.SubscriptionToken{ID: id}
	}
	return record
}

// syncQuota 写操作落库后统一处理：先按配额/到期刷新用户生效状态，再热应用内核。
// 失败时已写出响应并返回错误（数据库改动保留，便于用户修正后重试）。
func (a *API) syncQuota(c *gin.Context) error {
	if _, err := a.ReconcileQuota(); err != nil {
		Fail(c, http.StatusInternalServerError, "刷新配额状态失败: "+err.Error())
		return err
	}
	if err := a.applyKernelConfig(c.Request.Context()); err != nil {
		Fail(c, http.StatusBadRequest, "已保存，但内核按新配置重建失败: "+err.Error())
		return err
	}
	return nil
}

// tokenView 查询单个订阅用户视图。
func (a *API) tokenView(id uint) (tokenView, error) {
	var record db.SubscriptionToken
	if err := a.db.First(&record, id).Error; err != nil {
		return tokenView{}, err
	}
	return a.tokenViewOf(record)
}

// tokenViewOf 组装单个订阅用户视图。
func (a *API) tokenViewOf(record db.SubscriptionToken) (tokenView, error) {
	nodes, err := a.tokenNodes(record.ID)
	if err != nil {
		return tokenView{}, err
	}
	used, err := a.usedBytes(record.ID)
	if err != nil {
		return tokenView{}, err
	}
	return tokenView{
		ID: record.ID, Token: record.Token, Name: record.Name, Revoked: record.Revoked,
		TrafficLimitBytes: record.TrafficLimitBytes, UsedBytes: used,
		ExpiresAt: record.ExpiresAt,
		Expired:   record.ExpiresAt != nil && time.Now().After(*record.ExpiresAt),
		OverQuota: record.TrafficLimitBytes > 0 && used >= record.TrafficLimitBytes,
		CreatedAt: record.CreatedAt, LastUsedAt: record.LastUsedAt, Nodes: nodes,
	}, nil
}

// tokenNodes 查询订阅用户名下节点视图（各节点展示自身累计用量）。
func (a *API) tokenNodes(tokenID uint) ([]tokenNodeView, error) {
	var bindings []db.SubscriptionNode
	if err := a.db.Where("token_id = ?", tokenID).Order("inbound_id, id").Find(&bindings).Error; err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return []tokenNodeView{}, nil
	}
	userIDs := make([]uint, 0, len(bindings))
	for _, b := range bindings {
		userIDs = append(userIDs, b.InboundUserID)
	}
	var users []db.InboundUser
	if err := a.db.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	userMeta := make(map[uint]db.InboundUser, len(users))
	for _, u := range users {
		userMeta[u.ID] = u
	}
	var inbounds []db.Inbound
	if err := a.db.Find(&inbounds).Error; err != nil {
		return nil, err
	}
	inMeta := make(map[uint]db.Inbound, len(inbounds))
	for _, in := range inbounds {
		inMeta[in.ID] = in
	}
	traffic, err := a.trafficByTag()
	if err != nil {
		return nil, err
	}
	views := make([]tokenNodeView, 0, len(bindings))
	for _, b := range bindings {
		u, ok := userMeta[b.InboundUserID]
		if !ok {
			continue
		}
		in := inMeta[u.InboundID]
		t := traffic[u.Name]
		views = append(views, tokenNodeView{
			UserID: u.ID, InboundID: u.InboundID, InboundName: in.Name, Protocol: in.Protocol,
			ListenPort: in.ListenPort, Name: u.Name, Enabled: u.Enabled,
			Disabled: u.Disabled, Suspended: u.Suspended,
			UploadBytes: t.UploadBytes, DownloadBytes: t.DownloadBytes,
		})
	}
	return views, nil
}

// tokenUserIDs 订阅用户名下的全部入站用户 ID。
func (a *API) tokenUserIDs(tokenID uint) ([]uint, error) {
	var bindings []db.SubscriptionNode
	if err := a.db.Where("token_id = ?", tokenID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(bindings))
	for _, b := range bindings {
		ids = append(ids, b.InboundUserID)
	}
	return ids, nil
}

// deleteToken 删除订阅用户及其名下全部入站用户与流量记录。
func (a *API) deleteToken(id uint) error {
	userIDs, err := a.tokenUserIDs(id)
	if err != nil {
		return err
	}
	return a.db.Transaction(func(tx *gorm.DB) error {
		if len(userIDs) > 0 {
			var users []db.InboundUser
			if err := tx.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
				return err
			}
			// 流量按用户名做键，用户删了但流量不清会残留；重建同名用户会继承旧用量
			for _, u := range users {
				if err := tx.Where("tag = ?", u.Name).Delete(&db.UserTraffic{}).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("id IN ?", userIDs).Delete(&db.InboundUser{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("token_id = ?", id).Delete(&db.SubscriptionNode{}).Error; err != nil {
			return err
		}
		return tx.Delete(&db.SubscriptionToken{}, id).Error
	})
}

// dedupeIDs 去重并保持顺序（忽略 0）。
func dedupeIDs(ids []uint) []uint {
	seen := make(map[uint]bool, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// randToken 生成 32 字符随机订阅令牌。
func randToken() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		panic(err) // 系统随机源不可用属致命错误
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
