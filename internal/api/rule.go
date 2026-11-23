package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/ai"
	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// maxRulePatternLen 单个匹配值（含正则）的最大长度。
const maxRulePatternLen = 256

// ruleView 规则响应视图（附规则集名称供列表展示）。
type ruleView struct {
	db.Rule
	RuleSetName string `json:"ruleSetName"` // 引用的规则集名称
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
	view, err := a.createRule(req)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	a.applyThenOK(c, view)
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
	view, err := a.updateRule(id, req)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	a.applyThenOK(c, view)
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
	a.applyThenOK(c, gin.H{"ordered": len(req.IDs)})
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
	// 启用时要校验出站仍在：规则停用期间出站可能已被删除，直接放行会留下不可用的配置
	if req.Enabled {
		if err := a.validateOutboundTag(rule.Outbound); err != nil {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
	}
	if err := a.db.Model(&rule).Update("enabled", req.Enabled).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "更新规则状态失败: "+err.Error())
		return
	}
	rule.Enabled = req.Enabled
	names, _ := a.ruleSetNames()
	a.applyThenOK(c, ruleToView(rule, names))
}

// HandleRuleDelete DELETE /api/rules/:id 删除规则。
func (a *API) HandleRuleDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.deleteRule(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	a.applyThenOK(c, deletedResp{Deleted: id})
}

// ---------- 供 REST handler 与 AI 工具共用的写操作 ----------

// createRule 创建全局路由规则（排序追加到末尾）。
func (a *API) createRule(req ruleReq) (ruleView, error) {
	rule := db.Rule{Enabled: true}
	if err := a.applyRuleReq(&rule, req); err != nil {
		return ruleView{}, err
	}
	var maxOrder int
	a.db.Model(&db.Rule{}).Select("COALESCE(MAX(`order`), 0)").Scan(&maxOrder)
	rule.Order = maxOrder + 1
	if err := a.db.Create(&rule).Error; err != nil {
		return ruleView{}, fmt.Errorf("创建规则失败: %w", err)
	}
	names, _ := a.ruleSetNames()
	return ruleToView(rule, names), nil
}

// updateRule 按 ID 更新规则；仅更新给出的字段。
func (a *API) updateRule(id uint, req ruleReq) (ruleView, error) {
	var rule db.Rule
	if err := a.db.First(&rule, id).Error; err != nil {
		return ruleView{}, fmt.Errorf("规则不存在: %d", id)
	}
	if err := a.applyRuleReq(&rule, req); err != nil {
		return ruleView{}, err
	}
	if err := a.db.Save(&rule).Error; err != nil {
		return ruleView{}, fmt.Errorf("保存规则失败: %w", err)
	}
	names, _ := a.ruleSetNames()
	return ruleToView(rule, names), nil
}

