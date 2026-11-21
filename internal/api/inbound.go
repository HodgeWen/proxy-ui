package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/ai"
	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// inboundView 入站响应视图：连接层配置 + 用户数 + 弱配置警示。
// 用户统一在「订阅用户」页管理，入站视图只给出其下用户数量。
type inboundView struct {
	ID         uint            `json:"id"`         // 主键
	Name       string          `json:"name"`       // 名称（内核入站 tag）
	Protocol   string          `json:"protocol"`   // 协议
	Listen     string          `json:"listen"`     // 监听地址
	ListenPort int             `json:"listenPort"` // 监听端口
	Enabled    bool            `json:"enabled"`    // 是否启用
	Config     json.RawMessage `json:"config"`     // 连接层配置（对象，不含用户凭据）
	Warnings   []string        `json:"warnings"`   // 弱配置警示
	UserCount  int64           `json:"userCount"`  // 该入站下的用户数
	CreatedAt  time.Time       `json:"createdAt"`  // 创建时间
	UpdatedAt  time.Time       `json:"updatedAt"`  // 更新时间
}

// inboundReq 创建/更新入站请求；Config 为空时按协议填默认值，ListenPort 为空时取随机空闲端口。
type inboundReq struct {
	Name       string          `json:"name"`       // 名称，空时按 协议-序号 生成
	Protocol   string          `json:"protocol"`   // 协议
	Listen     string          `json:"listen"`     // 监听地址，空时默认双栈
	ListenPort int             `json:"listenPort"` // 监听端口，0 时随机
	Enabled    *bool           `json:"enabled"`    // 是否启用，缺省 true
	Config     json.RawMessage `json:"config"`     // 连接层配置 JSON
}

// HandleInboundsList GET /api/inbounds 入站列表（含各自用户）。
func (a *API) HandleInboundsList(c *gin.Context) {
	views, err := a.inboundViews()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询入站列表失败: "+err.Error())
		return
	}
	OK(c, views)
}

// inboundViews 查询全部入站并组装视图。
func (a *API) inboundViews() ([]inboundView, error) {
	var records []db.Inbound
	if err := a.db.Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	counts, err := a.inboundUserCounts()
	if err != nil {
		return nil, err
	}
	views := make([]inboundView, 0, len(records))
	for _, in := range records {
		views = append(views, a.inboundToView(in, counts[in.ID]))
	}
	return views, nil
}

// inboundUserCounts 一轮分组统计各入站下的用户数（键为入站 ID）。
func (a *API) inboundUserCounts() (map[uint]int64, error) {
	var rows []struct {
		InboundID uint
		Count     int64
	}
	if err := a.db.Model(&db.InboundUser{}).
		Select("inbound_id, COUNT(*) AS count").Group("inbound_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[uint]int64, len(rows))
	for _, r := range rows {
		counts[r.InboundID] = r.Count
	}
	return counts, nil
}

// inboundUserCount 统计单个入站下的用户数。
func (a *API) inboundUserCount(inboundID uint) (int64, error) {
	var count int64
	if err := a.db.Model(&db.InboundUser{}).Where("inbound_id = ?", inboundID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// HandleInboundCreate POST /api/inbounds 创建入站（空配置填默认值，端口默认随机空闲端口）。
func (a *API) HandleInboundCreate(c *gin.Context) {
	var req inboundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	view, err := a.createInbound(req)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	a.applyThenOK(c, view)
}

// HandleInboundTemplate GET /api/inbounds/template/:protocol 按协议返回默认连接层配置模板
// （VLESS 含预生成的 Reality 密钥材料、Shadowsocks 含随机 PSK）。新建弹框的默认值以此端点
// 为单一来源，前端不再自带一份默认配置。
func (a *API) HandleInboundTemplate(c *gin.Context) {
	protocol := c.Param("protocol")
	if !supportedProtocol(protocol) {
		Fail(c, http.StatusBadRequest, "不支持的入站协议: "+protocol)
		return
	}
	cfg, err := singbox.NewTemplateConfig(protocol)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "生成默认配置失败: "+err.Error())
		return
	}
	OK(c, gin.H{"config": cfg})
}

// HandleInboundGet GET /api/inbounds/:id 入站详情。
func (a *API) HandleInboundGet(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in db.Inbound
	if err := a.db.First(&in, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "入站不存在")
		return
	}
	count, err := a.inboundUserCount(in.ID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询用户数失败: "+err.Error())
		return
	}
	OK(c, a.inboundToView(in, count))
}

