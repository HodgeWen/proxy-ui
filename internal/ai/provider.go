package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"gorm.io/gorm"

	"proxy-ui/internal/db"
)

// ReasoningLevel 推理等级：Value 为写入请求 reasoning_effort 的值，Label 为 UI 文案。
type ReasoningLevel struct {
	Value string `json:"value"` // 写入请求的值（如 low / medium / high）
	Label string `json:"label"` // UI 文案（如「低」）
}

// ProviderInput 供应商创建/更新入参；更新时 APIKey 为空表示保留已存 Key（避免回显-回传丢 Key）。
type ProviderInput struct {
	Name    string // 供应商名称
	BaseURL string // OpenAI 兼容接口地址（base-url）
	APIKey  string // API Key
	Enabled bool   // 是否启用
}

// ModelInput 模型创建/更新入参。
type ModelInput struct {
	ModelID               string           // 模型 id（即发往上游的模型名，跨供应商全局唯一）
	Label                 string           // 显示名
	Description           string           // 一句话描述
	ReasoningLevels       []ReasoningLevel // 推理等级列表
	DefaultReasoningLevel string           // 默认推理等级（须落在列表内）
	ContextWindow         int              // 上下文窗口（token），0 为未知
	Enabled               bool             // 是否启用
}

// ---------- 供应商 ----------

// ListProviders 全部供应商（按 sort、id 升序）。
func (s *Service) ListProviders() ([]db.AIProvider, error) {
	var providers []db.AIProvider
	if err := s.db.Order("`sort`, id").Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("查询 AI 供应商失败: %w", err)
	}
	return providers, nil
}

// CreateProvider 新增供应商，排序值追加到末尾。
func (s *Service) CreateProvider(in ProviderInput) (db.AIProvider, error) {
	normalizeProvider(&in)
	if err := validateProvider(in); err != nil {
		return db.AIProvider{}, err
	}
	var maxSort int
	s.db.Model(&db.AIProvider{}).Select("COALESCE(MAX(`sort`), 0)").Scan(&maxSort)
	record := db.AIProvider{
		Name: in.Name, BaseURL: in.BaseURL, APIKey: in.APIKey,
		Enabled: in.Enabled, Sort: maxSort + 1,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return db.AIProvider{}, fmt.Errorf("创建 AI 供应商失败: %w", err)
	}
	return record, nil
}

// UpdateProvider 更新供应商；apiKey 为空时保留已存 Key。
func (s *Service) UpdateProvider(id uint, in ProviderInput) (db.AIProvider, error) {
	var record db.AIProvider
	if err := s.db.First(&record, id).Error; err != nil {
		return db.AIProvider{}, fmt.Errorf("AI 供应商不存在: %d", id)
	}
	normalizeProvider(&in)
	if err := validateProvider(in); err != nil {
		return db.AIProvider{}, err
	}
	if in.APIKey == "" {
		in.APIKey = record.APIKey
	}
	record.Name, record.BaseURL, record.APIKey, record.Enabled = in.Name, in.BaseURL, in.APIKey, in.Enabled
	if err := s.db.Save(&record).Error; err != nil {
		return db.AIProvider{}, fmt.Errorf("保存 AI 供应商失败: %w", err)
	}
	return record, nil
}

// OrderProviders 按给定 ID 顺序重排供应商（缺失的 ID 保持原相对顺序排在末尾）。
func (s *Service) OrderProviders(ids []uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var existing []db.AIProvider
		if err := tx.Select("id").Order("`sort`, id").Find(&existing).Error; err != nil {
			return fmt.Errorf("查询 AI 供应商失败: %w", err)
		}
		exists := make(map[uint]bool, len(existing))
		for _, p := range existing {
			exists[p.ID] = true
		}
		ordered := make([]uint, 0, len(existing))
		seen := make(map[uint]bool, len(existing))
		for _, id := range ids {
			if !exists[id] || seen[id] {
				continue
			}
			seen[id] = true
			ordered = append(ordered, id)
		}
		for _, p := range existing {
			if !seen[p.ID] {
				ordered = append(ordered, p.ID)
			}
		}
		for i, id := range ordered {
			if err := tx.Model(&db.AIProvider{}).Where("id = ?", id).Update("`sort`", i+1).Error; err != nil {
				return fmt.Errorf("调整 AI 供应商排序失败: %w", err)
			}
		}
		return nil
	})
}

// DeleteProvider 删除供应商及其下全部模型。
func (s *Service) DeleteProvider(id uint) error {
	var record db.AIProvider
	if err := s.db.First(&record, id).Error; err != nil {
		return fmt.Errorf("AI 供应商不存在: %d", id)
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("provider_id = ?", id).Delete(&db.AIModel{}).Error; err != nil {
			return fmt.Errorf("删除供应商模型失败: %w", err)
		}
		if err := tx.Delete(&record).Error; err != nil {
			return fmt.Errorf("删除 AI 供应商失败: %w", err)
		}
		return nil
	})
}

