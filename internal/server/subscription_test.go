package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"proxy-ui/internal/db"
)

// itoa 整数转字符串（测试路径拼接用）。
func itoa(n uint) string { return strconv.FormatUint(uint64(n), 10) }

// getSub 拉取订阅响应，返回状态码、Content-Type 与响应体。
func getSub(t *testing.T, url, ua string) (int, string, string) {
	t.Helper()
	code, hdr, body := getSubWithHeaders(t, url, ua)
	return code, hdr.Get("Content-Type"), body
}

// getSubWithHeaders 拉取订阅响应，返回状态码、响应头与响应体。
func getSubWithHeaders(t *testing.T, url, ua string) (int, http.Header, string) {
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
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, buf.String()
}

// createInbound 创建入站并返回其视图。
func createInbound(t *testing.T, client *http.Client, srvURL string, body map[string]any) inboundResp {
	t.Helper()
	code, env := doJSON(t, client, http.MethodPost, srvURL+"/api/inbounds", body)
	if code != http.StatusOK {
		t.Fatalf("创建入站失败: code=%d msg=%s", code, env.Msg)
	}
	var in inboundResp
	decodeData(t, env, &in)
	return in
}

// tokenResp 订阅用户视图（测试断言用）。
type tokenResp struct {
	ID                uint   `json:"id"`
	Token             string `json:"token"`
	Name              string `json:"name"`
	Revoked           bool   `json:"revoked"`
	TrafficLimitBytes int64  `json:"trafficLimitBytes"`
	UsedBytes         int64  `json:"usedBytes"`
	OverQuota         bool   `json:"overQuota"`
	Nodes             []struct {
		UserID      uint   `json:"userId"`
		InboundID   uint   `json:"inboundId"`
		InboundName string `json:"inboundName"`
		Protocol    string `json:"protocol"`
		ListenPort  int    `json:"listenPort"`
		Name        string `json:"name"`
		Enabled     bool   `json:"enabled"`
		Suspended   bool   `json:"suspended"`
	} `json:"nodes"`
}

// createToken 新建订阅用户并返回视图。
func createToken(t *testing.T, client *http.Client, srvURL string, body map[string]any) tokenResp {
	t.Helper()
	code, env := doJSON(t, client, http.MethodPost, srvURL+"/api/subscription/tokens", body)
	if code != http.StatusOK {
		t.Fatalf("创建订阅用户失败: code=%d msg=%s", code, env.Msg)
	}
	var view tokenResp
	decodeData(t, env, &view)
	return view
}

// subOutbounds 解析 sing-box 订阅为出站列表（tag -> 对象）。
func subOutbounds(t *testing.T, body string) map[string]map[string]any {
	t.Helper()
	cfg := map[string]any{}
	if err := json.Unmarshal([]byte(body), &cfg); err != nil {
		t.Fatalf("sing-box 订阅应为合法 JSON: %v\n%s", err, body)
	}
	out := map[string]map[string]any{}
	list, _ := cfg["outbounds"].([]any)
	for _, item := range list {
		ob, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if tag, ok := ob["tag"].(string); ok {
			out[tag] = ob
		}
	}
	return out
}

