package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/ai"
	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// ---------- 供应商配置 ----------

// aiConfigView AI 供应商配置视图（api-key 不回显，仅给出是否已配置）。
type aiConfigView struct {
	BaseURL         string `json:"baseUrl"`         // 接口地址
	Model           string `json:"model"`           // 模型名
	ReasoningEffort string `json:"reasoningEffort"` // 推理强度：low/medium/high，空为不指定
	HasKey          bool   `json:"hasKey"`          // 是否已配置 api-key
	Configured      bool   `json:"configured"`      // 配置是否完整（可发起对话）
}

// aiConfigReq 保存供应商配置请求；apiKey 为空表示保留已存 Key。
type aiConfigReq struct {
	BaseURL         string `json:"baseUrl"`         // 接口地址
	Model           string `json:"model"`           // 模型名
	APIKey          string `json:"apiKey"`          // API Key（可选，空为保留）
	ReasoningEffort string `json:"reasoningEffort"` // 推理强度
}

// aiConfigToView 配置转响应视图。
func aiConfigToView(cfg ai.Config) aiConfigView {
	return aiConfigView{
		BaseURL: cfg.BaseURL, Model: cfg.Model, ReasoningEffort: cfg.ReasoningEffort,
		HasKey: cfg.APIKey != "", Configured: cfg.Ready(),
	}
}

// HandleAIConfigGet GET /api/ai/config AI 供应商配置。
func (a *API) HandleAIConfigGet(c *gin.Context) {
	cfg, err := a.ai.LoadConfig()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, aiConfigToView(cfg))
}

// HandleAIConfigPut PUT /api/ai/config 保存 AI 供应商配置。
func (a *API) HandleAIConfigPut(c *gin.Context) {
	var req aiConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	cfg, err := a.ai.SaveConfig(ai.Config{
		BaseURL: req.BaseURL, Model: req.Model, APIKey: req.APIKey, ReasoningEffort: req.ReasoningEffort,
	})
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, aiConfigToView(cfg))
}

// ---------- 流式对话（SSE） ----------

// maxAIChatBody 对话代理请求体大小上限。
const maxAIChatBody = 16 << 20

// HandleAIChat POST /api/ai/chat OpenAI 兼容流式对话代理：
// 注入供应商 api-key 与缺省参数后转发上游，SSE 原样回传。
// 响应不经统一 envelope（由前端 @veltra/ai transport 按 OpenAI 协议解析）。
func (a *API) HandleAIChat(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxAIChatBody+1))
	if err != nil {
		failAI(c, http.StatusBadRequest, "读取请求失败")
		return
	}
	if len(body) > maxAIChatBody {
		failAI(c, http.StatusRequestEntityTooLarge, "请求体过大")
		return
	}
	resp, err := a.ai.Chat(c.Request.Context(), body)
	if err != nil {
		switch {
		case errors.Is(err, ai.ErrNotConfigured):
			failAI(c, http.StatusServiceUnavailable, "AI 供应商未配置，请先在设置页完成配置")
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
	result, err := a.ai.Call(c.Request.Context(), req.Name, req.Arguments)
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
		Description: "列出面板全部入站（含协议、监听端口、启用状态、完整配置与弱配置警示）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.inboundViews()
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_inbound",
		Description: "按 ID 查询单个入站详情（含完整配置与弱配置警示）",
		Parameters:  toolParams(map[string]any{"id": toolProp("integer", "入站 ID")}, "id"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			var in db.Inbound
			if err := a.db.First(&in, arg.ID).Error; err != nil {
				return nil, fmt.Errorf("入站不存在: %d", arg.ID)
			}
			return a.inboundToView(in), nil
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
		Name:        "list_rules",
		Description: "列出全部全局路由规则（按生效顺序，含类型、匹配值与出站动作）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.ruleViews()
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
		Description: "列出全部订阅令牌（含匿名 token、备注、吊销状态与订阅服务器地址设置）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			var records []db.SubscriptionToken
			if err := a.db.Order("id").Find(&records).Error; err != nil {
				return nil, err
			}
			views := make([]tokenView, len(records))
			for i := range records {
				views[i] = tokenView(records[i])
			}
			return gin.H{"server": a.getSetting(db.SettingSubServer), "tokens": views}, nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_kernel_status",
		Description: "查询 sing-box 内核运行状态（是否安装、是否运行、版本、运行时长）",
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
		}, ""),
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
			snap := a.stats.Snapshot()
			return gin.H{
				"up": rates.Up, "down": rates.Down, "memory": snap.Memory,
				"uploadTotal": snap.UploadTotal, "downloadTotal": snap.DownloadTotal,
				"activeConnections": len(snap.Connections), "at": at,
			}, nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_connections",
		Description: "查询当前连接列表（含入站/出站 tag、规则、上下行流量与开始时间）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.stats.Snapshot(), nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_inbound_traffic",
		Description: "查询按入站持久化累计的上下行流量（面板重启不丢失）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.inboundTrafficViews()
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "get_settings",
		Description: "查询面板全局设置：anti-AD 规则源、未命中规则兜底出站、订阅节点服务器地址与下载镜像前缀",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return gin.H{
				"antiAdSource":        a.getSetting(settingAntiAD),
				"antiAdSourceDefault": singbox.DefaultAntiADURL,
				"routeFinal":          a.getSetting(settingRouteFinal),
				"subscriptionServer":  a.getSetting(db.SettingSubServer),
				"mirrorPrefix":        a.updates.MirrorPrefix(),
			}, nil
		},
	})
}

