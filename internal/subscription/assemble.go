package subscription

import (
	"fmt"

	"proxy-ui/internal/singbox"
)

// FallbackGroup 无可用策略组时合成的默认组名。
const FallbackGroup = "proxy"

// Node 订阅节点：由启用入站映射出的客户端连接信息（Direct 入站无凭据，不产生节点）。
type Node struct {
	Tag      string // 节点 tag（入站名）
	Protocol string // 入站协议
	Port     int    // 服务器端口（入站监听端口）
	Config   any    // singbox.ParseInboundConfig 解析结果
}

// Data 组装后的订阅数据：节点与已映射为客户端可用 tag 的策略组/规则。
type Data struct {
	Server string               // 节点服务器地址（面板设置或请求 Host）
	Nodes  []Node               // 启用入站对应节点
	Groups []singbox.GroupEntry // 策略组（成员映射为节点 tag/组名/direct）
	Rules  []singbox.RuleEntry  // 路由规则（出站映射为 direct/block/组名）
	Final  string               // 兜底出站 tag（已映射，非空）
	AntiAD string               // anti-AD 规则源地址，空用官方默认源
}

// InboundSpec 订阅组装输入的入站条目（Config 为面板存储的协议配置 JSON 原文）。
type InboundSpec struct {
	Tag      string // 入站名
	Protocol string // 入站协议
	Port     int    // 监听端口
	Config   string // 协议配置 JSON
}

// Assemble 组装订阅数据：启用入站解析为节点，策略组成员与规则/兜底出站映射为客户端
// 可用 tag——入站名保留为节点 tag，direct 与其余策略组名保留，服务端自定义出站在
// 客户端不存在（成员剔除、规则改走兜底组）；无可用策略组时以全部节点合成默认组。
func Assemble(server string, inbounds []InboundSpec, groups []singbox.GroupEntry, rules []singbox.RuleEntry, final, antiAD string) (Data, error) {
	d := Data{Server: server, AntiAD: antiAD}
	nodeTags := make([]string, 0, len(inbounds))
	for _, in := range inbounds {
		if in.Protocol == singbox.ProtoDirect {
			continue // Direct 入站为明文直连监听，无客户端凭据可导出
		}
		cfg, err := singbox.ParseInboundConfig(in.Protocol, in.Config)
		if err != nil {
			return Data{}, fmt.Errorf("入站 %s: %w", in.Tag, err)
		}
		d.Nodes = append(d.Nodes, Node{Tag: in.Tag, Protocol: in.Protocol, Port: in.Port, Config: cfg})
		nodeTags = append(nodeTags, in.Tag)
	}

	// 策略组成员映射；映射后无成员的组回退为全部节点（面板模板组即此形态）
	nodeSet := make(map[string]bool, len(nodeTags))
	for _, t := range nodeTags {
		nodeSet[t] = true
	}
	groupNames := make(map[string]bool, len(groups))
	for _, g := range groups {
		groupNames[g.Name] = true
	}
	for _, g := range groups {
		members := make([]string, 0, len(g.Members))
		for _, m := range g.Members {
			if nodeSet[m] || m == "direct" || (groupNames[m] && m != g.Name) {
				members = append(members, m)
			}
		}
		if len(members) == 0 {
			if len(nodeTags) == 0 {
				continue // 无任何节点时组无从取成员，整组剔除
			}
			members = nodeTags
		}
		d.Groups = append(d.Groups, singbox.GroupEntry{Name: g.Name, Mode: g.Mode, Members: members})
	}
	fallback := ""
	if len(d.Groups) > 0 {
		fallback = d.Groups[0].Name
	} else if len(nodeTags) > 0 {
		// 无策略组：以全部节点合成固定代理默认组，保证规则与兜底可引用
		d.Groups = append(d.Groups, singbox.GroupEntry{
			Name: FallbackGroup, Mode: singbox.GroupModeFixed, Members: nodeTags,
		})
		fallback = FallbackGroup
	}
	if fallback == "" {
		fallback = "direct" // 无节点亦无组：规则与兜底退化为直连
	}

	// 规则与兜底出站映射：block/direct/组名保留，其余（服务端自定义出站）改走兜底组
	mapOutbound := func(tag string) string {
		switch {
		case tag == singbox.OutboundBlock, tag == "direct", groupNames[tag]:
			return tag
		default:
			return fallback
		}
	}
	for _, r := range rules {
		d.Rules = append(d.Rules, singbox.RuleEntry{Type: r.Type, Value: r.Value, Outbound: mapOutbound(r.Outbound)})
	}
	if final != "" {
		d.Final = mapOutbound(final)
	} else {
		d.Final = fallback
	}
	return d, nil
}