// TestSubscriptionNodeCreation 订阅用户勾选入站后在该入站下创建用户并出现在列表与订阅中。
func TestSubscriptionNodeCreation(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	in := createInbound(t, client, srv.URL, map[string]any{"name": "vless-main", "protocol": "vless", "listenPort": 0})
	if in.ListenPort < 10000 || in.ListenPort > 60000 {
		t.Fatalf("入站端口 = %d, 应在随机端口段内", in.ListenPort)
	}

	view := createToken(t, client, srv.URL, map[string]any{
		"name": "alice", "nodeInbounds": []uint{in.ID},
	})
	if len(view.Nodes) != 1 {
		t.Fatalf("订阅用户应有 1 个节点: %+v", view.Nodes)
	}
	node := view.Nodes[0]
	wantName := "alice-" + in.Name
	if node.Name != wantName {
		t.Errorf("节点用户名 = %q, want %q", node.Name, wantName)
	}
	if node.InboundID != in.ID || node.ListenPort != in.ListenPort || node.Protocol != "vless" {
		t.Errorf("节点字段不符: %+v", node)
	}
	if !node.Enabled || node.Suspended {
		t.Errorf("新建节点应启用且未暂停: %+v", node)
	}

	// 入站列表里能看到该用户
	code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/inbounds", nil)
	if code != http.StatusOK {
		t.Fatalf("查询入站列表失败: %s", env.Msg)
	}
	var list []inboundResp
	decodeData(t, env, &list)
	if len(list) != 1 || list[0].UserCount != 1 {
		t.Fatalf("入站下应有 1 个用户: %+v", list)
	}

	// GET /api/subscription 里节点带该入站端口
	code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/subscription", nil)
	if code != http.StatusOK {
		t.Fatalf("查询订阅用户失败: %s", env.Msg)
	}
	var tokens []tokenResp
	decodeData(t, env, &tokens)
	if len(tokens) != 1 || len(tokens[0].Nodes) != 1 {
		t.Fatalf("订阅用户列表不符: %+v", tokens)
	}
	if tokens[0].Nodes[0].ListenPort != in.ListenPort {
		t.Errorf("订阅节点端口 = %d, want %d", tokens[0].Nodes[0].ListenPort, in.ListenPort)
	}

	// 订阅端点返回该节点，且响应头带订阅用户名而非 config.json/config.yaml
	code, hdr, body := getSubWithHeaders(t, srv.URL+"/sub/"+view.Token, "SFM/1.0 sing-box/1.14.4")
	contentType := hdr.Get("Content-Type")
	if code != http.StatusOK {
		t.Fatalf("订阅状态码 = %d: %s", code, body)
	}
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("sing-box 订阅 Content-Type = %q", contentType)
	}
	if cd := hdr.Get("Content-Disposition"); !strings.Contains(cd, "alice") || strings.Contains(cd, "config.json") {
		t.Errorf("Content-Disposition 应含订阅名 alice 且不含 config.json: %q", cd)
	}
	outbounds := subOutbounds(t, body)
	node2, ok := outbounds[in.Name]
	if !ok {
		t.Fatalf("订阅应包含节点 %s: %v", in.Name, outbounds)
	}
	if outbounds[wantName] != nil {
		t.Errorf("订阅节点名不应带订阅用户前缀 %s: %v", wantName, outbounds)
	}
	if node2["type"] != "vless" || node2["uuid"] == "" {
		t.Errorf("订阅节点字段不符: %v", node2)
	}
	if port, ok := node2["server_port"].(float64); !ok || int(port) != in.ListenPort {
		t.Errorf("订阅节点端口 = %v, want %d", node2["server_port"], in.ListenPort)
	}

	// 取消勾选入站 → 节点与其入站用户一并删除
	code, env = doJSON(t, client, http.MethodPut, srv.URL+"/api/subscription/tokens/"+itoa(view.ID),
		map[string]any{"nodeInbounds": []uint{}})
	if code != http.StatusOK {
		t.Fatalf("取消勾选入站失败: %s", env.Msg)
	}
	var updated tokenResp
	decodeData(t, env, &updated)
	if len(updated.Nodes) != 0 {
		t.Errorf("取消勾选后应无节点: %+v", updated.Nodes)
	}
	code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/inbounds/"+itoa(in.ID), nil)
	if code != http.StatusOK {
		t.Fatalf("查询入站失败: %s", env.Msg)
	}
	var detail struct {
		UserCount int64 `json:"userCount"`
	}
	decodeData(t, env, &detail)
	if detail.UserCount != 0 {
		t.Errorf("取消勾选后入站用户应被删除: %d", detail.UserCount)
	}
}

