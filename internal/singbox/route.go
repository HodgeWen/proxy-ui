package singbox

import (
	"fmt"
	"strings"

	"proxy-ui/internal/db"
)

// 规则匹配对象（与 db 常量一一口径，配置生成按对象决定规则字段）。
const (
	MatchDomain  = db.RuleMatchDomain  // 域名
	MatchIP      = db.RuleMatchIP      // IP
	MatchRuleSet = db.RuleMatchRuleSet // 规则集
)

// 规则匹配方式。
const (
	MethodExact   = db.RuleMethodExact   // 精确
	MethodSuffix  = db.RuleMethodSuffix  // 后缀
	MethodKeyword = db.RuleMethodKeyword // 关键字
	MethodRegex   = db.RuleMethodRegex   // 正则
	MethodCIDR    = db.RuleMethodCIDR    // 网段
)

// OutboundBlock 拦截动作：生成规则时映射为 reject，不产生出站。
const OutboundBlock = "block"

// 策略组模式。
const (
	GroupModeFailover = "failover" // 故障转移
	GroupModeFixed    = "fixed"    // 固定代理
	GroupModeAuto     = "auto"     // 自动选择
)

// SupportedGroupModes 支持的策略组模式列表。
var SupportedGroupModes = []string{GroupModeFailover, GroupModeFixed, GroupModeAuto}

// 内置规则集 tag 与其 mihomo 侧默认行为。
const (
	AntiADTag        = "anti-ad"                                  // anti-AD 广告拦截
	AntiADBuiltinURL = "https://anti-ad.net/anti-ad-sing-box.srs" // anti-AD sing-box 官方源
	AntiADMihomoURL  = "https://anti-ad.net/clash.yaml"           // anti-AD mihomo 官方源（clash.yaml，domain 行为）
	GeoCNTag         = "geosite-cn"                               // 国内域名
	GeoCNMihomoURL   = "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.mrs"
	GeoIPCNTag       = "geoip-cn" // 国内 IP
	GeoIPCNMihomoURL = "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/cn.mrs"
	geositeURLFormat = "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-%s.srs"
	geoipURLFormat   = "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-%s.srs"
)

// 内置规则集定义：名称、说明与 sing-box 源地址（mihomo 源地址见 BuiltinMihomoSource）。
var BuiltinRuleSets = []struct {
	Name        string
	Description string
	SingBoxURL  string
}{
	{AntiADTag, "anti-AD 广告拦截（sing-box 官方 .srs / mihomo 官方 clash.yaml）", AntiADBuiltinURL},
	{GeoCNTag, "国内域名（sing-geosite / mihomo geosite 规则）", fmt.Sprintf(geositeURLFormat, "cn")},
	{GeoIPCNTag, "国内 IP 段（sing-geoip / mihomo geoip 规则）", fmt.Sprintf(geoipURLFormat, "cn")},
}

// BuiltinMihomoSource 返回内置规则集的 mihomo 源地址与行为；非内置 tag 返回 ok=false。
func BuiltinMihomoSource(tag string) (url, behavior string, ok bool) {
	switch tag {
	case AntiADTag:
		return AntiADMihomoURL, "domain", true
	case GeoCNTag:
		return GeoCNMihomoURL, "domain", true
	case GeoIPCNTag:
		return GeoIPCNMihomoURL, "ipcidr", true
	}
	return "", "", false
}

// RuleSetSource 规则集的 sing-box 下载定义（mihomo 侧走 MihomoRuleProvider）。
type RuleSetSource struct {
	Tag            string // 内核规则集 tag
	URL            string // sing-box .srs 地址
	MihomoURL      string // mihomo 源地址（.mrs / .yaml）
	MihomoBehavior string // mihomo 行为：domain / ipcidr / classical
}

// RuleEntry 路由规则条目：匹配对象与方式已解析为内核字段，规则集引用携带下载源。
type RuleEntry struct {
	MatchType   string         // 匹配对象：domain / ip / rule_set
	MatchMethod string         // 匹配方式（rule_set 为空）
	Value       string         // 匹配值（多值英文逗号分隔；rule_set 为空）
	RuleSet     *RuleSetSource // rule_set 类型的下载源（启用中的内容项）
	Outbound    string         // 动作出站 tag：direct、block、策略组或自定义出站
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
}

// RoutingRuleJSON 把规则条目映射为 sing-box route rule JSON 对象，并返回引用的规则集
// 下载定义（nil 表示该规则不引用规则集；订阅客户端渲染复用）。
func RoutingRuleJSON(r RuleEntry) (map[string]any, *RuleSetConfig, error) {
	rule := map[string]any{}
	var refSet *RuleSetConfig
	switch r.MatchType {
	case MatchRuleSet:
		if r.RuleSet == nil {
			return nil, nil, fmt.Errorf("规则集引用缺失（规则集可能未配置启用中的内容项）")
		}
		rule["rule_set"] = []string{r.RuleSet.Tag}
		refSet = &RuleSetConfig{Type: "remote", Tag: r.RuleSet.Tag, Format: "binary", URL: r.RuleSet.URL}
	default:
		items := splitRuleValue(r.Value)
		if len(items) == 0 {
			return nil, nil, fmt.Errorf("规则的匹配值不能为空")
		}
		field, err := matchField(r.MatchType, r.MatchMethod)
		if err != nil {
			return nil, nil, err
		}
		rule[field] = items
	}
	if r.Outbound == OutboundBlock {
		rule["action"] = "reject"
	} else {
		rule["outbound"] = r.Outbound
	}
	return rule, refSet, nil
}

// matchField 把匹配对象 + 方式映射为 sing-box 规则字段名。
func matchField(matchType, method string) (string, error) {
	switch matchType {
	case MatchDomain:
		switch method {
		case MethodExact:
			return "domain", nil
		case MethodSuffix:
			return "domain_suffix", nil
		case MethodKeyword:
			return "domain_keyword", nil
		case MethodRegex:
			return "domain_regex", nil
		}
		return "", fmt.Errorf("域名规则的匹配方式不支持: %s", method)
	case MatchIP:
		if method != MethodCIDR {
			return "", fmt.Errorf("IP 规则只支持网段匹配，当前为: %s", method)
		}
		return "ip_cidr", nil
	}
	return "", fmt.Errorf("不支持的匹配对象: %s", matchType)
}

// GroupOutboundJSON 把策略组映射为 sing-box selector/urltest 出站 JSON 对象
// （订阅客户端渲染复用）。
func GroupOutboundJSON(g GroupEntry) (map[string]any, error) {
	if len(g.Members) == 0 {
		return nil, fmt.Errorf("策略组 %s 未配置成员", g.Name)
	}
	switch g.Mode {
	case GroupModeFixed:
		// 固定代理：selector 默认选中首个成员，客户端里可手动切换
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
