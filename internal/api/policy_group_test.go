package api

import (
	"encoding/json"
	"strings"
	"testing"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// newGroupTestAPI 构造带临时库的 API（不启动内核）。
func newGroupTestAPI(t *testing.T) *API {
	t.Helper()
	return newInboundTestAPI(t)
}

// mustCreateGroup 创建策略组并返回视图。
func mustCreateGroup(t *testing.T, a *API, req groupReq) groupView {
	t.Helper()
	view, err := a.createGroup(req)
	if err != nil {
		t.Fatalf("创建策略组失败: %v", err)
	}
	return view
}

// TestCreateGroupValidatesMembers 成员必须是可用 tag，且不能自引用 / 循环引用。
func TestCreateGroupValidatesMembers(t *testing.T) {
	a := newGroupTestAPI(t)
	mustCreateGroup(t, a, groupReq{Name: "inner", Mode: singbox.GroupModeAuto, Members: []string{"direct"}})

	// 不存在的成员：保存处直接报错，而不是拖到内核重建
	if _, err := a.createGroup(groupReq{Name: "bad", Mode: singbox.GroupModeAuto, Members: []string{"ghost"}}); err == nil ||
		!strings.Contains(err.Error(), "不存在") {
		t.Errorf("不存在的成员应报错, got %v", err)
	}
	// 合法：引用已有策略组与 direct
	if _, err := a.createGroup(groupReq{Name: "outer", Mode: singbox.GroupModeFixed, Members: []string{"inner", "direct"}}); err != nil {
		t.Errorf("合法成员应通过: %v", err)
	}
	// 自引用
	if _, err := a.createGroup(groupReq{Name: "self", Mode: singbox.GroupModeAuto, Members: []string{"self"}}); err == nil {
		t.Error("自引用应报错")
	}
	// 循环引用：inner 引用 outer，outer 已引用 inner
	inner := a.groupByName(t, "inner")
	if _, err := a.updateGroup(inner.ID, groupReq{Members: []string{"outer"}}); err == nil ||
		!strings.Contains(err.Error(), "循环") {
		t.Errorf("循环引用应报错, got %v", err)
	}
	// 与内置动作同名的名称被拒绝
	for _, name := range []string{"direct", "block"} {
		if _, err := a.createGroup(groupReq{Name: name, Mode: singbox.GroupModeAuto, Members: []string{"direct"}}); err == nil {
			t.Errorf("名称 %s 为保留字，应报错", name)
		}
	}
}

// TestRenameGroupUpdatesReferences 策略组改名要级联改掉规则、其它组成员与兜底出站里的引用。
func TestRenameGroupUpdatesReferences(t *testing.T) {
	a := newGroupTestAPI(t)
	inner := mustCreateGroup(t, a, groupReq{Name: "inner", Mode: singbox.GroupModeAuto, Members: []string{"direct"}})
	mustCreateGroup(t, a, groupReq{Name: "outer", Mode: singbox.GroupModeFixed, Members: []string{"inner"}})
	if _, err := a.createRule(ruleReq{
		MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodSuffix, Value: "x.com", Outbound: "inner",
	}); err != nil {
		t.Fatalf("创建规则失败: %v", err)
	}
	if err := a.setSetting(db.SettingRouteFinal, "inner"); err != nil {
		t.Fatal(err)
	}

	if _, err := a.updateGroup(inner.ID, groupReq{Name: "renamed"}); err != nil {
		t.Fatalf("改名失败: %v", err)
	}

	var rules []db.Rule
	a.db.Find(&rules)
	if len(rules) != 1 || rules[0].Outbound != "renamed" {
		t.Errorf("规则出站应跟随改名: %+v", rules)
	}
	var groups []db.PolicyGroup
	a.db.Order("id").Find(&groups)
	outer := groupEntryOf(groups[1])
	if len(outer.Members) != 1 || outer.Members[0] != "renamed" {
		t.Errorf("其它策略组成员应跟随改名: %+v", outer.Members)
	}
	if got := a.getSetting(db.SettingRouteFinal); got != "renamed" {
		t.Errorf("兜底出站应跟随改名, got %q", got)
	}
	// 改名后内核配置应能聚合（无悬空引用）
	if _, err := a.buildKernelConfig(); err != nil {
		t.Errorf("改名后内核配置应可聚合: %v", err)
	}
}

// TestDeleteGroupRejectsReferenced 被引用的策略组不得删除；解除引用后可删。
func TestDeleteGroupRejectsReferenced(t *testing.T) {
	a := newGroupTestAPI(t)
	view := mustCreateGroup(t, a, groupReq{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"direct"}})
	if _, err := a.createRule(ruleReq{
		MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodSuffix, Value: "x.com", Outbound: "proxy",
	}); err != nil {
		t.Fatalf("创建规则失败: %v", err)
	}

	if err := a.deleteGroup(view.ID); err == nil || !strings.Contains(err.Error(), "引用") {
		t.Fatalf("被规则引用的策略组应拒绝删除, got %v", err)
	}

	var rules []db.Rule
	a.db.Find(&rules)
	if err := a.deleteRule(rules[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := a.deleteGroup(view.ID); err != nil {
		t.Errorf("解除引用后应可删除: %v", err)
	}
}

// TestRenameOutboundUpdatesReferences 自定义出站改名同样级联规则与策略组成员。
func TestRenameOutboundUpdatesReferences(t *testing.T) {
	a := newGroupTestAPI(t)
	ob, err := a.createOutbound(outboundReq{
		Name: "hk", Type: "socks", Config: []byte(`{"server":"127.0.0.1","server_port":1080}`),
	})
	if err != nil {
		t.Fatalf("创建出站失败: %v", err)
	}
	if _, err := a.createGroup(groupReq{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"hk"}}); err != nil {
		t.Fatalf("创建策略组失败: %v", err)
	}
	if _, err := a.createRule(ruleReq{
		MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodSuffix, Value: "x.com", Outbound: "hk",
	}); err != nil {
		t.Fatalf("创建规则失败: %v", err)
	}

	if _, err := a.updateOutbound(ob.ID, outboundReq{Name: "hk2"}); err != nil {
		t.Fatalf("出站改名失败: %v", err)
	}
	var rules []db.Rule
	a.db.Find(&rules)
	if len(rules) != 1 || rules[0].Outbound != "hk2" {
		t.Errorf("规则出站应跟随改名: %+v", rules)
	}
	groups := a.groupByName(t, "proxy")
	if len(groups.Members) != 1 || groups.Members[0] != "hk2" {
		t.Errorf("策略组成员应跟随改名: %+v", groups.Members)
	}
	if _, err := a.buildKernelConfig(); err != nil {
		t.Errorf("改名后内核配置应可聚合: %v", err)
	}
}

// TestDeleteOutboundRejectsReferenced 被规则或策略组引用的出站不得删除。
func TestDeleteOutboundRejectsReferenced(t *testing.T) {
	a := newGroupTestAPI(t)
	ob, err := a.createOutbound(outboundReq{
		Name: "hk", Type: "socks", Config: []byte(`{"server":"127.0.0.1","server_port":1080}`),
	})
	if err != nil {
		t.Fatalf("创建出站失败: %v", err)
	}
	if _, err := a.createGroup(groupReq{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"hk"}}); err != nil {
		t.Fatalf("创建策略组失败: %v", err)
	}
	if err := a.deleteOutbound(ob.ID); err == nil || !strings.Contains(err.Error(), "引用") {
		t.Fatalf("被策略组引用的出站应拒绝删除, got %v", err)
	}
}

