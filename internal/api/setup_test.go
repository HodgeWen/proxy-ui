package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
)

// memSessions 测试用内存会话管理器（记录 Clear 调用次数）。
type memSessions struct {
	mu      sync.Mutex
	tokens  map[string]bool
	cleared int
}

func newMemSessions() *memSessions { return &memSessions{tokens: make(map[string]bool)} }

func (m *memSessions) Create() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	token := fmt.Sprintf("token-%d", len(m.tokens))
	m.tokens[token] = true
	return token
}

func (m *memSessions) Remove(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, token)
}

func (m *memSessions) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens = make(map[string]bool)
	m.cleared++
}

// newAuthTestAPI 在指定库文件上构造 API 并挂载鉴权路由（不依赖内核）。
func newAuthTestAPI(t *testing.T, dbPath string) (*API, *gin.Engine, *memSessions) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	sessions := newMemSessions()
	a := New(database, sessions, nil, nil, nil, t.TempDir())
	r := gin.New()
	r.POST("/api/setup", a.HandleSetup)
	r.POST("/api/login", a.HandleLogin)
	r.PUT("/api/password", a.HandleChangePassword)
	return a, r, sessions
}

// doAuthJSON 发送 JSON 请求并返回状态码（可在并发 goroutine 中使用）。
func doAuthJSON(r *gin.Engine, method, path string, body map[string]string) int {
	raw, err := json.Marshal(body)
	if err != nil {
		return -1
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w.Code
}

// currentSetupCode 持锁读取当前设置码（与 handler 并发访问时避免数据竞态）。
func currentSetupCode(a *API) string {
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	return a.setupCode
}

// adminCount 数据库中的管理员数量。
func adminCount(t *testing.T, a *API) int64 {
	t.Helper()
	var n int64
	if err := a.db.Model(&db.Admin{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// wrongCodeOf 对码首字符做字符集内轮转，得到一个确定不同的错码。
func wrongCodeOf(code string) string {
	b := []byte(code)
	b[0] = setupCodeCharset[(strings.IndexByte(setupCodeCharset, b[0])+1)%len(setupCodeCharset)]
	return string(b)
}

// setupBody 构造首次设置请求体。
func setupBody(code string) map[string]string {
	return map[string]string{"username": "admin", "password": "password123", "setupCode": code}
}

// TestNewSetupCodeFormat 设置码长度、字符集与随机性。
func TestNewSetupCodeFormat(t *testing.T) {
	if len(setupCodeCharset) != 32 {
		t.Fatalf("字符集长度 = %d, want 32（保证取模映射无偏）", len(setupCodeCharset))
	}
	seen := make(map[string]bool, 1000)
	for range 1000 {
		code := newSetupCode()
		if len(code) != setupCodeLen {
			t.Fatalf("设置码长度 = %d, want %d", len(code), setupCodeLen)
		}
		for _, ch := range code {
			if !strings.ContainsRune(setupCodeCharset, ch) {
				t.Fatalf("设置码含字符集外字符 %q: %s", ch, code)
			}
		}
		if seen[code] {
			t.Fatalf("设置码重复: %s", code)
		}
		seen[code] = true
	}
}

// TestHandleSetupCodeGate 缺码 400、错码 401、对码 200（忽略大小写）且成功后码失效、重复设置 409。
func TestHandleSetupCodeGate(t *testing.T) {
	a, r, _ := newAuthTestAPI(t, filepath.Join(t.TempDir(), "panel.db"))
	code := currentSetupCode(a)

	if got := doAuthJSON(r, http.MethodPost, "/api/setup",
		map[string]string{"username": "admin", "password": "password123"}); got != http.StatusBadRequest {
		t.Fatalf("缺设置码状态码 = %d, want 400", got)
	}
	if got := doAuthJSON(r, http.MethodPost, "/api/setup", setupBody(wrongCodeOf(code))); got != http.StatusUnauthorized {
		t.Fatalf("错码状态码 = %d, want 401", got)
	}
	if got := doAuthJSON(r, http.MethodPost, "/api/setup", setupBody(strings.ToLower(code))); got != http.StatusOK {
		t.Fatalf("对码（小写）状态码 = %d, want 200", got)
	}
	if n := adminCount(t, a); n != 1 {
		t.Fatalf("管理员数量 = %d, want 1", n)
	}
	if currentSetupCode(a) != "" {
		t.Fatal("设置成功后设置码应失效")
	}
	if got := doAuthJSON(r, http.MethodPost, "/api/setup", setupBody(code)); got != http.StatusConflict {
		t.Fatalf("重复设置状态码 = %d, want 409", got)
	}
	if n := adminCount(t, a); n != 1 {
		t.Fatalf("重复设置后管理员数量 = %d, want 1", n)
	}
}

// TestHandleSetupLockedByWrongCode 连续错码触发限流，锁定期间对码也被 429。
func TestHandleSetupLockedByWrongCode(t *testing.T) {
	a, r, _ := newAuthTestAPI(t, filepath.Join(t.TempDir(), "panel.db"))
	code := currentSetupCode(a)
	for i := range failThreshold {
		if got := doAuthJSON(r, http.MethodPost, "/api/setup", setupBody(wrongCodeOf(code))); got != http.StatusUnauthorized {
			t.Fatalf("第 %d 次错码状态码 = %d, want 401", i+1, got)
		}
	}
	if got := doAuthJSON(r, http.MethodPost, "/api/setup", setupBody(code)); got != http.StatusTooManyRequests {
		t.Fatalf("锁定期间对码状态码 = %d, want 429", got)
	}
	if n := adminCount(t, a); n != 0 {
		t.Fatalf("锁定期间不应创建管理员: %d", n)
	}
}

// TestConcurrentSetupSingleAdmin 并发 setup 至多建出一个管理员。
func TestConcurrentSetupSingleAdmin(t *testing.T) {
	a, r, _ := newAuthTestAPI(t, filepath.Join(t.TempDir(), "panel.db"))
	code := currentSetupCode(a)

	const n = 32
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = doAuthJSON(r, http.MethodPost, "/api/setup", setupBody(code))
		}(i)
	}
	wg.Wait()

	ok, conflict := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflict++
		default:
			t.Fatalf("意外状态码 %d", c)
		}
	}
	if ok != 1 || conflict != n-1 {
		t.Fatalf("成功 = %d, 冲突 = %d, want 1/%d", ok, conflict, n-1)
	}
	if got := adminCount(t, a); got != 1 {
		t.Fatalf("管理员数量 = %d, want 1", got)
	}
}

