package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

// ruleSetResp 规则集视图（测试断言用）。
type ruleSetResp struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Builtin     bool   `json:"builtin"`
	UsedByRules int    `json:"usedByRules"`
	Items       []struct {
		ID             uint   `json:"id"`
		Remark         string `json:"remark"`
		SingBoxURL     string `json:"singBoxUrl"`
		MihomoURL      string `json:"mihomoUrl"`
		MihomoBehavior string `json:"mihomoBehavior"`
		Enabled        bool   `json:"enabled"`
	} `json:"items"`
}

// listRuleSets 查询全部规则集。
func listRuleSets(t *testing.T, client *http.Client, srvURL string) []ruleSetResp {
	t.Helper()
	code, env := doJSON(t, client, http.MethodGet, srvURL+"/api/rule-sets", nil)
	if code != http.StatusOK {
		t.Fatalf("查询规则集失败: code=%d msg=%s", code, env.Msg)
	}
	var list []ruleSetResp
	decodeData(t, env, &list)
	return list
}

// findRuleSet 按名称取规则集。
func findRuleSet(t *testing.T, list []ruleSetResp, name string) ruleSetResp {
	t.Helper()
	for _, s := range list {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("规则集 %s 不存在: %+v", name, list)
	return ruleSetResp{}
}

// TestRuleValidation 规则创建校验：规则集引用、IP 匹配方式、域名匹配方式。
func TestRuleValidation(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	// 先查一次规则集以写入内置项
	sets := listRuleSets(t, client, srv.URL)
	if len(sets) == 0 {
		t.Fatal("应有内置规则集")
	}
	antiAD := findRuleSet(t, sets, "anti-ad")
	if !antiAD.Builtin || len(antiAD.Items) == 0 {
		t.Fatalf("anti-ad 应为内置且带内容项: %+v", antiAD)
	}

	// 引用内置规则集 → 200
	code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/rules", map[string]any{
		"matchType": "rule_set", "ruleSetId": antiAD.ID, "outbound": "block",
	})
	if code != http.StatusOK {
		t.Fatalf("引用内置规则集应成功: code=%d msg=%s", code, env.Msg)
	}
	var rule struct {
		ID          uint   `json:"id"`
		MatchType   string `json:"matchType"`
		RuleSetID   uint   `json:"ruleSetId"`
		RuleSetName string `json:"ruleSetName"`
		Outbound    string `json:"outbound"`
		Enabled     bool   `json:"enabled"`
	}
	decodeData(t, env, &rule)
	if rule.RuleSetName != "anti-ad" || rule.MatchType != "rule_set" || !rule.Enabled {
		t.Errorf("规则视图不符: %+v", rule)
	}

	// IP 规则用 suffix → 400
	code, env = doJSON(t, client, http.MethodPost, srv.URL+"/api/rules", map[string]any{
		"matchType": "ip", "matchMethod": "suffix", "value": "10.0.0.0/8", "outbound": "direct",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("IP + suffix 应 400, got %d: %s", code, env.Msg)
	}

	// IP 规则用 cidr → 200
	code, _ = doJSON(t, client, http.MethodPost, srv.URL+"/api/rules", map[string]any{
		"matchType": "ip", "matchMethod": "cidr", "value": "10.0.0.0/8", "outbound": "direct",
	})
	if code != http.StatusOK {
		t.Fatalf("IP + cidr 应成功, got %d", code)
	}

	// rule_set 缺 ruleSetId → 400
	code, env = doJSON(t, client, http.MethodPost, srv.URL+"/api/rules", map[string]any{
		"matchType": "rule_set", "outbound": "block",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("rule_set 缺 ruleSetId 应 400, got %d: %s", code, env.Msg)
	}

	// 引用不存在的规则集 → 400
	code, _ = doJSON(t, client, http.MethodPost, srv.URL+"/api/rules", map[string]any{
		"matchType": "rule_set", "ruleSetId": 999999, "outbound": "block",
	})
	if code != http.StatusBadRequest {
		t.Errorf("引用不存在规则集应 400, got %d", code)
	}

	// 域名规则匹配方式非法 → 400；空匹配值 → 400；空出站 → 400
	for _, body := range []map[string]any{
		{"matchType": "domain", "matchMethod": "cidr", "value": "a.com", "outbound": "direct"},
		{"matchType": "domain", "matchMethod": "suffix", "value": "  ", "outbound": "direct"},
		{"matchType": "domain", "matchMethod": "suffix", "value": "a.com"},
		{"matchType": "geoip", "matchMethod": "cidr", "value": "cn", "outbound": "direct"},
	} {
		if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/rules", body); code != http.StatusBadRequest {
			t.Errorf("非法规则应 400: %+v, got %d: %s", body, code, env.Msg)
		}
	}

	// 列表按排序返回
	code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/rules", nil)
	if code != http.StatusOK {
		t.Fatalf("查询规则失败: %s", env.Msg)
	}
	var rules []map[string]any
	decodeData(t, env, &rules)
	if len(rules) != 2 {
		t.Fatalf("应有 2 条规则, got %d: %v", len(rules), rules)
	}

	// 删除内置规则集 → 400
	code, env = doJSON(t, client, http.MethodDelete, srv.URL+"/api/rule-sets/"+itoa(antiAD.ID), nil)
	if code != http.StatusBadRequest {
		t.Fatalf("删除内置规则集应 400, got %d: %s", code, env.Msg)
	}
}

