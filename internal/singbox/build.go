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

// BuildConfig 聚合启用中的入站/出站/路由生成完整内核配置（日志、clash_api、缓存文件与
// direct 兜底出站自动包含）；routing 为 nil 或全空时不生成 route 段。
func BuildConfig(inbounds []InboundEntry, outbounds []OutboundEntry, routing *Routing) (Config, error) {
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
	if routing != nil {
		if err := applyRouting(&cfg, routing); err != nil {
			return Config{}, err
		}
	}
	if !hasDirect {
		// direct 兜底出站：无任何 direct 出站时补齐，供路由规则引用
		cfg.Outbounds = append(cfg.Outbounds, map[string]any{"type": "direct", "tag": "direct"})
	}
	return cfg, nil
}

// applyRouting 把策略组出站与路由规则写入配置，校验出站引用与策略组循环引用。
func applyRouting(cfg *Config, routing *Routing) error {
	tags := map[string]bool{"direct": true} // direct 兜底视为始终可用
	for _, ob := range cfg.Outbounds {
		if tag, _ := ob["tag"].(string); tag != "" {
			tags[tag] = true
		}
	}
	groupTags := map[string]bool{}
	for _, g := range routing.Groups {
		if tags[g.Name] {
			return fmt.Errorf("策略组 %s 与出站同名", g.Name)
		}
		groupTags[g.Name] = true
	}
	for _, g := range routing.Groups {
		for _, m := range g.Members {
			if m == g.Name {
				return fmt.Errorf("策略组 %s 不能引用自身", g.Name)
			}
			if !tags[m] && !groupTags[m] {
				return fmt.Errorf("策略组 %s 的成员 %s 不存在", g.Name, m)
			}
		}
	}
	if err := detectGroupCycle(routing.Groups); err != nil {
		return err
	}
	for _, g := range routing.Groups {
		ob, err := GroupOutboundJSON(g)
		if err != nil {
			return err
		}
		cfg.Outbounds = append(cfg.Outbounds, ob)
	}
	route := &RouteConfig{}
	var ruleSetTags []string
	seen := map[string]bool{}
	for _, r := range routing.Rules {
		if r.Outbound != OutboundBlock && !tags[r.Outbound] && !groupTags[r.Outbound] {
			return fmt.Errorf("规则 %s 的出站 %s 不存在", r.Value, r.Outbound)
		}
		rule, refs, err := RoutingRuleJSON(r)
		if err != nil {
			return err
		}
		route.Rules = append(route.Rules, rule)
		for _, tag := range refs {
			if !seen[tag] {
				seen[tag] = true
				ruleSetTags = append(ruleSetTags, tag)
			}
		}
	}
	for _, tag := range ruleSetTags {
		rs, err := RuleSetConfigOf(tag, routing.AntiAD)
		if err != nil {
			return err
		}
		route.RuleSet = append(route.RuleSet, rs)
	}
	if routing.Final != "" {
		if !tags[routing.Final] && !groupTags[routing.Final] {
			return fmt.Errorf("兜底出站 %s 不存在", routing.Final)
		}
		route.Final = routing.Final
	}
	if len(route.Rules) > 0 || route.Final != "" {
		cfg.Route = route
	}
	return nil
}

// detectGroupCycle 以 DFS 检测策略组间循环引用。
func detectGroupCycle(groups []GroupEntry) error {
	members := map[string][]string{}
	for _, g := range groups {
		members[g.Name] = g.Members
	}
	const visiting, done = 1, 2
	state := map[string]int{}
	var visit func(name string) error
	visit = func(name string) error {
		state[name] = visiting
		for _, m := range members[name] {
			if _, isGroup := members[m]; !isGroup {
				continue
			}
			switch state[m] {
			case visiting:
				return fmt.Errorf("策略组 %s 与 %s 存在循环引用", name, m)
			case done:
				continue
			}
			if err := visit(m); err != nil {
				return err
			}
		}
		state[name] = done
		return nil
	}
	for _, g := range groups {
		if state[g.Name] == 0 {
			if err := visit(g.Name); err != nil {
				return err
			}
		}
	}
	return nil
}