// TestValidateRouteFinal 兜底出站取值校验：block 与不存在的 tag 拒绝，direct / 出站 / 策略组通过。
func TestValidateRouteFinal(t *testing.T) {
	a := newGroupTestAPI(t)
	if _, err := a.createOutbound(outboundReq{
		Name: "hk", Type: "socks", Config: []byte(`{"server":"127.0.0.1","server_port":1080}`),
	}); err != nil {
		t.Fatalf("创建出站失败: %v", err)
	}
	mustCreateGroup(t, a, groupReq{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"hk"}})

	for _, ok := range []string{"", "direct", "hk", "proxy"} {
		if err := a.validateRouteFinal(ok); err != nil {
			t.Errorf("兜底出站 %q 应合法: %v", ok, err)
		}
	}
	for _, bad := range []string{"block", "ghost"} {
		if err := a.validateRouteFinal(bad); err == nil {
			t.Errorf("兜底出站 %q 应被拒绝", bad)
		}
	}
	// 停用的策略组不能作为兜底（内核里不存在该出站）
	group := a.groupByName(t, "proxy")
	if err := a.db.Model(&db.PolicyGroup{}).Where("id = ?", group.ID).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.validateRouteFinal("proxy"); err == nil {
		t.Error("停用的策略组不应作为兜底出站")
	}
}

