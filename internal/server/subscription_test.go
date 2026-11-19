package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// setupSubscription 准备带 VLESS/Hysteria2 入站、anti-AD 模板规则与订阅令牌的面板服务，
// 返回服务 URL 与生成的匿名令牌。
func setupSubscription(t *testing.T) (string, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "panel.db")
	dataDir := t.TempDir()
	srv := httptest.NewServer(mustNewEngine(dbPath, dataDir))
	t.Cleanup(srv.Close)
	client := setupAdmin(t, srv.URL)

	// 两个启用入站（空配置按协议填默认值）
	for _, in := range []map[string]any{
		{"name": "vless-main", "protocol": "vless", "listenPort": 443},
		{"name": "hy2-main", "protocol": "hysteria2", "listenPort": 8443},
	} {
		if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/inbounds", in); code != http.StatusOK {
			t.Fatalf("创建入站失败: %s", env.Msg)
		}
	}
	// 应用「国内直连+海外代理+广告拦截」模板提供策略组与 anti-AD 规则
	if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-templates/cn-direct-adblock/apply", nil); code != http.StatusOK {
		t.Fatalf("应用模板失败: %s", env.Msg)
	}
	// 生成订阅令牌
	code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/subscription/tokens",
		map[string]string{"name": "手机"})
	if code != http.StatusOK {
		t.Fatalf("生成令牌失败: %s", env.Msg)
	}
	var view struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(env.Data, &view); err != nil {
		t.Fatal(err)
	}
	return srv.URL, view.Token
}

// getSub 拉取订阅响应。
func getSub(t *testing.T, url, ua string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	return resp, buf.String()
}

func TestSubscriptionEndpoint(t *testing.T) {
	srvURL, token := setupSubscription(t)

	// sing-box UA → sing-box JSON 配置：含节点、策略组与 anti-AD 规则集
	resp, body := getSub(t, srvURL+"/sub/"+token, "SFM/1.0 sing-box/1.11.0")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sing-box 订阅状态码 = %d: %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("sing-box 订阅 Content-Type = %q", ct)
	}
	cfg := map[string]any{}
	if err := json.Unmarshal([]byte(body), &cfg); err != nil {
		t.Fatalf("sing-box 订阅应为 JSON: %v\n%s", err, body)
	}
	tags := map[string]bool{}
	for _, ob := range cfg["outbounds"].([]any) {
		tags[ob.(map[string]any)["tag"].(string)] = true
	}
	if !tags["vless-main"] || !tags["hy2-main"] || !tags["proxy"] || !tags["direct"] {
		t.Fatalf("订阅应包含全部启用入站节点、策略组与 direct: %v", tags)
	}
	route := cfg["route"].(map[string]any)
	if route["final"] != "proxy" {
		t.Fatalf("兜底出站 = %v, want proxy", route["final"])
	}
	if findRule(t, cfg, "anti-ad") == nil || findRuleSet(t, cfg, "anti-ad") == nil {
		t.Fatal("订阅应包含 anti-AD 规则与规则集")
	}

	// mihomo UA → clash YAML 配置
	resp, body = getSub(t, srvURL+"/sub/"+token, "clash.meta/1.19.0 Mihomo")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clash 订阅状态码 = %d: %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/yaml") {
		t.Fatalf("clash 订阅 Content-Type = %q", ct)
	}
	for _, key := range []string{"proxies:", "proxy-groups:", "rules:", "rule-providers:", "RULE-SET,anti-ad,REJECT"} {
		if !strings.Contains(body, key) {
			t.Fatalf("clash 订阅缺少 %q:\n%s", key, body)
		}
	}

	// 参数覆盖 UA：format=clash 强制 clash
	_, body = getSub(t, srvURL+"/sub/"+token+"?format=clash", "SFM sing-box")
	if !strings.Contains(body, "proxy-groups:") {
		t.Fatalf("format=clash 应覆盖 UA 返回 clash 配置:\n%s", body)
	}
	// 参数覆盖 UA：format=singbox 强制 sing-box
	_, body = getSub(t, srvURL+"/sub/"+token+"?format=singbox", "Mihomo/1.19")
	if !strings.Contains(body, `"outbounds"`) {
		t.Fatalf("format=singbox 应覆盖 UA 返回 sing-box 配置:\n%s", body)
	}

	// 错误 token → 404；订阅端点无需会话
	if resp, body := getSub(t, srvURL+"/sub/not-a-token", ""); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("错误 token 状态码 = %d, want 404: %s", resp.StatusCode, body)
	}
}