// TestRuleSetItemExclusive 同一规则集同时只有一条启用中的内容项。
func TestRuleSetItemExclusive(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	// 自定义规则集，初始无内容项
	code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-sets", map[string]any{
		"name": "custom-set", "description": "自定义",
	})
	if code != http.StatusOK {
		t.Fatalf("创建规则集失败: code=%d msg=%s", code, env.Msg)
	}
	var set ruleSetResp
	decodeData(t, env, &set)
	if set.Builtin {
		t.Error("自定义规则集不应标记为内置")
	}

	// 第一条内容项（启用）
	code, env = doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-sets/"+itoa(set.ID)+"/items",
		map[string]any{
			"remark": "源一", "singBoxUrl": "https://example.com/a.srs",
			"mihomoUrl": "https://example.com/a.mrs", "mihomoBehavior": "domain", "enabled": true,
		})
	if code != http.StatusOK {
		t.Fatalf("创建内容项失败: code=%d msg=%s", code, env.Msg)
	}
	var first struct {
		ID uint `json:"id"`
	}
	decodeData(t, env, &first)

	// 第二条内容项（启用）→ 第一条应被停用
	code, env = doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-sets/"+itoa(set.ID)+"/items",
		map[string]any{
			"remark": "源二", "singBoxUrl": "https://example.com/b.srs",
			"mihomoUrl": "https://example.com/b.yaml", "mihomoBehavior": "ipcidr", "enabled": true,
		})
	if code != http.StatusOK {
		t.Fatalf("创建第二条内容项失败: code=%d msg=%s", code, env.Msg)
	}

	// 列表里只有一条启用
	sets := listRuleSets(t, client, srv.URL)
	updated := findRuleSet(t, sets, "custom-set")
	if len(updated.Items) != 2 {
		t.Fatalf("应有 2 条内容项: %+v", updated.Items)
	}
	enabled := 0
	for _, it := range updated.Items {
		if it.Enabled {
			enabled++
			if it.Remark != "源二" {
				t.Errorf("启用项应是最新添加的: %+v", it)
			}
		}
	}
	if enabled != 1 {
		t.Fatalf("应恰有 1 条启用中的内容项, got %d: %+v", enabled, updated.Items)
	}

	// 切回第一条 → 第二条被停用
	code, env = doJSON(t, client, http.MethodPut, srv.URL+"/api/rule-sets/items/"+itoa(first.ID),
		map[string]any{
			"remark": "源一", "singBoxUrl": "https://example.com/a.srs",
			"mihomoUrl": "https://example.com/a.mrs", "mihomoBehavior": "domain", "enabled": true,
		})
	if code != http.StatusOK {
		t.Fatalf("切换启用项失败: code=%d msg=%s", code, env.Msg)
	}
	updated = findRuleSet(t, listRuleSets(t, client, srv.URL), "custom-set")
	enabled = 0
	for _, it := range updated.Items {
		if it.Enabled {
			enabled++
			if it.ID != first.ID {
				t.Errorf("启用项应切回第一条: %+v", it)
			}
		}
	}
	if enabled != 1 {
		t.Errorf("切换后应恰有 1 条启用, got %d", enabled)
	}

	// 非法源地址 / 行为 → 400
	for _, body := range []map[string]any{
		{"singBoxUrl": "ftp://example.com/a.srs", "enabled": true},
		{"singBoxUrl": "https://example.com/a.srs", "mihomoBehavior": "nope", "enabled": true},
		{"remark": "空源", "enabled": true},
	} {
		if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-sets/"+itoa(set.ID)+"/items", body); code != http.StatusBadRequest {
			t.Errorf("非法内容项应 400: %+v, got %d: %s", body, code, env.Msg)
		}
	}
}