// groupByName 查询策略组视图；不存在时终止测试。
func (a *API) groupByName(t *testing.T, name string) groupView {
	t.Helper()
	views, err := a.groupViews()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range views {
		if v.Name == name {
			return v
		}
	}
	t.Fatalf("策略组 %s 不存在: %+v", name, views)
	return groupView{}
}

// TestDirectOutboundStillCreatable 内置 direct 出站仍可创建/编辑（保留字只挡自定义类型冒名）。
func TestDirectOutboundStillCreatable(t *testing.T) {
	a := newGroupTestAPI(t)
	view, err := a.createOutbound(outboundReq{Type: db.OutboundTypeDirect})
	if err != nil {
		t.Fatalf("创建 direct 出站应成功: %v", err)
	}
	if view.Name != db.OutboundTypeDirect {
		t.Errorf("direct 出站名称 = %q", view.Name)
	}
	// 编辑同一个 direct 出站同样应成功
	if _, err := a.updateOutbound(view.ID, outboundReq{Type: db.OutboundTypeDirect, Name: db.OutboundTypeDirect}); err != nil {
		t.Errorf("编辑 direct 出站应成功: %v", err)
	}
	// 但自定义类型不得冒名 direct
	if _, err := a.createOutbound(outboundReq{
		Name: db.OutboundTypeDirect, Type: "socks", Config: []byte(`{"server":"127.0.0.1","server_port":1080}`),
	}); err == nil {
		t.Error("自定义出站使用 direct 作为名称应被拒绝")
	}
}

// TestUpdatePathRejectsDisablingReferenced 停用被引用项的校验必须覆盖 PUT / AI 工具路径，
// 不能只有 PATCH 启停接口被挡住。
func TestUpdatePathRejectsDisablingReferenced(t *testing.T) {
	a := newGroupTestAPI(t)
	ob, err := a.createOutbound(outboundReq{
		Name: "hk", Type: "socks", Config: []byte(`{"server":"127.0.0.1","server_port":1080}`),
	})
	if err != nil {
		t.Fatalf("创建出站失败: %v", err)
	}
	group := mustCreateGroup(t, a, groupReq{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"hk"}})
	if _, err := a.createRule(ruleReq{
		MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodSuffix, Value: "x.com", Outbound: "proxy",
	}); err != nil {
		t.Fatalf("创建规则失败: %v", err)
	}

	// PUT 路径（updateGroup / updateOutbound）带 enabled=false
	if _, err := a.updateGroup(group.ID, groupReq{Enabled: new(false)}); err == nil ||
		!strings.Contains(err.Error(), "引用") {
		t.Errorf("update_policy_group 停用被引用的组应被拒绝, got %v", err)
	}
	if _, err := a.updateOutbound(ob.ID, outboundReq{Enabled: new(false)}); err == nil ||
		!strings.Contains(err.Error(), "引用") {
		t.Errorf("update_outbound 停用被引用的出站应被拒绝, got %v", err)
	}
	// 拒绝后状态不应被改动
	var reloaded db.PolicyGroup
	if err := a.db.First(&reloaded, group.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reloaded.Enabled {
		t.Error("被拒绝的停用请求不应改动数据")
	}
}

