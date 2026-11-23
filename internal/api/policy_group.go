package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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
	// 停用校验在 applyGroupReq 内（REST 与 AI 工具共用同一口径）
	if err := a.applyGroupReq(a.db, &group, groupReq{Enabled: &req.Enabled}, id); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
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
	var view groupView
	err := a.db.Transaction(func(tx *gorm.DB) error {
		group := db.PolicyGroup{Enabled: true}
		if err := a.applyGroupReq(tx, &group, req, 0); err != nil {
			return err
		}
		if err := tx.Create(&group).Error; err != nil {
			return fmt.Errorf("创建策略组失败: %w", err)
		}
		view = groupToView(group)
		return nil
	})
	return view, err
}

// updateGroup 按 ID 更新策略组；仅更新给出的字段。
//
// 改名要级联改掉所有引用（规则出站、其它组成员、兜底出站），否则内核配置聚合会因找不到
// 出站而失败，面板从此每次保存都报错并停在回滚配置上——级联在 applyGroupReq 内完成。
func (a *API) updateGroup(id uint, req groupReq) (groupView, error) {
	var view groupView
	err := a.db.Transaction(func(tx *gorm.DB) error {
		var group db.PolicyGroup
		if err := tx.First(&group, id).Error; err != nil {
			return fmt.Errorf("策略组不存在: %d", id)
		}
		if err := a.applyGroupReq(tx, &group, req, id); err != nil {
			return err
		}
		if err := tx.Save(&group).Error; err != nil {
			return fmt.Errorf("保存策略组失败: %w", err)
		}
		view = groupToView(group)
		return nil
	})
	return view, err
}

// deleteGroup 按 ID 删除策略组。
//
// 仍被规则、其它策略组或兜底出站引用时拒绝删除：硬删会留下悬空 tag，让内核配置聚合
// 永久失败。用户需先把引用改到别处，错误信息里给出具体处数。自身成员不算引用（那是
// 自引用坏数据，删掉正好是修复途径）。
func (a *API) deleteGroup(id uint) error {
	var group db.PolicyGroup
	if err := a.db.First(&group, id).Error; err != nil {
		return fmt.Errorf("策略组不存在: %d", id)
	}
	refs, err := a.selfExcludedRefCount(a.db, group.ID, group.Name)
	if err != nil {
		return fmt.Errorf("查询策略组引用失败: %w", err)
	}
	if refs > 0 {
		return fmt.Errorf("策略组 %s 仍被 %d 处引用（路由规则 / 其它策略组 / 兜底出站 / DNS detour），请先改为其它出站或策略组", group.Name, refs)
	}
	if err := a.db.Delete(&db.PolicyGroup{}, id).Error; err != nil {
		return fmt.Errorf("删除策略组失败: %w", err)
	}
	return nil
}

// selfExcludedRefCount 与 tagRefCount 同口径，但跳过指定策略组自身的成员（自引用）。
func (a *API) selfExcludedRefCount(tx *gorm.DB, selfID uint, name string) (int, error) {
	var ruleCount int64
	if err := tx.Model(&db.Rule{}).Where("outbound = ? AND enabled = ?", name, true).
		Count(&ruleCount).Error; err != nil {
		return 0, err
	}
	var groups []db.PolicyGroup
	if err := tx.Where("enabled = ? AND id <> ?", true, selfID).Find(&groups).Error; err != nil {
		return 0, err
	}
	total := int(ruleCount)
	for _, g := range groups {
		for _, m := range groupEntryOf(g).Members {
			if m == name {
				total++
			}
		}
	}
	var final db.Setting
	if err := tx.First(&final, "key = ?", db.SettingRouteFinal).Error; err == nil && final.Value == name {
		total++
	}
	detours, err := a.dnsDetours(tx)
	if err != nil {
		return 0, err
	}
	for _, d := range detours {
		if d == name {
			total++
		}
	}
	return total, nil
}

// applyGroupReq 把请求字段应用到策略组记录并校验模式、名称唯一性与成员有效性。
//
// 成员在聚合内核配置时也会校验，但那时整份配置已无法生成——把校验前移到保存处，
// 用户能直接看到是哪一组的哪个成员不存在，而不是一条「内核重建失败」。
func (a *API) applyGroupReq(tx *gorm.DB, group *db.PolicyGroup, req groupReq, id uint) error {
	oldName := group.Name
	wasEnabled := group.Enabled
	if req.Name != "" {
		group.Name = strings.TrimSpace(req.Name)
	}
	if req.Mode != "" {
		group.Mode = req.Mode
	}
	if req.Members != nil {
		group.Members = marshalMembers(dedupeMembers(req.Members))
	}
	if req.Enabled != nil {
		group.Enabled = *req.Enabled
	}
	if group.Name == "" {
		return fmt.Errorf("名称不能为空")
	}
	// direct / block 是内核内置语义，占用后配置里会出现同 tag 的两个出站或无法拦截
	if group.Name == db.OutboundTypeDirect || group.Name == singbox.OutboundBlock {
		return fmt.Errorf("名称 %s 为内置动作保留字，请换一个名称", group.Name)
	}
	if !slices.Contains(singbox.SupportedGroupModes, group.Mode) {
		return fmt.Errorf("不支持的模式: %s（可选 failover/fixed/auto）", group.Mode)
	}
	if group.Members == "" {
		group.Members = "[]"
	}
	var dup int64
	tx.Model(&db.PolicyGroup{}).Where("name = ? AND id <> ?", group.Name, id).Count(&dup)
	if dup > 0 {
		return fmt.Errorf("名称 %s 已存在", group.Name)
	}
	var obDup int64
	tx.Model(&db.Outbound{}).Where("name = ?", group.Name).Count(&obDup)
	if obDup > 0 {
		return fmt.Errorf("名称 %s 与出站同名", group.Name)
	}
	// 停用一个仍被引用的组会让规则/兜底出站悬空：校验放在这里，REST 与 AI 工具的
	// 全部写路径都走同一个口径（此前只挡住 PATCH 启停接口）
	if wasEnabled && !group.Enabled && id != 0 {
		refs, err := a.tagRefCount(tx, oldName)
		if err != nil {
			return fmt.Errorf("查询策略组引用失败: %w", err)
		}
		if refs > 0 {
			return fmt.Errorf("策略组 %s 仍被 %d 处引用（路由规则 / 其它策略组 / 兜底出站 / DNS detour），停用会让这些引用失效，请先改为其它出站或策略组", oldName, refs)
		}
	}
	// 先按改名后的名字重写其它组的引用再校验：否则改名与改成员同一次请求时，
	// 校验看到的是其它组的旧引用，改名完成后才形成环。
	if group.Name != oldName && id != 0 {
		if err := a.renameTagRefs(tx, oldName, group.Name); err != nil {
			return fmt.Errorf("更新引用「%s」失败: %w", oldName, err)
		}
	}
	return a.validateGroupMembers(tx, group, id)
}

