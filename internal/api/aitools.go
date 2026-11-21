package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/ai"
)

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

// registerAITools 把面板主要接口注册为 AI 工具：按资源分函数注册，
// 各注册函数落在对应资源文件里，查询与写操作一律委托共用业务方法。
func (a *API) registerAITools() {
	a.registerInboundAITools()
	a.registerCertAITools()
	a.registerOutboundAITools()
	a.registerRuleSetAITools()
	a.registerRuleAITools()
	a.registerGroupAITools()
	a.registerTokenAITools()
	a.registerDNSAITools()
	a.registerStatsAITools()
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
