package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"proxy-ui/internal/db"
)

// envelope 统一响应结构（测试断言用）。
type envelope struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

// newTestServer 以临时数据库与内存静态资源构造面板服务。
func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "panel.db")
	dataDir := t.TempDir()
	srv := httptest.NewServer(mustNewEngine(dbPath, dataDir))
	t.Cleanup(srv.Close)
	return srv, dbPath
}

// mustNewEngine 在指定库文件与数据目录上装配引擎（重启场景复用同一文件）。
func mustNewEngine(dbPath, dataDir string) http.Handler {
	database, err := db.Open(dbPath)
	if err != nil {
		panic(err)
	}
	dist := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>proxy-ui</html>")}}
	return New(Options{DB: database, Dist: dist, Version: "test", DataDir: dataDir})
}

// newClient 带会话 Cookie 的客户端。
func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

// doJSON 发送 JSON 请求并解析 envelope。
func doJSON(t *testing.T, client *http.Client, method, url string, body any) (int, envelope) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	return resp.StatusCode, env
}

// setupAdmin 完成首次设置并返回带会话的客户端。
func setupAdmin(t *testing.T, srvURL string) *http.Client {
	t.Helper()
	client := newClient(t)
	if code, env := doJSON(t, client, http.MethodPost, srvURL+"/api/setup",
		map[string]string{"username": "admin", "password": "password123"}); code != http.StatusOK {
		t.Fatalf("setup 失败: %s", env.Msg)
	}
	return client
}

// decodeData 把 envelope.Data 解析到目标结构。
func decodeData(t *testing.T, env envelope, dst any) {
	t.Helper()
	if err := json.Unmarshal(env.Data, dst); err != nil {
		t.Fatalf("解析响应数据失败: %v, data=%s", err, env.Data)
	}
}

// inboundResp 入站响应视图（测试断言用）。
type inboundResp struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	Listen     string `json:"listen"`
	ListenPort int    `json:"listenPort"`
	Enabled    bool   `json:"enabled"`
	Config     map[string]any
	Warnings   []string `json:"warnings"`
	Users      []struct {
		ID        uint   `json:"id"`
		InboundID uint   `json:"inboundId"`
		Name      string `json:"name"`
		Enabled   bool   `json:"enabled"`
		Suspended bool   `json:"suspended"`
		TokenID   uint   `json:"tokenId"`
		TokenName string `json:"tokenName"`
	} `json:"users"`
}

// TestUninitialized 未初始化时的守卫行为。
func TestUninitialized(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"健康检查放行", http.MethodGet, "/api/health", http.StatusOK},
		{"未初始化登录拒绝", http.MethodPost, "/api/login", http.StatusUnauthorized},
		{"未初始化受保护接口拒绝", http.MethodGet, "/api/me", http.StatusUnauthorized},
		{"未初始化入站接口拒绝", http.MethodGet, "/api/inbounds", http.StatusUnauthorized},
	}
	srv, _ := newTestServer(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, env := doJSON(t, newClient(t), tt.method, srv.URL+tt.path, map[string]string{})
			if code != tt.want {
				t.Fatalf("状态码 = %d, want %d, msg = %s", code, tt.want, env.Msg)
			}
		})
	}
}

// TestSetupLoginFlow 首次设置 + 登录 + 改密 + 登出全链路。
func TestSetupLoginFlow(t *testing.T) {
	srv, _ := newTestServer(t)
	client := newClient(t)

	// 首次设置：创建管理员并自动登录
	code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/setup",
		map[string]string{"username": "admin", "password": "password123"})
	if code != http.StatusOK || !env.Success {
		t.Fatalf("setup 失败: code=%d msg=%s", code, env.Msg)
	}
	// 自动登录后可访问受保护接口
	if code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/me", nil); code != http.StatusOK {
		t.Fatalf("me 应放行: code=%d msg=%s", code, env.Msg)
	}
	var me struct {
		Username string `json:"username"`
	}
	decodeData(t, env, &me)
	if me.Username != "admin" {
		t.Errorf("当前管理员 = %q, want admin", me.Username)
	}
	// 健康检查反映已初始化
	code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/health", nil)
	var health struct {
		Status      string `json:"status"`
		Initialized bool   `json:"initialized"`
	}
	decodeData(t, env, &health)
	if code != http.StatusOK || !health.Initialized || health.Status != "ok" {
		t.Fatalf("健康检查不符: code=%d %+v", code, health)
	}
	// 重复 setup 被拒
	if code, _ = doJSON(t, client, http.MethodPost, srv.URL+"/api/setup",
		map[string]string{"username": "hacker", "password": "password123"}); code != http.StatusConflict {
		t.Fatalf("重复 setup 状态码 = %d, want 409", code)
	}
	// 登出后受保护接口 401
	if code, _ = doJSON(t, client, http.MethodPost, srv.URL+"/api/logout", nil); code != http.StatusOK {
		t.Fatal("logout 失败")
	}
	if code, _ = doJSON(t, client, http.MethodGet, srv.URL+"/api/me", nil); code != http.StatusUnauthorized {
		t.Fatalf("登出后 me 状态码 = %d, want 401", code)
	}
	// 错误密码登录被拒
	if code, _ = doJSON(t, client, http.MethodPost, srv.URL+"/api/login",
		map[string]string{"username": "admin", "password": "wrong-password"}); code != http.StatusUnauthorized {
		t.Fatalf("错误密码状态码 = %d, want 401", code)
	}
	// 正确密码登录
	if code, _ = doJSON(t, client, http.MethodPost, srv.URL+"/api/login",
		map[string]string{"username": "admin", "password": "password123"}); code != http.StatusOK {
		t.Fatal("登录失败")
	}
	// 修改密码：原密码错误被拒
	if code, _ = doJSON(t, client, http.MethodPut, srv.URL+"/api/password",
		map[string]string{"oldPassword": "bad", "newPassword": "newpassword456"}); code != http.StatusBadRequest {
		t.Fatalf("原密码错误状态码 = %d, want 400", code)
	}
	// 修改密码成功后用新密码可登录
	if code, _ = doJSON(t, client, http.MethodPut, srv.URL+"/api/password",
		map[string]string{"oldPassword": "password123", "newPassword": "newpassword456"}); code != http.StatusOK {
		t.Fatal("修改密码失败")
	}
	if code, _ = doJSON(t, client, http.MethodPost, srv.URL+"/api/login",
		map[string]string{"username": "admin", "password": "newpassword456"}); code != http.StatusOK {
		t.Fatal("新密码登录失败")
	}
}

