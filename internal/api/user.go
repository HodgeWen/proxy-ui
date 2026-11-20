package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// inboundUserView 入站内用户响应视图（含累计流量与订阅归属）。
type inboundUserView struct {
	ID            uint      `json:"id"`            // 主键
	InboundID     uint      `json:"inboundId"`     // 所属入站
	Name          string    `json:"name"`          // 用户名（内核统计名、客户端节点名）
	Enabled       bool      `json:"enabled"`       // 生效状态
	Disabled      bool      `json:"disabled"`      // 管理员手动停用
	Suspended     bool      `json:"suspended"`     // 被系统暂停（配额用尽/到期）
	UploadBytes   int64     `json:"uploadBytes"`   // 上行累计字节数
	DownloadBytes int64     `json:"downloadBytes"` // 下行累计字节数
	TokenID       uint      `json:"tokenId"`       // 归属的订阅用户（0 表示未绑定）
	TokenName     string    `json:"tokenName"`     // 归属订阅用户备注
	Remark        string    `json:"remark"`        // 独立用户备注（未绑定订阅用户时的手动用户）
	CreatedAt     time.Time `json:"createdAt"`     // 创建时间
}

// userReq 创建/更新入站用户请求。
type userReq struct {
	Name    string `json:"name"`    // 用户名，空时自动生成
	Enabled *bool  `json:"enabled"` // 是否启用，缺省 true
}

// HandleInboundUsersList GET /api/inbounds/:id/users 某入站的用户列表。
func (a *API) HandleInboundUsersList(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	views, err := a.userViews(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询用户失败: "+err.Error())
		return
	}
	OK(c, views)
}

// HandleInboundUserCreate POST /api/inbounds/:id/users 在入站下新建独立用户（自动生成凭据）。
func (a *API) HandleInboundUserCreate(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req userReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var in db.Inbound
	if err := a.db.First(&in, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "入站不存在")
		return
	}
	user, err := a.createUser(&in, strings.TrimSpace(req.Name), req.Enabled)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	views, _ := a.userViews(id)
	for _, v := range views {
		if v.ID == user.ID {
			OK(c, v)
			return
		}
	}
	OK(c, userToView(user))
}

// HandleInboundUserDelete DELETE /api/users/:id 删除入站用户（连同其流量与订阅绑定）。
func (a *API) HandleInboundUserDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.deleteUser(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	OK(c, gin.H{"deleted": id})
}

// HandleInboundUserSetEnabled PATCH /api/users/:id/enabled 启停入站用户。
// 被配额/到期暂停的用户不允许在此恢复（需先清零流量或延长到期），避免绕过配额。
func (a *API) HandleInboundUserSetEnabled(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req enabledReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var user db.InboundUser
	if err := a.db.First(&user, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if req.Enabled && user.Suspended {
		Fail(c, http.StatusBadRequest, "该用户因配额用尽或到期被暂停，请先在订阅页清零流量或延长到期时间")
		return
	}
	// 手动停用记在 disabled 上，配额协调时不会被覆盖
	disabled := !req.Enabled
	if err := a.db.Model(&user).Updates(map[string]any{
		"disabled": disabled, "enabled": !disabled && !user.Suspended,
	}).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "更新用户状态失败: "+err.Error())
		return
	}
	user.Disabled = disabled
	user.Enabled = !disabled && !user.Suspended
	if !a.applyOrFail(c) {
		return
	}
	OK(c, userToView(user))
}

// createUser 在入站下新建用户并生成该协议凭据；name 为空按「订阅备注-入站名」或「入站名-序号」生成。
func (a *API) createUser(in *db.Inbound, name string, enabled *bool) (db.InboundUser, error) {
	cfg, err := singbox.ParseInboundConfig(in.Protocol, in.Config)
	if err != nil {
		return db.InboundUser{}, err
	}
	cred, err := a.newCredential(in.Protocol, cfg)
	if err != nil {
		return db.InboundUser{}, err
	}
	rawCred, err := json.Marshal(cred)
	if err != nil {
		return db.InboundUser{}, fmt.Errorf("序列化凭据失败: %w", err)
	}
	if name == "" {
		name = a.uniqueUserName(in.ID, in.Protocol)
	}
	if err := a.checkUserName(in.ID, name, 0); err != nil {
		return db.InboundUser{}, err
	}
	user := db.InboundUser{
		InboundID: in.ID, Name: name, Credential: string(rawCred), Enabled: true,
	}
	if enabled != nil {
		user.Enabled = *enabled
	}
	if err := a.db.Create(&user).Error; err != nil {
		return db.InboundUser{}, fmt.Errorf("创建用户失败: %w", err)
	}
	return user, nil
}

// newCredential 生成协议凭据；Shadowsocks 需要按入站方法生成定长密钥。
func (a *API) newCredential(protocol string, cfg any) (singbox.Credential, error) {
	if protocol == singbox.ProtoShadowsocks {
		method := "2022-blake3-aes-128-gcm"
		if c, ok := cfg.(*singbox.ShadowsocksConfig); ok && c.Method != "" {
			method = c.Method
		}
		key, err := singbox.NewSSKey(method)
		if err != nil {
			return singbox.Credential{}, err
		}
		return singbox.Credential{Password: key}, nil
	}
	return singbox.NewCredential(protocol)
}