// TestRenameAndMemberChangeInOneRequest 改名与改成员同一次请求时不能绕出循环引用。
func TestRenameAndMemberChangeInOneRequest(t *testing.T) {
	a := newGroupTestAPI(t)
	a1 := mustCreateGroup(t, a, groupReq{Name: "a", Mode: singbox.GroupModeAuto, Members: []string{"direct"}})
	mustCreateGroup(t, a, groupReq{Name: "b", Mode: singbox.GroupModeAuto, Members: []string{"a"}})

	// 把 a 改名为 c 并让 c 引用 b：改名后 b→c，再让 c→b 就成环，必须被拒绝
	if _, err := a.updateGroup(a1.ID, groupReq{Name: "c", Members: []string{"b"}}); err == nil ||
		!strings.Contains(err.Error(), "循环") {
		t.Fatalf("改名+改成员构成环应被拒绝, got %v", err)
	}
	if _, err := a.buildKernelConfig(); err != nil {
		t.Errorf("拒绝后库内不应留下环: %v", err)
	}
}

// TestTemplateApplyWithProxyNamedOutbound 存在名为 proxy 的自定义出站时，应用模板
// 不能生成「组与出站同名 / 组引用自身」的坏数据。
func TestTemplateApplyWithProxyNamedOutbound(t *testing.T) {
	a := newGroupTestAPI(t)
	if _, err := a.createOutbound(outboundReq{
		Name: "proxy", Type: "socks", Config: []byte(`{"server":"127.0.0.1","server_port":1080}`),
	}); err != nil {
		t.Fatalf("创建出站失败: %v", err)
	}
	if _, err := a.createInbound(inboundReq{Name: "in-1", Protocol: singbox.ProtoVLESS}); err != nil {
		t.Fatalf("创建入站失败: %v", err)
	}
	if err := a.ensureBuiltinRuleSets(); err != nil {
		t.Fatal(err)
	}
	members := a.proxyMembers()
	for _, m := range members {
		if m == templateGroupName {
			t.Fatalf("模板组成员不应包含同名出站 proxy: %v", members)
		}
	}
}

// TestDirectOutboundNotRenamable 内置 direct 出站不能改名：所有 direct 引用指的都是直连语义。
func TestDirectOutboundNotRenamable(t *testing.T) {
	a := newGroupTestAPI(t)
	view, err := a.createOutbound(outboundReq{Type: db.OutboundTypeDirect})
	if err != nil {
		t.Fatalf("创建 direct 出站应成功: %v", err)
	}
	if _, err := a.createRule(ruleReq{
		MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodSuffix, Value: "cn", Outbound: "direct",
	}); err != nil {
		t.Fatalf("创建直连规则失败: %v", err)
	}
	if _, err := a.updateOutbound(view.ID, outboundReq{Name: "hk-direct"}); err == nil {
		t.Fatal("内置 direct 出站改名应被拒绝")
	}
	var rules []db.Rule
	a.db.Find(&rules)
	if len(rules) != 1 || rules[0].Outbound != "direct" {
		t.Errorf("直连规则不应被改名波及: %+v", rules)
	}
}

