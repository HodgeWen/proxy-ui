package server

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// envelope 统一响应结构（测试断言用）。
type envelope struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

// setupCodes 各测试服务的一次性设置码（newTestServer 从启动日志截获存入，setupAdmin 取用）。
var setupCodes sync.Map // 服务 URL → 设置码

// newServerWithCode 以临时数据库与内存静态资源构造面板服务，并返回启动日志中的一次性设置码。
func newServerWithCode(t *testing.T) (*httptest.Server, string, string) {
	t.Helper()
	// 面板的所有写接口都会热应用内核，内核起不来时这些用例没有意义
	requireKernel(t)
	dbPath := filepath.Join(t.TempDir(), "panel.db")
	dataDir := t.TempDir()
	handler, code := mustNewEngineWithCode(t.Context(), dbPath, dataDir)
	srv := httptest.NewServer(handler)
	setupCodes.Store(srv.URL, code)
	t.Cleanup(srv.Close)
	return srv, dbPath, code
}

// newTestServer 以临时数据库与内存静态资源构造面板服务。
func newTestServer(t *testing.T) (*httptest.Server, string) {
	srv, dbPath, _ := newServerWithCode(t)
	return srv, dbPath
}

// mustNewEngine 在指定生命周期上下文、库文件与数据目录上装配引擎（重启场景复用同一文件）。
func mustNewEngine(ctx context.Context, dbPath, dataDir string) http.Handler {
	database, err := db.Open(dbPath)
	if err != nil {
		panic(err)
	}
	dist := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>proxy-ui</html>")}}
	return New(Options{Context: ctx, DB: database, Dist: dist, Version: "test", DataDir: dataDir})
}

// kernelBuildable 探测当前测试二进制里的内嵌内核能否真正启动。
//
// 内核以库形式编进面板，缺 with_v2ray_api 等构建标签时 box.New 会直接失败，而面板的
// 写操作一律保存即热应用，因此这类构建下的接口测试必然走不通。此时明确跳过并提示用
// `make test`（带 BUILD_TAGS）跑，避免把「构建标签缺失」误报成业务回归。
var kernelBuildable = sync.OnceValue(func() bool {
	dir, err := os.MkdirTemp("", "proxy-ui-probe")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	mgr := singbox.New(dir)
	defer func() { _ = mgr.Stop() }()
	return mgr.Start(singbox.MinConfig()) == nil
})

// requireKernel 内核不可用时跳过当前用例。
func requireKernel(t *testing.T) {
	t.Helper()
	if kernelBuildable() {
		return
	}
	t.Skip("当前测试二进制未带内核构建标签（内核无法启动），请用 make test 运行")
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

// setupAdmin 携带一次性设置码完成首次设置并返回带会话的客户端。
func setupAdmin(t *testing.T, srvURL string) *http.Client {
	t.Helper()
	codeAny, ok := setupCodes.Load(srvURL)
	if !ok {
		t.Fatalf("未找到 %s 的一次性设置码", srvURL)
	}
	client := newClient(t)
	if code, env := doJSON(t, client, http.MethodPost, srvURL+"/api/setup",
		map[string]string{"username": "admin", "password": "password123", "setupCode": codeAny.(string)}); code != http.StatusOK {
		t.Fatalf("setup 失败: %s", env.Msg)
	}
	return client
}

// createTestCert 上传一份自签证书并返回其 ID（供需要真实证书的入站使用）。
func createTestCert(t *testing.T, client *http.Client, srvURL string) uint {
	t.Helper()
	certPem, keyPem := selfSignedPEM(t)
	code, env := doJSON(t, client, http.MethodPost, srvURL+"/api/certs", map[string]any{
		"name": "test-cert", "mode": "managed", "certPem": certPem, "keyPem": keyPem,
	})
	if code != http.StatusOK {
		t.Fatalf("创建证书失败: code=%d msg=%s", code, env.Msg)
	}
	var cert struct {
		ID uint `json:"id"`
	}
	decodeData(t, env, &cert)
	return cert.ID
}

// selfSignedPEM 生成 localhost 自签证书与私钥 PEM（有效期一年）。
func selfSignedPEM(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成私钥失败: %v", err)
	}
	tpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("生成证书失败: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("序列化私钥失败: %v", err)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return certPEM, keyPEM
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
	UserCount  int64    `json:"userCount"`
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
	srv, _, setupCode := newServerWithCode(t)
	client := newClient(t)

	// 首次设置：携带启动日志中的一次性设置码创建管理员并自动登录
	code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/setup",
		map[string]string{"username": "admin", "password": "password123", "setupCode": setupCode})
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
	// 重复 setup 被拒（设置码已消费，带上原码也不行）
	if code, _ = doJSON(t, client, http.MethodPost, srv.URL+"/api/setup",
		map[string]string{"username": "hacker", "password": "password123", "setupCode": setupCode}); code != http.StatusConflict {
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
		UserCount  int64           `json:"userCount"`
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
	if created.UserCount != 0 {
		t.Errorf("新建入站不应有用户: %d", created.UserCount)
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
