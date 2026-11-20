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

// ruleSetItemView 规则集内容项视图。
type ruleSetItemView struct {
	ID             uint   `json:"id"`             // 主键
	Remark         string `json:"remark"`         // 备注
	SingBoxURL     string `json:"singBoxUrl"`     // sing-box 规则源
	MihomoURL      string `json:"mihomoUrl"`      // mihomo 规则源
	MihomoBehavior string `json:"mihomoBehavior"` // mihomo 行为
	Enabled        bool   `json:"enabled"`        // 是否启用
}

// ruleSetView 规则集视图：附带全部内容项与是否被规则引用。
type ruleSetView struct {
	ID          uint              `json:"id"`          // 主键
	Name        string            `json:"name"`        // 规则集 tag
	Description string            `json:"description"` // 说明
	Builtin     bool              `json:"builtin"`     // 是否内置
	UsedByRules int               `json:"usedByRules"` // 被多少条规则引用
	Items       []ruleSetItemView `json:"items"`       // 内容项列表
	CreatedAt   time.Time         `json:"createdAt"`   // 创建时间
	UpdatedAt   time.Time         `json:"updatedAt"`   // 更新时间
}

// ruleSetReq 创建/更新规则集请求（名称与说明）。
type ruleSetReq struct {
	Name        string `json:"name"`        // 规则集 tag
	Description string `json:"description"` // 说明
}

// ruleSetItemReq 创建/更新规则集内容项请求；Enabled 为 true 时停用该规则集其它内容项。
type ruleSetItemReq struct {
	Remark         string `json:"remark"`         // 备注
	SingBoxURL     string `json:"singBoxUrl"`     // sing-box 规则源
	MihomoURL      string `json:"mihomoUrl"`      // mihomo 规则源
	MihomoBehavior string `json:"mihomoBehavior"` // mihomo 行为
	Enabled        bool   `json:"enabled"`        // 是否启用
}

// mihomo 规则集行为取值。
var mihomoBehaviors = []string{"domain", "ipcidr", "classical"}

// HandleRuleSetsList GET /api/rule-sets 规则集列表（内置 + 自定义，含内容项）。
func (a *API) HandleRuleSetsList(c *gin.Context) {
	views, err := a.ruleSetViews()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询规则集失败: "+err.Error())
		return
	}
	OK(c, views)
}

// ruleSetViews 查询全部规则集并组装视图。
func (a *API) ruleSetViews() ([]ruleSetView, error) {
	if err := a.ensureBuiltinRuleSets(); err != nil {
		return nil, err
	}
	var sets []db.RuleSet
	if err := a.db.Order("id").Find(&sets).Error; err != nil {
		return nil, err
	}
	var items []db.RuleSetItem
	if err := a.db.Order("id").Find(&items).Error; err != nil {
		return nil, err
	}
	bySet := make(map[uint][]db.RuleSetItem, len(sets))
	for _, it := range items {
		bySet[it.RuleSetID] = append(bySet[it.RuleSetID], it)
	}
	used, err := a.ruleSetUsage()
	if err != nil {
		return nil, err
	}
	views := make([]ruleSetView, 0, len(sets))
	for _, s := range sets {
		view := ruleSetView{
			ID: s.ID, Name: s.Name, Description: s.Description, Builtin: s.Builtin,
			UsedByRules: used[s.ID], CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
			Items: make([]ruleSetItemView, 0, len(bySet[s.ID])),
		}
		for _, it := range bySet[s.ID] {
			view.Items = append(view.Items, ruleSetItemView{
				ID: it.ID, Remark: it.Remark, SingBoxURL: it.SingBoxURL,
				MihomoURL: it.MihomoURL, MihomoBehavior: it.MihomoBehavior, Enabled: it.Enabled,
			})
		}
		views = append(views, view)
	}
	return views, nil
}

