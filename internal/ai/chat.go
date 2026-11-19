package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrNotConfigured 供应商未配置完整（缺 base-url / 模型 / api-key）。
var ErrNotConfigured = errors.New("AI 供应商未配置")

// Chat 向上游发起 OpenAI 兼容流式对话请求并返回上游响应（SSE 由调用方转发）。
// body 为浏览器端 transport 生成的 chat/completions 请求 JSON；服务端补齐缺省
// 模型、强制流式，并在请求未携带推理强度时按配置注入 reasoning_effort。
func (s *Service) Chat(ctx context.Context, body []byte) (*http.Response, error) {
	cfg, err := s.LoadConfig()
	if err != nil {
		return nil, err
	}
	if !cfg.Ready() {
		return nil, ErrNotConfigured
	}
	payload, err := enrichChatBody(body, cfg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("构造上游请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	return s.client.Do(req)
}

// enrichChatBody 按供应商配置补齐请求体：缺省模型、强制 stream、按需注入推理强度。
func enrichChatBody(body []byte, cfg Config) ([]byte, error) {
	payload := map[string]any{}
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("请求体不是合法 JSON: %w", err)
		}
	}
	if model, _ := payload["model"].(string); strings.TrimSpace(model) == "" {
		payload["model"] = cfg.Model
	}
	payload["stream"] = true
	if _, ok := payload["reasoning_effort"]; !ok && cfg.ReasoningEffort != "" {
		payload["reasoning_effort"] = cfg.ReasoningEffort
	}
	return json.Marshal(payload)
}