// normalizeProvider 归一化供应商入参（去空白、去尾部斜杠）。
func normalizeProvider(in *ProviderInput) {
	in.Name = strings.TrimSpace(in.Name)
	in.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	in.APIKey = strings.TrimSpace(in.APIKey)
}

// validateProvider 校验供应商入参。
func validateProvider(in ProviderInput) error {
	if in.Name == "" {
		return fmt.Errorf("供应商名称不能为空")
	}
	if in.BaseURL == "" {
		return fmt.Errorf("接口地址不能为空")
	}
	if !strings.HasPrefix(in.BaseURL, "http://") && !strings.HasPrefix(in.BaseURL, "https://") {
		return fmt.Errorf("接口地址须以 http:// 或 https:// 开头")
	}
	return nil
}

// ---------- 模型 ----------

// CreateModel 在供应商下新增模型；模型 id 跨供应商全局唯一。
func (s *Service) CreateModel(providerID uint, in ModelInput) (db.AIModel, error) {
	var provider db.AIProvider
	if err := s.db.First(&provider, providerID).Error; err != nil {
		return db.AIModel{}, fmt.Errorf("AI 供应商不存在: %d", providerID)
	}
	levels, err := normalizeModel(&in)
	if err != nil {
		return db.AIModel{}, err
	}
	if err := s.ensureModelIDUnique(in.ModelID, 0); err != nil {
		return db.AIModel{}, err
	}
	record := db.AIModel{
		ProviderID: providerID, ModelID: in.ModelID, Label: in.Label, Description: in.Description,
		ReasoningLevels: levels, DefaultReasoningLevel: in.DefaultReasoningLevel,
		ContextWindow: in.ContextWindow, Enabled: in.Enabled,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return db.AIModel{}, fmt.Errorf("创建模型失败: %w", err)
	}
	return record, nil
}

// UpdateModel 更新模型；模型 id 全局唯一校验同样适用。
func (s *Service) UpdateModel(id uint, in ModelInput) (db.AIModel, error) {
	var record db.AIModel
	if err := s.db.First(&record, id).Error; err != nil {
		return db.AIModel{}, fmt.Errorf("模型不存在: %d", id)
	}
	levels, err := normalizeModel(&in)
	if err != nil {
		return db.AIModel{}, err
	}
	if err := s.ensureModelIDUnique(in.ModelID, id); err != nil {
		return db.AIModel{}, err
	}
	record.ModelID, record.Label, record.Description = in.ModelID, in.Label, in.Description
	record.ReasoningLevels, record.DefaultReasoningLevel = levels, in.DefaultReasoningLevel
	record.ContextWindow, record.Enabled = in.ContextWindow, in.Enabled
	if err := s.db.Save(&record).Error; err != nil {
		return db.AIModel{}, fmt.Errorf("保存模型失败: %w", err)
	}
	return record, nil
}

// ImportModels 批量导入模型 id（从上游 /models 多选导入）；已被任意供应商占用的 id 跳过。
// 返回实际导入的条数。
func (s *Service) ImportModels(providerID uint, modelIDs []string) (int, error) {
	var provider db.AIProvider
	if err := s.db.First(&provider, providerID).Error; err != nil {
		return 0, fmt.Errorf("AI 供应商不存在: %d", providerID)
	}
	imported := 0
	for _, id := range modelIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		taken, err := s.modelIDTaken(id, 0)
		if err != nil {
			return imported, err
		}
		if taken {
			continue
		}
		record := db.AIModel{ProviderID: providerID, ModelID: id, Enabled: true}
		if err := s.db.Create(&record).Error; err != nil {
			return imported, fmt.Errorf("导入模型 %s 失败: %w", id, err)
		}
		imported++
	}
	return imported, nil
}

// DeleteModel 删除模型。
func (s *Service) DeleteModel(id uint) error {
	if err := s.db.Delete(&db.AIModel{}, id).Error; err != nil {
		return fmt.Errorf("删除模型失败: %w", err)
	}
	return nil
}

// ensureModelIDUnique 校验模型 id 未被其它模型（任意供应商）占用。
func (s *Service) ensureModelIDUnique(modelID string, excludeID uint) error {
	taken, err := s.modelIDTaken(modelID, excludeID)
	if err != nil {
		return err
	}
	if taken {
		return fmt.Errorf("模型 id「%s」已被使用（模型 id 须跨供应商全局唯一）", modelID)
	}
	return nil
}

// modelIDTaken 查询模型 id 是否已被占用；excludeID 排除自身（更新场景）。
func (s *Service) modelIDTaken(modelID string, excludeID uint) (bool, error) {
	var count int64
	q := s.db.Model(&db.AIModel{}).Where("model_id = ?", modelID)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Count(&count).Error; err != nil {
		return false, fmt.Errorf("查询模型失败: %w", err)
	}
	return count > 0, nil
}