// TestSubscriptionUserNameSanitize 订阅备注经清洗后才作为节点名（非法字符转短横线、超长截断）。
func TestSubscriptionUserNameSanitize(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	in := createInbound(t, client, srv.URL, map[string]any{"name": "vless-san", "protocol": "vless"})

	// 空白与符号各转为一个 -，首尾 - 去掉；中文保留
	view := createToken(t, client, srv.URL, map[string]any{
		"name": " 手机 / 香港 ", "nodeInbounds": []uint{in.ID},
	})
	if len(view.Nodes) != 1 {
		t.Fatalf("应有 1 个节点: %+v", view.Nodes)
	}
	want := "手机---香港-vless-san"
	if view.Nodes[0].Name != want {
		t.Errorf("节点名 = %q, want %q", view.Nodes[0].Name, want)
	}
	// 清洗后的名字必须是合法内核 tag（只含字母/数字/._-）
	for _, r := range view.Nodes[0].Name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '_' && r != '-' {
			t.Errorf("节点名含非法字符 %q: %q", r, view.Nodes[0].Name)
		}
	}

	// 超长备注按字符截断，节点名不超过 60 个字符
	long := strings.Repeat("机", 80)
	view = createToken(t, client, srv.URL, map[string]any{
		"name": long, "nodeInbounds": []uint{in.ID},
	})
	if len(view.Nodes) != 1 {
		t.Fatalf("应有 1 个节点: %+v", view.Nodes)
	}
	if n := len([]rune(view.Nodes[0].Name)); n > 60+len("-vless-san") {
		t.Errorf("节点名过长（%d 字符）: %q", n, view.Nodes[0].Name)
	}

	// 纯符号备注清洗后只剩入站名；但该名字已被入站 tag 占用（用户名与入站名共用流量表的键，
	// 同名会让两者用量互相覆盖），因此必须自动追加序号
	view = createToken(t, client, srv.URL, map[string]any{
		"name": "!!!", "nodeInbounds": []uint{in.ID},
	})
	if len(view.Nodes) != 1 {
		t.Fatalf("纯符号备注也应能创建节点: %+v", view.Nodes)
	}
	if got := view.Nodes[0].Name; got == in.Name || !strings.HasPrefix(got, in.Name+"-") {
		t.Errorf("节点名不得与入站 tag 相同（应为 %s-序号）: %q", in.Name, got)
	}
}

