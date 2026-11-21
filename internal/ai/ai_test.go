package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"proxy-ui/internal/db"
)

// newService 以临时数据库构造 AI 服务。
func newService(t *testing.T) *Service {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	return New(database)
}

// mustProvider 建一个配置完整的供应商并返回其 ID。
func mustProvider(t *testing.T, s *Service, name string) uint {
	t.Helper()
	p, err := s.CreateProvider(ProviderInput{
		Name: name, BaseURL: "https://" + name + ".example.com/v1/", APIKey: "sk-" + name, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

// mustModel 建一个模型并返回其 ID。
func mustModel(t *testing.T, s *Service, providerID uint, modelID string) uint {
	t.Helper()
	m, err := s.CreateModel(providerID, ModelInput{
		ModelID: modelID, Enabled: true,
		ReasoningLevels:       []ReasoningLevel{{Value: "low"}, {Value: "high", Label: "高"}},
		DefaultReasoningLevel: "low",
	})
	if err != nil {
		t.Fatal(err)
	}
	return m.ID
}

// ---------- 供应商读写 ----------

func TestProviderCRUD(t *testing.T) {
	s := newService(t)

	// 归一化：名称/地址去空白、地址去尾部斜杠
	p, err := s.CreateProvider(ProviderInput{
		Name: " OpenAI ", BaseURL: " https://api.example.com/v1/ ", APIKey: " sk-1 ", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "OpenAI" || p.BaseURL != "https://api.example.com/v1" || p.APIKey != "sk-1" || p.Sort != 1 {
		t.Fatalf("供应商未归一化: %+v", p)
	}

	// 排序值追加到末尾
	p2, err := s.CreateProvider(ProviderInput{Name: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", APIKey: "k", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if p2.Sort != 2 {
		t.Fatalf("新增供应商应排到末尾: %+v", p2)
	}
	list, err := s.ListProviders()
	if err != nil || len(list) != 2 || list[0].ID != p.ID || list[1].ID != p2.ID {
		t.Fatalf("供应商列表顺序不符合预期: %+v err=%v", list, err)
	}

	// 回显-回传场景：不携带 api-key 更新应保留已存 Key
	updated, err := s.UpdateProvider(p.ID, ProviderInput{
		Name: "OpenAI", BaseURL: "https://api.example.com/v1", Enabled: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.APIKey != "sk-1" || updated.Enabled {
		t.Fatalf("空 api-key 更新不应丢 Key、启停应生效: %+v", updated)
	}

	// 重排
	if err := s.OrderProviders([]uint{p2.ID, p.ID}); err != nil {
		t.Fatal(err)
	}
	list, err = s.ListProviders()
	if err != nil || len(list) != 2 || list[0].ID != p2.ID || list[1].ID != p.ID {
		t.Fatalf("重排后顺序不符合预期: %+v err=%v", list, err)
	}

	// 删除供应商应连带删除其模型
	mustModel(t, s, p.ID, "gpt-test")
	if err := s.DeleteProvider(p.ID); err != nil {
		t.Fatal(err)
	}
	var models []db.AIModel
	if err := s.db.Where("provider_id = ?", p.ID).Find(&models).Error; err != nil || len(models) != 0 {
		t.Fatalf("删除供应商后其模型应一并删除: %+v err=%v", models, err)
	}
	if err := s.DeleteProvider(p.ID); err == nil {
		t.Fatal("删除不存在的供应商应报错")
	}
}

func TestProviderValidation(t *testing.T) {
	s := newService(t)
	cases := []ProviderInput{
		{Name: "", BaseURL: "https://api.example.com", APIKey: "k"},
		{Name: "x", BaseURL: "api.example.com", APIKey: "k"},
		{Name: "x", BaseURL: "ftp://api.example.com", APIKey: "k"},
		{Name: "x", BaseURL: " ", APIKey: "k"},
	}
	for i, in := range cases {
		if _, err := s.CreateProvider(in); err == nil {
			t.Fatalf("case %d 应校验失败: %+v", i, in)
		}
	}
}

// ---------- 模型读写 ----------

func TestModelUniqueness(t *testing.T) {
	s := newService(t)
	a, b := mustProvider(t, s, "a"), mustProvider(t, s, "b")

	mustModel(t, s, a, "gpt-test")

	// 模型 id 跨供应商全局唯一
	if _, err := s.CreateModel(b, ModelInput{ModelID: "gpt-test", Enabled: true}); err == nil {
		t.Fatal("重复模型 id 应被拒绝")
	} else if !strings.Contains(err.Error(), "全局唯一") {
		t.Fatalf("唯一性错误应可读: %v", err)
	}

	// 更新撞别的模型的 id 同样拒绝
	id := mustModel(t, s, b, "ds-test")
	if _, err := s.UpdateModel(id, ModelInput{ModelID: "gpt-test", Enabled: true}); err == nil {
		t.Fatal("更新撞全局唯一 id 应被拒绝")
	}
	// 更新保持自身 id 不受影响
	if _, err := s.UpdateModel(id, ModelInput{ModelID: "ds-test", Label: "DS", Enabled: true}); err != nil {
		t.Fatal(err)
	}
}

func TestModelValidation(t *testing.T) {
	s := newService(t)
	p := mustProvider(t, s, "a")
	cases := []ModelInput{
		{ModelID: "  ", Enabled: true},                              // 空 id
		{ModelID: "m", DefaultReasoningLevel: "low", Enabled: true}, // 无等级列表却给默认
		{ModelID: "m", DefaultReasoningLevel: "ultra", Enabled: true, ReasoningLevels: []ReasoningLevel{{Value: "low"}}}, // 默认不在列表
		{ModelID: "m", Enabled: true, ReasoningLevels: []ReasoningLevel{{Value: "low"}, {Value: "low"}}},                 // 值重复
		{ModelID: "m", Enabled: true, ReasoningLevels: []ReasoningLevel{{Value: " "}}},                                   // 空值
		{ModelID: "m", ContextWindow: -1, Enabled: true},                                                                 // 负上下文窗口
	}
	for i, in := range cases {
		if _, err := s.CreateModel(p, in); err == nil {
			t.Fatalf("case %d 应校验失败: %+v", i, in)
		}
	}
	// label 缺省回填为值
	m, err := s.CreateModel(p, ModelInput{
		ModelID: "m", Enabled: true,
		ReasoningLevels: []ReasoningLevel{{Value: "low"}, {Value: "high", Label: "高"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	levels, err := ParseReasoningLevels(m.ReasoningLevels)
	if err != nil || len(levels) != 2 || levels[0].Label != "low" || levels[1].Label != "高" {
		t.Fatalf("推理等级归一化不符合预期: %+v err=%v", levels, err)
	}
	if _, err := ParseReasoningLevels("not-json"); err == nil {
		t.Fatal("损坏的推理等级 JSON 应报错")
	}
}

func TestImportModels(t *testing.T) {
	s := newService(t)
	a, b := mustProvider(t, s, "a"), mustProvider(t, s, "b")
	mustModel(t, s, a, "gpt-test")

	imported, err := s.ImportModels(b, []string{"gpt-test", " ds-new ", ""})
	if err != nil {
		t.Fatal(err)
	}
	if imported != 1 {
		t.Fatalf("已占用 id 应跳过、空 id 忽略: imported=%d", imported)
	}
	var models []db.AIModel
	if err := s.db.Where("provider_id = ?", b).Order("id").Find(&models).Error; err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ModelID != "ds-new" || !models[0].Enabled {
		t.Fatalf("导入结果不符合预期: %+v", models)
	}
}

// ---------- transport 元数据 ----------

func TestTransport(t *testing.T) {
	s := newService(t)

	ok := mustProvider(t, s, "ok")        // 启用 + 有 Key：进 transport
	disabled := mustProvider(t, s, "off") // 停用：不进
	// 无 Key：不进
	if _, err := s.CreateProvider(ProviderInput{Name: "nokey", BaseURL: "https://nokey.example.com/v1", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateProvider(disabled, ProviderInput{Name: "off", BaseURL: "https://off.example.com", Enabled: false}); err != nil {
		t.Fatal(err)
	}

	mustModel(t, s, ok, "gpt-test")
	id := mustModel(t, s, ok, "gpt-mini") // 之后停用：不应出现在 transport
	if _, err := s.UpdateModel(id, ModelInput{
		ModelID: "gpt-mini", Enabled: false,
		ReasoningLevels:       []ReasoningLevel{{Value: "low"}},
		DefaultReasoningLevel: "low",
	}); err != nil {
		t.Fatal(err)
	}
	mustModel(t, s, disabled, "off-model") // 供应商停用，模型不进

	transport, err := s.Transport()
	if err != nil {
		t.Fatal(err)
	}
	if len(transport.Providers) != 1 {
		t.Fatalf("只有配置完整的启用供应商进入 transport: %+v", transport.Providers)
	}
	p := transport.Providers[0]
	if p.ID != "1" || p.Label != "ok" || p.Mode != TransportModeChat {
		t.Fatalf("供应商字段不符合预期: %+v", p)
	}
	if p.Endpoint != "/api/ai/providers/1/chat" {
		t.Fatalf("endpoint 应指向该供应商的对话代理: %s", p.Endpoint)
	}
	if len(p.Models) != 1 {
		t.Fatalf("停用模型不应出现: %+v", p.Models)
	}
	m := p.Models[0]
	if m.ID != "gpt-test" || m.ContextWindow != 0 {
		t.Fatalf("模型字段不符合预期: %+v", m)
	}
	if len(m.ReasoningLevels) != 2 || m.DefaultReasoningLevel != "low" {
		t.Fatalf("推理等级不符合预期: %+v", m.ReasoningLevels)
	}

	// 序列化形状与前端 @veltra/ai ChatProvider/ChatModel 字段对齐
	raw, _ := json.Marshal(transport)
	for _, key := range []string{`"id"`, `"label"`, `"endpoint"`, `"mode"`, `"models"`, `"reasoningLevels"`, `"defaultReasoningLevel"`} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("transport 序列化缺少字段 %s: %s", key, raw)
		}
	}
}

// ---------- 对话代理 ----------

func TestChat(t *testing.T) {
	s := newService(t)
	var gotPath, gotAuth, gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"ok\":true}\n\n"))
	}))
	defer upstream.Close()

	p, err := s.CreateProvider(ProviderInput{Name: "up", BaseURL: upstream.URL, APIKey: "sk-up", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	mustModel(t, s, p.ID, "gpt-test") // 默认推理等级 low

	// 缺省推理等级注入 + 上游地址与鉴权
	resp, err := s.Chat(context.Background(), p.ID, json.RawMessage(`{"model":"gpt-test","messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if gotPath != "/chat/completions" || gotAuth != "Bearer sk-up" {
		t.Fatalf("上游路由或鉴权不符合预期: %s %s", gotPath, gotAuth)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(gotBody), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["model"] != "gpt-test" || payload["stream"] != true || payload["reasoning_effort"] != "low" {
		t.Fatalf("缺省时应注入模型默认推理等级: %s", gotBody)
	}

	// 前端显式推理等级优先
	resp, err = s.Chat(context.Background(), p.ID, json.RawMessage(`{"model":"gpt-test","reasoning_effort":"high"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	payload = map[string]any{}
	_ = json.Unmarshal([]byte(gotBody), &payload)
	if payload["reasoning_effort"] != "high" {
		t.Fatalf("显式推理等级不应被覆盖: %s", gotBody)
	}

	// 错误分支：模型不存在 / 未指定模型 / 非法 JSON / 供应商停用 / 未配置 Key
	_, err = s.Chat(context.Background(), p.ID, json.RawMessage(`{"model":"missing"}`))
	if !errors.Is(err, ErrModelNotFound) {
		t.Fatalf("模型不存在应返回 ErrModelNotFound: %v", err)
	}
	_, err = s.Chat(context.Background(), p.ID, json.RawMessage(`{"messages":[]}`))
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("未指定模型应返回 ErrInvalidRequest: %v", err)
	}
	_, err = s.Chat(context.Background(), p.ID, json.RawMessage(`not-json`))
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("非法 JSON 应返回 ErrInvalidRequest: %v", err)
	}
	if _, err := s.UpdateProvider(p.ID, ProviderInput{Name: "up", BaseURL: upstream.URL, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Chat(context.Background(), p.ID, nil); !errors.Is(err, ErrProviderDisabled) {
		t.Fatalf("停用供应商应返回 ErrProviderDisabled: %v", err)
	}
	if _, err := s.UpdateProvider(p.ID, ProviderInput{Name: "up", BaseURL: upstream.URL, APIKey: "sk-up", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateProvider(p.ID, ProviderInput{Name: "up", BaseURL: upstream.URL, Enabled: true}); err != nil {
		t.Fatal(err)
	} // 空 Key 保留语义下再显式清空 Key 验证未配置分支
	if err := s.db.Model(&db.AIProvider{}).Where("id = ?", p.ID).Update("api_key", "").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Chat(context.Background(), p.ID, nil); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("缺 API Key 应返回 ErrNotConfigured: %v", err)
	}
	if _, err := s.Chat(context.Background(), 9999, nil); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("供应商不存在应返回 ErrInvalidRequest: %v", err)
	}
}

// ---------- 上游 /models 拉取 ----------

func TestFetchModels(t *testing.T) {
	s := newService(t)

	// OpenAI 标准 {data:[...]} 形状：归一化、按 id 排序
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-up" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"zeta"},{"id":"alpha","owned_by":"openai"},{"id":"alpha"},{"id":" "}]}`))
	}))
	defer srv.Close()
	p, err := s.CreateProvider(ProviderInput{Name: "up", BaseURL: srv.URL, APIKey: "sk-up", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	models, err := s.FetchModels(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].ID != "alpha" || models[0].OwnedBy != "openai" || models[1].ID != "zeta" {
		t.Fatalf("/models 归一化不符合预期: %+v", models)
	}

	// 裸数组形状
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"m1","owned_by":"x"}]`))
	}))
	defer srv2.Close()
	if _, err := s.UpdateProvider(p.ID, ProviderInput{Name: "up", BaseURL: srv2.URL, APIKey: "sk-up", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	models, err = s.FetchModels(context.Background(), p.ID)
	if err != nil || len(models) != 1 || models[0].ID != "m1" {
		t.Fatalf("裸数组 /models 应可解析: %+v err=%v", models, err)
	}

	// 上游报错：错误信息包含状态码与响应摘要
	srv3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom happened", http.StatusInternalServerError)
	}))
	defer srv3.Close()
	if _, err := s.UpdateProvider(p.ID, ProviderInput{Name: "up", BaseURL: srv3.URL, APIKey: "sk-up", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	_, err = s.FetchModels(context.Background(), p.ID)
	if err == nil || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("错误信息应含上游状态码与响应摘要: %v", err)
	}

	if _, err := s.FetchModels(context.Background(), 9999); err == nil {
		t.Fatal("供应商不存在应报错")
	}
}

// ---------- 工具注册表 ----------

func TestToolRegistry(t *testing.T) {
	s := newService(t)
	s.Register(Tool{
		Name: "echo", Description: "回显参数",
		Parameters: map[string]any{"type": "object"},
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var input struct {
				Text string `json:"text"`
			}
			if len(args) > 0 {
				if err := json.Unmarshal(args, &input); err != nil {
					return nil, err
				}
			}
			return map[string]any{"echo": input.Text}, nil
		},
	})

	result, err := s.Call(context.Background(), "echo", json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(result)
	if string(raw) != `{"echo":"hello"}` {
		t.Fatalf("工具执行结果不符合预期: %s", raw)
	}

	if _, err := s.Call(context.Background(), "missing", nil); err == nil {
		t.Fatal("未注册工具应报错")
	}

	infos := s.ToolInfos()
	if len(infos) != 1 || infos[0].Name != "echo" || infos[0].Description != "回显参数" {
		t.Fatalf("工具描述不符合预期: %+v", infos)
	}
	infoJSON, err := json.Marshal(infos)
	if err != nil {
		t.Fatalf("序列化工具描述失败: %v", err)
	}
	if !strings.Contains(string(infoJSON), `"needsConfirm":false`) || strings.Contains(string(infoJSON), `"Exec"`) {
		t.Fatalf("工具描述 JSON 标签不符: %s", infoJSON)
	}
}

// TestOrderProvidersPartial 部分 ID 重排时，缺失 ID 自动追加到末尾且保留原相对顺序，无效/重复 ID 被忽略。
func TestOrderProvidersPartial(t *testing.T) {
	s := newService(t)
	id1 := mustProvider(t, s, "p1")
	id2 := mustProvider(t, s, "p2")
	id3 := mustProvider(t, s, "p3")
	id4 := mustProvider(t, s, "p4")

	if err := s.OrderProviders([]uint{id3, 9999, id3, id1}); err != nil {
		t.Fatalf("部分重排失败: %v", err)
	}
	list, err := s.ListProviders()
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []uint{id3, id1, id2, id4}
	if len(list) != len(wantIDs) {
		t.Fatalf("供应商数量 = %d, want %d", len(list), len(wantIDs))
	}
	for i, wantID := range wantIDs {
		if list[i].ID != wantID || list[i].Sort != i+1 {
			t.Errorf("位置 %d: id=%d sort=%d, want id=%d sort=%d", i, list[i].ID, list[i].Sort, wantID, i+1)
		}
	}
}

// TestSummarizeBodyUTF8 超 200 个中文字符截断结果为合法 UTF-8，不出现半个字符乱码。
func TestSummarizeBodyUTF8(t *testing.T) {
	in := strings.Repeat("中", 250)
	got := summarizeBody([]byte(in))
	if !utf8.ValidString(got) {
		t.Fatalf("截断结果不是合法 UTF-8: %q", got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("超长文本应带省略号: %q", got)
	}
	if n := utf8.RuneCountInString(got); n != 203 {
		t.Errorf("截断后字符数 = %d, want 203", n)
	}
}