// HandleInboundUpdate PUT /api/inbounds/:id 更新入站（全量替换请求给出的字段）。
func (a *API) HandleInboundUpdate(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req inboundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	view, err := a.updateInbound(id, req)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	a.applyThenOK(c, view)
}

// HandleInboundSetEnabled PATCH /api/inbounds/:id/enabled 启停入站。
func (a *API) HandleInboundSetEnabled(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req enabledReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var in db.Inbound
	if err := a.db.First(&in, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "入站不存在")
		return
	}
	if err := a.db.Model(&in).Update("enabled", req.Enabled).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "更新入站状态失败: "+err.Error())
		return
	}
	in.Enabled = req.Enabled
	count, _ := a.inboundUserCount(in.ID)
	a.applyThenOK(c, a.inboundToView(in, count))
}

// HandleInboundDelete DELETE /api/inbounds/:id 删除入站（连同其用户与订阅绑定）。
func (a *API) HandleInboundDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.deleteInbound(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	a.applyThenOK(c, deletedResp{Deleted: id})
}

// applyInboundReq 把请求字段应用到入站：归一化连接层配置、校验协议/端口/名称唯一性与证书引用。
// applyInboundReq 把请求字段应用到入站；返回是否需要重建该入站下的 Shadowsocks 用户密钥
// （方法变更时密钥长度不符，必须在保存成功后整体重建）。所有校验通过后才写库。
func (a *API) applyInboundReq(in *db.Inbound, req inboundReq, id uint, origProtocol, origName string) (bool, error) {
	if req.Protocol != "" {
		in.Protocol = req.Protocol
	}
	if req.Name != "" {
		in.Name = req.Name
	}
	if req.Listen != "" {
		if !validListen(req.Listen) {
			return false, fmt.Errorf("监听地址不合法: %s", req.Listen)
		}
		in.Listen = req.Listen
	}
	if in.Listen == "" {
		in.Listen = singbox.DefaultListen
	}
	if req.Enabled != nil {
		in.Enabled = *req.Enabled
	}
	if in.Protocol == "" {
		return false, fmt.Errorf("协议不能为空")
	}
	if !supportedProtocol(in.Protocol) {
		return false, fmt.Errorf("不支持的入站协议: %s", in.Protocol)
	}
	if in.Name == "" {
		in.Name = a.uniqueInboundName(in.Protocol, id)
	}
	// 端口：显式给出则校验，否则分配随机空闲端口（项目不用 443 做默认值）
	if req.ListenPort != 0 {
		if req.ListenPort < 1 || req.ListenPort > 65535 {
			return false, fmt.Errorf("监听端口需在 1-65535 之间")
		}
		in.ListenPort = req.ListenPort
	} else if in.ListenPort == 0 {
		port, err := a.allocateInboundPort(in.Protocol, id)
		if err != nil {
			return false, err
		}
		in.ListenPort = port
	}
	rawCfg := string(req.Config)
	if rawCfg == "" && id != 0 {
		if in.Protocol != origProtocol {
			return false, fmt.Errorf("变更协议时需同时提供完整配置")
		}
		rawCfg = in.Config
	}
	cfg, err := singbox.ParseInboundConfig(in.Protocol, rawCfg)
	if err != nil {
		return false, err
	}
	// Reality 缺私钥会让内核拒绝启动：保存时补齐（经归一化落库，后续重建复用同一密钥）
	if err := singbox.CompleteRealityKeys(cfg); err != nil {
		return false, err
	}
	// Shadowsocks-2022 的入站 PSK 与各用户密钥长度都取决于方法：方法变更或首次配置时
	// 必须重新生成，否则内核会以 bad key 拒绝启动（多用户模式下 PSK 还要下发给客户端）。
	// 这里只做校验与内存内赋值；真正落库在全部校验通过之后（见 commitSSKeys），
	// 避免一次被拒绝的编辑把用户密钥改坏、进而让内核起不来。
	ssRegen := false
	if ss, ok := cfg.(*singbox.ShadowsocksConfig); ok {
		regen, err := a.prepareSSKeys(in.ID, ss, id != 0)
		if err != nil {
			return false, err
		}
		ssRegen = regen
	}
	if tls := singbox.InboundTLS(cfg); tls != nil && tls.CertMode == singbox.CertModeManaged && tls.CertID != 0 {
		var count int64
		a.db.Model(&db.Certificate{}).Where("id = ?", tls.CertID).Count(&count)
		if count == 0 {
			return false, fmt.Errorf("引用的证书不存在: %d", tls.CertID)
		}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return false, fmt.Errorf("归一化入站配置失败: %w", err)
	}
	in.Config = string(raw)

	var dup int64
	a.db.Model(&db.Inbound{}).Where("name = ? AND id <> ?", in.Name, id).Count(&dup)
	if dup > 0 {
		return false, fmt.Errorf("名称 %s 已存在", in.Name)
	}
	// 入站名与用户名共用流量表的键：改成某个已有用户的名字会让两者用量互相覆盖
	if in.Name != origName {
		var userClash int64
		a.db.Model(&db.InboundUser{}).Where("name = ?", in.Name).Count(&userClash)
		if userClash > 0 {
			return false, fmt.Errorf("名称 %s 与已有用户名冲突，请换一个", in.Name)
		}
	}
	a.db.Model(&db.Inbound{}).Where("listen_port = ? AND id <> ?", in.ListenPort, id).Count(&dup)
	if dup > 0 {
		return false, fmt.Errorf("端口 %d 已被其它入站占用", in.ListenPort)
	}
	return ssRegen, nil
}

