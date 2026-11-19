// Package ai 提供 AI 助手：OpenAI 兼容供应商配置存取、SSE 流式对话代理与
// 面板工具注册表（工具由 internal/api 注册，执行回落到业务模块）。
package ai

import (
	"net/http"
	"sync"

	"gorm.io/gorm"
)

// Service AI 助手服务：持有供应商配置与面板工具注册表。
type Service struct {
	db     *gorm.DB
	client *http.Client

	mu    sync.RWMutex
	tools map[string]Tool
}

// New 构造 AI 服务；上游请求生命周期由请求 context 控制，客户端不设整体超时。
func New(database *gorm.DB) *Service {
	return &Service{db: database, client: &http.Client{}, tools: map[string]Tool{}}
}