func TestSubscriptionTokenRevoke(t *testing.T) {
	srvURL, token := setupSubscription(t)
	client := newClient(t)
	if code, env := doJSON(t, client, http.MethodPost, srvURL+"/api/login",
		map[string]string{"username": "admin", "password": "password123"}); code != http.StatusOK {
		t.Fatalf("登录失败: %s", env.Msg)
	}

	// 查询令牌列表拿到 ID 后吊销
	code, env := doJSON(t, client, http.MethodGet, srvURL+"/api/subscription", nil)
	if code != http.StatusOK {
		t.Fatalf("订阅设置查询失败: %s", env.Msg)
	}
	var sub struct {
		Server string `json:"server"`
		Tokens []struct {
			ID    int    `json:"id"`
			Token string `json:"token"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(env.Data, &sub); err != nil {
		t.Fatal(err)
	}
	var id int
	for _, tk := range sub.Tokens {
		if tk.Token == token {
			id = tk.ID
		}
	}
	if id == 0 {
		t.Fatalf("令牌列表中未找到 %s: %+v", token, sub.Tokens)
	}

	// 吊销后订阅返回 404
	if code, env := doJSON(t, client, http.MethodPatch,
		srvURL+"/api/subscription/tokens/"+itoa(id)+"/revoked",
		map[string]bool{"enabled": true}); code != http.StatusOK {
		t.Fatalf("吊销失败: %s", env.Msg)
	}
	if resp, body := getSub(t, srvURL+"/sub/"+token, "SFM"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("吊销后订阅状态码 = %d, want 404: %s", resp.StatusCode, body)
	}

	// 重置后换新令牌可用，旧令牌仍 404
	code, env = doJSON(t, client, http.MethodPost, srvURL+"/api/subscription/tokens/"+itoa(id)+"/reset", nil)
	if code != http.StatusOK {
		t.Fatalf("重置失败: %s", env.Msg)
	}
	var reset struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(env.Data, &reset); err != nil {
		t.Fatal(err)
	}
	if reset.Token == "" || reset.Token == token {
		t.Fatalf("重置应生成新令牌: %q", reset.Token)
	}
	if resp, body := getSub(t, srvURL+"/sub/"+reset.Token, "SFM"); resp.StatusCode != http.StatusOK {
		t.Fatalf("重置后新令牌订阅状态码 = %d: %s", resp.StatusCode, body)
	}
	if resp, _ := getSub(t, srvURL+"/sub/"+token, "SFM"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("重置后旧令牌状态码 = %d, want 404", resp.StatusCode)
	}

	// 服务器地址设置保存后节点地址随之使用
	if code, env := doJSON(t, client, http.MethodPut, srvURL+"/api/subscription/server",
		map[string]string{"server": "vpn.example.com"}); code != http.StatusOK {
		t.Fatalf("保存服务器地址失败: %s", env.Msg)
	}
	_, body := getSub(t, srvURL+"/sub/"+reset.Token, "SFM")
	if !strings.Contains(body, `"server": "vpn.example.com"`) {
		t.Fatalf("节点服务器地址应取面板设置:\n%s", body)
	}
}

// itoa 整数转字符串（测试路径拼接用）。
func itoa(n int) string { return strconv.Itoa(n) }
