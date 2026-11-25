package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/ai"
	"proxy-ui/internal/db"
	"proxy-ui/internal/subscription"
)

// tokenView 订阅用户响应视图（含配额用量与名下节点）。
type tokenView struct {
	db.SubscriptionToken
	UsedBytes int64           `json:"usedBytes"` // 已用流量（上+下行）
	Expired   bool            `json:"expired"`   // 是否已到期
	OverQuota bool            `json:"overQuota"` // 是否超出配额
	Nodes     []tokenNodeView `json:"nodes"`     // 该用户在各入站下的节点
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
	Suspended     bool   `json:"suspended"`     // 是否被系统暂停
	UploadBytes   int64  `json:"uploadBytes"`   // 上行累计
	DownloadBytes int64  `json:"downloadBytes"` // 下行累计
}

// previewResp 订阅预览响应。
type previewResp struct {
	Format  string `json:"format"`  // 输出格式
	Content string `json:"content"` // 渲染内容
}

// subscriptionSettingsView 订阅全局设置视图（附加 IPv6 入口与本机探测地址）。
type subscriptionSettingsView struct {
	ServerIPv6    string   `json:"serverIpv6"`    // 订阅附加 IPv6 节点入口（空为关闭）
	DetectedIPv6s []string `json:"detectedIpv6s"` // 本机网卡探测到的公网 IPv6 地址
}

// subscriptionSettingsReq 更新订阅全局设置请求。
type subscriptionSettingsReq struct {
	ServerIPv6 string `json:"serverIpv6"` // 订阅附加 IPv6 节点入口（空为关闭）
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

// HandleSubscriptionSettingsGet GET /api/subscription/settings 查询订阅附加 IPv6 入口设置与本机公网 IPv6 探测结果。
func (a *API) HandleSubscriptionSettingsGet(c *gin.Context) {
	OK(c, a.subscriptionSettings())
}

// HandleSubscriptionSettingsSet PUT /api/subscription/settings 保存订阅附加 IPv6 入口（只影响订阅导出，不重建内核）。
func (a *API) HandleSubscriptionSettingsSet(c *gin.Context) {
	var req subscriptionSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	view, err := a.saveSubscriptionSettings(req.ServerIPv6)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, view)
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
	OK(c, previewResp{Format: previewFormatLabel(format), Content: string(content)})
}

// previewFormatLabel 预览响应的格式标识：对外统一称 mihomo（即 clash 格式）。
func previewFormatLabel(format subscription.Format) string {
	if format == subscription.FormatClash {
		return "mihomo"
	}
	return "sing-box"
}

// tokenViews 组装全部订阅用户视图（节点与用量一轮批量查询，避免逐用户往返）。
func (a *API) tokenViews() ([]tokenView, error) {
	var records []db.SubscriptionToken
	if err := a.db.Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	nodesByToken, err := a.tokenNodesByToken()
	if err != nil {
		return nil, err
	}
	usedByToken, err := a.usedBytesByToken()
	if err != nil {
		return nil, err
	}
	views := make([]tokenView, 0, len(records))
	for _, r := range records {
		nodes := nodesByToken[r.ID]
		if nodes == nil {
			nodes = []tokenNodeView{}
		}
		views = append(views, tokenViewOf(r, nodes, usedByToken[r.ID]))
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
	view, err := a.createToken(req)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	a.applyThenOK(c, view)
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
	view, err := a.updateToken(id, req)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	a.applyThenOK(c, view)
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
	a.applyThenOK(c, deletedResp{Deleted: id})
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
	view, err := a.tokenReconciledView(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	a.applyThenOK(c, view)
}

// HandleTokenReset POST /api/subscription/tokens/:id/reset 重置订阅链接（换新令牌并恢复可用）。
func (a *API) HandleTokenReset(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	view, err := a.resetTokenLink(id)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	a.applyThenOK(c, view)
}

// HandleTokenResetTraffic POST /api/subscription/tokens/:id/reset-traffic 清零该用户全部节点累计流量。
func (a *API) HandleTokenResetTraffic(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	view, err := a.resetTokenTraffic(id)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	a.applyThenOK(c, view)
}

// ---------- 供 REST handler 与 AI 工具共用的写操作 ----------

// createToken 创建订阅用户（可同时勾选入站）：记录创建与节点同步同处一个事务，
// 同步失败整体回滚（无需补偿删除），成功后按配额对账并返回最新视图。
func (a *API) createToken(req tokenReq) (tokenView, error) {
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
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("创建订阅用户失败: %w", err)
		}
		if req.NodeInbounds != nil {
			return a.syncTokenNodes(tx, &record, dedupeIDs(*req.NodeInbounds))
		}
		return nil
	})
	if err != nil {
		return tokenView{}, err
	}
	return a.tokenReconciledView(record.ID)
}