// TestRuleSetDeleteGuards 内置规则集不可删；被规则引用时不可删。
func TestRuleSetDeleteGuards(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	// 自定义规则集 + 内容项
	code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-sets", map[string]any{"name": "ref-set"})
	if code != http.StatusOK {
		t.Fatalf("创建规则集失败: %s", env.Msg)
	}
	var set ruleSetResp
	decodeData(t, env, &set)
	code, env = doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-sets/"+itoa(set.ID)+"/items",
		map[string]any{"singBoxUrl": "https://example.com/a.srs", "enabled": true})
	if code != http.StatusOK {
		t.Fatalf("创建内容项失败: %s", env.Msg)
	}

	// 被规则引用 → 删除 400
	code, env = doJSON(t, client, http.MethodPost, srv.URL+"/api/rules", map[string]any{
		"matchType": "rule_set", "ruleSetId": set.ID, "outbound": "block",
	})
	if code != http.StatusOK {
		t.Fatalf("创建规则失败: %s", env.Msg)
	}
	var rule struct {
		ID uint `json:"id"`
	}
	decodeData(t, env, &rule)
	code, env = doJSON(t, client, http.MethodDelete, srv.URL+"/api/rule-sets/"+itoa(set.ID), nil)
	if code != http.StatusBadRequest {
		t.Fatalf("被引用规则集删除应 400, got %d: %s", code, env.Msg)
	}

	// 删除规则后可删
	if code, env := doJSON(t, client, http.MethodDelete, srv.URL+"/api/rules/"+itoa(rule.ID), nil); code != http.StatusOK {
		t.Fatalf("删除规则失败: %s", env.Msg)
	}
	if code, env := doJSON(t, client, http.MethodDelete, srv.URL+"/api/rule-sets/"+itoa(set.ID), nil); code != http.StatusOK {
		t.Fatalf("删除未被引用的自定义规则集应成功: %s", env.Msg)
	}

	// 至少保留一条内容项
	sets := listRuleSets(t, client, srv.URL)
	antiAD := findRuleSet(t, sets, "anti-ad")
	if len(antiAD.Items) != 1 {
		t.Fatalf("内置规则集应有 1 条内容项: %+v", antiAD.Items)
	}
	code, env = doJSON(t, client, http.MethodDelete,
		srv.URL+"/api/rule-sets/items/"+strconv.FormatUint(uint64(antiAD.Items[0].ID), 10), nil)
	if code != http.StatusBadRequest {
		t.Errorf("删除唯一内容项应 400, got %d: %s", code, env.Msg)
	}
}