// TestCreateInboundDefaults 新建入站：端口随机且不为 443、配置不含凭据、用户为空。
func TestCreateInboundDefaults(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	// 未指定端口 → 随机分配
	code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/inbounds", map[string]any{"protocol": "vless"})
	if code != http.StatusOK {
		t.Fatalf("创建入站失败: %s", env.Msg)
	}
	var created struct {
		ID         uint            `json:"id"`
		Name       string          `json:"name"`
		Protocol   string          `json:"protocol"`
		Listen     string          `json:"listen"`
		ListenPort int             `json:"listenPort"`
		Enabled    bool            `json:"enabled"`
		Config     json.RawMessage `json:"config"`
		Users      []any           `json:"users"`
	}
	decodeData(t, env, &created)
	if created.ListenPort < 10000 || created.ListenPort > 60000 {
		t.Errorf("默认端口 = %d, 应在 %d-%d 之间", created.ListenPort, 10000, 60000)
	}
	if created.ListenPort == 443 {
		t.Error("默认端口不应为 443")
	}
	if created.Protocol != "vless" || created.Name == "" || !created.Enabled {
		t.Errorf("入站基础字段不符: %+v", created)
	}
	if created.Listen != "::" {
		t.Errorf("默认监听地址 = %q, want ::", created.Listen)
	}
	if len(created.Users) != 0 {
		t.Errorf("新建入站不应有用户: %v", created.Users)
	}
	// 连接层配置为对象且不含任何用户凭据
	cfg := map[string]any{}
	if err := json.Unmarshal(created.Config, &cfg); err != nil {
		t.Fatalf("配置应为 JSON 对象: %v", err)
	}
	for _, key := range []string{"uuid", "password", "psk"} {
		if _, ok := cfg[key]; ok {
			t.Errorf("入站配置不应包含用户凭据字段 %s: %v", key, cfg)
		}
	}
	if cfg["reality"] == nil {
		t.Errorf("VLESS 默认配置应含 Reality 段: %v", cfg)
	}

	// 显式端口被占用时拒绝
	code, env = doJSON(t, client, http.MethodPost, srv.URL+"/api/inbounds",
		map[string]any{"protocol": "tuic", "listenPort": created.ListenPort, "name": "dup-port"})
	if code != http.StatusBadRequest {
		t.Fatalf("端口重复应 400, got %d: %s", code, env.Msg)
	}

	// 不支持的协议被拒
	code, _ = doJSON(t, client, http.MethodPost, srv.URL+"/api/inbounds", map[string]any{"protocol": "wireguard"})
	if code != http.StatusBadRequest {
		t.Errorf("不支持的协议应 400, got %d", code)
	}

	// 列表可查回该入站
	code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/inbounds", nil)
	if code != http.StatusOK {
		t.Fatalf("查询入站列表失败: %s", env.Msg)
	}
	var list []inboundResp
	decodeData(t, env, &list)
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("入站列表不符: %+v", list)
	}
}

// TestSystemStatus 系统状态接口不得要求 root 或特定系统。
func TestSystemStatus(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/system/status", nil)
	if code != http.StatusOK {
		t.Fatalf("系统状态接口失败: code=%d msg=%s", code, env.Msg)
	}
	var status struct {
		Platform      string   `json:"platform"`
		Supported     bool     `json:"supported"`
		Privileged    bool     `json:"privileged"`
		BBREnabled    bool     `json:"bbrEnabled"`
		AvailableCC   []string `json:"availableCc"`
		Reason        string   `json:"reason"`
		ManualCommand string   `json:"manualCommand"`
	}
	decodeData(t, env, &status)
	if status.Platform == "" {
		t.Error("platform 不应为空")
	}
	// 非 Linux 或非 root 时可一键开启应为 false，且给出原因与手动命令
	if status.ManualCommand == "" {
		t.Error("应提供手动命令")
	}
	if !status.Supported && status.Reason == "" {
		t.Error("不可一键开启时应说明原因")
	}
}

// TestSpaFallback 前端回退与未知 API 404。
func TestSpaFallback(t *testing.T) {
	srv, _ := newTestServer(t)
	client := newClient(t)

	resp, err := client.Get(srv.URL + "/inbounds/3")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("proxy-ui")) {
		t.Fatalf("未知前端路由应回退 index.html: %q", buf.String())
	}
	if code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/nope", nil); code != http.StatusNotFound {
		t.Fatalf("未知 API 状态码 = %d, msg=%s", code, env.Msg)
	}
}
