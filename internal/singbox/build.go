package singbox

import "fmt"

// CertResolver 把面板存储证书 ID 解析为证书/私钥绝对路径（含存在性校验，由 api 提供）。
type CertResolver func(certID uint) (certPath, keyPath string, err error)

// InboundEntry 聚合生成的入站条目（Config 为面板存储的协议配置 JSON 原文）。
type InboundEntry struct {
	Tag        string       // sing-box inbound tag（取入站名称）
	Protocol   string       // 入站协议
	ListenPort int          // 监听端口
	Config     string       // 协议配置 JSON
	Cert       CertResolver // 面板存储证书解析（managed 模式证书使用）
}

// OutboundEntry 聚合生成的出站条目（Config 为 type/tag 之外字段的 JSON 对象）。
type OutboundEntry struct {
	Tag    string // sing-box outbound tag（取出站名称）
	Type   string // 出站类型：direct 或自定义出站协议
	Config string // 出站配置 JSON 对象
}

// BuildConfig 聚合启用中的入站/出站生成完整内核配置（日志、clash_api 与 direct 兜底出站自动包含）。
func BuildConfig(inbounds []InboundEntry, outbounds []OutboundEntry) (Config, error) {
	cfg := MinConfig()
	cfg.Outbounds = nil // 出站全部由条目聚合，无 direct 出站时末尾补兜底
	for _, e := range inbounds {
		parsed, err := ParseInboundConfig(e.Protocol, e.Config)
		if err != nil {
			return Config{}, fmt.Errorf("入站 %s: %w", e.Tag, err)
		}
		in, err := inboundJSON(e, parsed)
		if err != nil {
			return Config{}, fmt.Errorf("入站 %s: %w", e.Tag, err)
		}
		cfg.Inbounds = append(cfg.Inbounds, in)
	}
	hasDirect := false
	for _, e := range outbounds {
		ob, err := outboundJSON(e)
		if err != nil {
			return Config{}, err
		}
		if e.Tag == "direct" {
			hasDirect = true
		}
		cfg.Outbounds = append(cfg.Outbounds, ob)
	}
	if !hasDirect {
		// direct 兜底出站：无任何 direct 出站时补齐，供路由规则引用
		cfg.Outbounds = append(cfg.Outbounds, map[string]any{"type": "direct", "tag": "direct"})
	}
	return cfg, nil
}