// TestDNSConfigEndpoints DNS 设置读取与校验。
func TestDNSConfigEndpoints(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	// GET：默认配置 + 类型 + 策略 + 预设
	code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/dns", nil)
	if code != http.StatusOK {
		t.Fatalf("查询 DNS 设置失败: code=%d msg=%s", code, env.Msg)
	}
	var view struct {
		Config struct {
			Enabled  bool `json:"enabled"`
			Strategy string
			Servers  []map[string]any `json:"servers"`
			Rules    []map[string]any `json:"rules"`
			Final    string           `json:"final"`
		} `json:"config"`
		Types   []string `json:"types"`
		Strats  []string `json:"strategies"`
		Presets []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Config struct {
				Enabled bool `json:"enabled"`
			} `json:"config"`
		} `json:"presets"`
	}
	decodeData(t, env, &view)
	if !view.Config.Enabled {
		t.Error("默认 DNS 配置应启用")
	}
	if len(view.Config.Servers) == 0 {
		t.Error("默认 DNS 配置应有服务器")
	}
	if len(view.Types) == 0 || len(view.Strats) == 0 {
		t.Errorf("类型/策略列表不应为空: types=%v strategies=%v", view.Types, view.Strats)
	}
	if len(view.Presets) == 0 {
		t.Error("应返回内置预设")
	}

	// PUT 非法策略 → 400
	code, env = doJSON(t, client, http.MethodPut, srv.URL+"/api/dns", map[string]any{
		"enabled": true, "strategy": "prefer_ipv7",
		"servers": []map[string]any{{"tag": "ali", "type": "udp", "server": "223.5.5.5"}},
	})
	if code != http.StatusBadRequest {
		t.Fatalf("非法解析策略应 400, got %d: %s", code, env.Msg)
	}

	// PUT 启用但无服务器 → 400
	code, _ = doJSON(t, client, http.MethodPut, srv.URL+"/api/dns", map[string]any{
		"enabled": true, "servers": []map[string]any{},
	})
	if code != http.StatusBadRequest {
		t.Errorf("启用但无服务器应 400, got %d", code)
	}

	// PUT 合法 → 200 且回读一致
	payload := map[string]any{
		"enabled":  true,
		"strategy": "ipv4_only",
		"servers": []map[string]any{
			{"tag": "ali", "type": "udp", "server": "223.5.5.5", "serverPort": 53},
			{"tag": "doh", "type": "https", "server": "1.1.1.1", "serverPort": 443, "path": "/dns-query"},
		},
		"rules": []map[string]any{{"domainSuffix": "cn", "server": "ali"}},
		"final": "doh",
	}
	code, env = doJSON(t, client, http.MethodPut, srv.URL+"/api/dns", payload)
	if code != http.StatusOK {
		t.Fatalf("保存 DNS 配置失败: code=%d msg=%s", code, env.Msg)
	}
	code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/dns", nil)
	if code != http.StatusOK {
		t.Fatalf("回读 DNS 配置失败: %s", env.Msg)
	}
	var reread struct {
		Config struct {
			Enabled  bool             `json:"enabled"`
			Strategy string           `json:"strategy"`
			Servers  []map[string]any `json:"servers"`
			Final    string           `json:"final"`
		} `json:"config"`
	}
	decodeData(t, env, &reread)
	if !reread.Config.Enabled || reread.Config.Strategy != "ipv4_only" || reread.Config.Final != "doh" {
		t.Errorf("DNS 配置回读不一致: %+v", reread.Config)
	}
	if len(reread.Config.Servers) != 2 {
		t.Errorf("DNS 服务器数量回读不一致: %+v", reread.Config.Servers)
	}

	// 非法 JSON 结构 → 400
	code, _ = doJSON(t, client, http.MethodPut, srv.URL+"/api/dns", "not-an-object")
	if code != http.StatusBadRequest {
		t.Errorf("非法请求体应 400, got %d", code)
	}
}

// TestRuleSetCreateValidation 规则集名称校验。
func TestRuleSetCreateValidation(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	for _, body := range []map[string]any{
		{"name": ""},
		{"name": "bad name with space"},
		{"name": "bad/name"},
	} {
		if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-sets", body); code != http.StatusBadRequest {
			t.Errorf("非法名称应 400: %+v, got %d: %s", body, code, env.Msg)
		}
	}

	// 与内置同名 → 400
	if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-sets",
		map[string]any{"name": "anti-ad"}); code != http.StatusBadRequest {
		t.Errorf("与内置同名应 400, got %d: %s", code, env.Msg)
	}

	// 合法 → 200
	code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-sets",
		map[string]any{"name": "my-set", "description": "说明"})
	if code != http.StatusOK {
		t.Fatalf("创建规则集失败: %s", env.Msg)
	}
	var set ruleSetResp
	decodeData(t, env, &set)
	if set.Name != "my-set" || set.Description != "说明" || len(set.Items) != 0 {
		t.Errorf("规则集视图不符: %+v", set)
	}
}

