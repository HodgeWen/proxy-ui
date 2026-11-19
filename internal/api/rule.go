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

// 设置键。
const (
	settingAntiAD     = "anti_ad_source" // anti-AD 规则源地址，空用官方默认源
	settingRouteFinal = "route_final"    // 未命中规则的兜底出站 tag，空用内核默认
)

// ruleView 规则响应视图。
type ruleView struct {
	ID        uint      `json:"id"`        // 主键
	Type      string    `json:"type"`      // 规则类型
	Value     string    `json:"value"`     // 匹配值
	Outbound  string    `json:"outbound"`  // 动作出站 tag
	Order     int       `json:"order"`     // 排序，值小者靠前
	Enabled   bool      `json:"enabled"`   // 是否启用
	CreatedAt time.Time `json:"createdAt"` // 创建时间
	UpdatedAt time.Time `json:"updatedAt"` // 更新时间
}

// ruleReq 创建/更新规则请求。
type ruleReq struct {
	Type     string `json:"type"`     // 规则类型
	Value    string `json:"value"`    // 匹配值，多值逗号分隔
	Outbound string `json:"outbound"` // 动作出站 tag
	Enabled  *bool  `json:"enabled"`  // 是否启用，缺省 true
}

// HandleRulesList GET /api/rules 规则列表（按排序）。
func (a *API) HandleRulesList(c *gin.Context) {
	var records []db.Rule
	if err := a.db.Order("`order`").Find(&records).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "查询规则列表失败: "+err.Error())
		return
	}
	views := make([]ruleView, len(records))
	for i, r := range records {
		views[i] = ruleView(r)
	}
	OK(c, views)
}

// HandleRuleCreate POST /api/rules 创建规则（排序追加到末尾）。
func (a *API) HandleRuleCreate(c *gin.Context) {
	var req ruleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	rule := db.Rule{Enabled: true}
	if err := applyRuleReq(&rule, req); err != nil {
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
	OK(c, ruleView(rule))
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
	if err := applyRuleReq(&rule, req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.db.Save(&rule).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "保存规则失败: "+err.Error())
		return
	}
	OK(c, ruleView(rule))
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
	OK(c, ruleView(rule))
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
	OK(c, gin.H{"deleted": id})
}

// applyRuleReq 把请求字段应用到规则记录并校验类型/取值。
func applyRuleReq(rule *db.Rule, req ruleReq) error {
	if req.Type != "" {
		rule.Type = req.Type
	}
	if req.Value != "" {
		rule.Value = req.Value
	}
	if req.Outbound != "" {
		rule.Outbound = req.Outbound
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if !supportedRuleType(rule.Type) {
		return fmt.Errorf("不支持的路由规则类型: %s", rule.Type)
	}
	if strings.TrimSpace(rule.Value) == "" {
		return fmt.Errorf("匹配值不能为空")
	}
	if rule.Outbound == "" {
		return fmt.Errorf("出站不能为空")
	}
	return nil
}

// supportedRuleType 判断是否为面板支持的规则类型。
func supportedRuleType(t string) bool {
	for _, v := range singbox.SupportedRuleTypes {
		if v == t {
			return true
		}
	}
	return false
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
	var records []db.PolicyGroup
	if err := a.db.Order("id").Find(&records).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "查询策略组列表失败: "+err.Error())
		return
	}
	views := make([]groupView, 0, len(records))
	for _, g := range records {
		views = append(views, groupToView(g))
	}
	OK(c, views)
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
	OK(c, gin.H{"deleted": id})
}

