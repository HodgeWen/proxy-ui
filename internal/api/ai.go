package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/ai"
	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
	"proxy-ui/internal/subscription"
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
	OK(c, gin.H{"deleted": id})
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
	OK(c, gin.H{"deleted": id})
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

// ---------- 工具查询与执行 ----------

// toolCallReq 工具执行请求。
type toolCallReq struct {
	Name      string          `json:"name"`      // 工具名
	Arguments json.RawMessage `json:"arguments"` // 参数（模型给出的 JSON 对象）
}

// requestHostKey 工具执行上下文里携带请求 Host 的键（预览类工具渲染节点服务器地址用）。
type requestHostKey struct{}

// requestHostFrom 取工具调用请求的 Host；取不到返回空串（渲染仍可用，仅服务器地址为空）。
func requestHostFrom(ctx context.Context) string {
	host, _ := ctx.Value(requestHostKey{}).(string)
	return host
}

// HandleAIToolsGet GET /api/ai/tools 面板工具描述列表（供前端注册为对话工具）。
func (a *API) HandleAIToolsGet(c *gin.Context) {
	OK(c, a.ai.ToolInfos())
}

// HandleAIToolCall POST /api/ai/tools/call 执行面板工具（回落到对应业务模块）。
func (a *API) HandleAIToolCall(c *gin.Context) {
	var req toolCallReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if req.Name == "" {
		Fail(c, http.StatusBadRequest, "工具名不能为空")
		return
	}
	ctx := context.WithValue(c.Request.Context(), requestHostKey{}, c.Request.Host)
	result, err := a.ai.Call(ctx, req.Name, req.Arguments)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"result": result})
}

// ---------- 面板工具注册 ----------

// decodeToolArgs 解码工具参数到目标结构；空参数保持目标零值。
func decodeToolArgs(args json.RawMessage, dst any) error {
	if len(args) == 0 {
		return nil
	}
	return json.Unmarshal(args, dst)
}

// dropEmptyConfig 把空对象/null 配置归一为未提供（模型常以 {} 表示「用默认值」）。
func dropEmptyConfig(req *inboundReq) {
	trimmed := strings.TrimSpace(string(req.Config))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		req.Config = nil
	}
}

// idArg 工具 ID 参数。
type idArg struct {
	ID uint `json:"id"` // 资源 ID
}

// registerAITools 把面板主要接口注册为 AI 工具（查询与修改回落到业务模块）。
func (a *API) registerAITools() {
	a.registerAIQueryTools()
	a.registerAIModifyTools()
}