// TestSubscriptionSharedInbound 同一入站下两个订阅用户各自独立凭据、共享端口。
func TestSubscriptionSharedInbound(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	in := createInbound(t, client, srv.URL, map[string]any{"name": "ss-main", "protocol": "shadowsocks"})

	alice := createToken(t, client, srv.URL, map[string]any{
		"name": "alice", "nodeInbounds": []uint{in.ID},
	})
	bob := createToken(t, client, srv.URL, map[string]any{
		"name": "bob", "nodeInbounds": []uint{in.ID},
	})

	// 入站下应有两条用户
	code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/inbounds/"+itoa(in.ID), nil)
	if code != http.StatusOK {
		t.Fatalf("查询入站失败: %s", env.Msg)
	}
	var detail struct {
		UserCount int64 `json:"userCount"`
	}
	decodeData(t, env, &detail)
	if detail.UserCount != 2 {
		t.Fatalf("同一入站下应有 2 个用户: %d", detail.UserCount)
	}

	// 两个订阅的节点端口相同、密码不同
	_, _, bodyA := getSub(t, srv.URL+"/sub/"+alice.Token, "SFM")
	_, _, bodyB := getSub(t, srv.URL+"/sub/"+bob.Token, "SFM")
	outA := subOutbounds(t, bodyA)
	outB := subOutbounds(t, bodyB)
	if len(outA) == 0 || len(outB) == 0 {
		t.Fatalf("订阅应包含节点: A=%v B=%v", outA, outB)
	}
	var nodeA, nodeB map[string]any
	for _, ob := range outA {
		if ob["type"] == "shadowsocks" {
			nodeA = ob
		}
	}
	for _, ob := range outB {
		if ob["type"] == "shadowsocks" {
			nodeB = ob
		}
	}
	if nodeA == nil || nodeB == nil {
		t.Fatalf("两个订阅都应含 shadowsocks 节点: A=%v B=%v", outA, outB)
	}
	if nodeA["server_port"] != nodeB["server_port"] {
		t.Errorf("同入站节点应共享端口: A=%v B=%v", nodeA["server_port"], nodeB["server_port"])
	}
	if nodeA["password"] == nodeB["password"] {
		t.Errorf("不同用户凭据不应相同: %v", nodeA["password"])
	}
	// SS-2022 密码为「入站PSK:用户密钥」
	if pw, _ := nodeA["password"].(string); !strings.Contains(pw, ":") {
		t.Errorf("SS-2022 节点密码应为「PSK:用户密钥」，got %q", pw)
	}
	if alice.Nodes[0].Name == bob.Nodes[0].Name {
		t.Errorf("不同订阅用户在服务端入站下的用户名不应相同: %q", alice.Nodes[0].Name)
	}
	if nodeA["tag"] != in.Name || nodeB["tag"] != in.Name {
		t.Errorf("不同订阅用户导出的节点 tag 均应为入站名 %q: A=%v B=%v", in.Name, nodeA["tag"], nodeB["tag"])
	}
}

// TestSubscriptionFormats 订阅格式自适应与参数覆盖。
func TestSubscriptionFormats(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	// Hysteria2 强制 TLS：绑定一份真实自签证书，否则保存时内核无法重建（保存即热应用）
	certID := createTestCert(t, client, srv.URL)
	in := createInbound(t, client, srv.URL, map[string]any{
		"name": "hy2-main", "protocol": "hysteria2",
		"config": map[string]any{
			"tls": map[string]any{"enabled": true, "certMode": "managed", "certId": certID, "alpn": []string{"h3"}},
		},
	})
	token := createToken(t, client, srv.URL, map[string]any{
		"name": "bob", "nodeInbounds": []uint{in.ID},
	})

	// UA 判为 clash → YAML
	code, contentType, body := getSub(t, srv.URL+"/sub/"+token.Token, "clash.meta/1.19.0 Mihomo")
	if code != http.StatusOK {
		t.Fatalf("clash 订阅状态码 = %d: %s", code, body)
	}
	if !strings.Contains(contentType, "yaml") {
		t.Errorf("clash 订阅 Content-Type = %q", contentType)
	}
	for _, want := range []string{"proxies:", "proxy-groups:", "rules:", "MATCH,"} {
		if !strings.Contains(body, want) {
			t.Errorf("clash 订阅缺少 %q:\n%s", want, body)
		}
	}
	if !strings.Contains(body, "type: hysteria2") {
		t.Errorf("clash 订阅应包含 hysteria2 节点:\n%s", body)
	}

	// ?format=clash 覆盖 sing-box UA
	_, _, body = getSub(t, srv.URL+"/sub/"+token.Token+"?format=clash", "SFM sing-box")
	if !strings.Contains(body, "proxy-groups:") {
		t.Errorf("format=clash 应覆盖 UA:\n%s", body)
	}
	// ?format=singbox 覆盖 clash UA
	_, _, body = getSub(t, srv.URL+"/sub/"+token.Token+"?format=singbox", "Mihomo/1.19")
	if !strings.Contains(body, `"outbounds"`) {
		t.Errorf("format=singbox 应覆盖 UA:\n%s", body)
	}

	// 错误 token → 404，且无需会话
	if code, _, _ := getSub(t, srv.URL+"/sub/not-a-token", ""); code != http.StatusNotFound {
		t.Errorf("错误 token 状态码 = %d, want 404", code)
	}
}