// ruleSetUsage 统计各规则集被规则引用的条数。
func (a *API) ruleSetUsage() (map[uint]int, error) {
	var rows []struct {
		RuleSetID uint
		Total     int
	}
	if err := a.db.Model(&db.Rule{}).Select("rule_set_id, COUNT(*) AS total").
		Where("match_type = ? AND rule_set_id > 0", db.RuleMatchRuleSet).
		Group("rule_set_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	used := make(map[uint]int, len(rows))
	for _, r := range rows {
		used[r.RuleSetID] = r.Total
	}
	return used, nil
}

// ensureBuiltinRuleSets 补齐内置规则集及其默认内容项（幂等，只在缺失时写入）。
func (a *API) ensureBuiltinRuleSets() error {
	for _, def := range singbox.BuiltinRuleSets {
		var set db.RuleSet
		err := a.db.Where("name = ?", def.Name).First(&set).Error
		if err == nil {
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		mihomoURL, behavior, _ := singbox.BuiltinMihomoSource(def.Name)
		set = db.RuleSet{Name: def.Name, Description: def.Description, Builtin: true}
		if err := a.db.Create(&set).Error; err != nil {
			return err
		}
		item := db.RuleSetItem{
			RuleSetID: set.ID, Remark: "内置默认源", SingBoxURL: def.SingBoxURL,
			MihomoURL: mihomoURL, MihomoBehavior: behavior, Enabled: true,
		}
		if err := a.db.Create(&item).Error; err != nil {
			return err
		}
	}
	return nil
}

// HandleRuleSetCreate POST /api/rule-sets 新建规则集。
func (a *API) HandleRuleSetCreate(c *gin.Context) {
	var req ruleSetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	set, err := a.createRuleSet(req)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	views, err := a.ruleSetViews()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	for _, v := range views {
		if v.ID == set.ID {
			OK(c, v)
			return
		}
	}
	Fail(c, http.StatusInternalServerError, "规则集创建后查询失败")
}

// isBuiltinRuleSetName 判断名称是否与内置规则集冲突（内置规则集可能尚未落库）。
func isBuiltinRuleSetName(name string) bool {
	for _, def := range singbox.BuiltinRuleSets {
		if def.Name == name {
			return true
		}
	}
	return false
}

// createRuleSet 创建规则集（校验名称唯一且不与内置冲突）。
func (a *API) createRuleSet(req ruleSetReq) (db.RuleSet, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return db.RuleSet{}, fmt.Errorf("规则集名称不能为空")
	}
	if !validTag(name) {
		return db.RuleSet{}, fmt.Errorf("名称只能包含字母、数字、下划线、点与短横线: %s", name)
	}
	// 内置规则集按需写入，未落库时同名判断会放过，这里显式挡住内置名称
	if isBuiltinRuleSetName(name) {
		return db.RuleSet{}, fmt.Errorf("规则集 %s 为内置名称，不可使用", name)
	}
	var dup int64
	a.db.Model(&db.RuleSet{}).Where("name = ?", name).Count(&dup)
	if dup > 0 {
		return db.RuleSet{}, fmt.Errorf("规则集 %s 已存在", name)
	}
	set := db.RuleSet{Name: name, Description: strings.TrimSpace(req.Description)}
	if err := a.db.Create(&set).Error; err != nil {
		return db.RuleSet{}, fmt.Errorf("创建规则集失败: %w", err)
	}
	return set, nil
}

// HandleRuleSetUpdate PUT /api/rule-sets/:id 更新规则集名称/说明（重命名同步改引用无需处理，引用按 ID）。
func (a *API) HandleRuleSetUpdate(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req ruleSetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var set db.RuleSet
	if err := a.db.First(&set, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "规则集不存在")
		return
	}
	updates := map[string]any{"description": strings.TrimSpace(req.Description)}
	if name := strings.TrimSpace(req.Name); name != "" && name != set.Name {
		if !validTag(name) {
			Fail(c, http.StatusBadRequest, "名称只能包含字母、数字、下划线、点与短横线: "+name)
			return
		}
		var dup int64
		a.db.Model(&db.RuleSet{}).Where("name = ? AND id <> ?", name, id).Count(&dup)
		if dup > 0 {
			Fail(c, http.StatusBadRequest, "规则集 "+name+" 已存在")
			return
		}
		updates["name"] = name
	}
	if err := a.db.Model(&set).Updates(updates).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "保存规则集失败: "+err.Error())
		return
	}
	OK(c, gin.H{"updated": id})
}

