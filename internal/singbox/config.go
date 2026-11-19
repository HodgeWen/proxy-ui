package singbox

// Config sing-box 运行配置（入站/出站由配置管理聚合生成；路由/DNS 由全局规则阶段接入）。
type Config struct {
	Log          *LogConfig       `json:"log,omitempty"`          // 日志
	DNS          any              `json:"dns,omitempty"`          // DNS 配置预留（全局规则阶段接入）
	Inbounds     []any            `json:"inbounds,omitempty"`     // 入站列表（BuildConfig 聚合生成）
	Outbounds    []map[string]any `json:"outbounds,omitempty"`    // 出站列表（含 direct 兜底）
	Route        any              `json:"route,omitempty"`        // 路由规则预留（全局规则阶段接入）
	Experimental *Experimental    `json:"experimental,omitempty"` // 实验特性：clash_api 供流量/连接统计
}

// LogConfig 内核日志配置。
type LogConfig struct {
	Level     string `json:"level"`     // 日志级别：trace/debug/info/warn/error/fatal/panic
	Timestamp bool   `json:"timestamp"` // 日志带时间戳
}

// Experimental 实验特性配置。
type Experimental struct {
	ClashAPI *ClashAPI `json:"clash_api,omitempty"` // Clash API（流量/连接统计阶段消费）
}

// ClashAPI Clash API 配置。
type ClashAPI struct {
	ExternalController string `json:"external_controller"` // 监听地址，仅本机访问
}

// MinConfig 最小可运行配置：日志、direct 出站兜底与 clash_api 预留，无任何入站。
func MinConfig() Config {
	return Config{
		Log: &LogConfig{Level: "info", Timestamp: true},
		Outbounds: []map[string]any{
			{"type": "direct", "tag": "direct"},
		},
		Experimental: &Experimental{
			ClashAPI: &ClashAPI{ExternalController: "127.0.0.1:19090"},
		},
	}
}