// updateToken 更新订阅用户（名称、配额、到期与勾选入站）。字段更新与节点同步同处一个事务：
// 失败整体回滚，不留半应用状态（如改了名但节点同步到一半失败）。
func (a *API) updateToken(id uint, req tokenReq) (tokenView, error) {
	var record db.SubscriptionToken
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&record, id).Error; err != nil {
			return fmt.Errorf("订阅用户不存在: %d", id)
		}
		updates := map[string]any{}
		if req.Name != nil {
			updates["name"] = strings.TrimSpace(*req.Name)
		}
		if req.TrafficLimitBytes != nil {
			updates["traffic_limit_bytes"] = max(*req.TrafficLimitBytes, 0)
		}
		switch {
		case req.ClearExpiry:
			updates["expires_at"] = nil
		case req.ExpiresAt != nil:
			updates["expires_at"] = *req.ExpiresAt
		}
		if len(updates) > 0 {
			if err := tx.Model(&record).Updates(updates).Error; err != nil {
				return fmt.Errorf("保存订阅用户失败: %w", err)
			}
			// 同步内存中的名称，否则新增节点会沿用改名前的旧前缀
			record = mustReloadToken(tx, record.ID)
		}
		if req.NodeInbounds != nil {
			return a.syncTokenNodes(tx, &record, dedupeIDs(*req.NodeInbounds))
		}
		return nil
	})
	if err != nil {
		return tokenView{}, err
	}
	return a.tokenReconciledView(id)
}

// resetTokenLink 重置订阅链接（换新随机值并恢复可用，旧链接立即失效）。
func (a *API) resetTokenLink(id uint) (tokenView, error) {
	var record db.SubscriptionToken
	if err := a.db.First(&record, id).Error; err != nil {
		return tokenView{}, fmt.Errorf("订阅用户不存在: %d", id)
	}
	if err := a.db.Model(&record).Updates(map[string]any{"token": randToken(), "revoked": false}).Error; err != nil {
		return tokenView{}, fmt.Errorf("重置订阅链接失败: %w", err)
	}
	return a.tokenReconciledView(id)
}

// resetTokenTraffic 清零某订阅用户全部节点的累计流量（配额已用尽的用户会随之恢复）。
func (a *API) resetTokenTraffic(id uint) (tokenView, error) {
	var record db.SubscriptionToken
	if err := a.db.First(&record, id).Error; err != nil {
		return tokenView{}, fmt.Errorf("订阅用户不存在: %d", id)
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
		return deleteUsersTraffic(tx, users)
	})
	if err != nil {
		return tokenView{}, fmt.Errorf("清零流量失败: %w", err)
	}
	return a.tokenReconciledView(id)
}

// mustReloadToken 重新读取订阅用户记录（改名后需要新名称给节点命名）。
func mustReloadToken(tx *gorm.DB, id uint) db.SubscriptionToken {
	var record db.SubscriptionToken
	if err := tx.First(&record, id).Error; err != nil {
		return db.SubscriptionToken{ID: id}
	}
	return record
}

