package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// dnsView DNS 设置视图：当前配置 + 可选类型/策略/预设（前端下拉与预设卡直接消费）。
type dnsView struct {
	Config  singbox.DNSConfig `json:"config"`     // 当前生效的 DNS 配置
	Types   []string          `json:"types"`      // 支持的服务器类型
	Strats  []string          `json:"strategies"` // 支持的域名解析策略
	Presets []dnsPresetView   `json:"presets"`    // 内置预设
}

// dnsPresetView DNS 预设视图。
type dnsPresetView struct {
	ID          string            `json:"id"`          // 预设 ID
	Name        string            `json:"name"`        // 名称
	Description string            `json:"description"` // 说明
	Config      singbox.DNSConfig `json:"config"`      // 预设配置
}

// HandleDNSGet GET /api/dns DNS 设置（未设置返回默认国内外分流配置）。
func (a *API) HandleDNSGet(c *gin.Context) {
	cfg, err := a.buildDNS()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "读取 DNS 配置失败: "+err.Error())
		return
	}
	presets := make([]dnsPresetView, 0, len(singbox.DNSPresets))
	for _, p := range singbox.DNSPresets {
		presets = append(presets, dnsPresetView{
			ID: p.ID, Name: p.Name, Description: p.Description, Config: p.Config,
		})
	}
	OK(c, dnsView{
		Config: *cfg, Types: singbox.SupportedDNSTypes,
		Strats: singbox.SupportedDNSStrategies, Presets: presets,
	})
}

// HandleDNSSet PUT /api/dns 保存 DNS 配置（校验后落库，需应用配置生效）。
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
	OK(c, cfg)
}