// TestSetupCodeRegeneratesOnRestart 未完成设置即重启：旧码失效、新码可用。
func TestSetupCodeRegeneratesOnRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "panel.db")
	a1, _, _ := newAuthTestAPI(t, dbPath)
	code1 := currentSetupCode(a1)

	a2, r2, _ := newAuthTestAPI(t, dbPath) // 同库重启，仍未初始化
	code2 := currentSetupCode(a2)
	if code2 == "" || code2 == code1 {
		t.Fatalf("重启应重新生成设置码: old=%s new=%s", code1, code2)
	}
	if got := doAuthJSON(r2, http.MethodPost, "/api/setup", setupBody(code1)); got != http.StatusUnauthorized {
		t.Fatalf("旧码状态码 = %d, want 401", got)
	}
	if got := doAuthJSON(r2, http.MethodPost, "/api/setup", setupBody(code2)); got != http.StatusOK {
		t.Fatalf("新码状态码 = %d, want 200", got)
	}
}

// TestChangePasswordClearsSessions 改密成功后吊销全部会话，新密码可登录。
func TestChangePasswordClearsSessions(t *testing.T) {
	a, r, sessions := newAuthTestAPI(t, filepath.Join(t.TempDir(), "panel.db"))
	code := currentSetupCode(a)
	if got := doAuthJSON(r, http.MethodPost, "/api/setup", setupBody(code)); got != http.StatusOK {
		t.Fatalf("setup 状态码 = %d, want 200", got)
	}
	if len(sessions.tokens) == 0 {
		t.Fatal("setup 后应存在会话")
	}
	if got := doAuthJSON(r, http.MethodPut, "/api/password",
		map[string]string{"oldPassword": "password123", "newPassword": "newpassword456"}); got != http.StatusOK {
		t.Fatalf("改密状态码 = %d, want 200", got)
	}
	if sessions.cleared == 0 || len(sessions.tokens) != 0 {
		t.Fatal("改密后应清空全部会话")
	}
	if got := doAuthJSON(r, http.MethodPost, "/api/login",
		map[string]string{"username": "admin", "password": "newpassword456"}); got != http.StatusOK {
		t.Fatalf("新密码登录状态码 = %d, want 200", got)
	}
}
