package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

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
	if err := a.ensureBuiltinRuleSets(); err != nil {
		Fail(c, http.StatusInternalServerError, "初始化内置规则集失败: "+err.Error())
		return
	}
	// 模板引用的规则集按名称解析为 ID
	setIDs, err := a.ruleSetIDsByName(singbox.AntiADTag, singbox.GeoCNTag, singbox.GeoIPCNTag)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询规则集失败: "+err.Error())
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
		rules = append(rules, db.Rule{
			MatchType: db.RuleMatchRuleSet, RuleSetID: setIDs[singbox.AntiADTag],
			Outbound: singbox.OutboundBlock, Enabled: true,
		})
		if tpl.ID == "cn-direct-adblock" {
			rules = append(rules,
				db.Rule{MatchType: db.RuleMatchRuleSet, RuleSetID: setIDs[singbox.GeoCNTag],
					Outbound: "direct", Enabled: true},
				db.Rule{MatchType: db.RuleMatchRuleSet, RuleSetID: setIDs[singbox.GeoIPCNTag],
					Outbound: "direct", Enabled: true},
			)
		}
	case "direct-adblock":
		rules = append(rules, db.Rule{
			MatchType: db.RuleMatchRuleSet, RuleSetID: setIDs[singbox.AntiADTag],
			Outbound: singbox.OutboundBlock, Enabled: true,
		})
	}
	final := "direct"
	if len(groups) > 0 {
		final = groups[0].Name
	}

	err = a.db.Transaction(func(tx *gorm.DB) error {
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
		return tx.Save(&db.Setting{Key: db.SettingRouteFinal, Value: final}).Error
	})
	if err != nil {
		Fail(c, http.StatusInternalServerError, "应用模板失败: "+err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	OK(c, gin.H{"template": tpl.ID, "rules": len(rules), "groups": len(groups), "final": final})
}

// ruleSetIDsByName 按名称批量查规则集 ID。
func (a *API) ruleSetIDsByName(names ...string) (map[string]uint, error) {
	var sets []db.RuleSet
	if err := a.db.Where("name IN ?", names).Find(&sets).Error; err != nil {
		return nil, err
	}
	ids := make(map[string]uint, len(sets))
	for _, s := range sets {
		ids[s.Name] = s.ID
	}
	for _, name := range names {
		if ids[name] == 0 {
			return nil, gorm.ErrRecordNotFound
		}
	}
	return ids, nil
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
