package singbox

// Config sing-box 运行配置（入站/出站/路由/DNS 由配置管理聚合生成）。
type Config struct {
	Log          *LogConfig       `json:"log,omitempty"`          // 日志
	DNS          map[string]any   `json:"dns,omitempty"`          // DNS（面板设置生成，未启用时省略）
	Inbounds     []any            `json:"inbounds,omitempty"`     // 入站列表（BuildConfig 聚合生成）
	Outbounds    []map[string]any `json:"outbounds,omitempty"`    // 出站列表（含策略组与 direct 兜底）
	Route        *RouteConfig     `json:"route,omitempty"`        // 路由规则（全局规则聚合生成）
	Experimental *Experimental    `json:"experimental,omitempty"` // 实验特性：统计服务与缓存文件
}

// RouteConfig 路由配置：规则、规则集、未命中兜底出站与默认域名解析器。
type RouteConfig struct {
	Rules                 []map[string]any `json:"rules,omitempty"`                   // 路由规则（按面板排序生效）
	RuleSet               []RuleSetConfig  `json:"rule_set,omitempty"`                // 远程规则集（规则集内容项）
	Final                 string           `json:"final,omitempty"`                   // 未命中规则的兜底出站 tag
	DefaultDomainResolver string           `json:"default_domain_resolver,omitempty"` // 默认域名解析使用的 DNS 服务器 tag
}

// RuleSetConfig 远程规则集。
type RuleSetConfig struct {
	Type           string `json:"type"`                      // 类型：remote
	Tag            string `json:"tag"`                       // 规则集 tag
	Format         string `json:"format"`                    // 格式：binary（.srs）
	URL            string `json:"url"`                       // 下载地址
	DownloadDetour string `json:"download_detour,omitempty"` // 下载经由此出站（订阅客户端配置 direct）
}

// LogConfig 内核日志配置。
type LogConfig struct {
	Level     string `json:"level"`     // 日志级别：trace/debug/info/warn/error/fatal/panic
	Timestamp bool   `json:"timestamp"` // 日志带时间戳
}

// Experimental 实验特性配置。
type Experimental struct {
	V2RayAPI  *V2RayAPI  `json:"v2ray_api,omitempty"`  // v2ray 统计服务（按用户/按入站统计流量）
	CacheFile *CacheFile `json:"cache_file,omitempty"` // 缓存文件（远程规则集持久化）
}

// V2RayAPI v2ray 统计服务配置。
type V2RayAPI struct {
	Listen string          `json:"listen"` // 监听地址（仅本机回环）
	Stats  *V2RayStatsConf `json:"stats"`  // 统计范围
}

// V2RayStatsConf 统计范围：哪些入站与用户参与计数。
type V2RayStatsConf struct {
	Enabled   bool     `json:"enabled"`             // 是否启用统计
	Inbounds  []string `json:"inbounds,omitempty"`  // 参与统计的入站 tag
	Outbounds []string `json:"outbounds,omitempty"` // 参与统计的出站 tag
	Users     []string `json:"users,omitempty"`     // 参与统计的用户名
}

// CacheFile 缓存文件配置：开启后远程规则集缓存到 cache.db，内核重启免重复下载。
type CacheFile struct {
	Enabled bool `json:"enabled"` // 是否启用
}

// MinConfig 最小可运行配置：日志、direct 出站兜底、统计服务与缓存文件，无任何入站。
func MinConfig() Config {
	return Config{
		Log: &LogConfig{Level: "info", Timestamp: true},
		Outbounds: []map[string]any{
			{"type": "direct", "tag": "direct"},
		},
		Experimental: &Experimental{
			V2RayAPI:  &V2RayAPI{Listen: StatsListen(), Stats: &V2RayStatsConf{Enabled: true}},
			CacheFile: &CacheFile{Enabled: true},
		},
	}
}
