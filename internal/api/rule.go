package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// ruleView 规则响应视图（附规则集名称供列表展示）。
type ruleView struct {
	ID          uint      `json:"id"`          // 主键
	MatchType   string    `json:"matchType"`   // 匹配对象：domain / ip / rule_set
	MatchMethod string    `json:"matchMethod"` // 匹配方式（rule_set 为空）
	Value       string    `json:"value"`       // 匹配值
	RuleSetID   uint      `json:"ruleSetId"`   // 引用的规则集 ID
	RuleSetName string    `json:"ruleSetName"` // 引用的规则集名称
	Outbound    string    `json:"outbound"`    // 动作出站 tag
	Order       int       `json:"order"`       // 排序
	Enabled     bool      `json:"enabled"`     // 是否启用
	CreatedAt   time.Time `json:"createdAt"`   // 创建时间
	UpdatedAt   time.Time `json:"updatedAt"`   // 更新时间
}

// ruleReq 创建/更新规则请求。
type ruleReq struct {
	MatchType   string `json:"matchType"`   // 匹配对象
	MatchMethod string `json:"matchMethod"` // 匹配方式
	Value       string `json:"value"`       // 匹配值，多值逗号分隔
	RuleSetID   uint   `json:"ruleSetId"`   // 引用的规则集（matchType=rule_set 时必填）
	Outbound    string `json:"outbound"`    // 动作出站 tag
	Enabled     *bool  `json:"enabled"`     // 是否启用，缺省 true
}

// HandleRulesList GET /api/rules 规则列表（按排序）。
func (a *API) HandleRulesList(c *gin.Context) {
	views, err := a.ruleViews()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询规则列表失败: "+err.Error())
		return
	}
	OK(c, views)
}

// ruleViews 查询全部规则并转为响应视图（按排序）。
func (a *API) ruleViews() ([]ruleView, error) {
	var records []db.Rule
	if err := a.db.Order("`order`").Find(&records).Error; err != nil {
		return nil, err
	}
	names, err := a.ruleSetNames()
	if err != nil {
		return nil, err
	}
	views := make([]ruleView, 0, len(records))
	for _, r := range records {
		views = append(views, ruleToView(r, names))
	}
	return views, nil
}

// ruleSetNames 规则集 ID -> 名称。
func (a *API) ruleSetNames() (map[uint]string, error) {
	var sets []db.RuleSet
	if err := a.db.Find(&sets).Error; err != nil {
		return nil, err
	}
	names := make(map[uint]string, len(sets))
	for _, s := range sets {
		names[s.ID] = s.Name
	}
	return names, nil
}

// HandleRuleCreate POST /api/rules 创建规则（排序追加到末尾）。
func (a *API) HandleRuleCreate(c *gin.Context) {
	var req ruleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	rule := db.Rule{Enabled: true}
	if err := a.applyRuleReq(&rule, req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	var maxOrder int
	a.db.Model(&db.Rule{}).Select("COALESCE(MAX(`order`), 0)").Scan(&maxOrder)
	rule.Order = maxOrder + 1
	if err := a.db.Create(&rule).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "创建规则失败: "+err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	names, _ := a.ruleSetNames()
	OK(c, ruleToView(rule, names))
}

// HandleRuleUpdate PUT /api/rules/:id 更新规则。
func (a *API) HandleRuleUpdate(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req ruleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var rule db.Rule
	if err := a.db.First(&rule, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "规则不存在")
		return
	}
	if err := a.applyRuleReq(&rule, req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.db.Save(&rule).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "保存规则失败: "+err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	names, _ := a.ruleSetNames()
	OK(c, ruleToView(rule, names))
}

// orderReq 规则排序请求。
type orderReq struct {
	IDs []uint `json:"ids"` // 按生效顺序给出的规则 ID 全集
}

// HandleRulesOrder PUT /api/rules/order 按给定 ID 顺序重排规则。
func (a *API) HandleRulesOrder(c *gin.Context) {
	var req orderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	err := a.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range req.IDs {
			if err := tx.Model(&db.Rule{}).Where("id = ?", id).Update("`order`", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		Fail(c, http.StatusInternalServerError, "调整规则排序失败: "+err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	OK(c, gin.H{"ordered": len(req.IDs)})
}

// HandleRuleSetEnabled PATCH /api/rules/:id/enabled 启停规则。
func (a *API) HandleRuleSetEnabled(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req enabledReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var rule db.Rule
	if err := a.db.First(&rule, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "规则不存在")
		return
	}
	if err := a.db.Model(&rule).Update("enabled", req.Enabled).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "更新规则状态失败: "+err.Error())
		return
	}
	rule.Enabled = req.Enabled
	if !a.applyOrFail(c) {
		return
	}
	names, _ := a.ruleSetNames()
	OK(c, ruleToView(rule, names))
}

// HandleRuleDelete DELETE /api/rules/:id 删除规则。
func (a *API) HandleRuleDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.db.Delete(&db.Rule{}, id).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "删除规则失败: "+err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	OK(c, gin.H{"deleted": id})
}

