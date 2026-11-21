package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/ai"
	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// groupView 策略组响应视图：members 以字符串数组返回。
type groupView struct {
	ID        uint      `json:"id"`        // 主键
	Name      string    `json:"name"`      // 策略组 tag
	Mode      string    `json:"mode"`      // 模式：failover/fixed/auto
	Members   []string  `json:"members"`   // 成员出站 tag 列表
	Enabled   bool      `json:"enabled"`   // 是否启用
	CreatedAt time.Time `json:"createdAt"` // 创建时间
	UpdatedAt time.Time `json:"updatedAt"` // 更新时间
}

// groupReq 创建/更新策略组请求。
type groupReq struct {
	Name    string   `json:"name"`    // 策略组 tag
	Mode    string   `json:"mode"`    // 模式
	Members []string `json:"members"` // 成员出站 tag 列表
	Enabled *bool    `json:"enabled"` // 是否启用，缺省 true
}

// HandleGroupsList GET /api/policy-groups 策略组列表。
func (a *API) HandleGroupsList(c *gin.Context) {
	views, err := a.groupViews()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询策略组列表失败: "+err.Error())
		return
	}
	OK(c, views)
}

// groupViews 查询全部策略组并转为响应视图。
func (a *API) groupViews() ([]groupView, error) {
	var records []db.PolicyGroup
	if err := a.db.Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	views := make([]groupView, 0, len(records))
	for _, g := range records {
		views = append(views, groupToView(g))
	}
	return views, nil
}

// HandleGroupCreate POST /api/policy-groups 创建策略组。
func (a *API) HandleGroupCreate(c *gin.Context) {
	var req groupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	view, err := a.createGroup(req)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	a.applyThenOK(c, view)
}

// HandleGroupUpdate PUT /api/policy-groups/:id 更新策略组。
func (a *API) HandleGroupUpdate(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req groupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	view, err := a.updateGroup(id, req)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	a.applyThenOK(c, view)
}

// HandleGroupSetEnabled PATCH /api/policy-groups/:id/enabled 启停策略组。
func (a *API) HandleGroupSetEnabled(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req enabledReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var group db.PolicyGroup
	if err := a.db.First(&group, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "策略组不存在")
		return
	}
	if err := a.db.Model(&group).Update("enabled", req.Enabled).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "更新策略组状态失败: "+err.Error())
		return
	}
	group.Enabled = req.Enabled
	a.applyThenOK(c, groupToView(group))
}

// HandleGroupDelete DELETE /api/policy-groups/:id 删除策略组。
func (a *API) HandleGroupDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.deleteGroup(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	a.applyThenOK(c, deletedResp{Deleted: id})
}

// createGroup 创建策略组（生成 sing-box selector/urltest 出站）。
func (a *API) createGroup(req groupReq) (groupView, error) {
	group := db.PolicyGroup{Enabled: true}
	if err := a.applyGroupReq(&group, req, 0); err != nil {
		return groupView{}, err
	}
	if err := a.db.Create(&group).Error; err != nil {
		return groupView{}, fmt.Errorf("创建策略组失败: %w", err)
	}
	return groupToView(group), nil
}

// updateGroup 按 ID 更新策略组；仅更新给出的字段。
func (a *API) updateGroup(id uint, req groupReq) (groupView, error) {
	var group db.PolicyGroup
	if err := a.db.First(&group, id).Error; err != nil {
		return groupView{}, fmt.Errorf("策略组不存在: %d", id)
	}
	if err := a.applyGroupReq(&group, req, id); err != nil {
		return groupView{}, err
	}
	if err := a.db.Save(&group).Error; err != nil {
		return groupView{}, fmt.Errorf("保存策略组失败: %w", err)
	}
	return groupToView(group), nil
}

// deleteGroup 按 ID 删除策略组。
func (a *API) deleteGroup(id uint) error {
	if err := a.db.Delete(&db.PolicyGroup{}, id).Error; err != nil {
		return fmt.Errorf("删除策略组失败: %w", err)
	}
	return nil
}

// applyGroupReq 把请求字段应用到策略组记录并校验模式与名称唯一性（成员存在性在聚合内核配置时校验）。
func (a *API) applyGroupReq(group *db.PolicyGroup, req groupReq, id uint) error {
	if req.Name != "" {
		group.Name = req.Name
	}
	if req.Mode != "" {
		group.Mode = req.Mode
	}
	if req.Members != nil {
		group.Members = marshalMembers(req.Members)
	}
	if req.Enabled != nil {
		group.Enabled = *req.Enabled
	}
	if group.Name == "" {
		return fmt.Errorf("名称不能为空")
	}
	if !slices.Contains(singbox.SupportedGroupModes, group.Mode) {
		return fmt.Errorf("不支持的模式: %s（可选 failover/fixed/auto）", group.Mode)
	}
	if group.Members == "" {
		group.Members = "[]"
	}
	var dup int64
	a.db.Model(&db.PolicyGroup{}).Where("name = ? AND id <> ?", group.Name, id).Count(&dup)
	if dup > 0 {
		return fmt.Errorf("名称 %s 已存在", group.Name)
	}
	var obDup int64
	a.db.Model(&db.Outbound{}).Where("name = ?", group.Name).Count(&obDup)
	if obDup > 0 {
		return fmt.Errorf("名称 %s 与出站同名", group.Name)
	}
	return nil
}

// groupToView 把策略组记录转为响应视图。
func groupToView(g db.PolicyGroup) groupView {
	members := []string{}
	if g.Members != "" {
		_ = json.Unmarshal([]byte(g.Members), &members)
	}
	return groupView{
		ID: g.ID, Name: g.Name, Mode: g.Mode, Members: members,
		Enabled: g.Enabled, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt,
	}
}

// ---------- 策略组 AI 工具 ----------

// registerGroupAITools 注册策略组相关的查询与写操作 AI 工具。
func (a *API) registerGroupAITools() {
	a.ai.Register(ai.Tool{
		Name:        "list_policy_groups",
		Description: "列出全部策略组（含模式 failover/fixed/auto 与成员出站列表）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.groupViews()
		},
	})
	a.registerModify(ai.Tool{
		Name:        "create_policy_group",
		Description: "创建策略组（生成 sing-box selector/urltest 出站）；members 为成员出站 tag 按顺序生效",
		Parameters: toolParams(map[string]any{
			"name":    toolProp("string", "策略组名称（供规则与兜底出站引用）"),
			"mode":    toolProp("string", "模式：failover 故障转移 / fixed 固定代理 / auto 自动选择", singbox.SupportedGroupModes...),
			"members": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "成员出站 tag 列表"},
		}, "name", "mode"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req groupReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			return a.createGroup(req)
		},
	})
	a.registerModify(ai.Tool{
		Name:        "update_policy_group",
		Description: "按 ID 更新策略组；仅更新给出的字段",
		Parameters: toolParams(map[string]any{
			"id":      toolProp("integer", "策略组 ID"),
			"name":    toolProp("string", "策略组名称"),
			"mode":    toolProp("string", "模式：failover/fixed/auto", singbox.SupportedGroupModes...),
			"members": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "成员出站 tag 列表"},
			"enabled": toolProp("boolean", "是否启用"),
		}, "id"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req groupReq
			var arg idArg
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			return a.updateGroup(arg.ID, req)
		},
	})
	a.registerModify(ai.Tool{
		Name:         "delete_policy_group",
		Description:  "按 ID 删除策略组",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "策略组 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if err := a.deleteGroup(arg.ID); err != nil {
				return nil, err
			}
			return deletedResp{Deleted: arg.ID}, nil
		},
	})
}
