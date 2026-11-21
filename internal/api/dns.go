package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/ai"
	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// dnsView DNS 设置视图：当前配置 + 可选类型/策略/预设（前端下拉与预设卡直接消费）。
type dnsView struct {
	Config  singbox.DNSConfig   `json:"config"`     // 当前生效的 DNS 配置
	Types   []string            `json:"types"`      // 支持的服务器类型
	Strats  []string            `json:"strategies"` // 支持的域名解析策略
	Presets []singbox.DNSPreset `json:"presets"`    // 内置预设
}

// HandleDNSGet GET /api/dns DNS 设置（未设置返回默认国内外分流配置）。
func (a *API) HandleDNSGet(c *gin.Context) {
	cfg, err := a.buildDNS()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "读取 DNS 配置失败: "+err.Error())
		return
	}
	OK(c, dnsView{
		Config: *cfg, Types: singbox.SupportedDNSTypes,
		Strats: singbox.SupportedDNSStrategies, Presets: singbox.DNSPresets,
	})
}

// HandleDNSSet PUT /api/dns 保存 DNS 配置（校验后落库并热应用内核）。
func (a *API) HandleDNSSet(c *gin.Context) {
	var cfg singbox.DNSConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if err := cfg.Validate(); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "序列化 DNS 配置失败: "+err.Error())
		return
	}
	if err := a.setSetting(db.SettingDNS, string(raw)); err != nil {
		Fail(c, http.StatusInternalServerError, "保存 DNS 配置失败: "+err.Error())
		return
	}
	a.applyThenOK(c, cfg)
}

// ---------- DNS AI 工具 ----------

// registerDNSAITools 注册 DNS 配置相关的查询与写操作 AI 工具。
func (a *API) registerDNSAITools() {
	a.ai.Register(ai.Tool{
		Name:        "get_dns_config",
		Description: "查询面板 DNS 配置（服务器列表、分流规则、默认域名策略与是否启用）",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return a.buildDNS()
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
}
