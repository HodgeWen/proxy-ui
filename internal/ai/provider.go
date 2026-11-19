package ai

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"proxy-ui/internal/db"
)

// ReasoningEfforts 允许的推理强度取值（空串表示不指定）。
var ReasoningEfforts = []string{"", "low", "medium", "high"}

// Config 供应商配置；APIKey 只在服务端内部使用，永不回显给前端。
type Config struct {
	BaseURL         string // OpenAI 兼容接口地址（base-url）
	Model           string // 模型名
	APIKey          string // API Key
	ReasoningEffort string // 推理强度：low/medium/high，空为不指定
}

// Ready 供应商是否已配置完整（可发起对话）。
func (c Config) Ready() bool { return c.BaseURL != "" && c.Model != "" && c.APIKey != "" }

// LoadConfig 读取供应商配置；未配置时返回零值。
func (s *Service) LoadConfig() (Config, error) {
	var record db.AIProvider
	err := s.db.First(&record, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("查询 AI 配置失败: %w", err)
	}
	return Config{
		BaseURL:         record.BaseURL,
		Model:           record.Model,
		APIKey:          record.APIKey,
		ReasoningEffort: record.ReasoningEffort,
	}, nil
}

// SaveConfig 保存供应商配置；apiKey 为空时保留已存 Key（避免回显-回传丢 Key）。
func (s *Service) SaveConfig(cfg Config) (Config, error) {
	existing, err := s.LoadConfig()
	if err != nil {
		return Config{}, err
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.Model = strings.TrimSpace(cfg.Model)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.ReasoningEffort = strings.TrimSpace(cfg.ReasoningEffort)
	if cfg.BaseURL == "" {
		return Config{}, fmt.Errorf("接口地址不能为空")
	}
	if !strings.HasPrefix(cfg.BaseURL, "http://") && !strings.HasPrefix(cfg.BaseURL, "https://") {
		return Config{}, fmt.Errorf("接口地址须以 http:// 或 https:// 开头")
	}
	if cfg.Model == "" {
		return Config{}, fmt.Errorf("模型名不能为空")
	}
	if !validReasoningEffort(cfg.ReasoningEffort) {
		return Config{}, fmt.Errorf("推理强度须为 low/medium/high 或留空")
	}
	if cfg.APIKey == "" {
		cfg.APIKey = existing.APIKey
	}
	record := db.AIProvider{
		ID: 1, BaseURL: cfg.BaseURL, APIKey: cfg.APIKey,
		Model: cfg.Model, ReasoningEffort: cfg.ReasoningEffort,
	}
	if err := s.db.Save(&record).Error; err != nil {
		return Config{}, fmt.Errorf("保存 AI 配置失败: %w", err)
	}
	return cfg, nil
}

// validReasoningEffort 校验推理强度取值。
func validReasoningEffort(v string) bool {
	for _, opt := range ReasoningEfforts {
		if v == opt {
			return true
		}
	}
	return false
}