// HandleRuleSetDelete DELETE /api/rule-sets/:id 删除规则集（内置或有规则引用时拒绝）。
func (a *API) HandleRuleSetDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var set db.RuleSet
	if err := a.db.First(&set, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "规则集不存在")
		return
	}
	if set.Builtin {
		Fail(c, http.StatusBadRequest, "内置规则集不可删除，可切换其内容项来源")
		return
	}
	used, err := a.ruleSetUsage()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询引用失败: "+err.Error())
		return
	}
	if used[id] > 0 {
		Fail(c, http.StatusBadRequest, fmt.Sprintf("该规则集被 %d 条规则引用，请先修改或删除这些规则", used[id]))
		return
	}
	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("rule_set_id = ?", id).Delete(&db.RuleSetItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&db.RuleSet{}, id).Error
	}); err != nil {
		Fail(c, http.StatusInternalServerError, "删除规则集失败: "+err.Error())
		return
	}
	OK(c, gin.H{"deleted": id})
}

// HandleRuleSetItemsList GET /api/rule-sets/:id/items 规则集内容项列表。
func (a *API) HandleRuleSetItemsList(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	views, err := a.ruleSetItemViews(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询内容项失败: "+err.Error())
		return
	}
	OK(c, views)
}

// ruleSetItemViews 查询某规则集的内容项视图。
func (a *API) ruleSetItemViews(setID uint) ([]ruleSetItemView, error) {
	var items []db.RuleSetItem
	if err := a.db.Where("rule_set_id = ?", setID).Order("id").Find(&items).Error; err != nil {
		return nil, err
	}
	views := make([]ruleSetItemView, 0, len(items))
	for _, it := range items {
		views = append(views, ruleSetItemView{
			ID: it.ID, Remark: it.Remark, SingBoxURL: it.SingBoxURL,
			MihomoURL: it.MihomoURL, MihomoBehavior: it.MihomoBehavior, Enabled: it.Enabled,
		})
	}
	return views, nil
}

// HandleRuleSetItemCreate POST /api/rule-sets/:id/items 新增内容项。
func (a *API) HandleRuleSetItemCreate(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req ruleSetItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var set db.RuleSet
	if err := a.db.First(&set, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "规则集不存在")
		return
	}
	item := db.RuleSetItem{RuleSetID: id}
	if err := a.applyRuleSetItem(&item, req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if item.Enabled {
			if err := tx.Model(&db.RuleSetItem{}).Where("rule_set_id = ?", id).
				Update("enabled", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&item).Error
	})
	if err != nil {
		Fail(c, http.StatusInternalServerError, "创建内容项失败: "+err.Error())
		return
	}
	OK(c, ruleSetItemViewOf(item))
}

