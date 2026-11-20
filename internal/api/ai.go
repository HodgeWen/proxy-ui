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

func (a *API) registerAIModifyTools() {
	a.ai.Register(ai.Tool{
		Name: "create_inbound",
		Description: "创建入站（真实监听）。入站只配连接层参数（TLS/证书/Reality/混淆/带宽/拥塞算法），" +
			"用户凭据在入站内单独创建；listenPort 省略时自动分配随机空闲端口；创建后需调用 apply_config 生效",
		Parameters: toolParams(map[string]any{
			"protocol": toolProp("string", "入站协议", singbox.SupportedProtocols...),
			"name":     toolProp("string", "入站名称，缺省按 协议-序号 生成"),
			"listen":   toolProp("string", "监听地址：:: 双栈 / 0.0.0.0 仅 IPv4 / 127.0.0.1 走反代，缺省 ::"),
			"config":   map[string]any{"type": "object", "description": "协议连接层配置对象，缺省按协议填默认值"},
			"enabled":  toolProp("boolean", "是否启用，缺省 true"),
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
	a.ai.Register(ai.Tool{
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
	a.ai.Register(ai.Tool{
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
	a.ai.Register(ai.Tool{
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
	a.ai.Register(ai.Tool{
		Name:        "update_subscription_token",
		Description: "更新订阅用户：名称、流量配额、到期时间、勾选的入站（会在入站内创建/移除该用户的身份）。改动后需 apply_config 生效",
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
	a.ai.Register(ai.Tool{
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
	a.ai.Register(ai.Tool{
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
	a.ai.Register(ai.Tool{
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
	a.ai.Register(ai.Tool{
		Name:        "set_dns_config",
		Description: "保存面板 DNS 配置（含服务器列表与分流规则），需 apply_config 生效。服务器 type 可选 udp/tcp/tls/https/h3/local",
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
	a.ai.Register(ai.Tool{
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
				err = a.core.Start(a.currentKernelConfig())
			case "stop":
				err = a.core.Stop()
			case "restart":
				err = a.core.Restart(a.currentKernelConfig())
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
		Description:  "聚合全部生效节点/出站/规则/DNS 重新生成 sing-box 配置并应用（内核运行中则按新配置重启）；配置类修改后需调用本工具生效",
		Parameters:   map[string]any{"type": "object", "properties": map[string]any{}},
		NeedsConfirm: true,
		Exec: func(ctx context.Context, _ json.RawMessage) (any, error) {
			if _, err := a.ReconcileQuota(); err != nil {
				return nil, err
			}
			result, _, err := a.applyKernel(ctx)
			return result, err
		},
	})
	a.ai.Register(ai.Tool{
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