// TestRuleTemplateApply 模板应用生成规则与策略组，并设置兜底出站。
func TestRuleTemplateApply(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-templates/cn-direct-adblock/apply", nil)
	if code != http.StatusOK {
		t.Fatalf("应用模板失败: code=%d msg=%s", code, env.Msg)
	}
	var result struct {
		Template string `json:"template"`
		Rules    int    `json:"rules"`
		Groups   int    `json:"groups"`
		Final    string `json:"final"`
	}
	decodeData(t, env, &result)
	if result.Template != "cn-direct-adblock" || result.Rules == 0 || result.Groups == 0 {
		t.Errorf("模板应用结果不符: %+v", result)
	}
	if result.Final != "proxy" {
		t.Errorf("兜底出站 = %q, want proxy", result.Final)
	}

	// 规则与策略组确实落库
	code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/rules", nil)
	var rules []map[string]any
	decodeData(t, env, &rules)
	if code != http.StatusOK || len(rules) != result.Rules {
		t.Errorf("规则数量不符: got %d, want %d", len(rules), result.Rules)
	}
	code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/policy-groups", nil)
	var groups []map[string]any
	decodeData(t, env, &groups)
	if code != http.StatusOK || len(groups) != result.Groups {
		t.Errorf("策略组数量不符: got %d, want %d", len(groups), result.Groups)
	}
	code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/rules/final", nil)
	var final struct {
		Final string `json:"final"`
	}
	decodeData(t, env, &final)
	if final.Final != "proxy" {
		t.Errorf("兜底出站设置 = %q, want proxy", final.Final)
	}

	// 未知模板 → 404
	if code, _ := doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-templates/nope/apply", nil); code != http.StatusNotFound {
		t.Errorf("未知模板应 404, got %d", code)
	}
}

// TestStatsEndpoints 统计接口在未启动内核时返回零值。
func TestStatsEndpoints(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	// 未登录 401
	if code, _ := doJSON(t, newClient(t), http.MethodGet, srv.URL+"/api/stats/realtime", nil); code != http.StatusUnauthorized {
		t.Errorf("未登录访问统计应 401")
	}

	code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/stats/realtime", nil)
	if code != http.StatusOK {
		t.Fatalf("实时统计查询失败: %s", env.Msg)
	}
	var realtime struct {
		Up   int64 `json:"up"`
		Down int64 `json:"down"`
	}
	decodeData(t, env, &realtime)
	if realtime.Up != 0 || realtime.Down != 0 {
		t.Errorf("内核未运行时速率应为零值: %+v", realtime)
	}

	// 按用户流量：新建用户后包含零值行
	in := createInbound(t, client, srv.URL, map[string]any{"name": "vless-stats", "protocol": "vless"})
	createToken(t, client, srv.URL, map[string]any{"name": "stat-user", "nodeInbounds": []uint{in.ID}})
	code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/stats/user-traffic", nil)
	if code != http.StatusOK {
		t.Fatalf("按用户流量查询失败: %s", env.Msg)
	}
	var rows []struct {
		Tag         string `json:"tag"`
		Subscriber  string `json:"subscriber"`
		InboundName string `json:"inboundName"`
		Protocol    string `json:"protocol"`
	}
	decodeData(t, env, &rows)
	if len(rows) != 1 {
		t.Fatalf("应有 1 行用户流量: %+v", rows)
	}
	if rows[0].Subscriber != "stat-user" || rows[0].InboundName != "vless-stats" || rows[0].Protocol != "vless" {
		t.Errorf("用户流量行不符: %+v", rows[0])
	}
}

// TestJSONFields 确保 envelope 结构可被前端消费（success/msg/data）。
func TestJSONFields(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/inbounds", nil)
	if code != http.StatusOK || !env.Success || env.Msg != "ok" {
		t.Fatalf("envelope 不符: code=%d %+v", code, env)
	}
	var list []json.RawMessage
	if err := json.Unmarshal(env.Data, &list); err != nil {
		t.Fatalf("data 应为数组: %v", err)
	}
	// 无入站时返回空数组而非 null
	if len(list) != 0 {
		t.Errorf("新建前应有空入站列表: %s", env.Data)
	}
}