// HandleRuleSetItemUpdate PUT /api/rule-sets/items/:itemId 更新内容项（含切换启用）。
func (a *API) HandleRuleSetItemUpdate(c *gin.Context) {
	itemID, ok := idParam(c)
	if !ok {
		return
	}
	var req ruleSetItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var item db.RuleSetItem
	if err := a.db.First(&item, itemID).Error; err != nil {
		Fail(c, http.StatusNotFound, "内容项不存在")
		return
	}
	if err := a.applyRuleSetItem(&item, req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if item.Enabled {
			if err := tx.Model(&db.RuleSetItem{}).
				Where("rule_set_id = ? AND id <> ?", item.RuleSetID, item.ID).
				Update("enabled", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(&item).Error
	})
	if err != nil {
		Fail(c, http.StatusInternalServerError, "保存内容项失败: "+err.Error())
		return
	}
	OK(c, ruleSetItemViewOf(item))
}

// HandleRuleSetItemDelete DELETE /api/rule-sets/items/:itemId 删除内容项（至少保留一条）。
func (a *API) HandleRuleSetItemDelete(c *gin.Context) {
	itemID, ok := idParam(c)
	if !ok {
		return
	}
	var item db.RuleSetItem
	if err := a.db.First(&item, itemID).Error; err != nil {
		Fail(c, http.StatusNotFound, "内容项不存在")
		return
	}
	var total int64
	a.db.Model(&db.RuleSetItem{}).Where("rule_set_id = ?", item.RuleSetID).Count(&total)
	if total <= 1 {
		Fail(c, http.StatusBadRequest, "每个规则集至少保留一条内容项")
		return
	}
	if err := a.db.Delete(&db.RuleSetItem{}, itemID).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "删除内容项失败: "+err.Error())
		return
	}
	// 删掉的是启用项时，把最早的一条补为启用；必须挑有 sing-box 源的那条，
	// 否则会被提升出一条服务端无法加载的规则集（空 URL 直接让内核起不来）
	if item.Enabled {
		var candidates []db.RuleSetItem
		if err := a.db.Where("rule_set_id = ? AND sing_box_url <> ?", item.RuleSetID, "").
			Order("id").Find(&candidates).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "查询内容项失败: "+err.Error())
			return
		}
		if len(candidates) > 0 {
			if err := a.db.Model(&db.RuleSetItem{}).Where("id = ?", candidates[0].ID).
				Update("enabled", true).Error; err != nil {
				Fail(c, http.StatusInternalServerError, "切换启用内容项失败: "+err.Error())
				return
			}
		}
	}
	OK(c, gin.H{"deleted": itemID})
}

// applyRuleSetItem 把请求字段应用到内容项并校验地址与行为。
func (a *API) applyRuleSetItem(item *db.RuleSetItem, req ruleSetItemReq) error {
	item.Remark = strings.TrimSpace(req.Remark)
	item.SingBoxURL = strings.TrimSpace(req.SingBoxURL)
	item.MihomoURL = strings.TrimSpace(req.MihomoURL)
	item.MihomoBehavior = req.MihomoBehavior
	item.Enabled = req.Enabled
	if item.SingBoxURL == "" && item.MihomoURL == "" {
		return fmt.Errorf("sing-box 与 mihomo 源地址至少填一个")
	}
	// 规则在服务端也要生效（内核跑在面板进程里），缺少 sing-box 源会让配置生成出无效规则集
	if item.Enabled && item.SingBoxURL == "" {
		return fmt.Errorf("启用中的内容项必须填写 sing-box 源地址（.srs），否则服务端内核无法加载该规则集")
	}
	for _, u := range []string{item.SingBoxURL, item.MihomoURL} {
		if u != "" && !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
			return fmt.Errorf("源地址须以 http:// 或 https:// 开头: %s", u)
		}
	}
	if item.MihomoBehavior == "" {
		item.MihomoBehavior = "domain"
	}
	if !contains(mihomoBehaviors, item.MihomoBehavior) {
		return fmt.Errorf("mihomo 行为不支持: %s（可选 domain/ipcidr/classical）", item.MihomoBehavior)
	}
	return nil
}

// ruleSetItemViewOf 内容项转视图。
func ruleSetItemViewOf(it db.RuleSetItem) ruleSetItemView {
	return ruleSetItemView{
		ID: it.ID, Remark: it.Remark, SingBoxURL: it.SingBoxURL,
		MihomoURL: it.MihomoURL, MihomoBehavior: it.MihomoBehavior, Enabled: it.Enabled,
	}
}

// ruleSetSourceOf 查询规则集启用中的内容项（配置生成用）。
func (a *API) ruleSetSourceOf(setID uint) (*singbox.RuleSetSource, error) {
	var set db.RuleSet
	if err := a.db.First(&set, setID).Error; err != nil {
		return nil, fmt.Errorf("规则集不存在: %d", setID)
	}
	var item db.RuleSetItem
	if err := a.db.Where("rule_set_id = ? AND enabled = ?", setID, true).First(&item).Error; err != nil {
		return nil, fmt.Errorf("规则集 %s 没有启用中的内容项", set.Name)
	}
	return &singbox.RuleSetSource{
		Tag: set.Name, URL: item.SingBoxURL,
		MihomoURL: item.MihomoURL, MihomoBehavior: item.MihomoBehavior,
	}, nil
}
