package server

import (
	"net/http"
	"strings"
	"testing"
)

// TestFixedGroupSubscriptionIncludesNodes 复现用户报告的场景：应用规则模板（无自定义出站，
// proxy 组成员在服务端只有 direct），把策略组模式改成「固定代理」后拉订阅——
// 客户端 proxy 组必须仍包含该用户的节点，不能只剩 DIRECT。
func TestFixedGroupSubscriptionIncludesNodes(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	in := createInbound(t, client, srv.URL, map[string]any{"name": "vless-fixed", "protocol": "vless"})
	view := createToken(t, client, srv.URL, map[string]any{
		"name": "fixed-user", "nodeInbounds": []uint{in.ID},
	})
	if code, env := doJSON(t, client, http.MethodPost,
		srv.URL+"/api/rule-templates/cn-direct-adblock/apply", nil); code != http.StatusOK {
		t.Fatalf("应用规则模板失败: code=%d msg=%s", code, env.Msg)
	}

	// 找到模板生成的 proxy 组，把模式改成固定代理
	code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/policy-groups", nil)
	if code != http.StatusOK {
		t.Fatalf("查询策略组失败: %s", env.Msg)
	}
	var groups []struct {
		ID      uint     `json:"id"`
		Name    string   `json:"name"`
		Mode    string   `json:"mode"`
		Members []string `json:"members"`
	}
	decodeData(t, env, &groups)
	if len(groups) != 1 || groups[0].Name != "proxy" {
		t.Fatalf("模板应生成 1 个 proxy 组: %+v", groups)
	}
	code, env = doJSON(t, client, http.MethodPut, srv.URL+"/api/policy-groups/"+itoa(groups[0].ID),
		map[string]any{"mode": "fixed"})
	if code != http.StatusOK {
		t.Fatalf("改成固定代理失败: code=%d msg=%s", code, env.Msg)
	}

	// sing-box 订阅：proxy selector 必须含节点
	code, _, body := getSub(t, srv.URL+"/sub/"+view.Token+"?format=singbox", "SFM")
	if code != http.StatusOK {
		t.Fatalf("拉取 sing-box 订阅失败: code=%d body=%s", code, body)
	}
	group := subOutbounds(t, body)["proxy"]
	if group == nil {
		t.Fatalf("订阅应包含 proxy 组:\n%s", body)
	}
	outbounds, _ := group["outbounds"].([]any)
	if len(outbounds) == 0 {
		t.Fatalf("固定代理组不应为空: %v", group)
	}
	hasNode := false
	for _, o := range outbounds {
		if o == "vless-fixed" {
			hasNode = true
		}
	}
	if !hasNode {
		t.Errorf("固定代理组必须包含订阅节点，不能只含 %v:\n%s", outbounds, body)
	}

	// clash 订阅同样不能只剩 DIRECT
	code, _, clashBody := getSub(t, srv.URL+"/sub/"+view.Token+"?format=clash", "clash-verge-rev")
	if code != http.StatusOK {
		t.Fatalf("拉取 clash 订阅失败: code=%d body=%s", code, clashBody)
	}
	if strings.Contains(clashBody, "- DIRECT\n") && !strings.Contains(clashBody, "vless-fixed") {
		t.Errorf("clash 固定代理组不应只剩 DIRECT:\n%s", clashBody)
	}
}