func (a *API) registerAIModifyTools() {
	a.ai.Register(ai.Tool{
		Name: "create_inbound",
		Description: "创建入站。protocol 为面板支持协议；config 缺省时按协议填充默认配置" +
			"（自动生成随机密码/UUID/Reality 密钥，TLS 类协议默认启用并待选择证书）；创建后配置未生效，需调用 apply_config 应用",
		Parameters: toolParams(map[string]any{
			"protocol":   toolProp("string", "入站协议", singbox.SupportedProtocols...),
			"listenPort": toolProp("integer", "监听端口（1-65535）"),
			"name":       toolProp("string", "名称（入站 tag），缺省按 协议-端口 生成"),
			"config":     map[string]any{"type": "object", "description": "协议特定配置对象，缺省按协议填默认值"},
			"enabled":    toolProp("boolean", "是否启用，缺省 true"),
		}, "protocol", "listenPort"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req inboundReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			dropEmptyConfig(&req)
			return a.createInbound(req)
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "update_inbound",
		Description: "按 ID 更新入站；仅更新给出的字段，config 为完整协议配置对象（变更协议时必须提供）",
		Parameters: toolParams(map[string]any{
			"id":         toolProp("integer", "入站 ID"),
			"name":       toolProp("string", "名称"),
			"listenPort": toolProp("integer", "监听端口"),
			"config":     map[string]any{"type": "object", "description": "完整协议配置对象"},
			"enabled":    toolProp("boolean", "是否启用"),
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
	a.ai.Register(ai.Tool{
		Name:         "delete_inbound",
		Description:  "按 ID 删除入站（连同其累计流量记录）",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "入站 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			return a.deleteInbound(arg.ID)
		},
	})
	a.ai.Register(ai.Tool{
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
	a.ai.Register(ai.Tool{
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
	a.ai.Register(ai.Tool{
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
	a.ai.Register(ai.Tool{
		Name:        "create_rule",
		Description: "创建全局路由规则（追加到规则末尾）；outbound 为动作 tag：direct、block、策略组或自定义出站",
		Parameters: toolParams(map[string]any{
			"type":     toolProp("string", "规则类型", singbox.SupportedRuleTypes...),
			"value":    toolProp("string", "匹配值（多值逗号分隔；geosite/geoip 填类别名，rule_set 填规则集 tag 如 anti-ad）"),
			"outbound": toolProp("string", "动作出站 tag"),
		}, "type", "value", "outbound"),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var req ruleReq
			if err := decodeToolArgs(args, &req); err != nil {
				return nil, err
			}
			rule := db.Rule{Enabled: true}
			if err := applyRuleReq(&rule, req); err != nil {
				return nil, err
			}
			var maxOrder int
			a.db.Model(&db.Rule{}).Select("COALESCE(MAX(`order`), 0)").Scan(&maxOrder)
			rule.Order = maxOrder + 1
			if err := a.db.Create(&rule).Error; err != nil {
				return nil, fmt.Errorf("创建规则失败: %w", err)
			}
			return ruleView(rule), nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "update_rule",
		Description: "按 ID 更新规则；仅更新给出的字段",
		Parameters: toolParams(map[string]any{
			"id":       toolProp("integer", "规则 ID"),
			"type":     toolProp("string", "规则类型", singbox.SupportedRuleTypes...),
			"value":    toolProp("string", "匹配值"),
			"outbound": toolProp("string", "动作出站 tag"),
			"enabled":  toolProp("boolean", "是否启用"),
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
			if err := applyRuleReq(&rule, req); err != nil {
				return nil, err
			}
			if err := a.db.Save(&rule).Error; err != nil {
				return nil, fmt.Errorf("保存规则失败: %w", err)
			}
			return ruleView(rule), nil
		},
	})
	a.ai.Register(ai.Tool{
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
	a.ai.Register(ai.Tool{
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
	a.ai.Register(ai.Tool{
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
	a.ai.Register(ai.Tool{
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
	a.ai.Register(ai.Tool{
		Name:        "create_subscription_token",
		Description: "生成订阅匿名令牌（客户端免登录拉取订阅链接使用）",
		Parameters: toolParams(map[string]any{
			"name": toolProp("string", "备注名"),
		}),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg struct {
				Name string `json:"name"`
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			record := db.SubscriptionToken{Token: randToken(), Name: strings.TrimSpace(arg.Name)}
			if err := a.db.Create(&record).Error; err != nil {
				return nil, fmt.Errorf("创建订阅令牌失败: %w", err)
			}
			return tokenView(record), nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:         "reset_subscription_token",
		Description:  "按 ID 重置订阅令牌（换新随机值并恢复可用，旧链接立即失效）",
		Parameters:   toolParams(map[string]any{"id": toolProp("integer", "令牌 ID")}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg idArg
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			var record db.SubscriptionToken
			if err := a.db.First(&record, arg.ID).Error; err != nil {
				return nil, fmt.Errorf("订阅令牌不存在: %d", arg.ID)
			}
			updates := map[string]any{"token": randToken(), "revoked": false}
			if err := a.db.Model(&record).Updates(updates).Error; err != nil {
				return nil, fmt.Errorf("重置订阅令牌失败: %w", err)
			}
			record.Token = updates["token"].(string)
			record.Revoked = false
			return tokenView(record), nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "revoke_subscription_token",
		Description: "按 ID 吊销（或恢复）订阅令牌；吊销后携带该 token 的订阅请求被拒绝",
		Parameters: toolParams(map[string]any{
			"id":      toolProp("integer", "令牌 ID"),
			"revoked": toolProp("boolean", "true 吊销 / false 恢复，缺省 true"),
		}, "id"),
		NeedsConfirm: true,
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg struct {
				ID      uint  `json:"id"`
				Revoked *bool `json:"revoked"`
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			revoked := true
			if arg.Revoked != nil {
				revoked = *arg.Revoked
			}
			var record db.SubscriptionToken
			if err := a.db.First(&record, arg.ID).Error; err != nil {
				return nil, fmt.Errorf("订阅令牌不存在: %d", arg.ID)
			}
			if err := a.db.Model(&record).Update("revoked", revoked).Error; err != nil {
				return nil, fmt.Errorf("更新订阅令牌失败: %w", err)
			}
			record.Revoked = revoked
			return tokenView(record), nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:         "kernel_control",
		Description:  "启动/停止/重启 sing-box 内核",
		Parameters:   toolParams(map[string]any{"action": toolProp("string", "动作", "start", "stop", "restart")}, "action"),
		NeedsConfirm: true,
		Exec: func(ctx context.Context, args json.RawMessage) (any, error) {
			var arg struct {
				Action string `json:"action"`
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			var err error
			switch arg.Action {
			case "start":
				err = a.core.Start()
			case "stop":
				err = a.core.Stop()
			case "restart":
				err = a.core.Restart()
			default:
				return nil, fmt.Errorf("不支持的动作: %s（可选 start/stop/restart）", arg.Action)
			}
			if err != nil {
				return nil, err
			}
			return a.kernelStatusOf(ctx), nil
		},
	})
	a.ai.Register(ai.Tool{
		Name:         "apply_config",
		Description:  "聚合全部启用入站/出站/规则重新生成 sing-box 配置并应用（内核运行中则按新配置重启）；配置类修改后需调用本工具生效",
		Parameters:   map[string]any{"type": "object", "properties": map[string]any{}},
		NeedsConfirm: true,
		Exec: func(ctx context.Context, _ json.RawMessage) (any, error) {
			result, _, err := a.applyKernel(ctx)
			return result, err
		},
	})
	a.ai.Register(ai.Tool{
		Name:        "set_settings",
		Description: "更新面板全局设置（至少提供一项）：anti-AD 规则源地址、未命中规则兜底出站 tag、订阅节点服务器地址",
		Parameters: toolParams(map[string]any{
			"antiAdSource":       toolProp("string", "anti-AD 规则源地址，空恢复官方默认源"),
			"routeFinal":         toolProp("string", "未命中规则的兜底出站 tag，空用内核默认"),
			"subscriptionServer": toolProp("string", "订阅节点服务器地址（域名或 IP），空按请求 Host 推断"),
		}),
		Exec: func(_ context.Context, args json.RawMessage) (any, error) {
			var arg struct {
				AntiAdSource       *string `json:"antiAdSource"`
				RouteFinal         *string `json:"routeFinal"`
				SubscriptionServer *string `json:"subscriptionServer"`
			}
			if err := decodeToolArgs(args, &arg); err != nil {
				return nil, err
			}
			if arg.AntiAdSource == nil && arg.RouteFinal == nil && arg.SubscriptionServer == nil {
				return nil, fmt.Errorf("至少提供一项设置")
			}
			updated := gin.H{}
			if arg.AntiAdSource != nil {
				url := strings.TrimSpace(*arg.AntiAdSource)
				if url != "" && !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
					return nil, fmt.Errorf("规则源地址须以 http:// 或 https:// 开头")
				}
				if err := a.setSetting(settingAntiAD, url); err != nil {
					return nil, err
				}
				updated["antiAdSource"] = url
			}
			if arg.RouteFinal != nil {
				final := strings.TrimSpace(*arg.RouteFinal)
				if err := a.setSetting(settingRouteFinal, final); err != nil {
					return nil, err
				}
				updated["routeFinal"] = final
			}
			if arg.SubscriptionServer != nil {
				server := strings.TrimSpace(*arg.SubscriptionServer)
				if err := a.setSetting(db.SettingSubServer, server); err != nil {
					return nil, err
				}
				updated["subscriptionServer"] = server
			}
			return updated, nil
		},
	})
}

// createInbound / updateInbound / deleteInbound 供 AI 工具复用的入站写操作（与 REST handler 同一校验路径）。
func (a *API) createInbound(req inboundReq) (inboundView, error) {
	in := db.Inbound{Enabled: true}
	if err := a.applyInboundReq(&in, req, 0, ""); err != nil {
		return inboundView{}, err
	}
	if err := a.db.Create(&in).Error; err != nil {
		return inboundView{}, fmt.Errorf("创建入站失败: %w", err)
	}
	return a.inboundToView(in), nil
}

func (a *API) updateInbound(id uint, req inboundReq) (inboundView, error) {
	var in db.Inbound
	if err := a.db.First(&in, id).Error; err != nil {
		return inboundView{}, fmt.Errorf("入站不存在: %d", id)
	}
	origProtocol := in.Protocol
	if err := a.applyInboundReq(&in, req, id, origProtocol); err != nil {
		return inboundView{}, err
	}
	if err := a.db.Save(&in).Error; err != nil {
		return inboundView{}, fmt.Errorf("保存入站失败: %w", err)
	}
	return a.inboundToView(in), nil
}

func (a *API) deleteInbound(id uint) (gin.H, error) {
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("inbound_id = ?", id).Delete(&db.InboundTraffic{}).Error; err != nil {
			return err
		}
		return tx.Delete(&db.Inbound{}, id).Error
	})
	if err != nil {
		return nil, fmt.Errorf("删除入站失败: %w", err)
	}
	return gin.H{"deleted": id}, nil
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