// parseSSConfig 从入站连接层配置里取 Shadowsocks 方法（非 SS 协议返回 false）。
func parseSSConfig(raw string) (string, bool) {
	cfg, err := singbox.ParseInboundConfig(singbox.ProtoShadowsocks, raw)
	if err != nil {
		return "", false
	}
	ss, ok := cfg.(*singbox.ShadowsocksConfig)
	if !ok {
		return "", false
	}
	return ss.Method, true
}

// prepareSSKeys 校验 Shadowsocks-2022 方法是否支持多用户，并在需要时于内存中补出/更新 PSK，
// 返回是否需要重建用户密钥（真正落库由 commitSSKeys 在全部校验通过后执行）。
func (a *API) prepareSSKeys(inboundID uint, cfg *singbox.ShadowsocksConfig, existing bool) (bool, error) {
	method := cfg.Method
	if method == "" {
		method = "2022-blake3-aes-128-gcm"
	}
	if !singbox.SSMethodMultiUser(method) {
		return false, fmt.Errorf("Shadowsocks 多用户模式只支持 2022-blake3-aes-128-gcm 与 2022-blake3-aes-256-gcm")
	}
	cfg.Method = method
	want, _ := singbox.SSKeyLength(method)
	regenUsers := false
	switch {
	case !existing:
		// 新建：PSK 必须现生成（用户密钥由 createUser 按方法生成）
	case keyLen(cfg.PSK) != want:
		regenUsers = true // 方法变了（或历史数据长度不符），用户密钥要跟着重建
	}
	if cfg.PSK == "" || regenUsers {
		psk, err := singbox.NewSSKey(method)
		if err != nil {
			return false, err
		}
		cfg.PSK = psk
	}
	if regenUsers || !existing {
		return regenUsers, nil
	}
	// PSK 未变时也要确认现有用户密钥长度与方法一致（例如数据曾被改坏）
	var users []db.InboundUser
	if err := a.db.Where("inbound_id = ?", inboundID).Find(&users).Error; err != nil {
		return false, err
	}
	for _, u := range users {
		cred, err := singbox.ParseCredential(singbox.ProtoShadowsocks, u.Credential)
		if err != nil || keyLen(cred.Password) != want {
			return true, nil
		}
	}
	return false, nil
}