// applyGroupReq 把请求字段应用到策略组记录并校验模式与名称唯一性（成员存在性在应用配置时校验）。
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
	if !supportedGroupMode(group.Mode) {
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

// marshalMembers 序列化成员列表；nil 视为未提供由调用方处理。
func marshalMembers(members []string) string {
	if members == nil {
		members = []string{}
	}
	raw, _ := json.Marshal(members)
	return string(raw)
}

// supportedGroupMode 判断是否为面板支持的策略组模式。
func supportedGroupMode(mode string) bool {
	for _, m := range singbox.SupportedGroupModes {
		if m == mode {
			return true
		}
	}
	return false
}

// ---------- 预设模板 ----------

// ruleTemplate 预设规则模板定义。
type ruleTemplate struct {
	ID          string `json:"id"`          // 模板 ID
	Name        string `json:"name"`        // 名称
	Description string `json:"description"` // 说明
}

// ruleTemplates 预设模板列表；proxy 组成员取全部启用自定义出站（无则兜底 direct）。
var ruleTemplates = []ruleTemplate{
	{ID: "cn-direct-adblock", Name: "国内直连 + 海外代理 + 广告拦截",
		Description: "国内域名与 IP 直连，anti-AD 拦截广告，其余流量走「proxy」策略组（自动选择）"},
	{ID: "global-proxy-adblock", Name: "全局代理 + 广告拦截",
		Description: "anti-AD 拦截广告，其余全部流量走「proxy」策略组（自动选择）"},
	{ID: "direct-adblock", Name: "全局直连 + 广告拦截",
		Description: "anti-AD 拦截广告，其余全部流量直连"},
}

// HandleRuleTemplates GET /api/rule-templates 预设模板列表。
func (a *API) HandleRuleTemplates(c *gin.Context) {
	OK(c, ruleTemplates)
}

// HandleRuleTemplateApply POST /api/rule-templates/:id/apply 一键应用模板：
// 覆盖现有全部规则与策略组并设置兜底出站（前端应用前确认）。
func (a *API) HandleRuleTemplateApply(c *gin.Context) {
	id := c.Param("id")
	var tpl *ruleTemplate
	for i := range ruleTemplates {
		if ruleTemplates[i].ID == id {
			tpl = &ruleTemplates[i]
			break
		}
	}
	if tpl == nil {
		Fail(c, http.StatusNotFound, "模板不存在")
		return
	}
	members := a.proxyMembers()
	var rules []db.Rule
	var groups []db.PolicyGroup
	switch tpl.ID {
	case "cn-direct-adblock", "global-proxy-adblock":
		groups = append(groups, db.PolicyGroup{
			Name: "proxy", Mode: singbox.GroupModeAuto, Members: marshalMembers(members), Enabled: true,
		})
		rules = append(rules, db.Rule{Type: singbox.RuleTypeRuleSet, Value: singbox.AntiADTag,
			Outbound: singbox.OutboundBlock, Enabled: true})
		if tpl.ID == "cn-direct-adblock" {
			rules = append(rules,
				db.Rule{Type: singbox.RuleTypeGeosite, Value: "cn", Outbound: "direct", Enabled: true},
				db.Rule{Type: singbox.RuleTypeGeoIP, Value: "cn", Outbound: "direct", Enabled: true},
			)
		}
	case "direct-adblock":
		rules = append(rules, db.Rule{Type: singbox.RuleTypeRuleSet, Value: singbox.AntiADTag,
			Outbound: singbox.OutboundBlock, Enabled: true})
	}
	final := "direct"
	if len(groups) > 0 {
		final = groups[0].Name
	}

	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&db.Rule{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&db.PolicyGroup{}).Error; err != nil {
			return err
		}
		for i := range rules {
			rules[i].Order = i + 1
			if err := tx.Create(&rules[i]).Error; err != nil {
				return err
			}
		}
		for i := range groups {
			if err := tx.Create(&groups[i]).Error; err != nil {
				return err
			}
		}
		return tx.Save(&db.Setting{Key: settingRouteFinal, Value: final}).Error
	})
	if err != nil {
		Fail(c, http.StatusInternalServerError, "应用模板失败: "+err.Error())
		return
	}
	OK(c, gin.H{"template": tpl.ID, "rules": len(rules), "groups": len(groups), "final": final})
}

// proxyMembers 收集启用中的自定义出站 tag 作为策略组成员；无自定义出站时兜底 direct。
func (a *API) proxyMembers() []string {
	var outbounds []db.Outbound
	a.db.Where("enabled = ? AND type <> ?", true, db.OutboundTypeDirect).Order("id").Find(&outbounds)
	members := make([]string, 0, len(outbounds))
	for _, ob := range outbounds {
		members = append(members, ob.Name)
	}
	if len(members) == 0 {
		members = []string{"direct"}
	}
	return members
}

// ---------- anti-AD 规则源与兜底出站 ----------

// HandleAntiADGet GET /api/anti-ad anti-AD 规则源地址（未设置返回官方默认源）。
func (a *API) HandleAntiADGet(c *gin.Context) {
	OK(c, gin.H{"url": a.getSetting(settingAntiAD), "default": singbox.DefaultAntiADURL})
}

// antiADReq anti-AD 规则源设置请求；url 空表示恢复官方默认源。
type antiADReq struct {
	URL string `json:"url"` // 规则源地址
}

// HandleAntiADSet PUT /api/anti-ad 保存 anti-AD 规则源地址。
func (a *API) HandleAntiADSet(c *gin.Context) {
	var req antiADReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	url := strings.TrimSpace(req.URL)
	if url != "" && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		Fail(c, http.StatusBadRequest, "规则源地址须以 http:// 或 https:// 开头")
		return
	}
	if err := a.setSetting(settingAntiAD, url); err != nil {
		Fail(c, http.StatusInternalServerError, "保存规则源失败: "+err.Error())
		return
	}
	OK(c, gin.H{"url": url, "default": singbox.DefaultAntiADURL})
}

// HandleRouteFinalGet GET /api/rules/final 兜底出站设置。
func (a *API) HandleRouteFinalGet(c *gin.Context) {
	OK(c, gin.H{"final": a.getSetting(settingRouteFinal)})
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
	if err := a.setSetting(settingRouteFinal, strings.TrimSpace(req.Final)); err != nil {
		Fail(c, http.StatusInternalServerError, "保存兜底出站失败: "+err.Error())
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
