package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
)

// outboundView 出站响应视图：config 以嵌套对象返回。
type outboundView struct {
	ID        uint            `json:"id"`        // 主键
	Name      string          `json:"name"`      // 出站 tag
	Type      string          `json:"type"`      // 出站类型：direct 或自定义出站协议
	Enabled   bool            `json:"enabled"`   // 是否启用
	Config    json.RawMessage `json:"config"`    // 出站配置（对象）
	CreatedAt time.Time       `json:"createdAt"` // 创建时间
	UpdatedAt time.Time       `json:"updatedAt"` // 更新时间
}

// outboundReq 创建/更新出站请求；Config 为 type/tag 之外的出站字段 JSON 对象。
type outboundReq struct {
	Name    string          `json:"name"`    // 出站 tag，direct 类型缺省 direct
	Type    string          `json:"type"`    // 出站类型
	Enabled *bool           `json:"enabled"` // 是否启用，缺省 true
	Config  json.RawMessage `json:"config"`  // 出站配置 JSON 对象
}

// HandleOutboundsList GET /api/outbounds 出站列表。
func (a *API) HandleOutboundsList(c *gin.Context) {
	views, err := a.outboundViews()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询出站列表失败: "+err.Error())
		return
	}
	OK(c, views)
}

// outboundViews 查询全部出站并转为响应视图。
func (a *API) outboundViews() ([]outboundView, error) {
	var records []db.Outbound
	if err := a.db.Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	views := make([]outboundView, 0, len(records))
	for _, ob := range records {
		views = append(views, outboundToView(ob))
	}
	return views, nil
}

// HandleOutboundCreate POST /api/outbounds 创建出站（direct 或自定义）。
func (a *API) HandleOutboundCreate(c *gin.Context) {
	var req outboundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	ob := db.Outbound{Enabled: true}
	if err := a.applyOutboundReq(&ob, req, 0); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.db.Create(&ob).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "创建出站失败: "+err.Error())
		return
	}
	OK(c, outboundToView(ob))
}

// HandleOutboundGet GET /api/outbounds/:id 出站详情。
func (a *API) HandleOutboundGet(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var ob db.Outbound
	if err := a.db.First(&ob, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "出站不存在")
		return
	}
	OK(c, outboundToView(ob))
}

// HandleOutboundUpdate PUT /api/outbounds/:id 更新出站。
func (a *API) HandleOutboundUpdate(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req outboundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var ob db.Outbound
	if err := a.db.First(&ob, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "出站不存在")
		return
	}
	if err := a.applyOutboundReq(&ob, req, id); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.db.Save(&ob).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "保存出站失败: "+err.Error())
		return
	}
	OK(c, outboundToView(ob))
}

// HandleOutboundSetEnabled PATCH /api/outbounds/:id/enabled 启停出站。
func (a *API) HandleOutboundSetEnabled(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req enabledReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var ob db.Outbound
	if err := a.db.First(&ob, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "出站不存在")
		return
	}
	if err := a.db.Model(&ob).Update("enabled", req.Enabled).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "更新出站状态失败: "+err.Error())
		return
	}
	ob.Enabled = req.Enabled
	OK(c, outboundToView(ob))
}

// HandleOutboundDelete DELETE /api/outbounds/:id 删除出站。
func (a *API) HandleOutboundDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.db.Delete(&db.Outbound{}, id).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "删除出站失败: "+err.Error())
		return
	}
	OK(c, gin.H{"deleted": id})
}

// applyOutboundReq 把请求字段应用到出站记录并校验（direct 出站无附加配置）。
func (a *API) applyOutboundReq(ob *db.Outbound, req outboundReq, id uint) error {
	if req.Type != "" {
		ob.Type = req.Type
	}
	if req.Name != "" {
		ob.Name = req.Name
	}
	if req.Enabled != nil {
		ob.Enabled = *req.Enabled
	}
	if ob.Type == "" {
		return fmt.Errorf("出站类型不能为空")
	}
	if ob.Type == "selector" || ob.Type == "urltest" {
		return fmt.Errorf("类型 %s 保留给策略组，请使用规则页配置", ob.Type)
	}
	if ob.Name == "" {
		if ob.Type != db.OutboundTypeDirect {
			return fmt.Errorf("名称不能为空")
		}
		ob.Name = db.OutboundTypeDirect
	}
	if ob.Type == db.OutboundTypeDirect {
		ob.Config = "{}"
	} else {
		if len(req.Config) > 0 {
			var fields map[string]any
			if err := json.Unmarshal(req.Config, &fields); err != nil {
				return fmt.Errorf("出站配置须为 JSON 对象: %w", err)
			}
			ob.Config = string(req.Config)
		} else if ob.Config == "" {
			ob.Config = "{}"
		}
	}
	var dup int64
	a.db.Model(&db.Outbound{}).Where("name = ? AND id <> ?", ob.Name, id).Count(&dup)
	if dup > 0 {
		return fmt.Errorf("名称 %s 已存在", ob.Name)
	}
	return nil
}

// outboundToView 把出站记录转为响应视图。
func outboundToView(ob db.Outbound) outboundView {
	return outboundView{
		ID: ob.ID, Name: ob.Name, Type: ob.Type, Enabled: ob.Enabled,
		Config: json.RawMessage(ob.Config), CreatedAt: ob.CreatedAt, UpdatedAt: ob.UpdatedAt,
	}
}