// uniqueUserName 生成入站内未占用的用户名（u-协议-序号）。
//
// 前缀 u- 是为了与入站 tag（协议-序号）区分：入站名与用户名共用流量表的 tag 键，
// 撞名会让两者的用量互相覆盖。
func (a *API) uniqueUserName(inboundID uint, protocol string) string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("u-%s-%d", protocol, i)
		if a.checkUserName(inboundID, name, 0) == nil {
			return name
		}
	}
}

// checkUserName 校验用户名合法且**全局**唯一（同时不得等于任何入站 tag）。
//
// 用户名是流量统计与节点展示的键，两个入站下同名用户会让用量混在一起，
// 因此唯一性必须是全局的，不能只在单个入站内检查。
func (a *API) checkUserName(inboundID uint, name string, excludeID uint) error {
	if !validTag(name) {
		return fmt.Errorf("用户名只能包含字母、数字、下划线、点与短横线: %s", name)
	}
	var dup int64
	a.db.Model(&db.InboundUser{}).Where("name = ? AND id <> ?", name, excludeID).Count(&dup)
	if dup > 0 {
		return fmt.Errorf("用户名 %s 已被占用，请换一个", name)
	}
	var clash int64
	a.db.Model(&db.Inbound{}).Where("name = ?", name).Count(&clash)
	if clash > 0 {
		return fmt.Errorf("用户名 %s 与入站名冲突，请换一个", name)
	}
	return nil
}

// deleteUser 删除入站用户及其流量记录与订阅绑定。
func (a *API) deleteUser(id uint) error {
	var user db.InboundUser
	if err := a.db.First(&user, id).Error; err != nil {
		return fmt.Errorf("用户不存在: %d", id)
	}
	return a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("inbound_user_id = ?", id).Delete(&db.SubscriptionNode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tag = ?", user.Name).Delete(&db.UserTraffic{}).Error; err != nil {
			return err
		}
		return tx.Delete(&db.InboundUser{}, id).Error
	})
}

// HandleUsersList GET /api/users 全部入站用户（跨入站汇总，含流量与订阅归属）。
func (a *API) HandleUsersList(c *gin.Context) {
	var inbounds []db.Inbound
	if err := a.db.Order("id").Find(&inbounds).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "查询入站失败: "+err.Error())
		return
	}
	views := make([]inboundUserView, 0)
	for _, in := range inbounds {
		list, err := a.userViews(in.ID)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "查询用户失败: "+err.Error())
			return
		}
		views = append(views, list...)
	}
	OK(c, views)
}

// userViews 查询某入站的用户视图（含流量与订阅归属）。
func (a *API) userViews(inboundID uint) ([]inboundUserView, error) {
	var users []db.InboundUser
	if err := a.db.Where("inbound_id = ?", inboundID).Order("id").Find(&users).Error; err != nil {
		return nil, err
	}
	traffic, err := a.trafficByTag()
	if err != nil {
		return nil, err
	}
	bindings, names, err := a.bindingMap()
	if err != nil {
		return nil, err
	}
	views := make([]inboundUserView, 0, len(users))
	for _, u := range users {
		view := userToView(u)
		t := traffic[u.Name]
		view.UploadBytes, view.DownloadBytes = t.UploadBytes, t.DownloadBytes
		if binding, ok := bindings[u.ID]; ok {
			view.TokenID = binding.TokenID
			view.TokenName = names[binding.TokenID]
		}
		views = append(views, view)
	}
	return views, nil
}

// userToView 入站用户转视图（不含流量，由调用方补齐）。
func userToView(u db.InboundUser) inboundUserView {
	return inboundUserView{
		ID: u.ID, InboundID: u.InboundID, Name: u.Name,
		Enabled: u.Enabled, Disabled: u.Disabled, Suspended: u.Suspended, CreatedAt: u.CreatedAt,
	}
}

// trafficByTag 查询全部累计流量（键为用户名/入站 tag）。
func (a *API) trafficByTag() (map[string]db.UserTraffic, error) {
	var rows []db.UserTraffic
	if err := a.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	byTag := make(map[string]db.UserTraffic, len(rows))
	for _, r := range rows {
		byTag[r.Tag] = r
	}
	return byTag, nil
}

// bindingMap 用户 ID -> 订阅绑定，以及订阅 ID -> 备注。
func (a *API) bindingMap() (map[uint]db.SubscriptionNode, map[uint]string, error) {
	var nodes []db.SubscriptionNode
	if err := a.db.Find(&nodes).Error; err != nil {
		return nil, nil, err
	}
	byUser := make(map[uint]db.SubscriptionNode, len(nodes))
	for _, n := range nodes {
		byUser[n.InboundUserID] = n
	}
	var tokens []db.SubscriptionToken
	if err := a.db.Find(&tokens).Error; err != nil {
		return nil, nil, err
	}
	names := make(map[uint]string, len(tokens))
	for _, t := range tokens {
		names[t.ID] = t.Name
	}
	return byUser, names, nil
}
