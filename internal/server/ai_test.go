package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// aiConfig AI 供应商配置响应（测试断言用）。
type aiConfig struct {
	BaseURL    string `json:"baseUrl"`
	Model      string `json:"model"`
	HasKey     bool   `json:"hasKey"`
	Configured bool   `json:"configured"`
}

// aiToolCall 工具执行请求。
type aiToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// aiInboundView AI 工具返回的入站视图。
type aiInboundView struct {
	ID         uint            `json:"id"`
	Name       string          `json:"name"`
	Protocol   string          `json:"protocol"`
	ListenPort int             `json:"listenPort"`
	Config     json.RawMessage `json:"config"`
	Warnings   []string        `json:"warnings"`
}

// callAITool 经 /api/ai/tools/call 执行工具并解包 data.result。
func callAITool[T any](t *testing.T, client *http.Client, srvURL string, call aiToolCall) (T, int, string) {
	t.Helper()
	code, env := doJSON(t, client, http.MethodPost, srvURL+"/api/ai/tools/call", call)
	if code != http.StatusOK || !env.Success {
		var zero T
		return zero, code, env.Msg
	}
	var result T
	if err := json.Unmarshal(env.Data, &struct {
		Result *T `json:"result"`
	}{Result: &result}); err != nil {
		t.Fatalf("解析工具结果失败: %v", err)
	}
	return result, code, env.Msg
}

func TestAIConfigEndpoints(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	// 未配置时返回未就绪视图
	code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/ai/config", nil)
	if code != http.StatusOK || !env.Success {
		t.Fatalf("查询 AI 配置失败: code=%d msg=%s", code, env.Msg)
	}
	var cfg aiConfig
	if err := json.Unmarshal(env.Data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Configured || cfg.HasKey {
		t.Fatalf("未配置时不应就绪: %+v", cfg)
	}

	// 非法 base-url 被拒
	if code, _ = doJSON(t, client, http.MethodPut, srv.URL+"/api/ai/config",
		map[string]string{"baseUrl": "api.example.com", "model": "m", "apiKey": "sk"}); code != http.StatusBadRequest {
		t.Fatalf("非法 base-url 状态码 = %d, want 400", code)
	}

	// 保存配置（key 不回显，仅 hasKey）
	code, env = doJSON(t, client, http.MethodPut, srv.URL+"/api/ai/config",
		map[string]string{"baseUrl": "https://api.example.com/v1", "model": "gpt-test", "apiKey": "sk-1", "reasoningEffort": "low"})
	if code != http.StatusOK || !env.Success {
		t.Fatalf("保存 AI 配置失败: code=%d msg=%s", code, env.Msg)
	}
	cfg = aiConfig{}
	if err := json.Unmarshal(env.Data, &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.Configured || !cfg.HasKey || cfg.BaseURL != "https://api.example.com/v1" || cfg.Model != "gpt-test" {
		t.Fatalf("保存后配置视图不符合预期: %+v", cfg)
	}

	// 回显视图不泄漏 key：data 序列化中不应出现明文
	raw, _ := json.Marshal(env.Data)
	if strings.Contains(string(raw), "sk-1") {
		t.Fatalf("配置视图泄漏 api-key: %s", raw)
	}

	// 不带 apiKey 保存保留已存 Key
	code, env = doJSON(t, client, http.MethodPut, srv.URL+"/api/ai/config",
		map[string]string{"baseUrl": "https://api.example.com/v1", "model": "gpt-test"})
	if code != http.StatusOK {
		t.Fatalf("更新 AI 配置失败: %s", env.Msg)
	}
	cfg = aiConfig{}
	_ = json.Unmarshal(env.Data, &cfg)
	if !cfg.HasKey {
		t.Fatal("空 apiKey 保存后 Key 不应丢失")
	}
}

func TestAIChatNotConfigured(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/ai/chat", strings.NewReader(`{"messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("未配置供应商时对话状态码 = %d, want 503", resp.StatusCode)
	}
}

func TestAIToolsInboundFlow(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "panel.db")
	dataDir := t.TempDir()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 上游按 OpenAI chat/completions 流式格式回一帧文本增量
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()
	srv := httptest.NewServer(mustNewEngine(dbPath, dataDir))
	defer srv.Close()
	client := setupAdmin(t, srv.URL)

	// 配置供应商指向本地假上游
	if code, env := doJSON(t, client, http.MethodPut, srv.URL+"/api/ai/config",
		map[string]string{"baseUrl": upstream.URL, "model": "gpt-test", "apiKey": "sk-1"}); code != http.StatusOK {
		t.Fatalf("保存 AI 配置失败: %s", env.Msg)
	}

	// 流式对话经代理回传 SSE
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/ai/chat", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("对话代理状态码 = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("对话代理 Content-Type = %s", ct)
	}
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	if !strings.Contains(string(buf[:n]), "data: [DONE]") {
		t.Fatalf("对话代理未回传 SSE 流: %q", string(buf[:n]))
	}

	// 工具列表含入站查询/创建
	code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/ai/tools", nil)
	if code != http.StatusOK || !env.Success {
		t.Fatalf("查询工具列表失败: code=%d msg=%s", code, env.Msg)
	}
	var tools []map[string]any
	if err := json.Unmarshal(env.Data, &tools); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, tool := range tools {
		found[tool["name"].(string)] = true
	}
	for _, name := range []string{"list_inbounds", "create_inbound", "get_kernel_status", "get_realtime_stats"} {
		if !found[name] {
			t.Fatalf("工具 %s 未注册: %v", name, found)
		}
	}

	// 「帮我创建一个 Hysteria2 入站」：工具按默认配置创建
	created, code, msg := callAITool[aiInboundView](t, client, srv.URL, aiToolCall{
		Name: "create_inbound", Arguments: json.RawMessage(`{"protocol":"hysteria2","listenPort":8443,"config":{}}`),
	})
	if code != http.StatusOK {
		t.Fatalf("AI 创建入站失败: code=%d msg=%s", code, msg)
	}
	if created.Protocol != "hysteria2" || created.ListenPort != 8443 || created.Name == "" {
		t.Fatalf("创建的入站不符合预期: %+v", created)
	}
	var cfg struct {
		Password string `json:"password"`
	}
	if err := json.Unmarshal(created.Config, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Password == "" {
		t.Fatalf("空 config 应按协议填充默认值（随机密码）: %s", created.Config)
	}

	// 「列出所有入站」：返回真实入站数据
	list, code, msg := callAITool[[]aiInboundView](t, client, srv.URL, aiToolCall{Name: "list_inbound"})
	if code != http.StatusOK {
		// 名称打错时应报「工具不存在」
		if code != http.StatusBadRequest || !strings.Contains(msg, "工具不存在") {
			t.Fatalf("未注册工具应 400 报工具不存在: code=%d msg=%s", code, msg)
		}
	}
	list, code, msg = callAITool[[]aiInboundView](t, client, srv.URL, aiToolCall{Name: "list_inbounds"})
	if code != http.StatusOK {
		t.Fatalf("AI 列出入站失败: code=%d msg=%s", code, msg)
	}
	if len(list) != 1 || list[0].ID != created.ID || list[0].Protocol != "hysteria2" {
		t.Fatalf("列出入站未包含新建记录: %+v", list)
	}
}
