package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/ai"
	"proxy-ui/internal/db"
)

// ---------- 供应商与模型 ----------

// aiProviderView 供应商视图（api-key 不回显，仅给出是否已配置）。
type aiProviderView struct {
	ID      uint          `json:"id"`      // 主键
	Name    string        `json:"name"`    // 供应商名称
	BaseURL string        `json:"baseUrl"` // OpenAI 兼容接口地址
	Enabled bool          `json:"enabled"` // 是否启用
	Sort    int           `json:"sort"`    // 排序，值小者靠前
	HasKey  bool          `json:"hasKey"`  // 是否已配置 api-key
	Models  []aiModelView `json:"models"`  // 供应商下的模型
}

// aiModelView 模型视图。
type aiModelView struct {
	ID                    uint                `json:"id"`                    // 主键
	ProviderID            uint                `json:"providerId"`            // 所属供应商
	ModelID               string              `json:"modelId"`               // 模型 id（跨供应商全局唯一）
	Label                 string              `json:"label"`                 // 显示名
	Description           string              `json:"description"`           // 一句话描述
	ReasoningLevels       []ai.ReasoningLevel `json:"reasoningLevels"`       // 推理等级列表
	DefaultReasoningLevel string              `json:"defaultReasoningLevel"` // 默认推理等级
	ContextWindow         int                 `json:"contextWindow"`         // 上下文窗口（token）
	Enabled               bool                `json:"enabled"`               // 是否启用
}

// aiModelToView 模型转视图（推理等级 JSON 解析为数组，损坏时置空不中断列表）。
func aiModelToView(m db.AIModel) aiModelView {
	levels, _ := ai.ParseReasoningLevels(m.ReasoningLevels)
	if levels == nil {
		levels = []ai.ReasoningLevel{}
	}
	return aiModelView{
		ID: m.ID, ProviderID: m.ProviderID, ModelID: m.ModelID, Label: m.Label,
		Description: m.Description, ReasoningLevels: levels,
		DefaultReasoningLevel: m.DefaultReasoningLevel,
		ContextWindow:         m.ContextWindow, Enabled: m.Enabled,
	}
}

// aiProviderViews 查询全部供应商并按所属组装模型。
func (a *API) aiProviderViews() ([]aiProviderView, error) {
	providers, err := a.ai.ListProviders()
	if err != nil {
		return nil, err
	}
	var models []db.AIModel
	if err := a.db.Order("provider_id, id").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询模型失败: %w", err)
	}
	byProvider := map[uint][]aiModelView{}
	for _, m := range models {
		byProvider[m.ProviderID] = append(byProvider[m.ProviderID], aiModelToView(m))
	}
	views := make([]aiProviderView, 0, len(providers))
	for _, p := range providers {
		models := byProvider[p.ID]
		if models == nil {
			models = []aiModelView{}
		}
		views = append(views, aiProviderView{
			ID: p.ID, Name: p.Name, BaseURL: p.BaseURL, Enabled: p.Enabled,
			Sort: p.Sort, HasKey: p.APIKey != "", Models: models,
		})
	}
	return views, nil
}

// aiProviderReq 供应商创建/更新请求；apiKey 为空表示保留已存 Key，enabled 缺省 true。
type aiProviderReq struct {
	Name    string `json:"name"`    // 供应商名称
	BaseURL string `json:"baseUrl"` // OpenAI 兼容接口地址
	APIKey  string `json:"apiKey"`  // API Key（可选，空为保留）
	Enabled *bool  `json:"enabled"` // 是否启用，缺省 true
}

// aiProviderInput 请求转服务层入参。
func (r aiProviderReq) input() ai.ProviderInput {
	enabled := r.Enabled == nil || *r.Enabled
	return ai.ProviderInput{Name: r.Name, BaseURL: r.BaseURL, APIKey: r.APIKey, Enabled: enabled}
}

// HandleAIProvidersList GET /api/ai/providers 供应商列表（含各自模型）。
func (a *API) HandleAIProvidersList(c *gin.Context) {
	views, err := a.aiProviderViews()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, views)
}

// HandleAIProviderCreate POST /api/ai/providers 新增供应商。
func (a *API) HandleAIProviderCreate(c *gin.Context) {
	var req aiProviderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	record, err := a.ai.CreateProvider(req.input())
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, aiProviderView{
		ID: record.ID, Name: record.Name, BaseURL: record.BaseURL, Enabled: record.Enabled,
		Sort: record.Sort, HasKey: record.APIKey != "", Models: []aiModelView{},
	})
}

// HandleAIProviderUpdate PUT /api/ai/providers/:id 更新供应商（apiKey 空为保留）。
func (a *API) HandleAIProviderUpdate(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req aiProviderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	record, err := a.ai.UpdateProvider(id, req.input())
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, aiProviderView{
		ID: record.ID, Name: record.Name, BaseURL: record.BaseURL, Enabled: record.Enabled,
		Sort: record.Sort, HasKey: record.APIKey != "", Models: []aiModelView{},
	})
}

// HandleAIProviderDelete DELETE /api/ai/providers/:id 删除供应商及其下全部模型。
func (a *API) HandleAIProviderDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.ai.DeleteProvider(id); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, deletedResp{Deleted: id})
}

// HandleAIProvidersOrder PUT /api/ai/providers/order 按给定 ID 顺序重排供应商。
func (a *API) HandleAIProvidersOrder(c *gin.Context) {
	var req orderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if err := a.ai.OrderProviders(req.IDs); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"ordered": len(req.IDs)})
}