// TestSubscriptionQuota 配额用尽 → 用户暂停、订阅 403；清零流量后恢复。
func TestSubscriptionQuota(t *testing.T) {
	srv, dbPath := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	in := createInbound(t, client, srv.URL, map[string]any{"name": "vless-q", "protocol": "vless"})
	const limit int64 = 1 << 20 // 1 MiB
	view := createToken(t, client, srv.URL, map[string]any{
		"name": "quota-user", "nodeInbounds": []uint{in.ID}, "trafficLimitBytes": limit,
	})
	if len(view.Nodes) != 1 {
		t.Fatalf("应有 1 个节点: %+v", view.Nodes)
	}
	// 写入超额流量（直连同一个库文件）
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	// 配额计数在 TokenTraffic（按订阅用户），与用户名无关
	if err := database.Save(&db.TokenTraffic{TokenID: view.ID, UploadBytes: limit / 2, DownloadBytes: limit / 2}).Error; err != nil {
		t.Fatalf("写入流量记录失败: %v", err)
	}

	// 触发配额对账
	code, env := doJSON(t, client, http.MethodPut, srv.URL+"/api/subscription/tokens/"+itoa(view.ID), map[string]any{})
	if code != http.StatusOK {
		t.Fatalf("更新订阅用户失败: code=%d msg=%s", code, env.Msg)
	}
	var after tokenResp
	decodeData(t, env, &after)
	if len(after.Nodes) != 1 {
		t.Fatalf("节点数量不符: %+v", after.Nodes)
	}
	if !after.Nodes[0].Suspended || after.Nodes[0].Enabled {
		t.Fatalf("超额后用户应暂停且停用: %+v", after.Nodes[0])
	}
	if !after.OverQuota || after.UsedBytes < limit {
		t.Errorf("超额状态不符: used=%d overQuota=%v", after.UsedBytes, after.OverQuota)
	}

	// 订阅返回 403
	if code, _, body := getSub(t, srv.URL+"/sub/"+view.Token, "SFM"); code != http.StatusForbidden {
		t.Fatalf("配额用尽时订阅状态码 = %d, want 403: %s", code, body)
	}

	// 清零流量 → 用量归零、用户恢复、订阅可用
	code, env = doJSON(t, client, http.MethodPost, srv.URL+"/api/subscription/tokens/"+itoa(view.ID)+"/reset-traffic", nil)
	if code != http.StatusOK {
		t.Fatalf("清零流量失败: code=%d msg=%s", code, env.Msg)
	}
	var reset tokenResp
	decodeData(t, env, &reset)
	if reset.UsedBytes != 0 {
		t.Errorf("清零后用量应为 0, got %d", reset.UsedBytes)
	}
	if len(reset.Nodes) != 1 || !reset.Nodes[0].Enabled || reset.Nodes[0].Suspended {
		t.Errorf("清零后用户应恢复启用: %+v", reset.Nodes)
	}
	if reset.OverQuota {
		t.Error("清零后不应处于超额状态")
	}
	if code, _, body := getSub(t, srv.URL+"/sub/"+view.Token, "SFM"); code != http.StatusOK {
		t.Fatalf("清零后订阅状态码 = %d, want 200: %s", code, body)
	}
}

