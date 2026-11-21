package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// 公网入口安全加固的端到端用例：设置码、登录限流、改密吊销会话、Cookie 加固。

// syncBuf 并发安全的日志缓冲（临时接管默认 logger 期间，server.New 派生的后台 goroutine 仍可能写入）。
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *syncBuf) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *syncBuf) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

// mustNewEngineWithCode 装配引擎并从启动日志截获一次性设置码（与运维从服务器控制台获取同源）。
func mustNewEngineWithCode(dbPath, dataDir string) (http.Handler, string) {
	var buf syncBuf
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	handler := mustNewEngine(dbPath, dataDir)
	slog.SetDefault(old)
	for _, line := range strings.Split(buf.String(), "\n") {
		var rec struct {
			SetupCode string `json:"setup_code"`
		}
		if json.Unmarshal([]byte(line), &rec) == nil && rec.SetupCode != "" {
			return handler, rec.SetupCode
		}
	}
	return handler, ""
}

// newAuthServer 鉴权链路测试服务（setup/login/password 不触内核，无需内核构建标签）。
func newAuthServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	handler, code := mustNewEngineWithCode(filepath.Join(t.TempDir(), "panel.db"), t.TempDir())
	if code == "" {
		t.Fatal("未截获到一次性设置码")
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, code
}

// TestLoginLockout 连续 5 次错误密码后锁定：第 6 次即使密码正确也 429。
func TestLoginLockout(t *testing.T) {
	srv, setupCode := newAuthServer(t)
	client := newClient(t)
	if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/setup",
		map[string]string{"username": "admin", "password": "password123", "setupCode": setupCode}); code != http.StatusOK {
		t.Fatalf("setup 失败: %s", env.Msg)
	}
	for i := 0; i < 5; i++ {
		if code, _ := doJSON(t, client, http.MethodPost, srv.URL+"/api/login",
			map[string]string{"username": "admin", "password": "wrong-password"}); code != http.StatusUnauthorized {
			t.Fatalf("第 %d 次错误密码状态码 = %d, want 401", i+1, code)
		}
	}
	if code, _ := doJSON(t, client, http.MethodPost, srv.URL+"/api/login",
		map[string]string{"username": "admin", "password": "password123"}); code != http.StatusTooManyRequests {
		t.Fatalf("锁定期间正确密码状态码 = %d, want 429", code)
	}
}

// TestLoginLockoutResetBySuccess 成功登录清零失败计数，反复「失败-成功」不触发锁定。
func TestLoginLockoutResetBySuccess(t *testing.T) {
	srv, setupCode := newAuthServer(t)
	client := newClient(t)
	if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/setup",
		map[string]string{"username": "admin", "password": "password123", "setupCode": setupCode}); code != http.StatusOK {
		t.Fatalf("setup 失败: %s", env.Msg)
	}
	for round := 0; round < 3; round++ {
		for i := 0; i < 4; i++ { // 每轮 4 次失败（差一次到阈值），随后成功清零
			if code, _ := doJSON(t, client, http.MethodPost, srv.URL+"/api/login",
				map[string]string{"username": "admin", "password": "wrong-password"}); code != http.StatusUnauthorized {
				t.Fatalf("第 %d 轮第 %d 次错误密码状态码 = %d, want 401", round+1, i+1, code)
			}
		}
		if code, _ := doJSON(t, client, http.MethodPost, srv.URL+"/api/login",
			map[string]string{"username": "admin", "password": "password123"}); code != http.StatusOK {
			t.Fatalf("第 %d 轮正确密码应放行, got %d", round+1, code)
		}
	}
}

// TestChangePasswordRevokesSessions 改密后所有旧会话 Cookie（含当前端）一律 401，新密码可登录。
func TestChangePasswordRevokesSessions(t *testing.T) {
	srv, setupCode := newAuthServer(t)
	clientA := newClient(t)
	if code, env := doJSON(t, clientA, http.MethodPost, srv.URL+"/api/setup",
		map[string]string{"username": "admin", "password": "password123", "setupCode": setupCode}); code != http.StatusOK {
		t.Fatalf("setup 失败: %s", env.Msg)
	}
	clientB := newClient(t)
	if code, _ := doJSON(t, clientB, http.MethodPost, srv.URL+"/api/login",
		map[string]string{"username": "admin", "password": "password123"}); code != http.StatusOK {
		t.Fatal("第二端登录失败")
	}
	if code, _ := doJSON(t, clientB, http.MethodGet, srv.URL+"/api/me", nil); code != http.StatusOK {
		t.Fatal("改密前第二端会话应可用")
	}
	if code, _ := doJSON(t, clientA, http.MethodPut, srv.URL+"/api/password",
		map[string]string{"oldPassword": "password123", "newPassword": "newpassword456"}); code != http.StatusOK {
		t.Fatal("改密失败")
	}
	if code, _ := doJSON(t, clientA, http.MethodGet, srv.URL+"/api/me", nil); code != http.StatusUnauthorized {
		t.Fatalf("改密后当前端状态码 = %d, want 401", code)
	}
	if code, _ := doJSON(t, clientB, http.MethodGet, srv.URL+"/api/me", nil); code != http.StatusUnauthorized {
		t.Fatalf("改密后另一端状态码 = %d, want 401", code)
	}
	if code, _ := doJSON(t, newClient(t), http.MethodPost, srv.URL+"/api/login",
		map[string]string{"username": "admin", "password": "newpassword456"}); code != http.StatusOK {
		t.Fatal("新密码登录失败")
	}
}

// TestSessionCookieFlags 会话 Cookie 恒为 SameSite=Lax；HTTPS 信道（直连 TLS 或反代声明）附加 Secure。
func TestSessionCookieFlags(t *testing.T) {
	srv, setupCode := newAuthServer(t)
	client := newClient(t)
	if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/setup",
		map[string]string{"username": "admin", "password": "password123", "setupCode": setupCode}); code != http.StatusOK {
		t.Fatalf("setup 失败: %s", env.Msg)
	}

	raw := doRawLogin(t, srv.URL, "")
	if !strings.Contains(raw, "SameSite=Lax") {
		t.Fatalf("明文信道 Cookie 应含 SameSite=Lax: %s", raw)
	}
	if strings.Contains(raw, "Secure") {
		t.Fatalf("明文信道 Cookie 不应含 Secure: %s", raw)
	}

	raw = doRawLogin(t, srv.URL, "https")
	if !strings.Contains(raw, "SameSite=Lax") || !strings.Contains(raw, "Secure") {
		t.Fatalf("反代 https 信道 Cookie 应含 SameSite=Lax 与 Secure: %s", raw)
	}
}

// doRawLogin 发送登录请求并返回 Set-Cookie 原文（不入 cookie jar）。
func doRawLogin(t *testing.T, srvURL, forwardedProto string) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{"username": "admin", "password": "password123"})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, srvURL+"/api/login", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if forwardedProto != "" {
		req.Header.Set("X-Forwarded-Proto", forwardedProto)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("登录状态码 = %d, want 200", resp.StatusCode)
	}
	return resp.Header.Get("Set-Cookie")
}