// TestReenablingRuleValidatesOutbound 规则停用期间出站被删后，重新启用必须被拒绝。
func TestReenablingRuleValidatesOutbound(t *testing.T) {
	a := newGroupTestAPI(t)
	ob, err := a.createOutbound(outboundReq{
		Name: "hk", Type: "socks", Config: []byte(`{"server":"127.0.0.1","server_port":1080}`),
	})
	if err != nil {
		t.Fatalf("创建出站失败: %v", err)
	}
	rule, err := a.createRule(ruleReq{
		MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodSuffix, Value: "x.com", Outbound: "hk",
	})
	if err != nil {
		t.Fatalf("创建规则失败: %v", err)
	}
	// 停用规则 → 删除出站 → 重新启用规则
	if err := a.db.Model(&db.Rule{}).Where("id = ?", rule.ID).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.deleteOutbound(ob.ID); err != nil {
		t.Fatalf("停用规则不参与的出站应可删除: %v", err)
	}
	if err := a.validateOutboundTag("direct"); err != nil {
		t.Fatal(err)
	}
	var reloaded db.Rule
	if err := a.db.First(&reloaded, rule.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.validateOutboundTag(reloaded.Outbound); err == nil {
		t.Error("出站已删除时，重新启用该规则应被拒绝")
	}
}

// TestRenameTagRefsDedupesMembers 改名重写引用后若产生重复成员，应一并去重。
func TestRenameTagRefsDedupesMembers(t *testing.T) {
	a := newGroupTestAPI(t)
	// 直接构造带重复潜力的数据：组内同时有 hk 与 hk2（hk2 先作为历史残留）
	group := db.PolicyGroup{Name: "proxy", Mode: singbox.GroupModeAuto, Members: `["hk","hk2"]`, Enabled: true}
	if err := a.db.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.renameTagRefs(a.db, "hk", "hk2"); err != nil {
		t.Fatalf("重写引用失败: %v", err)
	}
	var reloaded db.PolicyGroup
	if err := a.db.First(&reloaded, group.ID).Error; err != nil {
		t.Fatal(err)
	}
	members := groupEntryOf(reloaded).Members
	if len(members) != 1 || members[0] != "hk2" {
		t.Errorf("改名后重复成员应去重, got %v", members)
	}
}

// TestTemplateApplyRejectsOutboundNameCollision 已存在名为 proxy 的自定义出站时，
// 模板应用必须报出可读错误，而不是写出同名组让配置永久起不来。
func TestTemplateApplyRejectsOutboundNameCollision(t *testing.T) {
	a := newGroupTestAPI(t)
	if _, err := a.createOutbound(outboundReq{
		Name: "proxy", Type: "socks", Config: []byte(`{"server":"127.0.0.1","server_port":1080}`),
	}); err != nil {
		t.Fatalf("创建出站失败: %v", err)
	}
	// proxyMembers 排除同名出站后，模板组只有一个 direct 成员，可以正常应用
	members := a.proxyMembers()
	if len(members) != 1 || members[0] != "direct" {
		t.Fatalf("proxyMembers 应排除同名出站: %v", members)
	}
	group := db.PolicyGroup{Name: templateGroupName, Mode: singbox.GroupModeAuto, Members: marshalMembers(members)}
	if err := a.applyGroupReq(a.db, &group, groupReq{
		Name: group.Name, Mode: group.Mode, Members: groupEntryOf(group).Members,
	}, 0); err == nil {
		t.Error("模板组与同名出站冲突时应被 applyGroupReq 拒绝")
	}
}

// TestGroupMembersDeduped 重复成员在保存时去重。
func TestGroupMembersDeduped(t *testing.T) {
	a := newGroupTestAPI(t)
	view := mustCreateGroup(t, a, groupReq{
		Name: "proxy", Mode: singbox.GroupModeFixed, Members: []string{"direct", "direct"},
	})
	if len(view.Members) != 1 || view.Members[0] != "direct" {
		t.Errorf("重复成员应去重: %+v", view.Members)
	}
}

// TestTagRefCount 引用计数口径：启用中的规则出站 + 启用中的策略组成员 + 兜底出站 + DNS detour。
func TestTagRefCount(t *testing.T) {
	a := newGroupTestAPI(t)
	if _, err := a.createOutbound(outboundReq{
		Name: "hk", Type: "socks", Config: []byte(`{"server":"127.0.0.1","server_port":1080}`),
	}); err != nil {
		t.Fatalf("创建出站失败: %v", err)
	}
	mustCreateGroup(t, a, groupReq{Name: "proxy", Mode: singbox.GroupModeAuto, Members: []string{"hk"}})
	rule, err := a.createRule(ruleReq{
		MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodSuffix, Value: "x.com", Outbound: "proxy",
	})
	if err != nil {
		t.Fatalf("创建规则失败: %v", err)
	}
	if err := a.setSetting(db.SettingRouteFinal, "proxy"); err != nil {
		t.Fatal(err)
	}

	// proxy：1 条规则 + 兜底出站 = 2；hk：1 个策略组成员 = 1
	if refs, err := a.tagRefCount(a.db, "proxy"); err != nil || refs != 2 {
		t.Errorf("proxy 引用数 = %d (err=%v), want 2", refs, err)
	}
	if refs, err := a.tagRefCount(a.db, "hk"); err != nil || refs != 1 {
		t.Errorf("hk 引用数 = %d (err=%v), want 1", refs, err)
	}

	// 删规则、清兜底后两者引用都归零
	if err := a.deleteRule(rule.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.setSetting(db.SettingRouteFinal, ""); err != nil {
		t.Fatal(err)
	}
	if refs, _ := a.tagRefCount(a.db, "proxy"); refs != 0 {
		t.Errorf("解除引用后 proxy 引用数应为 0, got %d", refs)
	}
	if refs, _ := a.tagRefCount(a.db, "hk"); refs != 1 {
		t.Errorf("hk 仍被策略组成员引用, got %d", refs)
	}

	// 停用的规则不参与聚合，因此不计入引用
	disabled, err := a.createRule(ruleReq{
		MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodSuffix, Value: "y.com", Outbound: "proxy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.db.Model(&db.Rule{}).Where("id = ?", disabled.ID).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if refs, _ := a.tagRefCount(a.db, "proxy"); refs != 0 {
		t.Errorf("停用的规则不应计入引用, got %d", refs)
	}
}

// TestDNSDetourReferentialIntegrity DNS 服务器的 detour 也是出站 tag 引用：改名要跟随，
// 被引用时不许删除。
func TestDNSDetourReferentialIntegrity(t *testing.T) {
	a := newGroupTestAPI(t)
	ob, err := a.createOutbound(outboundReq{
		Name: "hk", Type: "socks", Config: []byte(`{"server":"127.0.0.1","server_port":1080}`),
	})
	if err != nil {
		t.Fatalf("创建出站失败: %v", err)
	}
	dns := singbox.DNSConfig{
		Enabled: true,
		Servers: []singbox.DNSServer{{Tag: "doh", Type: singbox.DNSTypeHTTPS, Server: "1.1.1.1", Detour: "hk"}},
		Final:   "doh",
	}
	raw, _ := json.Marshal(dns)
	if err := a.setSetting(db.SettingDNS, string(raw)); err != nil {
		t.Fatal(err)
	}

	if refs, err := a.tagRefCount(a.db, "hk"); err != nil || refs != 1 {
		t.Fatalf("DNS detour 应计入引用, refs=%d err=%v", refs, err)
	}
	if err := a.deleteOutbound(ob.ID); err == nil {
		t.Fatal("被 DNS detour 引用的出站应拒绝删除")
	}
	if _, err := a.updateOutbound(ob.ID, outboundReq{Name: "hk2"}); err != nil {
		t.Fatalf("出站改名失败: %v", err)
	}
	var reloaded db.Setting
	if err := a.db.First(&reloaded, "key = ?", db.SettingDNS).Error; err != nil {
		t.Fatal(err)
	}
	var after singbox.DNSConfig
	if err := json.Unmarshal([]byte(reloaded.Value), &after); err != nil {
		t.Fatal(err)
	}
	if after.Servers[0].Detour != "hk2" {
		t.Errorf("DNS detour 应跟随出站改名, got %q", after.Servers[0].Detour)
	}
}

// TestRuleUpdateWithoutOutboundToleratesStaleTag 历史规则指向已停用出站时，仍能启停或改匹配值。
func TestRuleUpdateWithoutOutboundToleratesStaleTag(t *testing.T) {
	a := newGroupTestAPI(t)
	rule, err := a.createRule(ruleReq{
		MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodSuffix, Value: "x.com", Outbound: "direct",
	})
	if err != nil {
		t.Fatalf("创建规则失败: %v", err)
	}
	// 直接把库里的出站改成不存在的 tag，模拟历史脏数据
	if err := a.db.Model(&db.Rule{}).Where("id = ?", rule.ID).
		Update("outbound", "gone").Error; err != nil {
		t.Fatal(err)
	}
	// 只改匹配值（不给 outbound）应成功
	if _, err := a.updateRule(rule.ID, ruleReq{Value: "y.com"}); err != nil {
		t.Errorf("未改动出站的更新应允许: %v", err)
	}
	// 显式给不存在的出站仍要拒绝
	if _, err := a.updateRule(rule.ID, ruleReq{Outbound: "gone"}); err == nil {
		t.Error("显式指向不存在出站的更新应被拒绝")
	}
}
