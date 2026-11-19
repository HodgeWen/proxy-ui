package singbox

import (
	"fmt"
	"strings"
)

// 面板支持的路由规则类型。
const (
	RuleTypeDomain        = "domain"        // 域名全匹配
	RuleTypeDomainSuffix  = "domain_suffix" // 域名后缀
	RuleTypeDomainKeyword = "domain_keyword"
	RuleTypeIPCIDR        = "ip_cidr"  // IP-CIDR
	RuleTypeGeosite       = "geosite"  // Geosite 规则集
	RuleTypeGeoIP         = "geoip"    // GeoIP 规则集
	RuleTypeRuleSet       = "rule_set" // 任意规则集 tag
)

// SupportedRuleTypes 支持的路由规则类型列表。
var SupportedRuleTypes = []string{
	RuleTypeDomain, RuleTypeDomainSuffix, RuleTypeDomainKeyword,
	RuleTypeIPCIDR, RuleTypeGeosite, RuleTypeGeoIP, RuleTypeRuleSet,
}

// 策略组模式。
const (
	GroupModeFailover = "failover" // 故障转移
	GroupModeFixed    = "fixed"    // 固定代理
	GroupModeAuto     = "auto"     // 自动选择
)

// SupportedGroupModes 支持的策略组模式列表。
var SupportedGroupModes = []string{GroupModeFailover, GroupModeFixed, GroupModeAuto}

// OutboundBlock 拦截动作：生成规则时映射为 reject，不产生出站。
const OutboundBlock = "block"

// 规则集 tag 与下载地址。
const (
	AntiADTag        = "anti-ad"                                  // anti-AD 广告拦截规则集 tag
	DefaultAntiADURL = "https://anti-ad.net/anti-ad-sing-box.srs" // anti-AD 官方规则源
	geositeURLFormat = "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-%s.srs"
	geoipURLFormat   = "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-%s.srs"
)

// RuleEntry 路由规则条目（Value 多值用英文逗号分隔）。
type RuleEntry struct {
	Type     string // 规则类型
	Value    string // 匹配值
	Outbound string // 动作出站 tag：direct、block、策略组或自定义出站
}

// GroupEntry 策略组条目（映射为 selector/urltest 出站）。
type GroupEntry struct {
	Name    string   // 策略组 tag
	Mode    string   // 模式：failover/fixed/auto
	Members []string // 成员出站 tag（按优先级排序）
}

// Routing 路由聚合输入：启用中的规则、策略组与兜底出站。
type Routing struct {
	Rules  []RuleEntry  // 启用中的规则（按面板排序）
	Groups []GroupEntry // 启用中的策略组
	Final  string       // 未命中规则的兜底出站 tag，空表示内核默认
	AntiAD string       // anti-AD 规则源地址，空用官方默认源
}

// ruleSetConfig 生成规则集 tag 对应的远程规则集定义。
func ruleSetConfig(tag, antiAD string) (RuleSetConfig, error) {
	var url string
	switch {
	case tag == AntiADTag:
		url = antiAD
		if url == "" {
			url = DefaultAntiADURL
		}
	case strings.HasPrefix(tag, "geosite-"):
		url = fmt.Sprintf(geositeURLFormat, strings.TrimPrefix(tag, "geosite-"))
	case strings.HasPrefix(tag, "geoip-"):
		url = fmt.Sprintf(geoipURLFormat, strings.TrimPrefix(tag, "geoip-"))
	default:
		return RuleSetConfig{}, fmt.Errorf("规则集 %s 无可用下载地址（支持 anti-ad、geosite-*、geoip-*）", tag)
	}
	return RuleSetConfig{Type: "remote", Tag: tag, Format: "binary", URL: url}, nil
}

// routingRuleJSON 把规则条目映射为 sing-box route rule JSON 对象，返回引用的规则集 tag。
func routingRuleJSON(r RuleEntry) (map[string]any, []string, error) {
	items := splitRuleValue(r.Value)
	if len(items) == 0 {
		return nil, nil, fmt.Errorf("规则 %s 的匹配值不能为空", r.Value)
	}
	rule := map[string]any{}
	var refs []string
	switch r.Type {
	case RuleTypeRuleSet:
		refs = items
		rule["rule_set"] = items
	case RuleTypeGeosite, RuleTypeGeoIP:
		for _, v := range items {
			refs = append(refs, r.Type+"-"+v)
		}
		rule["rule_set"] = refs
	case RuleTypeDomain, RuleTypeDomainSuffix, RuleTypeDomainKeyword, RuleTypeIPCIDR:
		rule[r.Type] = items
	default:
		return nil, nil, fmt.Errorf("不支持的路由规则类型: %s", r.Type)
	}
	if r.Outbound == OutboundBlock {
		rule["action"] = "reject"
	} else {
		rule["outbound"] = r.Outbound
	}
	return rule, refs, nil
}

// groupOutboundJSON 把策略组映射为 sing-box selector/urltest 出站 JSON 对象。
func groupOutboundJSON(g GroupEntry) (map[string]any, error) {
	if len(g.Members) == 0 {
		return nil, fmt.Errorf("策略组 %s 未配置成员", g.Name)
	}
	switch g.Mode {
	case GroupModeFixed:
		// 固定代理：selector 默认选中首个成员，可经 Clash API 手动切换
		return map[string]any{
			"type": "selector", "tag": g.Name, "outbounds": g.Members, "default": g.Members[0],
		}, nil
	case GroupModeAuto, GroupModeFailover:
		ob := map[string]any{"type": "urltest", "tag": g.Name, "outbounds": g.Members}
		if g.Mode == GroupModeFailover {
			// 故障转移近似：高容差使 urltest 倾向保持在成员列表靠前的节点
			ob["tolerance"] = 500
		}
		return ob, nil
	default:
		return nil, fmt.Errorf("策略组 %s 模式不支持: %s", g.Name, g.Mode)
	}
}

// splitRuleValue 拆分逗号分隔的规则值并去空白。
func splitRuleValue(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			items = append(items, v)
		}
	}
	return items
}
