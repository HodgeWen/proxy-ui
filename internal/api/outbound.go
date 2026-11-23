package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/ai"
	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
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
	view, err := a.createOutbound(req)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	a.applyThenOK(c, view)
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
	view, err := a.updateOutbound(id, req)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	a.applyThenOK(c, view)
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
	// 停用校验在 applyOutboundReq 内（REST 与 AI 工具共用同一口径）
	if err := a.applyOutboundReq(a.db, &ob, outboundReq{Enabled: &req.Enabled}, id); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.db.Model(&ob).Update("enabled", req.Enabled).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "更新出站状态失败: "+err.Error())
		return
	}
	ob.Enabled = req.Enabled
	a.applyThenOK(c, outboundToView(ob))
}

// HandleOutboundDelete DELETE /api/outbounds/:id 删除出站。
func (a *API) HandleOutboundDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.deleteOutbound(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	a.applyThenOK(c, deletedResp{Deleted: id})
}

// ---------- 供 REST handler 与 AI 工具共用的写操作 ----------

// createOutbound 创建出站（direct 或自定义）。
func (a *API) createOutbound(req outboundReq) (outboundView, error) {
	var view outboundView
	err := a.db.Transaction(func(tx *gorm.DB) error {
		ob := db.Outbound{Enabled: true}
		if err := a.applyOutboundReq(tx, &ob, req, 0); err != nil {
			return err
		}
		if err := tx.Create(&ob).Error; err != nil {
			return fmt.Errorf("创建出站失败: %w", err)
		}
		view = outboundToView(ob)
		return nil
	})
	return view, err
}

// updateOutbound 按 ID 更新出站；仅更新给出的字段。
//
// 改名要级联改掉规则、策略组成员与兜底出站里的引用（与策略组同一口径），否则内核配置
// 聚合会因找不到出站而失败——级联在 applyOutboundReq 内完成。
func (a *API) updateOutbound(id uint, req outboundReq) (outboundView, error) {
	var view outboundView
	err := a.db.Transaction(func(tx *gorm.DB) error {
		var ob db.Outbound
		if err := tx.First(&ob, id).Error; err != nil {
			return fmt.Errorf("出站不存在: %d", id)
		}
		if err := a.applyOutboundReq(tx, &ob, req, id); err != nil {
			return err
		}
		if err := tx.Save(&ob).Error; err != nil {
			return fmt.Errorf("保存出站失败: %w", err)
		}
		view = outboundToView(ob)
		return nil
	})
	return view, err
}

// deleteOutbound 按 ID 删除出站（仍被规则 / 策略组 / 兜底出站引用时拒绝）。
func (a *API) deleteOutbound(id uint) error {
	var ob db.Outbound
	if err := a.db.First(&ob, id).Error; err != nil {
		return fmt.Errorf("出站不存在: %d", id)
	}
	refs, err := a.tagRefCount(a.db, ob.Name)
	if err != nil {
		return fmt.Errorf("查询出站引用失败: %w", err)
	}
	if refs > 0 {
		return fmt.Errorf("出站 %s 仍被 %d 处引用（路由规则 / 策略组成员 / 兜底出站 / DNS detour），请先改为其它出站或策略组", ob.Name, refs)
	}
	if err := a.db.Delete(&db.Outbound{}, id).Error; err != nil {
		return fmt.Errorf("删除出站失败: %w", err)
	}
	return nil
}