// TestSubscriptionTokenLifecycle 停用/重置/删除订阅用户的订阅可见性。
func TestSubscriptionTokenLifecycle(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	in := createInbound(t, client, srv.URL, map[string]any{"name": "vless-l", "protocol": "vless"})
	view := createToken(t, client, srv.URL, map[string]any{
		"name": "lifecycle", "nodeInbounds": []uint{in.ID},
	})

	// 停用 → 订阅 404；用户被停用
	if code, env := doJSON(t, client, http.MethodPatch,
		srv.URL+"/api/subscription/tokens/"+itoa(view.ID)+"/revoked", map[string]bool{"enabled": true}); code != http.StatusOK {
		t.Fatalf("停用订阅用户失败: %s", env.Msg)
	}
	if code, _, _ := getSub(t, srv.URL+"/sub/"+view.Token, "SFM"); code != http.StatusNotFound {
		t.Errorf("停用后订阅状态码 = %d, want 404", code)
	}

	// 重置令牌 → 新令牌可用、旧令牌失效
	code, env := doJSON(t, client, http.MethodPost,
		srv.URL+"/api/subscription/tokens/"+itoa(view.ID)+"/reset", nil)
	if code != http.StatusOK {
		t.Fatalf("重置令牌失败: %s", env.Msg)
	}
	var reset tokenResp
	decodeData(t, env, &reset)
	if reset.Token == "" || reset.Token == view.Token {
		t.Fatalf("重置应生成新令牌: %q", reset.Token)
	}
	if code, _, body := getSub(t, srv.URL+"/sub/"+reset.Token, "SFM"); code != http.StatusOK {
		t.Fatalf("新令牌订阅状态码 = %d: %s", code, body)
	}
	if code, _, _ := getSub(t, srv.URL+"/sub/"+view.Token, "SFM"); code != http.StatusNotFound {
		t.Errorf("旧令牌状态码 = %d, want 404", code)
	}

	// 服务器地址取请求 Host（httptest 为 127.0.0.1）
	_, _, body := getSub(t, srv.URL+"/sub/"+reset.Token, "SFM")
	if !strings.Contains(body, `"server": "127.0.0.1"`) {
		t.Errorf("节点服务器地址应取请求 Host:\n%s", body)
	}

	// 删除订阅用户 → 订阅 404，入站用户一并删除
	if code, env := doJSON(t, client, http.MethodDelete,
		srv.URL+"/api/subscription/tokens/"+itoa(view.ID), nil); code != http.StatusOK {
		t.Fatalf("删除订阅用户失败: %s", env.Msg)
	}
	if code, _, _ := getSub(t, srv.URL+"/sub/"+reset.Token, "SFM"); code != http.StatusNotFound {
		t.Errorf("删除后订阅状态码 = %d, want 404", code)
	}
	code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/inbounds/"+itoa(in.ID), nil)
	if code != http.StatusOK {
		t.Fatalf("查询入站失败: %s", env.Msg)
	}
	var detail struct {
		UserCount int64 `json:"userCount"`
	}
	decodeData(t, env, &detail)
	if detail.UserCount != 0 {
		t.Errorf("删除订阅用户后其入站用户应清除: %d", detail.UserCount)
	}
}

// TestSubscriptionWithRuleTemplate 应用「国内直连 + 海外代理」模板（无自定义出站）后，
// 订阅的 proxy 策略组必须自动填入用户节点而非仅剩 DIRECT。
func TestSubscriptionWithRuleTemplate(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	in := createInbound(t, client, srv.URL, map[string]any{"name": "vless-tpl", "protocol": "vless"})
	view := createToken(t, client, srv.URL, map[string]any{
		"name": "tpl-user", "nodeInbounds": []uint{in.ID},
	})
	if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-templates/cn-direct-adblock/apply", nil); code != http.StatusOK {
		t.Fatalf("应用规则模板失败: code=%d msg=%s", code, env.Msg)
	}
	code, _, body := getSub(t, srv.URL+"/sub/"+view.Token+"?format=clash", "clash-verge-rev")
	if code != http.StatusOK {
		t.Fatalf("拉取 clash 订阅失败: code=%d body=%s", code, body)
	}
	if !strings.Contains(body, "vless-tpl") {
		t.Errorf("订阅应包含节点 vless-tpl:\n%s", body)
	}
	if strings.Contains(body, "tpl-user-vless-tpl") {
		t.Errorf("订阅节点名不应带订阅用户前缀 tpl-user-:\n%s", body)
	}
	if strings.Contains(body, "proxies:\n  - DIRECT") {
		t.Errorf("应用模板后 proxy 组不应仅剩 DIRECT:\n%s", body)
	}
}

