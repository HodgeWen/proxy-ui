package subscription

import (
	"fmt"

	"proxy-ui/internal/singbox"
)

// FallbackGroup 无可用策略组时合成的默认组名。
const FallbackGroup = "proxy"

// Node 订阅节点：该订阅用户在某入站下的身份（共享入站端口）。
type Node struct {
	Tag      string             // 节点 tag（用户名）
	Protocol string             // 入站协议
	Port     int                // 入站端口（同入站下所有用户共享）
	Config   any                // singbox.ParseInboundConfig 解析的连接层配置
	Cred     singbox.Credential // 该用户的凭据
	Insecure bool               // 是否跳过 TLS 证书校验（自签证书场景）
}

// UserInfo 订阅流量与到期信息（用于 Subscription-Userinfo 响应头）。
type UserInfo struct {
	Upload   int64 // 已用上行字节
	Download int64 // 已用下行字节
	Total    int64 // 总配额字节，0 表示不限
	Expire   int64 // 到期 Unix 时间戳（秒），0 表示永不过期
}

// Data 组装后的订阅数据。
type Data struct {
	Title    string               // 订阅名称（用于响应头展示）
	UserInfo *UserInfo            // 流量与到期信息（预览路径可为空）
	Server   string               // 节点服务器地址（请求 Host 去端口）
	Nodes    []Node               // 该订阅用户的节点
	Groups   []singbox.GroupEntry // 策略组
	Rules    []singbox.RuleEntry  // 路由规则
	Final    string               // 兜底出站 tag（已映射，非空）
	DNS      *singbox.DNSConfig   // DNS 配置
}

// NodeSpec 订阅组装输入的节点条目。
type NodeSpec struct {
	Tag      string // 节点 tag（用户名）
	Protocol string // 入站协议
	Port     int    // 入站端口
	Config   string // 连接层配置 JSON
	CredJSON string // 该用户凭据 JSON
	Insecure bool   // 是否跳过 TLS 证书校验（自签证书场景）
}

// Assemble 组装订阅数据：节点解析为客户端节点，策略组成员与规则/兜底出站映射为
// 客户端可用 tag；无可用策略组时以全部节点合成默认组。
func Assemble(server string, nodes []NodeSpec, groups []singbox.GroupEntry, rules []singbox.RuleEntry, final string, dns *singbox.DNSConfig) (Data, error) {
	d := Data{Server: server, DNS: dns}
	nodeTags := make([]string, 0, len(nodes))
	for _, n := range nodes {
		cfg, err := singbox.ParseInboundConfig(n.Protocol, n.Config)
		if err != nil {
			return Data{}, fmt.Errorf("节点 %s: %w", n.Tag, err)
		}
		cred, err := singbox.ParseCredential(n.Protocol, n.CredJSON)
		if err != nil {
			return Data{}, fmt.Errorf("节点 %s: %w", n.Tag, err)
		}
		d.Nodes = append(d.Nodes, Node{
			Tag: n.Tag, Protocol: n.Protocol, Port: n.Port, Config: cfg, Cred: cred, Insecure: n.Insecure,
		})
		nodeTags = append(nodeTags, n.Tag)
	}

	nodeSet := make(map[string]bool, len(nodeTags))
	for _, t := range nodeTags {
		nodeSet[t] = true
	}
	activeGroups := make(map[string]bool, len(groups))
	for _, g := range groups {
		activeGroups[g.Name] = true
	}
	// 多轮收敛：过滤无效成员后若整组为空且无节点则丢弃，并继续清理其它组对该组的引用
	candidates := groups
	for {
		kept := make([]singbox.GroupEntry, 0, len(candidates))
		nextActive := make(map[string]bool, len(candidates))
		dropped := false
		for _, g := range candidates {
			members := make([]string, 0, len(g.Members))
			for _, m := range g.Members {
				if nodeSet[m] || m == "direct" || (activeGroups[m] && m != g.Name) {
					members = append(members, m)
				}
			}
			// 规则模板无自定义出站时在服务端用 ["direct"] 占位（auto/failover 组）；
			// 下发订阅且有可用节点时自动填充订阅节点，避免 proxy 组只剩 DIRECT。
			templatePlaceholder := len(nodeTags) > 0 && g.Mode != singbox.GroupModeFixed && len(members) == 1 && members[0] == "direct"
			if len(members) == 0 || templatePlaceholder {
				if len(nodeTags) == 0 {
					dropped = true
					continue
				}
				members = nodeTags
			}
			kept = append(kept, singbox.GroupEntry{Name: g.Name, Mode: g.Mode, Members: members})
			nextActive[g.Name] = true
		}
		candidates = kept
		activeGroups = nextActive
		if !dropped {
			break
		}
	}
	d.Groups = candidates

	fallback := ""
	if len(d.Groups) > 0 {
		fallback = d.Groups[0].Name
	} else if len(nodeTags) > 0 {
		d.Groups = append(d.Groups, singbox.GroupEntry{
			Name: FallbackGroup, Mode: singbox.GroupModeFixed, Members: nodeTags,
		})
		activeGroups[FallbackGroup] = true
		fallback = FallbackGroup
	}
	if fallback == "" {
		fallback = "direct"
	}

	mapOutbound := func(tag string) string {
		switch {
		case tag == singbox.OutboundBlock, tag == "direct", activeGroups[tag]:
			return tag
		default:
			return fallback
		}
	}
	for _, r := range rules {
		mapped := r
		mapped.Outbound = mapOutbound(r.Outbound)
		d.Rules = append(d.Rules, mapped)
	}
	if final != "" {
		d.Final = mapOutbound(final)
	} else {
		d.Final = fallback
	}
	return d, nil
}
