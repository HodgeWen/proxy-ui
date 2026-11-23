package subscription

import (
	"fmt"

	"proxy-ui/internal/singbox"
)

// FallbackGroup 无可用策略组时合成的默认组名。
const FallbackGroup = "proxy"

// directTag 直连出站 tag（与内核内置出站同名）。
const directTag = "direct"

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
	// 节点 tag 是用户名，策略组是同一 tag 命名空间里的出站：重名会让客户端报
	// duplicate outbound tag 而拒绝加载整份配置。重名的组改个不冲突的名字，
	// 规则与兜底出站里的引用用同一份映射跟着改（否则会静默指向另一个组）。
	groups, renamed := renameCollidingGroups(groups, nodeSet)
	activeGroups := make(map[string]bool, len(groups))
	for _, g := range groups {
		activeGroups[g.Name] = true
	}
	// mapMembers 把服务端的成员列表映射为客户端可用成员：订阅节点、direct 与仍存在的策略组
	// 原样保留；服务端专属的自定义出站在客户端并不存在，必须剔除——否则该组在客户端没有实际
	// 代理出口。成员按原顺序保留，所以「固定代理默认选首个成员」的语义不变。
	//
	// 组内已无任何代理出口（只剩 direct，或成员全被剔除）时用该订阅用户的全部节点填充：
	//   - 固定代理（selector）把 direct 留在末尾，客户端里仍可手动切到直连；
	//   - 自动选择 / 故障转移（urltest）不补 direct，否则延迟近 0 的 DIRECT 必然夺走整组。
	//
	// allowGroup 决定某个策略组引用是否保留（自引用与已丢弃的组由调用方排除）。
	mapMembers := func(g singbox.GroupEntry, allowGroup func(name string) bool) []string {
		members := make([]string, 0, len(g.Members))
		seen := make(map[string]bool, len(g.Members))
		hasProxyExit := false
		explicitDirect := false
		for _, m := range g.Members {
			if seen[m] {
				continue // 内核拒绝同组内重复 tag
			}
			switch {
			case nodeSet[m]:
				hasProxyExit = true
			case m == directTag:
				explicitDirect = true
			case allowGroup(m):
				hasProxyExit = true
			default:
				continue
			}
			seen[m] = true
			members = append(members, m)
		}
		if hasProxyExit || len(nodeTags) == 0 {
			// 自动选择 / 故障转移组不能混入 direct：DIRECT 延迟近 0，urltest 必然选中它，
			// 整组等于变成直连（固定代理是手动选择器，保留 direct 才有意义）。
			if g.Mode != singbox.GroupModeFixed {
				return removeDirect(members)
			}
			return members
		}
		if explicitDirect && g.Mode == singbox.GroupModeFixed {
			return append(append(make([]string, 0, len(nodeTags)+1), nodeTags...), directTag)
		}
		return nodeTags
	}

	// 策略组之间可以互相引用，但成环（A→B→A）会让客户端拒绝加载整份配置。先剔除闭合环的那条
	// 引用（保留其余引用与声明顺序），再由下面的收敛循环处理因此变空的组。
	candidates := singbox.BreakGroupCycles(groups)
	// 多轮收敛：组内成员清理后若整组为空（无节点、无 direct、无可用子组）则丢弃该组，
	// 并继续清理其它组对该组的引用。
	for {
		kept := make([]singbox.GroupEntry, 0, len(candidates))
		nextActive := make(map[string]bool, len(candidates))
		dropped := false
		for _, g := range candidates {
			members := mapMembers(g, func(name string) bool {
				return activeGroups[name] && name != g.Name
			})
			if len(members) == 0 {
				dropped = true
				continue
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

	// 无可用策略组时合成包含全部节点的兜底组：客户端仍需一个节点选择入口
	if len(d.Groups) == 0 && len(nodeTags) > 0 {
		name := FallbackGroup
		for nodeSet[name] {
			name += "-group"
		}
		d.Groups = append(d.Groups, singbox.GroupEntry{
			Name: name, Mode: singbox.GroupModeFixed, Members: nodeTags,
		})
		activeGroups[name] = true
	}
	proxyGroup := directTag
	if len(d.Groups) > 0 {
		proxyGroup = d.Groups[0].Name
	}

	// 指向服务端专属出站的规则在客户端没有对应节点，原样保留会写出悬空 tag，
	// 统一回落到代理组以保持「这条规则本来就不走直连」的意图。
	mapOutbound := func(tag string) string {
		if to, ok := renamed[tag]; ok {
			return to
		}
		switch {
		case tag == singbox.OutboundBlock, tag == directTag, activeGroups[tag]:
			return tag
		default:
			return proxyGroup
		}
	}
	for _, r := range rules {
		mapped := r
		mapped.Outbound = mapOutbound(r.Outbound)
		d.Rules = append(d.Rules, mapped)
	}
	// 兜底出站：拦截动作没有同名出站，不能作为 route.final / MATCH，按代理组处理
	if final == "" || final == singbox.OutboundBlock {
		d.Final = proxyGroup
	} else {
		d.Final = mapOutbound(final)
	}
	return d, nil
}

// renameCollidingGroups 给与节点 tag 重名的策略组追加后缀（并同步更新其它组成员里的引用），
// 保证客户端配置里每个出站 tag 唯一；返回改名后的组与「原名→新名」映射
// （规则与兜底出站里的引用要用同一份映射改写，否则会指向另一个组）。
func renameCollidingGroups(groups []singbox.GroupEntry, nodeSet map[string]bool) ([]singbox.GroupEntry, map[string]string) {
	// taken 先登记全部既有 tag（节点与所有组名），避免改名后的新名字又撞上另一个组
	taken := make(map[string]bool, len(groups)+len(nodeSet))
	for t := range nodeSet {
		taken[t] = true
	}
	for _, g := range groups {
		taken[g.Name] = true
	}
	renamed := make(map[string]string)
	out := make([]singbox.GroupEntry, len(groups))
	for i, g := range groups {
		name := g.Name
		if nodeSet[name] {
			candidate := name + "-group"
			for taken[candidate] {
				candidate += "-2"
			}
			taken[candidate] = true
			renamed[name] = candidate
			name = candidate
		}
		out[i] = singbox.GroupEntry{Name: name, Mode: g.Mode, Members: append([]string(nil), g.Members...)}
	}
	if len(renamed) == 0 {
		return out, renamed
	}
	for i := range out {
		for j, m := range out[i].Members {
			if to, ok := renamed[m]; ok {
				out[i].Members[j] = to
			}
		}
	}
	return out, renamed
}

// removeDirect 剔除成员列表里的 direct（保持其余顺序）。
func removeDirect(members []string) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		if m != directTag {
			out = append(out, m)
		}
	}
	return out
}

// primaryProxyGroup 返回订阅里作为默认代理出口的策略组名（供客户端规则集下载与远程 DNS 绕过防火墙）；
// 优先取非 direct/block 的 Final，其次取首个策略组；无可用策略组时返回空串。
func primaryProxyGroup(d Data) string {
	if d.Final != "" && d.Final != "direct" && d.Final != singbox.OutboundBlock {
		return d.Final
	}
	if len(d.Groups) > 0 {
		return d.Groups[0].Name
	}
	return ""
}
