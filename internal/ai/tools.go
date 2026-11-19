package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// Tool 面板工具：Name/Description/Parameters 以 OpenAI function calling 形态发给
// 模型，Exec 在面板内执行（由 internal/api 注册并回落到业务模块）。
type Tool struct {
	Name         string                                                       // 工具名（唯一）
	Description  string                                                       // 工具描述（发给模型）
	Parameters   map[string]any                                               // 参数 JSON Schema（object）
	NeedsConfirm bool                                                         // 前端执行前需用户确认
	Exec         func(ctx context.Context, args json.RawMessage) (any, error) // 执行实现
}

// ToolInfo 工具对外描述（不含执行实现），供前端注册为对话工具。
type ToolInfo struct {
	Name         string         `json:"name"`         // 工具名
	Description  string         `json:"description"`  // 工具描述
	Parameters   map[string]any `json:"parameters"`   // 参数 JSON Schema
	NeedsConfirm bool           `json:"needsConfirm"` // 执行前是否需用户确认
}

// Register 注册工具；重名覆盖。
func (s *Service) Register(tool Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = tool
}

// ToolInfos 返回全部工具描述（按名排序，结果稳定）。
func (s *Service) ToolInfos() []ToolInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	infos := make([]ToolInfo, 0, len(s.tools))
	for _, t := range s.tools {
		infos = append(infos, ToolInfo{
			Name: t.Name, Description: t.Description, Parameters: t.Parameters, NeedsConfirm: t.NeedsConfirm,
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos
}

// Call 按名执行工具；args 为模型给出的参数 JSON（可为空）。
func (s *Service) Call(ctx context.Context, name string, args json.RawMessage) (any, error) {
	s.mu.RLock()
	tool, ok := s.tools[name]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("工具不存在: %s", name)
	}
	return tool.Exec(ctx, args)
}