// deleteRule 按 ID 删除规则。
func (a *API) deleteRule(id uint) error {
	if err := a.db.Delete(&db.Rule{}, id).Error; err != nil {
		return fmt.Errorf("删除规则失败: %w", err)
	}
	return nil
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
	// 只在本次请求给出出站或把规则重新启用时校验：否则「出站后来被停用」的历史规则连停用
	// 或改匹配值都做不了，用户就没有出口去修它了
	outboundChanged := req.Outbound != ""
	if outboundChanged {
		rule.Outbound = req.Outbound
	}
	reEnabled := req.Enabled != nil && *req.Enabled
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	switch rule.MatchType {
	case db.RuleMatchDomain:
		if !slices.Contains([]string{db.RuleMethodExact, db.RuleMethodSuffix, db.RuleMethodKeyword, db.RuleMethodRegex}, rule.MatchMethod) {
			return fmt.Errorf("域名规则的匹配方式需为 exact/suffix/keyword/regex，当前为: %s", rule.MatchMethod)
		}
		// 正则保存前预校验：坏正则拖到内核 Apply 才失败会让用户面对一大串内核错误
		if rule.MatchMethod == db.RuleMethodRegex {
			for p := range strings.SplitSeq(rule.Value, ",") {
				if p = strings.TrimSpace(p); p == "" {
					continue
				}
				if len(p) > maxRulePatternLen {
					return fmt.Errorf("单个正则最长 %d 字符", maxRulePatternLen)
				}
				if _, err := regexp.Compile(p); err != nil {
					return fmt.Errorf("正则不合法 %q: %w", p, err)
				}
			}
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
	// 规则出站只存 tag：新给出的出站、以及本次要把规则重新启用，都要校验出站仍在，
	// 否则悬空引用会拖到内核重建才失败（停用期间出站可能已被删除）
	if outboundChanged || reEnabled {
		if err := a.validateOutboundTag(rule.Outbound); err != nil {
			return err
		}
	}
	return nil
}

// validateOutboundTag 校验规则动作 tag：block、direct，或启用中的自定义出站 / 策略组。
func (a *API) validateOutboundTag(tag string) error {
	if tag == singbox.OutboundBlock || tag == db.OutboundTypeDirect {
		return nil
	}
	var outboundDup int64
	a.db.Model(&db.Outbound{}).Where("name = ? AND enabled = ?", tag, true).Count(&outboundDup)
	if outboundDup > 0 {
		return nil
	}
	var groupDup int64
	a.db.Model(&db.PolicyGroup{}).Where("name = ? AND enabled = ?", tag, true).Count(&groupDup)
	if groupDup > 0 {
		return nil
	}
	return fmt.Errorf("出站 %s 不存在或已停用（可选 block、direct 或启用中的出站 / 策略组）", tag)
}

// ruleToView 规则记录转视图（补规则集名称）。
func ruleToView(r db.Rule, names map[uint]string) ruleView {
	return ruleView{Rule: r, RuleSetName: names[r.RuleSetID]}
}

// ---------- 兜底出站 ----------

// finalResp 兜底出站响应。
type finalResp struct {
	Final string `json:"final"` // 出站 tag
}

// HandleRouteFinalGet GET /api/rules/final 兜底出站设置。
func (a *API) HandleRouteFinalGet(c *gin.Context) {
	OK(c, finalResp{Final: a.getSetting(db.SettingRouteFinal)})
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
	final := strings.TrimSpace(req.Final)
	if err := a.validateRouteFinal(final); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.setSetting(db.SettingRouteFinal, final); err != nil {
		Fail(c, http.StatusInternalServerError, "保存兜底出站失败: "+err.Error())
		return
	}
	a.applyThenOK(c, finalResp{Final: final})
}

// validateRouteFinal 校验兜底出站取值：空串表示内核默认（首个出站），否则必须是 direct、
// 启用中的自定义出站或启用中的策略组。block 不能作为兜底——拦截动作没有同名出站，内核
// 会因 route.final 指向不存在的出站而启动失败。
func (a *API) validateRouteFinal(final string) error {
	if final == "" || final == db.OutboundTypeDirect {
		return nil
	}
	if final == singbox.OutboundBlock {
		return fmt.Errorf("拦截（block）不能作为兜底出站：它是规则动作而非出站，请选择 direct、出站或策略组")
	}
	var outboundDup int64
	a.db.Model(&db.Outbound{}).Where("name = ? AND enabled = ?", final, true).Count(&outboundDup)
	if outboundDup > 0 {
		return nil
	}
	var groupDup int64
	a.db.Model(&db.PolicyGroup{}).Where("name = ? AND enabled = ?", final, true).Count(&groupDup)
	if groupDup > 0 {
		return nil
	}
	return fmt.Errorf("兜底出站 %s 不存在（可选 direct、启用中的出站或策略组）", final)
}

// ---------- 路由规则 AI 工具 ----------

// registerRuleAITools 注册全局路由规则相关的查询与写操作 AI 工具。
func (a *API) registerRuleAITools() {
	a.ai.Register(ai.Tool{
		Name:        "list_rules",
		Description: "列出全部全局路由规则（按生效顺序，含匹配对象 matchType、匹配方式 matchMethod、匹配值、引用的规则集与出站动作）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.ruleViews()
		},
	})
	a.registerModify(ai.Tool{
		Name: "create_rule",
		Description: "创建全局路由规则（追加到规则末尾）。matchType 为匹配对象：domain 域名 / ip IP / rule_set 规则集；" +
			"域名需给出 matchMethod（exact/suffix/keyword/regex）与 value；IP 固定 cidr + value；规则集只给 ruleSetId",
		Parameters: toolParams(map[string]any{
			"matchType":   toolProp("string", "匹配对象", db.RuleMatchDomain, db.RuleMatchIP, db.RuleMatchRuleSet),
			"matchMethod": toolProp("string", "匹配方式（域名规则必填，IP 固定 cidr）", db.RuleMethodExact, db.RuleMethodSuffix, db.RuleMethodKeyword, db.RuleMethodRegex, db.RuleMethodCIDR),
			"value":       toolProp("string", "匹配值（多值逗号分隔；规则集留空）"),
			"ruleSetId":   toolProp("integer", "规则集 ID（matchType=rule_set 时必填，可先用 list_rule_sets 查询）"),
			"outbound":    toolProp("string", "动作出站 tag：direct、block、策略组或自定义出站"),
		}, "matchType", "outbound"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req ruleReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			return a.createRule(req)
		},
	})
	a.registerModify(ai.Tool{
		Name:        "update_rule",
		Description: "按 ID 更新规则；仅更新给出的字段",
		Parameters: toolParams(map[string]any{
			"id":          toolProp("integer", "规则 ID"),
			"matchType":   toolProp("string", "匹配对象", db.RuleMatchDomain, db.RuleMatchIP, db.RuleMatchRuleSet),
			"matchMethod": toolProp("string", "匹配方式", db.RuleMethodExact, db.RuleMethodSuffix, db.RuleMethodKeyword, db.RuleMethodRegex, db.RuleMethodCIDR),
			"value":       toolProp("string", "匹配值"),
			"ruleSetId":   toolProp("integer", "规则集 ID"),
			"outbound":    toolProp("string", "动作出站 tag"),
			"enabled":     toolProp("boolean", "是否启用"),
		}, "id"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req ruleReq
			var arg idArg
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			return a.updateRule(arg.ID, req)
		},
	})
	a.registerModify(ai.Tool{
		Name:         "delete_rule",
		Description:  "按 ID 删除规则",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "规则 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if err := a.deleteRule(arg.ID); err != nil {
				return nil, err
			}
			return deletedResp{Deleted: arg.ID}, nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_route_final",
		Description: "查询兜底出站设置（未命中任何路由规则的流量走向；空表示内核默认取首个出站）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return finalResp{Final: a.getSetting(db.SettingRouteFinal)}, nil
		},
	})
	a.registerModify(ai.Tool{
		Name:        "set_route_final",
		Description: "设置兜底出站（未命中任何路由规则的流量走向）；final 为出站 tag：direct、策略组或自定义出站（不能是 block，它不是出站），空串恢复内核默认",
		Parameters: toolParams(map[string]any{
			"final": toolProp("string", "出站 tag，空串表示内核默认（首个出站）"),
		}, "final"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req finalReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			final := strings.TrimSpace(req.Final)
			if err := a.validateRouteFinal(final); err != nil {
				return nil, err
			}
			if err := a.setSetting(db.SettingRouteFinal, final); err != nil {
				return nil, fmt.Errorf("保存兜底出站失败: %w", err)
			}
			return finalResp{Final: final}, nil
		},
	})
}