// commitSSKeys 在入站保存成功后重建该入站下全部用户的 Shadowsocks 密钥（方法变更场景）。
func (a *API) commitSSKeys(inboundID uint, method string) error {
	return a.db.Transaction(func(tx *gorm.DB) error {
		var users []db.InboundUser
		if err := tx.Where("inbound_id = ?", inboundID).Find(&users).Error; err != nil {
			return err
		}
		for _, u := range users {
			key, err := singbox.NewSSKey(method)
			if err != nil {
				return err
			}
			raw, err := json.Marshal(singbox.Credential{Password: key})
			if err != nil {
				return fmt.Errorf("序列化凭据失败: %w", err)
			}
			if err := tx.Model(&db.InboundUser{}).Where("id = ?", u.ID).
				Update("credential", string(raw)).Error; err != nil {
				return fmt.Errorf("更新用户 %s 密钥失败: %w", u.Name, err)
			}
		}
		return nil
	})
}

// keyLen 返回 base64 密钥解码后的字节数；解码失败返回 -1。
func keyLen(encoded string) int {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return -1
	}
	return len(raw)
}

// uniqueInboundName 生成未占用的入站名（协议-序号）。
func (a *API) uniqueInboundName(protocol string, id uint) string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("%s-%d", protocol, i)
		var count int64
		a.db.Model(&db.Inbound{}).Where("name = ? AND id <> ?", name, id).Count(&count)
		if count == 0 {
			return name
		}
	}
}

// allocateInboundPort 分配一个未被占用且系统层面可绑定的随机端口。
func (a *API) allocateInboundPort(protocol string, excludeID uint) (int, error) {
	quic := singbox.QUICProtocol(protocol)
	for range 200 {
		port := singbox.RandomPortMin + rand.IntN(singbox.RandomPortMax-singbox.RandomPortMin)
		var used int64
		a.db.Model(&db.Inbound{}).Where("listen_port = ? AND id <> ?", port, excludeID).Count(&used)
		if used > 0 {
			continue
		}
		if !portFree(port, quic) {
			continue
		}
		return port, nil
	}
	return 0, fmt.Errorf("未能在 %d-%d 范围内找到空闲端口", singbox.RandomPortMin, singbox.RandomPortMax)
}

// portFree 检查端口在 TCP（及 QUIC 协议的 UDP）上是否可绑定。
func portFree(port int, quic bool) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	if !quic {
		return true
	}
	pc, err := net.ListenPacket("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = pc.Close()
	return true
}

// inboundToView 把入站转为响应视图（解析配置计算弱配置警示）。
func (a *API) inboundToView(in db.Inbound, userCount int64) inboundView {
	cfg, err := singbox.ParseInboundConfig(in.Protocol, in.Config)
	view := inboundView{
		ID: in.ID, Name: in.Name, Protocol: in.Protocol, Listen: in.Listen,
		ListenPort: in.ListenPort, Enabled: in.Enabled,
		CreatedAt: in.CreatedAt, UpdatedAt: in.UpdatedAt,
		UserCount: userCount,
	}
	if err != nil { // 库内数据异常时原样返回配置，不阻断列表
		view.Config = json.RawMessage(in.Config)
		return view
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		view.Config = json.RawMessage(in.Config)
		return view
	}
	view.Config = raw
	view.Warnings = singbox.InboundWarnings(in.Protocol, cfg)
	return view
}

