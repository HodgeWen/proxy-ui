package api

import (
	"strings"
	"testing"

	"proxy-ui/internal/db"
)

// TestApplyRuleReqRegex 域名正则规则保存前预校验：坏正则与超长 pattern 拒绝，合法单值/多值通过。
func TestApplyRuleReqRegex(t *testing.T) {
	a := newInboundTestAPI(t)

	// 合法正则（多值逗号分隔）通过
	rule := db.Rule{}
	if err := a.applyRuleReq(&rule, ruleReq{
		MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodRegex,
		Value:    `^(www\.)?example\.com$,^[a-z]+\.cn$`,
		Outbound: "direct",
	}); err != nil {
		t.Fatalf("合法正则应通过: %v", err)
	}

	// 多值里混入坏正则时拒绝，给出可读业务错误
	err := a.applyRuleReq(&db.Rule{}, ruleReq{
		MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodRegex,
		Value: `example\.com,([a-z`, Outbound: "direct",
	})
	if err == nil || !strings.Contains(err.Error(), "正则") {
		t.Errorf("坏正则应报业务错误, got %v", err)
	}

	// 超长 pattern 拒绝，恰好上限通过
	over := strings.Repeat("a", maxRulePatternLen+1)
	err = a.applyRuleReq(&db.Rule{}, ruleReq{
		MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodRegex, Value: over, Outbound: "direct",
	})
	if err == nil || !strings.Contains(err.Error(), "最长") {
		t.Errorf("超长正则应报错, got %v", err)
	}
	err = a.applyRuleReq(&db.Rule{}, ruleReq{
		MatchType: db.RuleMatchDomain, MatchMethod: db.RuleMethodRegex,
		Value: strings.Repeat("a", maxRulePatternLen), Outbound: "direct",
	})
	if err != nil {
		t.Errorf("恰好上限长度的正则应通过: %v", err)
	}
}