// applyOutboundReq 把请求字段应用到出站记录并校验（direct 出站无附加配置）。
func (a *API) applyOutboundReq(tx *gorm.DB, ob *db.Outbound, req outboundReq, id uint) error {
	oldName := ob.Name
	wasEnabled := ob.Enabled
	if req.Type != "" {
		ob.Type = req.Type
	}
	if req.Name != "" {
		ob.Name = strings.TrimSpace(req.Name)
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
	// block 是规则动作而非出站，占用后无法拦截；direct 允许作为 direct 类型出站的名字
	// （面板本来就有一个名为 direct 的直连出站），但不允许自定义类型的出站冒名顶替它。
	if ob.Name == singbox.OutboundBlock {
		return fmt.Errorf("名称 block 为内置拦截动作保留字，请换一个名称")
	}
	if ob.Name == db.OutboundTypeDirect && ob.Type != db.OutboundTypeDirect {
		return fmt.Errorf("名称 direct 只属于内置直连出站，请换一个名称")
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
	tx.Model(&db.Outbound{}).Where("name = ? AND id <> ?", ob.Name, id).Count(&dup)
	if dup > 0 {
		return fmt.Errorf("名称 %s 已存在", ob.Name)
	}
	var groupDup int64
	tx.Model(&db.PolicyGroup{}).Where("name = ?", ob.Name).Count(&groupDup)
	if groupDup > 0 {
		return fmt.Errorf("名称 %s 与策略组同名", ob.Name)
	}
	// direct 是内核内置出站 tag，规则/兜底/成员里的 direct 指向的是内置直连语义而不是
	// 这条记录：把它改名会静默改写所有「直连」规则（加上类型变更就变成走代理了）。
	if oldName == db.OutboundTypeDirect && ob.Name != db.OutboundTypeDirect {
		return fmt.Errorf("内置直连出站不能改名（规则与兜底里的 direct 指的是直连语义），如需自定义出口请新建出站")
	}
	// 停用一个仍被引用的出站会让规则/策略组成员/兜底/DNS detour 悬空：校验放在这里，
	// REST 与 AI 工具的全部写路径都走同一个口径
	if wasEnabled && !ob.Enabled && id != 0 {
		refs, err := a.tagRefCount(tx, oldName)
		if err != nil {
			return fmt.Errorf("查询出站引用失败: %w", err)
		}
		if refs > 0 {
			return fmt.Errorf("出站 %s 仍被 %d 处引用（路由规则 / 策略组成员 / 兜底出站 / DNS detour），停用会让这些引用失效，请先改为其它出站或策略组", oldName, refs)
		}
	}
	// 先按新名字重写引用，改名后配置里才不会有悬空 tag
	if ob.Name != oldName && id != 0 {
		if err := a.renameTagRefs(tx, oldName, ob.Name); err != nil {
			return fmt.Errorf("更新引用「%s」失败: %w", oldName, err)
		}
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

// ---------- 出站 AI 工具 ----------

// registerOutboundAITools 注册出站相关的查询与写操作 AI 工具。
func (a *API) registerOutboundAITools() {
	a.ai.Register(ai.Tool{
		Name:        "list_outbounds",
		Description: "列出面板全部出站（direct 与自定义出站，含类型与配置）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.outboundViews()
		},
	})
	a.registerModify(ai.Tool{
		Name: "create_outbound",
		Description: "创建出站（type 为 sing-box 出站类型，如 direct、socks、http、shadowsocks、vmess、trojan、vless、hysteria2、wireguard 等）；" +
			"config 为该类型出站的字段对象（如 server、server_port、密码等）",
		Parameters: toolParams(map[string]any{
			"type": toolProp("string", "出站类型"),
			"name": toolProp("string", "出站 tag，direct 类型缺省 direct"),
			"config": map[string]any{
				"type": "object", "description": "出站字段对象（server/server_port/密码等），direct 无需提供",
			},
		}, "type"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req outboundReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			return a.createOutbound(req)
		},
	})
	a.registerModify(ai.Tool{
		Name:        "update_outbound",
		Description: "按 ID 更新出站；仅更新给出的字段",
		Parameters: toolParams(map[string]any{
			"id":      toolProp("integer", "出站 ID"),
			"name":    toolProp("string", "出站 tag"),
			"type":    toolProp("string", "出站类型"),
			"config":  map[string]any{"type": "object", "description": "出站字段对象"},
			"enabled": toolProp("boolean", "是否启用"),
		}, "id"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req outboundReq
			var arg idArg
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			return a.updateOutbound(arg.ID, req)
		},
	})
	a.registerModify(ai.Tool{
		Name:         "delete_outbound",
		Description:  "按 ID 删除出站",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "出站 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if err := a.deleteOutbound(arg.ID); err != nil {
				return nil, err
			}
			return deletedResp{Deleted: arg.ID}, nil
		},
	})
}
