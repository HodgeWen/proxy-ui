package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// inboundView 入站响应视图：config 以嵌套对象返回并附弱配置警示。
type inboundView struct {
	ID         uint            `json:"id"`         // 主键
	Name       string          `json:"name"`       // 名称（即内核入站 tag）
	Protocol   string          `json:"protocol"`   // 协议
	ListenPort int             `json:"listenPort"` // 监听端口
	Enabled    bool            `json:"enabled"`    // 是否启用
	Config     json.RawMessage `json:"config"`     // 协议配置（对象）
	Warnings   []string        `json:"warnings"`   // 弱配置警示
	CreatedAt  time.Time       `json:"createdAt"`  // 创建时间
	UpdatedAt  time.Time       `json:"updatedAt"`  // 更新时间
}

// inboundReq 创建/更新入站请求；Config 为空时按协议填充默认值。
type inboundReq struct {
	Name       string          `json:"name"`       // 名称，空时按 协议-端口 生成
	Protocol   string          `json:"protocol"`   // 协议
	ListenPort int             `json:"listenPort"` // 监听端口
	Enabled    *bool           `json:"enabled"`    // 是否启用，缺省 true
	Config     json.RawMessage `json:"config"`     // 协议配置 JSON
}

// HandleInboundsList GET /api/inbounds 入站列表。
func (a *API) HandleInboundsList(c *gin.Context) {
	var records []db.Inbound
	if err := a.db.Order("id").Find(&records).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "查询入站列表失败: "+err.Error())
		return
	}
	views := make([]inboundView, 0, len(records))
	for _, in := range records {
		views = append(views, a.inboundToView(in))
	}
	OK(c, views)
}

// HandleInboundCreate POST /api/inbounds 创建入站（空配置按协议填默认值）。
func (a *API) HandleInboundCreate(c *gin.Context) {
	var req inboundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	in := db.Inbound{Enabled: true}
	if err := a.applyInboundReq(&in, req, 0, ""); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.db.Create(&in).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "创建入站失败: "+err.Error())
		return
	}
	OK(c, a.inboundToView(in))
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
	OK(c, a.inboundToView(in))
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
	var in db.Inbound
	if err := a.db.First(&in, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "入站不存在")
		return
	}
	origProtocol := in.Protocol
	if err := a.applyInboundReq(&in, req, id, origProtocol); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.db.Save(&in).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "保存入站失败: "+err.Error())
		return
	}
	OK(c, a.inboundToView(in))
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
	OK(c, a.inboundToView(in))
}

// HandleInboundDelete DELETE /api/inbounds/:id 删除入站。
func (a *API) HandleInboundDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.db.Delete(&db.Inbound{}, id).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "删除入站失败: "+err.Error())
		return
	}
	OK(c, gin.H{"deleted": id})
}

// applyInboundReq 把请求字段应用到入站记录：归一化协议配置（创建时空配置填默认值，
// 更新时未携带配置沿用现有配置）、校验协议/端口/名称唯一性与证书引用存在性。
func (a *API) applyInboundReq(in *db.Inbound, req inboundReq, id uint, origProtocol string) error {
	if req.Protocol != "" {
		in.Protocol = req.Protocol
	}
	if req.Name != "" {
		in.Name = req.Name
	}
	if req.ListenPort != 0 {
		in.ListenPort = req.ListenPort
	}
	if req.Enabled != nil {
		in.Enabled = *req.Enabled
	}
	if in.Protocol == "" {
		return fmt.Errorf("协议不能为空")
	}
	if !supportedProtocol(in.Protocol) {
		return fmt.Errorf("不支持的入站协议: %s", in.Protocol)
	}
	if in.Name == "" {
		in.Name = fmt.Sprintf("%s-%d", in.Protocol, in.ListenPort)
	}
	if in.ListenPort < 1 || in.ListenPort > 65535 {
		return fmt.Errorf("监听端口需在 1-65535 之间")
	}
	rawCfg := string(req.Config)
	if rawCfg == "" && id != 0 {
		if in.Protocol != origProtocol {
			return fmt.Errorf("变更协议时需同时提供完整配置")
		}
		rawCfg = in.Config
	}
	cfg, err := singbox.ParseInboundConfig(in.Protocol, rawCfg)
	if err != nil {
		return err
	}
	if tls := singbox.InboundTLS(cfg); tls != nil && tls.CertMode == singbox.CertModeManaged && tls.CertID != 0 {
		var count int64
		a.db.Model(&db.Certificate{}).Where("id = ?", tls.CertID).Count(&count)
		if count == 0 {
			return fmt.Errorf("引用的证书不存在: %d", tls.CertID)
		}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("归一化入站配置失败: %w", err)
	}
	in.Config = string(raw)

	var dup int64
	a.db.Model(&db.Inbound{}).Where("name = ? AND id <> ?", in.Name, id).Count(&dup)
	if dup > 0 {
		return fmt.Errorf("名称 %s 已存在", in.Name)
	}
	a.db.Model(&db.Inbound{}).Where("listen_port = ? AND id <> ?", in.ListenPort, id).Count(&dup)
	if dup > 0 {
		return fmt.Errorf("端口 %d 已被其它入站占用", in.ListenPort)
	}
	return nil
}

// inboundToView 把入站记录转为响应视图（解析配置计算弱配置警示）。
func (a *API) inboundToView(in db.Inbound) inboundView {
	cfg, err := singbox.ParseInboundConfig(in.Protocol, in.Config)
	view := inboundView{
		ID: in.ID, Name: in.Name, Protocol: in.Protocol, ListenPort: in.ListenPort,
		Enabled: in.Enabled, CreatedAt: in.CreatedAt, UpdatedAt: in.UpdatedAt,
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
	for _, p := range singbox.SupportedProtocols {
		if p == protocol {
			return true
		}
	}
	return false
}