func (a *API) registerAIQueryTools() {
	a.ai.Register(ai.Tool{
		Name:        "list_inbounds",
		Description: "列出面板全部入站（含协议、监听地址与端口、启用状态、连接层配置、其下用户与弱配置警示）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.inboundViews()
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "list_inbound_users",
		Description: "列出全部入站用户（入站内的认证身份，含所属入站、用户名、启用/暂停状态）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			var users []db.InboundUser
			if err := a.db.Order("inbound_id, id").Find(&users).Error; err != nil {
				return nil, err
			}
			return users, nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "list_outbounds",
		Description: "列出面板全部出站（direct 与自定义出站，含类型与配置）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.outboundViews()
		},
	})
	a.ai.Register(ai.Tool{
		Name: "list_certs",
		Description: "列出面板全部 TLS 证书（含引用方式 managed/path、域名、到期时间与剩余天数、" +
			"解析失败原因，以及各证书被哪些入站引用）",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			views, err := a.certViews()
			if err != nil {
				return nil, err
			}
			for i := range views {
				if refs := a.certReferencedBy(views[i].Certificate); len(refs) > 0 {
					views[i].ReferencedBy = refs
				}
			}
			return views, nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "list_rules",
		Description: "列出全部全局路由规则（按生效顺序，含匹配对象 matchType、匹配方式 matchMethod、匹配值、引用的规则集与出站动作）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.ruleViews()
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "list_rule_sets",
		Description: "列出全部规则集（含 sing-box 与 mihomo 源地址内容项、启用项与是否被规则引用）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.ruleSetViews()
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "list_policy_groups",
		Description: "列出全部策略组（含模式 failover/fixed/auto 与成员出站列表）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.groupViews()
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "list_subscription_tokens",
		Description: "列出全部订阅用户（含匿名 token、名称、配额、已用流量、到期时间、停用状态与各自节点）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.tokenViews()
		},
	})
	a.ai.Register(ai.Tool{
		Name: "preview_subscription",
		Description: "按订阅用户 ID 预览其订阅配置的真实渲染内容（与页面预览一致）；" +
			"忽略停用、配额与到期状态，只要有启用中的节点就返回完整配置，无节点时报明确原因",
		Parameters: toolParams(map[string]any{
			"id":     toolProp("integer", "订阅用户 ID（可用 list_subscription_tokens 查询）"),
			"format": toolProp("string", "输出格式，缺省 sing-box", "sing-box", "mihomo"),
		}, "id"),
		Exec: func(ctx context.Context, args json.RawMessage) (any, error) {
			var arg struct {
				ID     uint   `json:"id"`
				Format string `json:"format"`
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if arg.ID == 0 {
				return nil, errors.New("缺少订阅用户 ID")
			}
			format := subscription.FormatSingBox
			if strings.TrimSpace(arg.Format) != "" {
				parsed, valid := subscription.ParseFormat(arg.Format)
				if !valid {
					return nil, errors.New("不支持的格式：仅支持 sing-box 或 mihomo")
				}
				format = parsed
			}
			content, err := a.subs.Preview(arg.ID, requestHostFrom(ctx), format)
			if err != nil {
				return nil, err
			}
			return gin.H{"format": previewFormatLabel(format), "content": string(content)}, nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_kernel_status",
		Description: "查询 sing-box 内核运行状态（内置内核、是否运行、版本、运行时长）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(ctx context.Context, _ json.RawMessage) (any, error) {
			return a.kernelStatusOf(ctx), nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_kernel_logs",
		Description: "查询内核最近日志",
		Parameters: toolParams(map[string]any{
			"limit": map[string]any{"type": "integer", "description": "返回的最近行数，缺省 100"},
		}),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg struct {
				Limit int `json:"limit"`
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if arg.Limit <= 0 {
				arg.Limit = 100
			}
			return a.core.Logs(arg.Limit), nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_realtime_stats",
		Description: "查询实时统计：当前上传/下载速率、内存占用、连接总数与累计流量",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			rates, at := a.stats.Rates()
			return gin.H{"up": rates.Up, "down": rates.Down, "at": at}, nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_user_traffic",
		Description: "查询按用户名/入站持久化累计的上下行流量（面板重启不丢失）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.trafficByTag()
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_dns_config",
		Description: "查询面板 DNS 配置（服务器列表、分流规则、默认域名策略与是否启用）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.buildDNS()
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_system_status",
		Description: "查询系统优化状态：当前 TCP 拥塞控制算法、是否启用 BBR 与可用的拥塞算法列表",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return readSystemStatus(), nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_settings",
		Description: "查询面板全局设置：未命中规则的兜底出站",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return gin.H{"routeFinal": a.getSetting(db.SettingRouteFinal)}, nil
		},
	})
}

// registerModify 注册写操作工具：执行成功后立即热应用内核（保存即生效）。
// 数据库改动已在业务模块完成，热应用失败时把原因回给模型与用户，便于修正后重试。
func (a *API) registerModify(tool ai.Tool) {
	exec := tool.Exec
	tool.Exec = func(ctx context.Context, args json.RawMessage) (any, error) {
		result, err := exec(ctx, args)
		if err != nil {
			return nil, err
		}
		if err := a.applyKernelConfig(ctx); err != nil {
			return nil, fmt.Errorf("已保存，但内核按新配置重建失败: %w", err)
		}
		return result, nil
	}
	a.ai.Register(tool)
}

func (a *API) registerAIModifyTools() {
	a.registerModify(ai.Tool{
		Name: "create_inbound",
		Description: "创建入站（真实监听）。入站只配连接层参数（TLS/证书/Reality/ECH/混淆/masquerade/带宽/拥塞算法），" +
			"用户凭据在入站内单独创建；listenPort 省略时自动分配随机空闲端口；保存后内核立即按新配置重建",
		Parameters: toolParams(map[string]any{
			"protocol": toolProp("string", "入站协议", singbox.SupportedProtocols...),
			"name":     toolProp("string", "入站名称，缺省按 协议-序号 生成"),
			"listen":   toolProp("string", "监听地址：:: 双栈 / 0.0.0.0 仅 IPv4 / 127.0.0.1 走反代，缺省 ::"),
			"config": map[string]any{"type": "object", "description": "协议连接层配置对象，缺省按协议填默认值。" +
				"通用：tls{enabled,serverName,alpn,minVersion,maxVersion,ech{enabled,key,keyPath},certMode,certId,certPath,keyPath}、" +
				"tcpFastOpen、multiplex{enabled,padding}（VLESS/Trojan/VMess/Shadowsocks）；" +
				"Hysteria2：obfsEnabled/obfsPassword、upMbps/downMbps、bbrProfile、masquerade{type,directory,proxyUrl,statusCode,content}；" +
				"TUIC：congestionControl（默认 bbr）、zeroRttHandshake、heartbeat；Shadowsocks：network（空为 TCP+UDP）"},
			"enabled": toolProp("boolean", "是否启用，缺省 true"),
		}, "protocol"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req inboundReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			dropEmptyConfig(&req)
			return a.createInbound(req)
		},
	})
	a.registerModify(ai.Tool{
		Name:        "update_inbound",
		Description: "按 ID 更新入站；仅更新给出的字段，config 为完整协议配置对象（变更协议时必须提供）",
		Parameters: toolParams(map[string]any{
			"id":      toolProp("integer", "入站 ID"),
			"name":    toolProp("string", "名称"),
			"listen":  toolProp("string", "监听地址"),
			"config":  map[string]any{"type": "object", "description": "完整协议配置对象"},
			"enabled": toolProp("boolean", "是否启用"),
		}, "id"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req inboundReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			dropEmptyConfig(&req)
			return a.updateInbound(arg.ID, req)
		},
	})
	a.registerModify(ai.Tool{
		Name:         "delete_inbound",
		Description:  "按 ID 删除入站（连同其下全部用户与累计流量）",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "入站 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if err := a.deleteInbound(arg.ID); err != nil {
				return nil, err
			}
			return gin.H{"deleted": arg.ID}, nil
		},
	})
	a.registerModify(ai.Tool{
		Name: "create_outbound",
		Description: "创建出站（type 为 sing-box 出站类型，如 direct、socks、http、shadowsocks、vmess、trojan、vless、hysteria2、wireguard 等）；" +
			"config 为该类型出站的字段对象（如 server、server_port、密码等）",
		Parameters: toolParams(map[string]any{
			"type": toolProp("string", "出站类型"),
			"name": toolProp("string", "出站 tag，direct 类型缺省 direct"),
			"config": map[string]any{
				"type": "object", "description": "出站字段对象（server/server_port/密码等），direct 无需提供",
			},
		}, "type"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req outboundReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			ob := db.Outbound{Enabled: true}
			if err := a.applyOutboundReq(&ob, req, 0); err != nil {
				return nil, err
			}
			if err := a.db.Create(&ob).Error; err != nil {
				return nil, fmt.Errorf("创建出站失败: %w", err)
			}
			return outboundToView(ob), nil
		},
	})
	a.registerModify(ai.Tool{
		Name:        "update_outbound",
		Description: "按 ID 更新出站；仅更新给出的字段",
		Parameters: toolParams(map[string]any{
			"id":      toolProp("integer", "出站 ID"),
			"name":    toolProp("string", "出站 tag"),
			"type":    toolProp("string", "出站类型"),
			"config":  map[string]any{"type": "object", "description": "出站字段对象"},
			"enabled": toolProp("boolean", "是否启用"),
		}, "id"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req outboundReq
			var arg idArg
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			var ob db.Outbound
			if err := a.db.First(&ob, arg.ID).Error; err != nil {
				return nil, fmt.Errorf("出站不存在: %d", arg.ID)
			}
			if err := a.applyOutboundReq(&ob, req, arg.ID); err != nil {
				return nil, err
			}
			if err := a.db.Save(&ob).Error; err != nil {
				return nil, fmt.Errorf("保存出站失败: %w", err)
			}
			return outboundToView(ob), nil
		},
	})
	a.registerModify(ai.Tool{
		Name:         "delete_outbound",
		Description:  "按 ID 删除出站",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "出站 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if err := a.db.Delete(&db.Outbound{}, arg.ID).Error; err != nil {
				return nil, fmt.Errorf("删除出站失败: %w", err)
			}
			return gin.H{"deleted": arg.ID}, nil
		},
	})
	a.registerModify(ai.Tool{
		Name: "create_rule",
		Description: "创建全局路由规则（追加到规则末尾）。matchType 为匹配对象：domain 域名 / ip IP / rule_set 规则集；" +
			"域名需给出 matchMethod（exact/suffix/keyword/regex）与 value；IP 固定 cidr + value；规则集只给 ruleSetId",
		Parameters: toolParams(map[string]any{
			"matchType":   toolProp("string", "匹配对象", db.RuleMatchDomain, db.RuleMatchIP, db.RuleMatchRuleSet),
			"matchMethod": toolProp("string", "匹配方式（域名规则必填，IP 固定 cidr）", db.RuleMethodExact, db.RuleMethodSuffix, db.RuleMethodKeyword, db.RuleMethodRegex, db.RuleMethodCIDR),
			"value":       toolProp("string", "匹配值（多值逗号分隔；规则集留空）"),
			"ruleSetId":   toolProp("integer", "规则集 ID（matchType=rule_set 时必填，可先用 list_rule_sets 查询）"),
			"outbound":    toolProp("string", "动作出站 tag：direct、block、策略组或自定义出站"),
		}, "matchType", "outbound"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req ruleReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			rule := db.Rule{Enabled: true}
			if err := a.applyRuleReq(&rule, req); err != nil {
				return nil, err
			}
			var maxOrder int
			a.db.Model(&db.Rule{}).Select("COALESCE(MAX(`order`), 0)").Scan(&maxOrder)
			rule.Order = maxOrder + 1
			if err := a.db.Create(&rule).Error; err != nil {
				return nil, fmt.Errorf("创建规则失败: %w", err)
			}
			names, _ := a.ruleSetNames()
			return ruleToView(rule, names), nil
		},
	})
	a.registerModify(ai.Tool{
		Name:        "update_rule",
		Description: "按 ID 更新规则；仅更新给出的字段",
		Parameters: toolParams(map[string]any{
			"id":          toolProp("integer", "规则 ID"),
			"matchType":   toolProp("string", "匹配对象", db.RuleMatchDomain, db.RuleMatchIP, db.RuleMatchRuleSet),
			"matchMethod": toolProp("string", "匹配方式", db.RuleMethodExact, db.RuleMethodSuffix, db.RuleMethodKeyword, db.RuleMethodRegex, db.RuleMethodCIDR),
			"value":       toolProp("string", "匹配值"),
			"ruleSetId":   toolProp("integer", "规则集 ID"),
			"outbound":    toolProp("string", "动作出站 tag"),
			"enabled":     toolProp("boolean", "是否启用"),
		}, "id"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req ruleReq
			var arg idArg
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			var rule db.Rule
			if err := a.db.First(&rule, arg.ID).Error; err != nil {
				return nil, fmt.Errorf("规则不存在: %d", arg.ID)
			}
			if err := a.applyRuleReq(&rule, req); err != nil {
				return nil, err
			}
			if err := a.db.Save(&rule).Error; err != nil {
				return nil, fmt.Errorf("保存规则失败: %w", err)
			}
			names, _ := a.ruleSetNames()
			return ruleToView(rule, names), nil
		},
	})
	a.registerModify(ai.Tool{
		Name:         "delete_rule",
		Description:  "按 ID 删除规则",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "规则 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if err := a.db.Delete(&db.Rule{}, arg.ID).Error; err != nil {
				return nil, fmt.Errorf("删除规则失败: %w", err)
			}
			return gin.H{"deleted": arg.ID}, nil
		},
	})
	a.registerModify(ai.Tool{
		Name:        "create_policy_group",
		Description: "创建策略组（生成 sing-box selector/urltest 出站）；members 为成员出站 tag 按顺序生效",
		Parameters: toolParams(map[string]any{
			"name":    toolProp("string", "策略组名称（供规则与兜底出站引用）"),
			"mode":    toolProp("string", "模式：failover 故障转移 / fixed 固定代理 / auto 自动选择", singbox.SupportedGroupModes...),
			"members": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "成员出站 tag 列表"},
		}, "name", "mode"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req groupReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			group := db.PolicyGroup{Enabled: true}
			if err := a.applyGroupReq(&group, req, 0); err != nil {
				return nil, err
			}
			if err := a.db.Create(&group).Error; err != nil {
				return nil, fmt.Errorf("创建策略组失败: %w", err)
			}
			return groupToView(group), nil
		},
	})
	a.registerModify(ai.Tool{
		Name:        "update_policy_group",
		Description: "按 ID 更新策略组；仅更新给出的字段",
		Parameters: toolParams(map[string]any{
			"id":      toolProp("integer", "策略组 ID"),
			"name":    toolProp("string", "策略组名称"),
			"mode":    toolProp("string", "模式：failover/fixed/auto", singbox.SupportedGroupModes...),
			"members": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "成员出站 tag 列表"},
			"enabled": toolProp("boolean", "是否启用"),
		}, "id"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req groupReq
			var arg idArg
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			var group db.PolicyGroup
			if err := a.db.First(&group, arg.ID).Error; err != nil {
				return nil, fmt.Errorf("策略组不存在: %d", arg.ID)
			}
			if err := a.applyGroupReq(&group, req, arg.ID); err != nil {
				return nil, err
			}
			if err := a.db.Save(&group).Error; err != nil {
				return nil, fmt.Errorf("保存策略组失败: %w", err)
			}
			return groupToView(group), nil
		},
	})
	a.registerModify(ai.Tool{
		Name:         "delete_policy_group",
		Description:  "按 ID 删除策略组",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "策略组 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if err := a.db.Delete(&db.PolicyGroup{}, arg.ID).Error; err != nil {
				return nil, fmt.Errorf("删除策略组失败: %w", err)
			}
			return gin.H{"deleted": arg.ID}, nil
		},
	})
	a.registerModify(ai.Tool{
		Name: "create_subscription_token",
		Description: "创建订阅用户（客户端免登录拉取订阅链接）。nodeInbounds 给出要勾选的入站 ID，" +
			"每个入站内会为该用户创建一个独立凭据的用户身份（共享入站端口），即该用户的一个节点",
		Parameters: toolParams(map[string]any{
			"name":              toolProp("string", "备注名"),
			"trafficLimitBytes": toolProp("integer", "流量配额字节（上+下行），0 或不填为不限"),
			"expiresAt":         toolProp("string", "到期时间（RFC3339），不填为永久"),
			"nodeInbounds":      map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "要勾选的入站 ID 列表"},
		}),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req tokenReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			record := db.SubscriptionToken{Token: randToken()}
			if req.Name != nil {
				record.Name = strings.TrimSpace(*req.Name)
			}
			if req.TrafficLimitBytes != nil && *req.TrafficLimitBytes > 0 {
				record.TrafficLimitBytes = *req.TrafficLimitBytes
			}
			if req.ExpiresAt != nil {
				record.ExpiresAt = req.ExpiresAt
			}
			if err := a.db.Create(&record).Error; err != nil {
				return nil, fmt.Errorf("创建订阅用户失败: %w", err)
			}
			if req.NodeInbounds != nil {
				if err := a.syncTokenNodes(&record, dedupeIDs(*req.NodeInbounds)); err != nil {
					_ = a.deleteToken(record.ID)
					return nil, err
				}
			}
			_, _ = a.ReconcileQuota()
			return a.tokenView(record.ID)
		},
	})
	a.registerModify(ai.Tool{
		Name:        "update_subscription_token",
		Description: "更新订阅用户：名称、流量配额、到期时间、勾选的入站（会在入站内创建/移除该用户的身份）。保存后内核立即按新配置重建",
		Parameters: toolParams(map[string]any{
			"id":                toolProp("integer", "订阅用户 ID"),
			"name":              toolProp("string", "备注名"),
			"trafficLimitBytes": toolProp("integer", "流量配额字节，0 不限"),
			"expiresAt":         toolProp("string", "到期时间（RFC3339）"),
			"clearExpiry":       toolProp("boolean", "true 表示清除到期时间（永久）"),
			"nodeInbounds":      map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "勾选的入站 ID 列表（整体替换）"},
		}, "id"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req tokenReq
			var arg idArg
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			var record db.SubscriptionToken
			if err := a.db.First(&record, arg.ID).Error; err != nil {
				return nil, fmt.Errorf("订阅用户不存在: %d", arg.ID)
			}
			updates := map[string]any{}
			if req.Name != nil {
				updates["name"] = strings.TrimSpace(*req.Name)
			}
			if req.TrafficLimitBytes != nil {
				limit := *req.TrafficLimitBytes
				if limit < 0 {
					limit = 0
				}
				updates["traffic_limit_bytes"] = limit
			}
			switch {
			case req.ClearExpiry:
				updates["expires_at"] = nil
			case req.ExpiresAt != nil:
				updates["expires_at"] = *req.ExpiresAt
			}
			if len(updates) > 0 {
				if err := a.db.Model(&record).Updates(updates).Error; err != nil {
					return nil, fmt.Errorf("保存订阅用户失败: %w", err)
				}
			}
			if req.NodeInbounds != nil {
				if err := a.syncTokenNodes(&record, dedupeIDs(*req.NodeInbounds)); err != nil {
					return nil, err
				}
			}
			if _, err := a.ReconcileQuota(); err != nil {
				return nil, err
			}
			return a.tokenView(arg.ID)
		},
	})
	a.registerModify(ai.Tool{
		Name:         "delete_subscription_token",
		Description:  "按 ID 删除订阅用户（连同其全部节点与累计流量）",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "订阅用户 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if err := a.deleteToken(arg.ID); err != nil {
				return nil, err
			}
			return gin.H{"deleted": arg.ID}, nil
		},
	})
	a.registerModify(ai.Tool{
		Name:         "reset_subscription_token",
		Description:  "按 ID 重置订阅链接（换新随机值并恢复可用，旧链接立即失效）",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "订阅用户 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			var record db.SubscriptionToken
			if err := a.db.First(&record, arg.ID).Error; err != nil {
				return nil, fmt.Errorf("订阅用户不存在: %d", arg.ID)
			}
			if err := a.db.Model(&record).Updates(map[string]any{"token": randToken(), "revoked": false}).Error; err != nil {
				return nil, fmt.Errorf("重置订阅链接失败: %w", err)
			}
			return a.tokenView(arg.ID)
		},
	})
	a.registerModify(ai.Tool{
		Name:         "reset_user_traffic",
		Description:  "按 ID 清零某订阅用户全部节点的累计流量（配额已用尽的用户会随之恢复）",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "订阅用户 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			// 配额计数在 TokenTraffic，清零后该用户的暂停状态才会解除
			if err := a.db.Where("token_id = ?", arg.ID).Delete(&db.TokenTraffic{}).Error; err != nil {
				return nil, err
			}
			var users []db.InboundUser
			if err := a.db.Where("id IN (?)", a.db.Model(&db.SubscriptionNode{}).
				Select("inbound_user_id").Where("token_id = ?", arg.ID)).Find(&users).Error; err != nil {
				return nil, err
			}
			for _, u := range users {
				if err := a.db.Where("tag = ?", u.Name).Delete(&db.UserTraffic{}).Error; err != nil {
					return nil, err
				}
			}
			if _, err := a.ReconcileQuota(); err != nil {
				return nil, err
			}
			return a.tokenView(arg.ID)
		},
	})
	a.registerModify(ai.Tool{
		Name:        "set_dns_config",
		Description: "保存面板 DNS 配置（含服务器列表与分流规则），保存后内核立即按新配置重建。服务器 type 可选 udp/tcp/tls/https/h3/local",
		Parameters: toolParams(map[string]any{
			"enabled":  toolProp("boolean", "是否启用自定义 DNS"),
			"strategy": toolProp("string", "默认域名解析策略", singbox.SupportedDNSStrategies...),
			"servers":  map[string]any{"type": "array", "items": map[string]any{"type": "object"}, "description": "DNS 服务器列表：{tag,type,server,serverPort,path,detour}"},
			"rules":    map[string]any{"type": "array", "items": map[string]any{"type": "object"}, "description": "分流规则：{domain,domainSuffix,domainKeyword,server}"},
			"final":    toolProp("string", "默认 DNS 服务器 tag"),
		}),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var cfg singbox.DNSConfig
			if err := decodeToolArgs(args, &cfg); err != nil {
				return nil, err
			}
			if err := cfg.Validate(); err != nil {
				return nil, err
			}
			raw, err := json.Marshal(cfg)
			if err != nil {
				return nil, err
			}
			if err := a.setSetting(db.SettingDNS, string(raw)); err != nil {
				return nil, err
			}
			return cfg, nil
		},
	})
	a.registerModify(ai.Tool{
		Name: "create_self_signed_cert",
		Description: "一键生成自签 TLS 证书（域名作 CN 并入 SAN，面板存储 managed 口径），" +
			"生成后可在入站 TLS 中按证书 ID 引用；保存后内核立即按新配置重建",
		Parameters: toolParams(map[string]any{
			"domain":  toolProp("string", "主域名（作 CN 与 SAN，不能是 IP；IP 请填入 sans）"),
			"sans":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "附加 SAN（域名或 IP）"},
			"days":    toolProp("integer", "有效期天数（1-36500）"),
			"name":    toolProp("string", "名称备注，缺省取主域名"),
			"keyType": toolProp("string", "密钥类型：p256（默认）/ rsa2048 / ed25519", SupportedCertKeyTypes...),
		}, "domain", "days"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req selfSignReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			return a.createSelfSigned(req)
		},
	})
	a.registerModify(ai.Tool{
		Name:         "enable_bbr",
		Description:  "一键开启/关闭服务器 BBR 加速（写入 /etc/sysctl.d 并立即生效）；仅 Linux 且面板以 root 运行时可用",
		Parameters:   toolParams(map[string]any{"enable": toolProp("boolean", "true 开启 / false 关闭，缺省 true")}),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg struct {
				Enable *bool `json:"enable"`
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			enable := true
			if arg.Enable != nil {
				enable = *arg.Enable
			}
			if err := applyBBR(enable); err != nil {
				return nil, err
			}
			return readSystemStatus(), nil
		},
	})
	a.registerModify(ai.Tool{
		Name:        "set_settings",
		Description: "更新面板全局设置：未命中规则的兜底出站 tag",
		Parameters: toolParams(map[string]any{
			"routeFinal": toolProp("string", "未命中规则的兜底出站 tag，空用内核默认"),
		}, "routeFinal"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg struct {
				RouteFinal string `json:"routeFinal"`
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			final := strings.TrimSpace(arg.RouteFinal)
			if err := a.setSetting(db.SettingRouteFinal, final); err != nil {
				return nil, err
			}
			return gin.H{"routeFinal": final}, nil
		},
	})
}

// toolProp 构造参数属性描述；values 非空时附加枚举。
func toolProp(typ, description string, values ...string) map[string]any {
	prop := map[string]any{"type": typ, "description": description}
	if len(values) > 0 {
		prop["enum"] = values
	}
	return prop
}

// toolParams 构造 object 参数 schema；required 为必填字段名。
func toolParams(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