// tokenReconciledView 按配额/到期刷新用户生效状态后组装订阅用户视图（订阅写操作统一收尾；
// 内核热应用由调用方负责）。
func (a *API) tokenReconciledView(id uint) (tokenView, error) {
	if _, err := a.ReconcileQuota(); err != nil {
		return tokenView{}, fmt.Errorf("刷新配额状态失败: %w", err)
	}
	return a.tokenView(id)
}

// tokenView 查询单个订阅用户视图。
func (a *API) tokenView(id uint) (tokenView, error) {
	var record db.SubscriptionToken
	if err := a.db.First(&record, id).Error; err != nil {
		return tokenView{}, err
	}
	nodes, err := a.tokenNodes(record.ID)
	if err != nil {
		return tokenView{}, err
	}
	used, err := a.usedBytes(record.ID)
	if err != nil {
		return tokenView{}, err
	}
	return tokenViewOf(record, nodes, used), nil
}

// tokenViewOf 组装单个订阅用户视图（节点与用量由调用方给出）。
func tokenViewOf(record db.SubscriptionToken, nodes []tokenNodeView, used int64) tokenView {
	if nodes == nil {
		nodes = []tokenNodeView{}
	}
	return tokenView{
		SubscriptionToken: record,
		UsedBytes:         used,
		Expired:           record.ExpiresAt != nil && time.Now().After(*record.ExpiresAt),
		OverQuota:         record.TrafficLimitBytes > 0 && used >= record.TrafficLimitBytes,
		Nodes:             nodes,
	}
}

// tokenNodes 查询某订阅用户名下节点视图（各节点展示自身累计用量）。
func (a *API) tokenNodes(tokenID uint) ([]tokenNodeView, error) {
	byToken, err := a.tokenNodesByToken(tokenID)
	if err != nil {
		return nil, err
	}
	if nodes, ok := byToken[tokenID]; ok {
		return nodes, nil
	}
	return []tokenNodeView{}, nil
}