// TestSubscriptionSelfSignedCert 使用一键自签证书的入站，端到端拉取订阅自动带 insecure / skip-cert-verify。
func TestSubscriptionSelfSignedCert(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	certID := createTestCert(t, client, srv.URL)
	in := createInbound(t, client, srv.URL, map[string]any{
		"name": "hy2-self", "protocol": "hysteria2",
		"config": map[string]any{
			"tls": map[string]any{"enabled": true, "certMode": "managed", "certId": certID, "alpn": []string{"h3"}},
		},
	})
	view := createToken(t, client, srv.URL, map[string]any{
		"name": "self-user", "nodeInbounds": []uint{in.ID},
	})

	code, _, sbBody := getSub(t, srv.URL+"/sub/"+view.Token+"?format=singbox", "SFM")
	if code != http.StatusOK {
		t.Fatalf("sing-box 订阅失败: code=%d body=%s", code, sbBody)
	}
	obs := subOutbounds(t, sbBody)
	node := obs["hy2-self"]
	if tls, _ := node["tls"].(map[string]any); tls["insecure"] != true {
		t.Errorf("自签证书 sing-box 节点应自动带 tls.insecure=true: %v", node)
	}

	code, _, clashBody := getSub(t, srv.URL+"/sub/"+view.Token+"?format=clash", "clash-verge-rev")
	if code != http.StatusOK {
		t.Fatalf("clash 订阅失败: code=%d body=%s", code, clashBody)
	}
	if !strings.Contains(clashBody, "skip-cert-verify: true") {
		t.Errorf("自签证书 clash 节点应自动带 skip-cert-verify: true:\n%s", clashBody)
	}
}

