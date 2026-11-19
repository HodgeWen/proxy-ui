package singbox

// Config sing-box 运行配置（入站/出站/路由由配置管理聚合生成；DNS 预留）。
type Config struct {
	Log          *LogConfig       `json:"log,omitempty"`          // 日志
	DNS          any              `json:"dns,omitempty"`          // DNS 配置预留
	Inbounds     []any            `json:"inbounds,omitempty"`     // 入站列表（BuildConfig 聚合生成）
	Outbounds    []map[string]any `json:"outbounds,omitempty"`    // 出站列表（含策略组与 direct 兜底）
	Route        *RouteConfig     `json:"route,omitempty"`        // 路由规则（全局规则聚合生成）
	Experimental *Experimental    `json:"experimental,omitempty"` // 实验特性：clash_api 与缓存文件
}

// RouteConfig 路由配置：规则、规则集与未命中兜底出站。
type RouteConfig struct {
	Rules   []map[string]any `json:"rules,omitempty"`    // 路由规则（按面板排序生效）
	RuleSet []RuleSetConfig  `json:"rule_set,omitempty"` // 远程规则集（geosite/geoip/anti-AD）
	Final   string           `json:"final,omitempty"`    // 未命中规则的兜底出站 tag
}

// RuleSetConfig 远程规则集。
type RuleSetConfig struct {
	Type           string `json:"type"`                      // 类型：remote
	Tag            string `json:"tag"`                       // 规则集 tag
	Format         string `json:"format"`                    // 格式：binary（.srs）
	URL            string `json:"url"`                       // 下载地址
	DownloadDetour string `json:"download_detour,omitempty"` // 下载经由此出站（订阅客户端配置 direct，避免代理未就绪时下载失败）
}

// LogConfig 内核日志配置。
type LogConfig struct {
	Level     string `json:"level"`     // 日志级别：trace/debug/info/warn/error/fatal/panic
	Timestamp bool   `json:"timestamp"` // 日志带时间戳
}

// Experimental 实验特性配置。
type Experimental struct {
	ClashAPI  *ClashAPI  `json:"clash_api,omitempty"`  // Clash API（流量/连接统计阶段消费）
	CacheFile *CacheFile `json:"cache_file,omitempty"` // 缓存文件（远程规则集持久化）
}

// ClashAPI Clash API 配置。
type ClashAPI struct {
	ExternalController string `json:"external_controller"` // 监听地址，仅本机访问
	Secret             string `json:"secret,omitempty"`    // Bearer 认证密钥（面板生成的配置留空）
}

// CacheFile 缓存文件配置：开启后远程规则集缓存到 cache.db，内核重启免重复下载。
type CacheFile struct {
	Enabled bool `json:"enabled"` // 是否启用
}

// MinConfig 最小可运行配置：日志、direct 出站兜底与 clash_api/缓存文件，无任何入站。
func MinConfig() Config {
	return Config{
		Log: &LogConfig{Level: "info", Timestamp: true},
		Outbounds: []map[string]any{
			{"type": "direct", "tag": "direct"},
		},
		Experimental: &Experimental{
			ClashAPI:  &ClashAPI{ExternalController: "127.0.0.1:19090"},
			CacheFile: &CacheFile{Enabled: true},
		},
	}
}