// supportedProtocol 判断是否为面板支持的入站协议。
func supportedProtocol(protocol string) bool {
	return slices.Contains(singbox.SupportedProtocols, protocol)
}

// validListen 校验监听地址（仅接受面板给出的三种语义地址）。
func validListen(listen string) bool {
	return slices.Contains([]string{singbox.ListenAll, singbox.ListenIPv4, singbox.ListenLoopback}, listen)
}

// ---------- 供 AI 工具复用的写操作 ----------

// createInbound 创建入站（与 REST handler 同一校验路径）。
func (a *API) createInbound(req inboundReq) (inboundView, error) {
	in := db.Inbound{Enabled: true}
	if _, err := a.applyInboundReq(&in, req, 0, "", ""); err != nil {
		return inboundView{}, err
	}
	if err := a.db.Create(&in).Error; err != nil {
		return inboundView{}, fmt.Errorf("创建入站失败: %w", err)
	}
	return a.inboundToView(in, 0), nil
}

// updateInbound 更新入站。
func (a *API) updateInbound(id uint, req inboundReq) (inboundView, error) {
	var in db.Inbound
	if err := a.db.First(&in, id).Error; err != nil {
		return inboundView{}, fmt.Errorf("入站不存在: %d", id)
	}
	regenSS, err := a.applyInboundReq(&in, req, id, in.Protocol, in.Name)
	if err != nil {
		return inboundView{}, err
	}
	if err := a.db.Save(&in).Error; err != nil {
		return inboundView{}, fmt.Errorf("保存入站失败: %w", err)
	}
	// 方法变更导致用户密钥长度不符：在保存成功后再整体重建，避免被拒绝的编辑改坏密钥
	if regenSS {
		if ss, ok := parseSSConfig(in.Config); ok {
			if err := a.commitSSKeys(in.ID, ss); err != nil {
				return inboundView{}, err
			}
		}
	}
	count, err := a.inboundUserCount(in.ID)
	if err != nil {
		return inboundView{}, fmt.Errorf("查询用户数失败: %w", err)
	}
	return a.inboundToView(in, count), nil
}

// deleteInbound 删除入站及其用户与订阅绑定。
func (a *API) deleteInbound(id uint) error {
	return a.db.Transaction(func(tx *gorm.DB) error {
		var users []db.InboundUser
		if err := tx.Where("inbound_id = ?", id).Find(&users).Error; err != nil {
			return err
		}
		ids := make([]uint, 0, len(users))
		for _, u := range users {
			ids = append(ids, u.ID)
		}
		if len(ids) > 0 {
			if err := tx.Where("inbound_user_id IN ?", ids).Delete(&db.SubscriptionNode{}).Error; err != nil {
				return err
			}
			if err := deleteUsersTraffic(tx, users); err != nil {
				return err
			}
		}
		if err := tx.Where("inbound_id = ?", id).Delete(&db.InboundUser{}).Error; err != nil {
			return err
		}
		return tx.Delete(&db.Inbound{}, id).Error
	})
}

// ---------- 入站 AI 工具 ----------

// inboundUserView 入站用户视图：与 db.InboundUser 字段一致但**不含 Credential**，
// 凭据明文（uuid/password）不得进入 AI 对话上下文。
type inboundUserView struct {
	ID        uint      `json:"id"`        // 主键
	InboundID uint      `json:"inboundId"` // 所属入站
	Name      string    `json:"name"`      // 用户名（客户端节点名）
	Suspended bool      `json:"suspended"` // 被系统暂停
	Enabled   bool      `json:"enabled"`   // 生效状态
	CreatedAt time.Time `json:"createdAt"` // 创建时间
	UpdatedAt time.Time `json:"updatedAt"` // 更新时间
}