// tokenNodesByToken 批量查询组装订阅用户名下的节点视图（键为订阅用户 ID）；
// 传入 tokenIDs 时仅查询对应绑定、用户、入站及流量，无参数时查询全部供列表页使用。
func (a *API) tokenNodesByToken(tokenIDs ...uint) (map[uint][]tokenNodeView, error) {
	scoped := len(tokenIDs) > 0
	query := a.db.Order("token_id, inbound_id, id")
	if scoped {
		query = query.Where("token_id IN ?", tokenIDs)
	}
	var bindings []db.SubscriptionNode
	if err := query.Find(&bindings).Error; err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return map[uint][]tokenNodeView{}, nil
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
	tags := make([]string, 0, len(users))
	inboundIDSet := make(map[uint]bool, len(users))
	inboundIDs := make([]uint, 0, len(users))
	for _, u := range users {
		userMeta[u.ID] = u
		tags = append(tags, u.Name)
		if !inboundIDSet[u.InboundID] {
			inboundIDSet[u.InboundID] = true
			inboundIDs = append(inboundIDs, u.InboundID)
		}
	}
	var inbounds []db.Inbound
	switch {
	case !scoped:
		if err := a.db.Find(&inbounds).Error; err != nil {
			return nil, err
		}
	case len(inboundIDs) > 0:
		if err := a.db.Where("id IN ?", inboundIDs).Find(&inbounds).Error; err != nil {
			return nil, err
		}
	}
	inMeta := make(map[uint]db.Inbound, len(inbounds))
	for _, in := range inbounds {
		inMeta[in.ID] = in
	}
	var traffic map[string]db.UserTraffic
	switch {
	case !scoped:
		var err error
		traffic, err = a.trafficByTag()
		if err != nil {
			return nil, err
		}
	case len(tags) > 0:
		var err error
		traffic, err = a.trafficByTags(tags)
		if err != nil {
			return nil, err
		}
	}
	byToken := make(map[uint][]tokenNodeView, len(bindings))
	for _, b := range bindings {
		u, ok := userMeta[b.InboundUserID]
		if !ok {
			continue
		}
		in := inMeta[u.InboundID]
		t := traffic[u.Name]
		byToken[b.TokenID] = append(byToken[b.TokenID], tokenNodeView{
			UserID: u.ID, InboundID: u.InboundID, InboundName: in.Name, Protocol: in.Protocol,
			ListenPort: in.ListenPort, Name: u.Name, Enabled: u.Enabled, Suspended: u.Suspended,
			UploadBytes: t.UploadBytes, DownloadBytes: t.DownloadBytes,
		})
	}
	return byToken, nil
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
			if err := deleteUsersTraffic(tx, users); err != nil {
				return err
			}
			if err := tx.Where("id IN ?", userIDs).Delete(&db.InboundUser{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("token_id = ?", id).Delete(&db.TokenTraffic{}).Error; err != nil {
			return err
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

// subscriptionSettings 读取当前订阅 IPv6 入口设置与本机公网 IPv6 探测地址。
func (a *API) subscriptionSettings() subscriptionSettingsView {
	return subscriptionSettingsView{
		ServerIPv6:    a.getSetting(db.SettingServerIPv6),
		DetectedIPv6s: detectPublicIPv6s(),
	}
}

// saveSubscriptionSettings 校验并保存订阅 IPv6 入口设置（不触发内核重建）。
func (a *API) saveSubscriptionSettings(raw string) (subscriptionSettingsView, error) {
	normalized, err := subscription.NormalizeIPv6Host(raw)
	if err != nil {
		return subscriptionSettingsView{}, err
	}
	if err := a.setSetting(db.SettingServerIPv6, normalized); err != nil {
		return subscriptionSettingsView{}, fmt.Errorf("保存 IPv6 入口设置失败: %w", err)
	}
	return subscriptionSettingsView{
		ServerIPv6:    normalized,
		DetectedIPv6s: detectPublicIPv6s(),
	}, nil
}

// detectPublicIPv6s 探测本机网卡上的全局单播公网 IPv6 地址（排除回环、链路本地、ULA 私网与 IPv4 映射）。
func detectPublicIPv6s() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return []string{}
	}
	seen := make(map[netip.Addr]bool)
	out := []string{}
	for _, item := range addrs {
		var addr netip.Addr
		switch v := item.(type) {
		case *net.IPNet:
			if parsed, ok := netip.AddrFromSlice(v.IP); ok {
				addr = parsed
			}
		case *net.IPAddr:
			if parsed, ok := netip.AddrFromSlice(v.IP); ok {
				addr = parsed
			}
		}
		addr = addr.WithZone("")
		if !addr.IsValid() || !addr.Is6() || addr.Is4In6() || !addr.IsGlobalUnicast() || addr.IsPrivate() {
			continue
		}
		if seen[addr] {
			continue
		}
		seen[addr] = true
		out = append(out, addr.String())
	}
	slices.Sort(out)
	return out
}

// ---------- 订阅用户 AI 工具 ----------

// registerTokenAITools 注册订阅用户相关的查询与写操作 AI 工具。
func (a *API) registerTokenAITools() {
	a.ai.Register(ai.Tool{
		Name:        "get_subscription_settings",
		Description: "查询订阅附加 IPv6 入口设置（serverIpv6）以及本机网卡探测到的公网 IPv6 地址列表（detectedIpv6s）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.subscriptionSettings(), nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "set_subscription_settings",
		Description: "设置订阅附加 IPv6 节点入口（填公网 IPv6 地址或纯 AAAA 域名，空串关闭；保存后订阅自动为双栈入站追加 -ipv6 节点，无需重建内核）",
		Parameters: toolParams(map[string]any{
			"serverIpv6": toolProp("string", "公网 IPv6 地址或纯 AAAA 域名，空串关闭"),
		}, "serverIpv6"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req subscriptionSettingsReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			return a.saveSubscriptionSettings(req.ServerIPv6)
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "list_subscription_tokens",
		Description: "列出全部订阅用户（含匿名 token、名称、配额、已用流量、到期时间、停用状态与各自节点）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.tokenViews()
		},
	})
	a.ai.Register(ai.Tool{
		Name: "preview_subscription",
		Description: "按订阅用户 ID 预览其订阅配置的真实渲染内容（与页面预览一致）；" +
			"忽略停用、配额与到期状态，只要有启用中的节点就返回完整配置，无节点时报明确原因",
		Parameters: toolParams(map[string]any{
			"id":     toolProp("integer", "订阅用户 ID（可用 list_subscription_tokens 查询）"),
			"format": toolProp("string", "输出格式，缺省 sing-box", "sing-box", "mihomo"),
		}, "id"),
		Exec: func(ctx context.Context, args json.RawMessage) (any, error) {
			var arg struct {
				ID     uint   `json:"id"`
				Format string `json:"format"`
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if arg.ID == 0 {
				return nil, errors.New("缺少订阅用户 ID")
			}
			format := subscription.FormatSingBox
			if strings.TrimSpace(arg.Format) != "" {
				parsed, valid := subscription.ParseFormat(arg.Format)
				if !valid {
					return nil, errors.New("不支持的格式：仅支持 sing-box 或 mihomo")
				}
				format = parsed
			}
			content, err := a.subs.Preview(arg.ID, requestHostFrom(ctx), format)
			if err != nil {
				return nil, err
			}
			return previewResp{Format: previewFormatLabel(format), Content: string(content)}, nil
		},
	})
	a.registerModify(ai.Tool{
		Name: "create_subscription_token",
		Description: "创建订阅用户（客户端免登录拉取订阅链接）。nodeInbounds 给出要勾选的入站 ID，" +
			"每个入站内会为该用户创建一个独立凭据的用户身份（共享入站端口），即该用户的一个节点",
		Parameters: toolParams(map[string]any{
			"name":              toolProp("string", "备注名"),
			"trafficLimitBytes": toolProp("integer", "流量配额字节（上+下行），0 或不填为不限"),
			"expiresAt":         toolProp("string", "到期时间（RFC3339），不填为永久"),
			"nodeInbounds":      map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "要勾选的入站 ID 列表"},
		}),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req tokenReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			return a.createToken(req)
		},
	})
	a.registerModify(ai.Tool{
		Name:        "update_subscription_token",
		Description: "更新订阅用户：名称、流量配额、到期时间、勾选的入站（会在入站内创建/移除该用户的身份）。保存后内核立即按新配置重建",
		Parameters: toolParams(map[string]any{
			"id":                toolProp("integer", "订阅用户 ID"),
			"name":              toolProp("string", "备注名"),
			"trafficLimitBytes": toolProp("integer", "流量配额字节，0 不限"),
			"expiresAt":         toolProp("string", "到期时间（RFC3339）"),
			"clearExpiry":       toolProp("boolean", "true 表示清除到期时间（永久）"),
			"nodeInbounds":      map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "勾选的入站 ID 列表（整体替换）"},
		}, "id"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req tokenReq
			var arg idArg
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			return a.updateToken(arg.ID, req)
		},
	})
	a.registerModify(ai.Tool{
		Name:         "delete_subscription_token",
		Description:  "按 ID 删除订阅用户（连同其全部节点与累计流量）",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "订阅用户 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if err := a.deleteToken(arg.ID); err != nil {
				return nil, err
			}
			return deletedResp{Deleted: arg.ID}, nil
		},
	})
	a.registerModify(ai.Tool{
		Name:         "reset_subscription_token",
		Description:  "按 ID 重置订阅链接（换新随机值并恢复可用，旧链接立即失效）",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "订阅用户 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			return a.resetTokenLink(arg.ID)
		},
	})
	a.registerModify(ai.Tool{
		Name:         "reset_user_traffic",
		Description:  "按 ID 清零某订阅用户全部节点的累计流量（配额已用尽的用户会随之恢复）",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "订阅用户 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			return a.resetTokenTraffic(arg.ID)
		},
	})
}