// validateGroupMembers 校验成员是可用 tag（内置 direct、启用中的自定义出站或启用中的其它策略组），
// 并排除自引用与循环引用。口径必须与内核聚合一致：聚合只用启用中的出站与策略组，停用的成员
// 在配置里并不存在，若放过就会拖到热应用才失败。
func (a *API) validateGroupMembers(tx *gorm.DB, group *db.PolicyGroup, id uint) error {
	members := groupEntryOf(*group).Members
	for _, m := range members {
		if m == group.Name {
			return fmt.Errorf("策略组 %s 不能引用自身", group.Name)
		}
		if m == db.OutboundTypeDirect {
			continue
		}
		if m == singbox.OutboundBlock {
			return fmt.Errorf("成员不能是拦截（block）：它是规则动作而非出站")
		}
		var outboundDup int64
		tx.Model(&db.Outbound{}).Where("name = ? AND enabled = ?", m, true).Count(&outboundDup)
		if outboundDup > 0 {
			continue
		}
		var groupDup int64
		tx.Model(&db.PolicyGroup{}).Where("name = ? AND enabled = ? AND id <> ?", m, true, id).Count(&groupDup)
		if groupDup > 0 {
			continue
		}
		return fmt.Errorf("成员 %s 不存在或已停用（可选 direct 或启用中的出站 / 策略组）", m)
	}
	// 循环引用在这里就挡住：内核与客户端都会拒绝加载整份配置。只检查本次改动可达的子图——
	// 库里若已有历史环，不应连带阻断与它无关的其它组的保存。
	entries, err := a.groupEntriesWith(tx, group, id)
	if err != nil {
		return err
	}
	if cycle := reachableCycle(entries, group.Name); cycle != nil {
		return fmt.Errorf("策略组成员构成循环引用: %s", strings.Join(cycle, " → "))
	}
	return nil
}

// reachableCycle 把循环检测限制在 start 可达的策略组子图内（start 为本次保存的组名）。
func reachableCycle(groups []singbox.GroupEntry, start string) []string {
	members := make(map[string][]string, len(groups))
	for _, g := range groups {
		members[g.Name] = g.Members
	}
	reachable := make(map[string]bool, len(groups))
	stack := []string{start}
	for len(stack) > 0 {
		name := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if reachable[name] {
			continue
		}
		reachable[name] = true
		for _, m := range members[name] {
			if _, isGroup := members[m]; isGroup {
				stack = append(stack, m)
			}
		}
	}
	sub := make([]singbox.GroupEntry, 0, len(reachable))
	for _, g := range groups {
		if reachable[g.Name] {
			sub = append(sub, g)
		}
	}
	return singbox.FindGroupCycle(sub)
}

// groupEntriesWith 取全部策略组条目，其中 id 对应的组以待保存的新值替换（id=0 为新建，追加到末尾）。
func (a *API) groupEntriesWith(tx *gorm.DB, current *db.PolicyGroup, id uint) ([]singbox.GroupEntry, error) {
	var records []db.PolicyGroup
	if err := tx.Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	entries := make([]singbox.GroupEntry, 0, len(records)+1)
	replaced := false
	for _, r := range records {
		if r.ID == id {
			entries = append(entries, groupEntryOf(*current))
			replaced = true
			continue
		}
		entries = append(entries, groupEntryOf(r))
	}
	if !replaced {
		entries = append(entries, groupEntryOf(*current))
	}
	return entries, nil
}

// groupEntryOf 把策略组记录解析为聚合/校验用条目（成员 JSON 损坏视为无成员，
// 需要报错的地方用 tagRefCount / renameTagRefs 的显式解析）。
func groupEntryOf(g db.PolicyGroup) singbox.GroupEntry {
	members := []string{}
	if g.Members != "" {
		_ = json.Unmarshal([]byte(g.Members), &members)
	}
	return singbox.GroupEntry{Name: g.Name, Mode: g.Mode, Members: members}
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