// normalizeModel 归一化模型入参并校验推理等级，返回推理等级 JSON。
func normalizeModel(in *ModelInput) (string, error) {
	in.ModelID = strings.TrimSpace(in.ModelID)
	in.Label = strings.TrimSpace(in.Label)
	in.Description = strings.TrimSpace(in.Description)
	in.DefaultReasoningLevel = strings.TrimSpace(in.DefaultReasoningLevel)
	if in.ModelID == "" {
		return "", fmt.Errorf("模型 id 不能为空")
	}
	if in.ContextWindow < 0 {
		return "", fmt.Errorf("上下文窗口不能为负数")
	}
	levels := make([]ReasoningLevel, 0, len(in.ReasoningLevels))
	seen := map[string]bool{}
	for _, level := range in.ReasoningLevels {
		level.Value = strings.TrimSpace(level.Value)
		level.Label = strings.TrimSpace(level.Label)
		if level.Value == "" {
			return "", fmt.Errorf("推理等级的值不能为空")
		}
		if seen[level.Value] {
			return "", fmt.Errorf("推理等级值重复: %s", level.Value)
		}
		if level.Label == "" {
			level.Label = level.Value
		}
		seen[level.Value] = true
		levels = append(levels, level)
	}
	if len(levels) == 0 && in.DefaultReasoningLevel != "" {
		return "", fmt.Errorf("未配置推理等级时不能指定默认等级")
	}
	if in.DefaultReasoningLevel != "" && !seen[in.DefaultReasoningLevel] {
		return "", fmt.Errorf("默认推理等级「%s」不在等级列表内", in.DefaultReasoningLevel)
	}
	if len(levels) == 0 {
		return "", nil
	}
	raw, err := json.Marshal(levels)
	if err != nil {
		return "", fmt.Errorf("序列化推理等级失败: %w", err)
	}
	return string(raw), nil
}

// ParseReasoningLevels 解析模型推理等级 JSON；空串返回 nil。
func ParseReasoningLevels(raw string) ([]ReasoningLevel, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var levels []ReasoningLevel
	if err := json.Unmarshal([]byte(raw), &levels); err != nil {
		return nil, fmt.Errorf("推理等级数据损坏: %w", err)
	}
	return levels, nil
}

// ---------- 上游 /models 拉取 ----------

// UpstreamModel 上游 /models 归一化后的模型条目。
type UpstreamModel struct {
	ID      string `json:"id"`      // 模型 id
	OwnedBy string `json:"ownedBy"` // 归属方（owned_by）
}

// maxFetchModelsBody 上游 /models 响应体大小上限。
const maxFetchModelsBody = 8 << 20

// FetchModels 请求供应商 {base-url}/models 并归一化返回 [{id, ownedBy}]（按 id 排序）。
// 失败时错误信息包含上游状态码与响应摘要，便于在设置页直接定位问题。
func (s *Service) FetchModels(ctx context.Context, providerID uint) ([]UpstreamModel, error) {
	var provider db.AIProvider
	if err := s.db.First(&provider, providerID).Error; err != nil {
		return nil, fmt.Errorf("AI 供应商不存在: %d", providerID)
	}
	if provider.BaseURL == "" {
		return nil, fmt.Errorf("供应商未配置接口地址")
	}
	url := provider.BaseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("构造 /models 请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if provider.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求上游 /models 失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxFetchModelsBody))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("上游 /models 返回状态码 %d: %s", resp.StatusCode, summarizeBody(body))
	}
	models, err := parseUpstreamModels(body)
	if err != nil {
		return nil, fmt.Errorf("上游 /models 响应解析失败（状态码 %d）: %w", resp.StatusCode, err)
	}
	return models, nil
}

// upstreamItem 上游 /models 条目的原始形态。
type upstreamItem struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by"`
}

// parseUpstreamModels 解析 OpenAI 兼容 /models 响应：{"data":[{id,owned_by}]} 或裸数组。
func parseUpstreamModels(body []byte) ([]UpstreamModel, error) {
	var wrapped struct {
		Data []upstreamItem `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.Data != nil {
		return normalizeUpstream(wrapped.Data), nil
	}
	var bare []upstreamItem
	if err := json.Unmarshal(body, &bare); err == nil && bare != nil {
		return normalizeUpstream(bare), nil
	}
	return nil, fmt.Errorf("不是合法 JSON 或缺少 data 数组: %s", summarizeBody(body))
}

// normalizeUpstream 过滤空 id、按 id 去重并排序，保证结果稳定。
func normalizeUpstream(items []upstreamItem) []UpstreamModel {
	seen := map[string]bool{}
	models := make([]UpstreamModel, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		models = append(models, UpstreamModel{ID: id, OwnedBy: strings.TrimSpace(item.OwnedBy)})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models
}

// summarizeBody 响应摘要：单行化并按 rune 截断到 200 个字符。
func summarizeBody(body []byte) string {
	text := strings.Join(strings.Fields(string(body)), " ")
	const maxRunes = 200
	if runes := []rune(text); len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return text
}