// aiModelReq 模型创建/更新请求；enabled 缺省 true。
type aiModelReq struct {
	ModelID               string              `json:"modelId"`               // 模型 id（跨供应商全局唯一）
	Label                 string              `json:"label"`                 // 显示名
	Description           string              `json:"description"`           // 一句话描述
	ReasoningLevels       []ai.ReasoningLevel `json:"reasoningLevels"`       // 推理等级列表
	DefaultReasoningLevel string              `json:"defaultReasoningLevel"` // 默认推理等级
	ContextWindow         int                 `json:"contextWindow"`         // 上下文窗口（token）
	Enabled               *bool               `json:"enabled"`               // 是否启用，缺省 true
}

// input 请求转服务层入参。
func (r aiModelReq) input() ai.ModelInput {
	enabled := r.Enabled == nil || *r.Enabled
	return ai.ModelInput{
		ModelID: r.ModelID, Label: r.Label, Description: r.Description,
		ReasoningLevels: r.ReasoningLevels, DefaultReasoningLevel: r.DefaultReasoningLevel,
		ContextWindow: r.ContextWindow, Enabled: enabled,
	}
}

// HandleAIModelCreate POST /api/ai/providers/:id/models 手工新增模型。
func (a *API) HandleAIModelCreate(c *gin.Context) {
	providerID, ok := idParam(c)
	if !ok {
		return
	}
	var req aiModelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	record, err := a.ai.CreateModel(providerID, req.input())
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, aiModelToView(record))
}

// aiModelsImportReq 模型批量导入请求（从上游 /models 多选导入）。
type aiModelsImportReq struct {
	ModelIDs []string `json:"modelIds"` // 模型 id 列表
}

// HandleAIModelsImport POST /api/ai/providers/:id/models/import 批量导入模型 id（已占用的跳过）。
func (a *API) HandleAIModelsImport(c *gin.Context) {
	providerID, ok := idParam(c)
	if !ok {
		return
	}
	var req aiModelsImportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	imported, err := a.ai.ImportModels(providerID, req.ModelIDs)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"imported": imported})
}

// fetchModelsTimeout 上游 /models 拉取超时：这是一次性的列表请求（非流式对话），
// 上游挂起时必须可失败返回，否则前端导入弹框会一直停在拉取中。
const fetchModelsTimeout = 15 * time.Second

// HandleAIModelsFetch POST /api/ai/providers/:id/models/fetch 拉取上游 /models 列表。
func (a *API) HandleAIModelsFetch(c *gin.Context) {
	providerID, ok := idParam(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), fetchModelsTimeout)
	defer cancel()
	models, err := a.ai.FetchModels(ctx, providerID)
	if err != nil {
		Fail(c, http.StatusBadGateway, err.Error())
		return
	}
	OK(c, models)
}

// HandleAIModelUpdate PUT /api/ai/models/:id 更新模型。
func (a *API) HandleAIModelUpdate(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req aiModelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	record, err := a.ai.UpdateModel(id, req.input())
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, aiModelToView(record))
}

// HandleAIModelDelete DELETE /api/ai/models/:id 删除模型。
func (a *API) HandleAIModelDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.ai.DeleteModel(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, deletedResp{Deleted: id})
}

// HandleAITransport GET /api/ai/transport 前端 createOpenAITransport 元数据。
func (a *API) HandleAITransport(c *gin.Context) {
	transport, err := a.ai.Transport()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, transport)
}

// ---------- 流式对话（SSE） ----------

// maxAIChatBody 对话代理请求体大小上限。
const maxAIChatBody = 16 << 20

// HandleAIChat POST /api/ai/providers/:id/chat OpenAI 兼容流式对话代理：
// 按路径供应商路由，注入其 api-key、校验模型归属并补默认推理等级后转发上游，SSE 原样回传。
// 响应不经统一 envelope（由前端 @veltra/ai transport 按 OpenAI 协议解析）。
func (a *API) HandleAIChat(c *gin.Context) {
	providerID, ok := idParam(c)
	if !ok {
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxAIChatBody+1))
	if err != nil {
		failAI(c, http.StatusBadRequest, "读取请求失败")
		return
	}
	if len(body) > maxAIChatBody {
		failAI(c, http.StatusRequestEntityTooLarge, "请求体过大")
		return
	}
	resp, err := a.ai.Chat(c.Request.Context(), providerID, body)
	if err != nil {
		switch {
		case errors.Is(err, ai.ErrNotConfigured), errors.Is(err, ai.ErrProviderDisabled):
			failAI(c, http.StatusServiceUnavailable, err.Error())
		case errors.Is(err, ai.ErrInvalidRequest), errors.Is(err, ai.ErrModelNotFound):
			failAI(c, http.StatusBadRequest, err.Error())
		default:
			failAI(c, http.StatusBadGateway, err.Error())
		}
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		text, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		c.Data(resp.StatusCode, "application/json; charset=utf-8", text)
		return
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/event-stream; charset=utf-8"
	}
	h := c.Writer.Header()
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = io.Copy(flushWriter{c.Writer}, resp.Body)
}

// failAI 输出 OpenAI 风格错误 JSON（对话端点不走统一 envelope）。
func failAI(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": gin.H{"message": msg}})
}

// flushWriter 包装 gin 响应流，逐次写出后立即 Flush（SSE 不缓冲）。
type flushWriter struct{ w gin.ResponseWriter }

func (f flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	f.w.Flush()
	return n, err
}
