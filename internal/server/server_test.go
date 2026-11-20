package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strconv"
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
		{"未初始化登出拒绝", http.MethodPost, "/api/logout", http.StatusUnauthorized},
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

func TestStatsEndpoints(t *testing.T) {
	srv, dbPath := newTestServer(t)
	client := newClient(t)
	if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/setup",
		map[string]string{"username": "admin", "password": "password123"}); code != http.StatusOK {
		t.Fatalf("setup 失败: %s", env.Msg)
	}

	// 统计接口受会话守卫：未登录 401
	anon := newClient(t)
	if code, _ := doJSON(t, anon, http.MethodGet, srv.URL+"/api/stats/realtime", nil); code != http.StatusUnauthorized {
		t.Errorf("未登录访问实时统计应 401, got %d", code)
	}

	// 内核未运行：实时统计零值、连接快照为空
	code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/stats/realtime", nil)
	if code != http.StatusOK || !env.Success {
		t.Fatalf("realtime 应成功: code=%d msg=%s", code, env.Msg)
	}
	var realtime struct {
		Up                int64 `json:"up"`
		Down              int64 `json:"down"`
		Memory            int64 `json:"memory"`
		ActiveConnections int   `json:"activeConnections"`
	}
	if err := json.Unmarshal(env.Data, &realtime); err != nil {
		t.Fatalf("解析 realtime 失败: %v", err)
	}
	if realtime.Up != 0 || realtime.Down != 0 || realtime.Memory != 0 || realtime.ActiveConnections != 0 {
		t.Errorf("内核未运行时统计应为零值: %+v", realtime)
	}
	if code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/stats/connections", nil); code != http.StatusOK {
		t.Fatalf("connections 应成功: %s", env.Msg)
	}
	var snapshot struct {
		Connections []any `json:"connections"`
	}
	if err := json.Unmarshal(env.Data, &snapshot); err != nil || len(snapshot.Connections) != 0 {
		t.Errorf("内核未运行时连接应为空: %v %s", err, env.Data)
	}

	// 建入站后累计查询应包含零值行
	code, env = doJSON(t, client, http.MethodPost, srv.URL+"/api/inbounds",
		map[string]any{"protocol": "hysteria2", "listenPort": 18443, "name": "hy-in"})
	if code != http.StatusOK {
		t.Fatalf("创建入站失败: %s", env.Msg)
	}
	var created struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatalf("解析入站响应失败: %v", err)
	}
	if code, env = doJSON(t, client, http.MethodGet, srv.URL+"/api/stats/inbound-traffic", nil); code != http.StatusOK {
		t.Fatalf("inbound-traffic 应成功: %s", env.Msg)
	}
	var rows []struct {
		InboundID     uint   `json:"inboundId"`
		Name          string `json:"name"`
		UploadBytes   int64  `json:"uploadBytes"`
		DownloadBytes int64  `json:"downloadBytes"`
	}
	if err := json.Unmarshal(env.Data, &rows); err != nil || len(rows) != 1 {
		t.Fatalf("应返回一条累计行: %v %s", err, env.Data)
	}
	if rows[0].Name != "hy-in" || rows[0].UploadBytes != 0 || rows[0].DownloadBytes != 0 {
		t.Errorf("累计行不符: %+v", rows[0])
	}

	// 删除入站应连带删除累计流量记录
	gormDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	if err := gormDB.Create(&db.InboundTraffic{InboundID: created.ID, UploadBytes: 100, DownloadBytes: 200}).Error; err != nil {
		t.Fatalf("写入累计流量失败: %v", err)
	}
	if code, env = doJSON(t, client, http.MethodDelete, srv.URL+"/api/inbounds/"+strconv.FormatUint(uint64(created.ID), 10), nil); code != http.StatusOK {
		t.Fatalf("删除入站失败: %s", env.Msg)
	}
	var count int64
	if err := gormDB.Model(&db.InboundTraffic{}).Count(&count).Error; err != nil || count != 0 {
		t.Errorf("删除入站后累计记录应清除: count=%d err=%v", count, err)
	}
}

func TestRestartPersistence(t *testing.T) {
	srv, dbPath := newTestServer(t)
	client := newClient(t)
	if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/setup",
		map[string]string{"username": "admin", "password": "password123"}); code != http.StatusOK {
		t.Fatalf("setup 失败: %s", env.Msg)
	}
	// 直接写一条设置，验证设置数据随库文件保留
	gormDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := gormDB.Create(&db.Setting{Key: "sample_setting", Value: "https://gh.example.com"}).Error; err != nil {
		t.Fatal(err)
	}

	// 模拟重启：在同一个库文件上重新装配服务
	srv2 := httptest.NewServer(mustNewEngine(dbPath, t.TempDir()))
	defer srv2.Close()
	if code, env := doJSON(t, newClient(t), http.MethodGet, srv2.URL+"/api/health", nil); code != http.StatusOK {
		t.Fatalf("重启后健康检查异常: code=%d msg=%s", code, env.Msg)
	}
	// 管理员与设置数据保留，旧凭据可直接登录
	if code, env := doJSON(t, newClient(t), http.MethodPost, srv2.URL+"/api/login",
		map[string]string{"username": "admin", "password": "password123"}); code != http.StatusOK {
		t.Fatalf("重启后登录失败: code=%d msg=%s", code, env.Msg)
	}
	var count int64
	gormDB.Model(&db.Setting{}).Count(&count)
	if count != 1 {
		t.Fatalf("设置数据丢失: count=%d", count)
	}
	// 重启后重复 setup 仍被拒
	if code, _ := doJSON(t, newClient(t), http.MethodPost, srv2.URL+"/api/setup",
		map[string]string{"username": "x", "password": "password123"}); code != http.StatusConflict {
		t.Fatalf("重启后重复 setup 状态码 = %d, want 409", code)
	}
}

func TestSpaFallback(t *testing.T) {
	srv, _ := newTestServer(t)
	client := newClient(t)
	tests := []struct {
		name string
		path string
		want string // 期望响应体包含
	}{
		{"根路径返回 index", "/", "proxy-ui"},
		{"未知前端路由回退 index", "/inbounds/3", "proxy-ui"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.Get(srv.URL + tt.path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			buf := new(bytes.Buffer)
			if _, err := buf.ReadFrom(resp.Body); err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(buf.Bytes(), []byte(tt.want)) {
				t.Fatalf("响应体 = %q, want 包含 %q", buf.String(), tt.want)
			}
		})
	}
	// 未知 API 返回 envelope 404
	if code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/nope", nil); code != http.StatusNotFound {
		t.Fatalf("未知 API 状态码 = %d, msg=%s", code, env.Msg)
	}
}
