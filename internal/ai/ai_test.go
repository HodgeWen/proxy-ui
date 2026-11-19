package ai

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

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

func TestConfigSaveLoad(t *testing.T) {
	s := newService(t)

	if cfg, err := s.LoadConfig(); err != nil || cfg.Ready() {
		t.Fatalf("未配置时应返回零值配置: cfg=%v err=%v", cfg, err)
	}

	saved, err := s.SaveConfig(Config{
		BaseURL: "https://api.example.com/v1/", Model: " gpt-test ", APIKey: "sk-1", ReasoningEffort: "low",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.BaseURL != "https://api.example.com/v1" || saved.Model != "gpt-test" {
		t.Fatalf("配置未归一化: %+v", saved)
	}

	loaded, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Ready() || loaded.APIKey != "sk-1" || loaded.ReasoningEffort != "low" {
		t.Fatalf("配置回读不一致: %+v", loaded)
	}

	// 回显-回传场景：不携带 api-key 保存应保留已存 Key
	saved, err = s.SaveConfig(Config{BaseURL: "https://api.example.com/v1", Model: "gpt-test", ReasoningEffort: ""})
	if err != nil {
		t.Fatal(err)
	}
	if saved.APIKey != "sk-1" {
		t.Fatalf("空 api-key 保存后不应丢失已存 Key: %+v", saved)
	}
}

func TestConfigValidation(t *testing.T) {
	s := newService(t)
	cases := []Config{
		{BaseURL: "", Model: "m", APIKey: "k"},
		{BaseURL: "api.example.com", Model: "m", APIKey: "k"},
		{BaseURL: "https://api.example.com", Model: " ", APIKey: "k"},
		{BaseURL: "https://api.example.com", Model: "m", APIKey: "k", ReasoningEffort: "ultra"},
	}
	for i, cfg := range cases {
		if _, err := s.SaveConfig(cfg); err == nil {
			t.Fatalf("case %d 应校验失败: %+v", i, cfg)
		}
	}
}

func TestEnrichChatBody(t *testing.T) {
	cfg := Config{BaseURL: "https://api.example.com/v1", Model: "gpt-test", APIKey: "sk", ReasoningEffort: "high"}

	// 补齐缺省模型、强制流式、注入推理强度
	out, err := enrichChatBody(json.RawMessage(`{"messages":[{"role":"user","content":"hi"}]}`), cfg)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["model"] != "gpt-test" || payload["stream"] != true || payload["reasoning_effort"] != "high" {
		t.Fatalf("请求体补齐不符合预期: %v", payload)
	}

	// 请求已带模型与推理强度时不覆盖
	out, err = enrichChatBody(json.RawMessage(`{"model":"other","stream":true,"reasoning_effort":"low"}`), cfg)
	if err != nil {
		t.Fatal(err)
	}
	payload = map[string]any{}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["model"] != "other" || payload["reasoning_effort"] != "low" {
		t.Fatalf("不应覆盖请求已有字段: %v", payload)
	}

	// 未配置推理强度时不注入
	out, err = enrichChatBody([]byte{}, Config{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	payload = map[string]any{}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["reasoning_effort"]; ok {
		t.Fatalf("未配置推理强度时不应注入: %v", payload)
	}

	if _, err := enrichChatBody(json.RawMessage(`not-json`), cfg); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
}

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
}
