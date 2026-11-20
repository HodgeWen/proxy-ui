package ai

import (
	"fmt"

	"proxy-ui/internal/db"
)

// ChatEndpointFormat 对话代理端点（同源相对路径，前端 @veltra/ai 的 ChatProvider.endpoint）。
const ChatEndpointFormat = "/api/ai/providers/%d/chat"

// TransportModeChat 面板代理固定为 chat/completions 协议（ChatProvider.mode）。
const TransportModeChat = "chat"

// TransportProvider 与前端 @veltra/ai 的 ChatProvider 字段一一对应。
type TransportProvider struct {
	ID       string           `json:"id"`       // 供应商 ID（字符串化，ChatProvider.id 为 string）
	Label    string           `json:"label"`    // UI 显示名
	Endpoint string           `json:"endpoint"` // 对话代理端点（相对路径，走面板会话鉴权）
	Mode     string           `json:"mode"`     // 请求协议模式，固定 chat
	Models   []TransportModel `json:"models"`   // 该供应商下启用中的模型
}

// TransportModel 与前端 @veltra/ai 的 ChatModel 字段一一对应（id 跨供应商全局唯一）。
type TransportModel struct {
	ID                    string           `json:"id"`                              // 模型 id（即发往上游的模型名）
	Label                 string           `json:"label,omitempty"`                 // 显示名，空时前端取 id
	Description           string           `json:"description,omitempty"`           // 一句话描述
	ReasoningLevels       []ReasoningLevel `json:"reasoningLevels,omitempty"`       // 推理等级列表，空数组不展示选择器
	DefaultReasoningLevel string           `json:"defaultReasoningLevel,omitempty"` // 默认推理等级
	ContextWindow         int              `json:"contextWindow,omitempty"`         // 上下文窗口（token）
}

// Transport 前端 createOpenAITransport({ providers }) 所需的元数据。
type Transport struct {
	Providers []TransportProvider `json:"providers"` // 可用供应商（启用且配置完整），空数组引导去设置页
}

// Transport 汇总对话可用元数据：只输出启用且配置完整（base-url + api-key）的供应商，
// 及其下启用中的模型；模型 id 由库层全局唯一索引保证不重复。
func (s *Service) Transport() (Transport, error) {
	providers, err := s.ListProviders()
	if err != nil {
		return Transport{}, err
	}
	var models []db.AIModel
	if err := s.db.Order("provider_id, id").Find(&models).Error; err != nil {
		return Transport{}, fmt.Errorf("查询模型失败: %w", err)
	}
	byProvider := map[uint][]TransportModel{}
	for _, m := range models {
		if !m.Enabled {
			continue
		}
		levels, err := ParseReasoningLevels(m.ReasoningLevels)
		if err != nil {
			return Transport{}, fmt.Errorf("模型 %s %w", m.ModelID, err)
		}
		byProvider[m.ProviderID] = append(byProvider[m.ProviderID], TransportModel{
			ID: m.ModelID, Label: m.Label, Description: m.Description,
			ReasoningLevels: levels, DefaultReasoningLevel: m.DefaultReasoningLevel,
			ContextWindow: m.ContextWindow,
		})
	}
	result := Transport{Providers: []TransportProvider{}}
	for _, p := range providers {
		if !p.Enabled || p.BaseURL == "" || p.APIKey == "" {
			continue
		}
		models := byProvider[p.ID]
		if models == nil {
			models = []TransportModel{}
		}
		result.Providers = append(result.Providers, TransportProvider{
			ID:       fmt.Sprintf("%d", p.ID),
			Label:    p.Name,
			Endpoint: fmt.Sprintf(ChatEndpointFormat, p.ID),
			Mode:     TransportModeChat,
			Models:   models,
		})
	}
	return result, nil
}