// TestSubscriptionIPv6Settings 订阅 IPv6 入口设置、校验拒绝、双格式下发与预览、清空恢复。
func TestSubscriptionIPv6Settings(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	// 查询默认设置
	code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/subscription/settings", nil)
	if code != http.StatusOK {
		t.Fatalf("查询订阅设置失败: code=%d msg=%s", code, env.Msg)
	}
	var initSettings struct {
		ServerIPv6    string   `json:"serverIpv6"`
		DetectedIPv6s []string `json:"detectedIpv6s"`
	}
	decodeData(t, env, &initSettings)
	if initSettings.ServerIPv6 != "" {
		t.Errorf("默认 serverIpv6 应为空, got %q", initSettings.ServerIPv6)
	}
	if initSettings.DetectedIPv6s == nil {
		t.Error("detectedIpv6s 应序列化为数组而非 null")
	}

	// 非法输入被 400 拒绝
	for _, bad := range []string{"1.2.3.4", "[2001:db8::1]:443", "v6.example.com:8443"} {
		if code, _ := doJSON(t, client, http.MethodPut, srv.URL+"/api/subscription/settings", map[string]string{"serverIpv6": bad}); code != http.StatusBadRequest {
			t.Errorf("非法 serverIpv6 %q 应返回 400, got %d", bad, code)
		}
	}

	// 保存规范化的 IPv6 入口
	code, env = doJSON(t, client, http.MethodPut, srv.URL+"/api/subscription/settings", map[string]string{"serverIpv6": "[2001:0db8::1]"})
	if code != http.StatusOK {
		t.Fatalf("保存 serverIpv6 失败: code=%d msg=%s", code, env.Msg)
	}
	var saved struct {
		ServerIPv6 string `json:"serverIpv6"`
	}
	decodeData(t, env, &saved)
	if saved.ServerIPv6 != "2001:db8::1" {
		t.Errorf("保存后应规范化为 2001:db8::1, got %q", saved.ServerIPv6)
	}

	// 创建双栈入站 + 仅 IPv4 入站 + 订阅用户
	inDual := createInbound(t, client, srv.URL, map[string]any{"name": "vless-dual", "protocol": "vless", "listen": "::"})
	inV4 := createInbound(t, client, srv.URL, map[string]any{"name": "trojan-v4", "protocol": "trojan", "listen": "0.0.0.0"})
	view := createToken(t, client, srv.URL, map[string]any{
		"name": "u1", "nodeInbounds": []uint{inDual.ID, inV4.ID},
	})

	// 拉取 sing-box 订阅：包含 vless-dual 与 vless-dual-ipv6，不包含 trojan-v4-ipv6
	code, _, sbBody := getSub(t, srv.URL+"/sub/"+view.Token+"?format=singbox", "SFM")
	if code != http.StatusOK {
		t.Fatalf("sing-box 订阅失败: code=%d body=%s", code, sbBody)
	}
	obs := subOutbounds(t, sbBody)
	if obs["vless-dual"] == nil || obs["vless-dual-ipv6"] == nil {
		t.Fatalf("双栈入站应同时生成主节点与 -ipv6 节点: %v", obs)
	}
	if obs["vless-dual-ipv6"]["server"] != "2001:db8::1" {
		t.Errorf("IPv6 节点 server = %v, want 2001:db8::1", obs["vless-dual-ipv6"]["server"])
	}
	if obs["trojan-v4-ipv6"] != nil {
		t.Errorf("仅 IPv4 入站不应生成 -ipv6 节点: %v", obs["trojan-v4-ipv6"])
	}

	// 拉取 clash 订阅 & 面板内预览：包含 ipv6: true、-ipv6 节点与裸 IPv6 地址（且裸 IPv6 不输出 ip-version）
	code, _, clashBody := getSub(t, srv.URL+"/sub/"+view.Token+"?format=clash", "clash-verge-rev")
	if code != http.StatusOK {
		t.Fatalf("clash 订阅失败: code=%d body=%s", code, clashBody)
	}
	for _, want := range []string{"ipv6: true", "vless-dual-ipv6", "server: 2001:db8::1"} {
		if !strings.Contains(clashBody, want) {
			t.Errorf("clash 订阅缺少 %q:\n%s", want, clashBody)
		}
	}
	if strings.Contains(clashBody, "u1-vless-dual") {
		t.Errorf("clash 节点名不应带订阅用户前缀 u1-:\n%s", clashBody)
	}
	if strings.Contains(clashBody, "ip-version: ipv6") {
		t.Errorf("裸 IPv6 地址节点不应输出 ip-version: ipv6:\n%s", clashBody)
	}

	code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/subscription/tokens/"+itoa(view.ID)+"/preview?format=mihomo", nil)
	if code != http.StatusOK {
		t.Fatalf("预览接口失败: code=%d msg=%s", code, env.Msg)
	}
	var preview struct {
		Content string `json:"content"`
	}
	decodeData(t, env, &preview)
	if !strings.Contains(preview.Content, "vless-dual-ipv6") {
		t.Errorf("预览内容应包含 -ipv6 节点:\n%s", preview.Content)
	}

	// 清空 serverIpv6 → 恢复仅下发单节点
	if code, env := doJSON(t, client, http.MethodPut, srv.URL+"/api/subscription/settings", map[string]string{"serverIpv6": ""}); code != http.StatusOK {
		t.Fatalf("清空 serverIpv6 失败: code=%d msg=%s", code, env.Msg)
	}
	_, _, sbBodyAfter := getSub(t, srv.URL+"/sub/"+view.Token+"?format=singbox", "SFM")
	if obsAfter := subOutbounds(t, sbBodyAfter); obsAfter["vless-dual-ipv6"] != nil {
		t.Errorf("清空 serverIpv6 后不应再下发 -ipv6 节点: %v", obsAfter)
	}
}