// applyRuleReq 把请求字段应用到规则记录并校验匹配对象/方式/取值与规则集引用。
func (a *API) applyRuleReq(rule *db.Rule, req ruleReq) error {
	if req.MatchType != "" {
		rule.MatchType = req.MatchType
	}
	if req.MatchMethod != "" {
		rule.MatchMethod = req.MatchMethod
	}
	if req.Value != "" {
		rule.Value = req.Value
	}
	if req.RuleSetID != 0 {
		rule.RuleSetID = req.RuleSetID
	}
	if req.Outbound != "" {
		rule.Outbound = req.Outbound
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	switch rule.MatchType {
	case db.RuleMatchDomain:
		if !contains([]string{db.RuleMethodExact, db.RuleMethodSuffix, db.RuleMethodKeyword, db.RuleMethodRegex}, rule.MatchMethod) {
			return fmt.Errorf("域名规则的匹配方式需为 exact/suffix/keyword/regex，当前为: %s", rule.MatchMethod)
		}
		rule.RuleSetID = 0
	case db.RuleMatchIP:
		if rule.MatchMethod != db.RuleMethodCIDR {
			return fmt.Errorf("IP 规则只支持网段匹配（cidr），当前为: %s", rule.MatchMethod)
		}
		rule.RuleSetID = 0
	case db.RuleMatchRuleSet:
		if rule.RuleSetID == 0 {
			return fmt.Errorf("请选择规则集")
		}
		var count int64
		a.db.Model(&db.RuleSet{}).Where("id = ?", rule.RuleSetID).Count(&count)
		if count == 0 {
			return fmt.Errorf("引用的规则集不存在: %d", rule.RuleSetID)
		}
		rule.MatchMethod = ""
		rule.Value = ""
	default:
		return fmt.Errorf("不支持的匹配对象: %s（可选 domain/ip/rule_set）", rule.MatchType)
	}
	if rule.MatchType != db.RuleMatchRuleSet && strings.TrimSpace(rule.Value) == "" {
		return fmt.Errorf("匹配值不能为空")
	}
	if rule.Outbound == "" {
		return fmt.Errorf("出站不能为空")
	}
	return nil
}

// ruleToView 规则记录转视图（补规则集名称）。
func ruleToView(r db.Rule, names map[uint]string) ruleView {
	return ruleView{
		ID: r.ID, MatchType: r.MatchType, MatchMethod: r.MatchMethod, Value: r.Value,
		RuleSetID: r.RuleSetID, RuleSetName: names[r.RuleSetID], Outbound: r.Outbound,
		Order: r.Order, Enabled: r.Enabled, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// ---------- 策略组 ----------

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
	group := db.PolicyGroup{Enabled: true}
	if err := a.applyGroupReq(&group, req, 0); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.db.Create(&group).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "创建策略组失败: "+err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	OK(c, groupToView(group))
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
	var group db.PolicyGroup
	if err := a.db.First(&group, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "策略组不存在")
		return
	}
	if err := a.applyGroupReq(&group, req, id); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.db.Save(&group).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "保存策略组失败: "+err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	OK(c, groupToView(group))
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
	if !a.applyOrFail(c) {
		return
	}
	OK(c, groupToView(group))
}

// HandleGroupDelete DELETE /api/policy-groups/:id 删除策略组。
func (a *API) HandleGroupDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.db.Delete(&db.PolicyGroup{}, id).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "删除策略组失败: "+err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	OK(c, gin.H{"deleted": id})
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
	if !contains(singbox.SupportedGroupModes, group.Mode) {
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
		_ = jsonUnmarshalStrings(g.Members, &members)
	}
	return groupView{
		ID: g.ID, Name: g.Name, Mode: g.Mode, Members: members,
		Enabled: g.Enabled, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt,
	}
}

// ---------- 兜底出站 ----------

// HandleRouteFinalGet GET /api/rules/final 兜底出站设置。
func (a *API) HandleRouteFinalGet(c *gin.Context) {
	OK(c, gin.H{"final": a.getSetting(db.SettingRouteFinal)})
}

// finalReq 兜底出站设置请求；final 空表示内核默认（首个出站）。
type finalReq struct {
	Final string `json:"final"` // 出站 tag：direct、策略组或自定义出站
}

// HandleRouteFinalSet PUT /api/rules/final 保存兜底出站设置。
func (a *API) HandleRouteFinalSet(c *gin.Context) {
	var req finalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if err := a.setSetting(db.SettingRouteFinal, strings.TrimSpace(req.Final)); err != nil {
		Fail(c, http.StatusInternalServerError, "保存兜底出站失败: "+err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	OK(c, gin.H{"final": strings.TrimSpace(req.Final)})
}

// getSetting 读取设置值，不存在返回空字符串。
func (a *API) getSetting(key string) string {
	var s db.Setting
	if err := a.db.First(&s, "key = ?", key).Error; err != nil {
		return ""
	}
	return s.Value
}

// setSetting 写入设置值（upsert）。
func (a *API) setSetting(key, value string) error {
	return a.db.Save(&db.Setting{Key: key, Value: value}).Error
}
