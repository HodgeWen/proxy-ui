package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"proxy-ui/internal/db"
)

// 对话代理错误哨兵：api 层按类型映射 HTTP 状态码，错误文案直接面向用户。
var (
	// ErrInvalidRequest 请求本身有问题（非法 JSON、未指定模型等）。
	ErrInvalidRequest = errors.New("请求不合法")
	// ErrNotConfigured 供应商未配置完整（缺 base-url / api-key）。
	ErrNotConfigured = errors.New("AI 供应商未配置完整（缺接口地址或 API Key），请先在设置页完成配置")
	// ErrProviderDisabled 供应商已停用。
	ErrProviderDisabled = errors.New("AI 供应商已停用，请在设置页启用后重试")
	// ErrModelNotFound 请求的模型不存在、未启用或不属于该供应商。
	ErrModelNotFound = errors.New("模型不存在或未启用")
)

// Chat 按供应商代理 OpenAI 兼容流式对话请求并返回上游响应（SSE 由调用方转发）。
// body 为前端 @veltra/ai transport 生成的 chat/completions 请求 JSON：按 body.model
// 校验模型归属与启用状态；服务端注入供应商 api-key、强制流式，并在请求未携带
// reasoning_effort 时注入该模型的默认推理等级（前端显式值优先于模型默认值）。
func (s *Service) Chat(ctx context.Context, providerID uint, body []byte) (*http.Response, error) {
	provider, model, payload, err := s.resolveChat(providerID, body)
	if err != nil {
		return nil, err
	}
	enriched, err := enrichChatBody(payload, model.DefaultReasoningLevel)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.BaseURL+"/chat/completions", bytes.NewReader(enriched))
	if err != nil {
		return nil, fmt.Errorf("构造上游请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	return s.client.Do(req)
}

// resolveChat 校验供应商与请求体，解析出目标模型与请求 payload。
func (s *Service) resolveChat(providerID uint, body []byte) (db.AIProvider, db.AIModel, map[string]any, error) {
	var provider db.AIProvider
	if err := s.db.First(&provider, providerID).Error; err != nil {
		return db.AIProvider{}, db.AIModel{}, nil, fmt.Errorf("%w: AI 供应商不存在（%d）", ErrInvalidRequest, providerID)
	}
	if !provider.Enabled {
		return db.AIProvider{}, db.AIModel{}, nil, ErrProviderDisabled
	}
	if provider.BaseURL == "" || provider.APIKey == "" {
		return db.AIProvider{}, db.AIModel{}, nil, ErrNotConfigured
	}
	payload := map[string]any{}
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return db.AIProvider{}, db.AIModel{}, nil, fmt.Errorf("%w: 请求体不是合法 JSON", ErrInvalidRequest)
		}
	}
	modelName, _ := payload["model"].(string)
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return db.AIProvider{}, db.AIModel{}, nil, fmt.Errorf("%w: 请求未指定模型", ErrInvalidRequest)
	}
	var model db.AIModel
	err := s.db.Where("provider_id = ? AND model_id = ?", providerID, modelName).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.AIProvider{}, db.AIModel{}, nil, fmt.Errorf("%w: %s", ErrModelNotFound, modelName)
	}
	if err != nil {
		return db.AIProvider{}, db.AIModel{}, nil, fmt.Errorf("查询模型失败: %w", err)
	}
	if !model.Enabled {
		return db.AIProvider{}, db.AIModel{}, nil, fmt.Errorf("%w: %s", ErrModelNotFound, modelName)
	}
	return provider, model, payload, nil
}

// enrichChatBody 补齐请求体：强制 stream、按需注入模型的默认推理等级。
// 前端已显式携带 reasoning_effort 时不覆盖（显式值优先）。
func enrichChatBody(payload map[string]any, defaultReasoningEffort string) ([]byte, error) {
	payload["stream"] = true
	if _, ok := payload["reasoning_effort"]; !ok && defaultReasoningEffort != "" {
		payload["reasoning_effort"] = defaultReasoningEffort
	}
	return json.Marshal(payload)
}