// inboundUserViews 查询全部入站用户并转为脱敏视图（不含凭据）。
func (a *API) inboundUserViews() ([]inboundUserView, error) {
	var users []db.InboundUser
	if err := a.db.Order("inbound_id, id").Find(&users).Error; err != nil {
		return nil, err
	}
	views := make([]inboundUserView, 0, len(users))
	for _, u := range users {
		views = append(views, inboundUserView{
			ID: u.ID, InboundID: u.InboundID, Name: u.Name,
			Suspended: u.Suspended, Enabled: u.Enabled,
			CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
		})
	}
	return views, nil
}

// registerInboundAITools 注册入站相关的查询与写操作 AI 工具（写操作委托 createInbound 等共用方法）。
func (a *API) registerInboundAITools() {
	a.ai.Register(ai.Tool{
		Name:        "list_inbounds",
		Description: "列出面板全部入站（含协议、监听地址与端口、启用状态、连接层配置、用户数与弱配置警示）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.inboundViews()
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "list_inbound_users",
		Description: "列出全部入站用户（订阅用户勾选入站生成的认证身份，含所属入站、用户名、启用/暂停状态）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.inboundUserViews()
		},
	})
	a.registerModify(ai.Tool{
		Name: "create_inbound",
		Description: "创建入站（真实监听）。入站只配连接层参数（TLS/证书/Reality/ECH/混淆/masquerade/带宽/拥塞算法），" +
			"用户凭据在入站内单独创建；listenPort 省略时自动分配随机空闲端口；保存后内核立即按新配置重建",
		Parameters: toolParams(map[string]any{
			"protocol": toolProp("string", "入站协议", singbox.SupportedProtocols...),
			"name":     toolProp("string", "入站名称，缺省按 协议-序号 生成"),
			"listen":   toolProp("string", "监听地址：:: 双栈 / 0.0.0.0 仅 IPv4 / 127.0.0.1 走反代，缺省 ::"),
			"config": map[string]any{"type": "object", "description": "协议连接层配置对象，缺省按协议填默认值。" +
				"通用：tls{enabled,serverName,alpn,minVersion,maxVersion,ech{enabled,key,keyPath},certMode,certId,certPath,keyPath}、" +
				"tcpFastOpen、multiplex{enabled,padding}（VLESS/Trojan/VMess/Shadowsocks）；" +
				"VLESS Reality：reality{enabled,handshakeServer,handshakePort}，密钥材料留空由面板自动生成、不要编造；" +
				"Hysteria2：obfsEnabled/obfsPassword、upMbps/downMbps、bbrProfile、masquerade{type,directory,proxyUrl,statusCode,content}；" +
				"TUIC：congestionControl（默认 bbr）、zeroRttHandshake、heartbeat；Shadowsocks：network（空为 TCP+UDP）"},
			"enabled": toolProp("boolean", "是否启用，缺省 true"),
		}, "protocol"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req inboundReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			dropEmptyConfig(&req)
			return a.createInbound(req)
		},
	})
	a.registerModify(ai.Tool{
		Name:        "update_inbound",
		Description: "按 ID 更新入站；仅更新给出的字段，config 为完整协议配置对象（变更协议时必须提供）",
		Parameters: toolParams(map[string]any{
			"id":      toolProp("integer", "入站 ID"),
			"name":    toolProp("string", "名称"),
			"listen":  toolProp("string", "监听地址"),
			"config":  map[string]any{"type": "object", "description": "完整协议配置对象"},
			"enabled": toolProp("boolean", "是否启用"),
		}, "id"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req inboundReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			dropEmptyConfig(&req)
			return a.updateInbound(arg.ID, req)
		},
	})
	a.registerModify(ai.Tool{
		Name:         "delete_inbound",
		Description:  "按 ID 删除入站（连同其下全部用户与累计流量）",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "入站 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if err := a.deleteInbound(arg.ID); err != nil {
				return nil, err
			}
			return deletedResp{Deleted: arg.ID}, nil
		},
	})
}
