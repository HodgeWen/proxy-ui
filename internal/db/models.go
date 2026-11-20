package db

import "time"

// Admin 管理员账号（单管理员，由首次设置向导创建）。
type Admin struct {
	ID           uint      `gorm:"primaryKey" json:"id"`                // 主键
	Username     string    `gorm:"uniqueIndex;size:64" json:"username"` // 用户名
	PasswordHash string    `gorm:"size:128" json:"-"`                   // 密码哈希（bcrypt），不回显
	CreatedAt    time.Time `json:"createdAt"`                           // 创建时间
	UpdatedAt    time.Time `json:"updatedAt"`                           // 更新时间
}

// Setting 面板设置键值对（复杂值存 JSON 字符串）。
type Setting struct {
	Key   string `gorm:"primaryKey;size:64" json:"key"` // 设置键
	Value string `gorm:"type:text" json:"value"`        // 设置值
}

// 面板设置键（跨 api / subscription 读写，键名以此为准）。
const (
	SettingRouteFinal = "route_final" // 未命中规则的兜底出站 tag，空用内核默认
	SettingDNS        = "dns_config"  // DNS 配置（JSON，结构见 singbox.DNSConfig）
)

// Inbound 入站：内核里真实的监听（协议 + 监听地址/端口 + TLS/Reality/混淆等连接层参数）。
// 同一入站下可挂多个用户（InboundUser），各自凭据、共享该端口。
type Inbound struct {
	ID         uint      `gorm:"primaryKey" json:"id"`            // 主键
	Name       string    `gorm:"uniqueIndex;size:64" json:"name"` // 名称（即内核入站 tag）
	Protocol   string    `gorm:"size:32;index" json:"protocol"`   // 协议：vless/hysteria2/tuic/anytls/trojan/vmess/shadowsocks
	Listen     string    `gorm:"size:64" json:"listen"`           // 监听地址：:: 双栈 / 0.0.0.0 仅 IPv4 / 127.0.0.1 走反代
	ListenPort int       `gorm:"uniqueIndex" json:"listenPort"`   // 监听端口（默认随机空闲端口，不默认 443）
	Enabled    bool      `gorm:"default:true" json:"enabled"`     // 是否启用
	Config     string    `gorm:"type:text" json:"config"`         // 连接层配置（JSON：TLS/Reality/混淆/带宽/算法，不含用户凭据）
	CreatedAt  time.Time `json:"createdAt"`                       // 创建时间
	UpdatedAt  time.Time `json:"updatedAt"`                       // 更新时间
}

// InboundUser 入站内的用户：持有该入站上的独立凭据；Name 同时是内核统计里的 user 名。
type InboundUser struct {
	ID         uint      `gorm:"primaryKey" json:"id"`                                     // 主键
	InboundID  uint      `gorm:"index;uniqueIndex:idx_inbound_user_name" json:"inboundId"` // 所属入站
	Name       string    `gorm:"size:64;uniqueIndex:idx_inbound_user_name" json:"name"`    // 用户名（内核统计 user 名、客户端节点名）
	Credential string    `gorm:"type:text" json:"credential"`                              // 凭据（JSON：uuid/password/flow 等，按协议取用）
	Disabled   bool      `json:"disabled"`                                                 // 管理员手动停用（配额协调不会覆盖）
	Suspended  bool      `json:"suspended"`                                                // 被系统暂停（配额用尽/到期/订阅停用）
	Enabled    bool      `gorm:"default:true" json:"enabled"`                              // 生效状态 = !Disabled && !Suspended
	CreatedAt  time.Time `json:"createdAt"`                                                // 创建时间
	UpdatedAt  time.Time `json:"updatedAt"`                                                // 更新时间
}

// OutboundTypeDirect direct 出站类型常量。
const OutboundTypeDirect = "direct"

// Outbound 出站配置；direct 之外的自定义出站参数存 Config JSON。
type Outbound struct {
	ID        uint      `gorm:"primaryKey" json:"id"`            // 主键
	Name      string    `gorm:"uniqueIndex;size:64" json:"name"` // 出站 tag 名称
	Type      string    `gorm:"size:32" json:"type"`             // 类型：direct 或自定义出站协议
	Enabled   bool      `gorm:"default:true" json:"enabled"`     // 是否启用
	Config    string    `gorm:"type:text" json:"config"`         // 协议特定配置（JSON）
	CreatedAt time.Time `json:"createdAt"`                       // 创建时间
	UpdatedAt time.Time `json:"updatedAt"`                       // 更新时间
}

// Certificate TLS 证书：managed 模式证书/私钥由面板存储（落数据目录 certs/），path 模式引用服务器本地路径。
type Certificate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`            // 主键
	Name      string    `gorm:"uniqueIndex;size:64" json:"name"` // 名称备注
	Mode      string    `gorm:"size:16" json:"mode"`             // 引用方式：managed 上传/粘贴存储 / path 服务器本地路径
	CertPath  string    `gorm:"size:256" json:"certPath"`        // managed：数据目录 certs/ 内相对路径；path：证书绝对路径
	KeyPath   string    `gorm:"size:256" json:"keyPath"`         // managed：私钥相对路径；path：私钥绝对路径
	CreatedAt time.Time `json:"createdAt"`                       // 创建时间
	UpdatedAt time.Time `json:"updatedAt"`                       // 更新时间
}

// PolicyGroup 策略组（生成 sing-box 配置时映射为 selector/urltest 出站）。
type PolicyGroup struct {
	ID        uint      `gorm:"primaryKey" json:"id"`            // 主键
	Name      string    `gorm:"uniqueIndex;size:64" json:"name"` // 策略组名称
	Mode      string    `gorm:"size:16" json:"mode"`             // 模式：failover 故障转移 / fixed 固定代理 / auto 自动选择
	Members   string    `gorm:"type:text" json:"members"`        // 成员出站 tag 列表（JSON 数组，按顺序生效）
	Enabled   bool      `gorm:"default:true" json:"enabled"`     // 是否启用
	CreatedAt time.Time `json:"createdAt"`                       // 创建时间
	UpdatedAt time.Time `json:"updatedAt"`                       // 更新时间
}

// 规则匹配对象。
const (
	RuleMatchDomain  = "domain"   // 域名：按精确 / 后缀 / 关键字 / 正则匹配
	RuleMatchIP      = "ip"       // IP：按 CIDR 网段匹配
	RuleMatchRuleSet = "rule_set" // 规则集：引用规则集中启用中的内容项
)

// 规则匹配方式（rule_set 无需匹配方式）。
const (
	RuleMethodExact   = "exact"   // 精确
	RuleMethodSuffix  = "suffix"  // 后缀
	RuleMethodKeyword = "keyword" // 关键字
	RuleMethodRegex   = "regex"   // 正则
	RuleMethodCIDR    = "cidr"    // CIDR 网段
)

// Rule 全局路由规则：匹配对象（域名 / IP / 规则集）× 匹配方式。
type Rule struct {
	ID          uint      `gorm:"primaryKey" json:"id"`           // 主键
	MatchType   string    `gorm:"size:16;index" json:"matchType"` // 匹配对象：domain / ip / rule_set
	MatchMethod string    `gorm:"size:16" json:"matchMethod"`     // 匹配方式：exact/suffix/keyword/regex/cidr（rule_set 留空）
	Value       string    `gorm:"size:512" json:"value"`          // 匹配值（英文逗号分隔多值；rule_set 留空）
	RuleSetID   uint      `gorm:"index" json:"ruleSetId"`         // rule_set 类型引用的规则集
	Outbound    string    `gorm:"size:64" json:"outbound"`        // 动作出站 tag：direct、block、策略组或自定义出站
	Order       int       `json:"order"`                          // 排序，值小者靠前
	Enabled     bool      `gorm:"default:true" json:"enabled"`    // 是否启用
	CreatedAt   time.Time `json:"createdAt"`                      // 创建时间
	UpdatedAt   time.Time `json:"updatedAt"`                      // 更新时间
}

// RuleSet 规则集：一个 tag + 多条内容项；规则引用它，各客户端按内容项里对应内核的源地址下载。
type RuleSet struct {
	ID          uint      `gorm:"primaryKey" json:"id"`            // 主键
	Name        string    `gorm:"uniqueIndex;size:64" json:"name"` // 规则集 tag（内核与客户端引用名）
	Description string    `gorm:"size:256" json:"description"`     // 说明
	Builtin     bool      `json:"builtin"`                         // 是否内置（内置项不可删除，可换源）
	CreatedAt   time.Time `json:"createdAt"`                       // 创建时间
	UpdatedAt   time.Time `json:"updatedAt"`                       // 更新时间
}

// RuleSetItem 规则集内容项：一条内容 = 备注 + 各内核的规则源地址；同一规则集同时只启用一条。
type RuleSetItem struct {
	ID             uint      `gorm:"primaryKey" json:"id"`          // 主键
	RuleSetID      uint      `gorm:"index" json:"ruleSetId"`        // 所属规则集
	Remark         string    `gorm:"size:128" json:"remark"`        // 备注（来源说明）
	SingBoxURL     string    `gorm:"size:512" json:"singBoxUrl"`    // sing-box 规则源（.srs）
	MihomoURL      string    `gorm:"size:512" json:"mihomoUrl"`     // mihomo 规则源（.mrs / .yaml）
	MihomoBehavior string    `gorm:"size:16" json:"mihomoBehavior"` // mihomo 行为：domain / ipcidr / classical
	Enabled        bool      `json:"enabled"`                       // 是否启用（每规则集最多一条）
	CreatedAt      time.Time `json:"createdAt"`                     // 创建时间
	UpdatedAt      time.Time `json:"updatedAt"`                     // 更新时间
}

// SubscriptionToken 订阅用户：匿名令牌 + 流量配额与到期时间，供客户端免登录拉取订阅。
type SubscriptionToken struct {
	ID                uint       `gorm:"primaryKey" json:"id"`             // 主键
	Token             string     `gorm:"uniqueIndex;size:64" json:"token"` // 匿名令牌（随机生成）
	Name              string     `gorm:"size:64" json:"name"`              // 备注（同时作为该用户在各入站的用户名前缀）
	Revoked           bool       `json:"revoked"`                          // 是否手动停用
	TrafficLimitBytes int64      `json:"trafficLimitBytes"`                // 流量配额（上+下行字节），0 表示不限
	ExpiresAt         *time.Time `json:"expiresAt"`                        // 到期时间，空表示永不过期
	CreatedAt         time.Time  `json:"createdAt"`                        // 创建时间
	LastUsedAt        *time.Time `json:"lastUsedAt"`                       // 最近一次使用时间
}

// SubscriptionNode 订阅用户在某入站上的身份：把 token 与该入站里的一个用户绑定，
// 一条记录即该订阅用户的一个节点（客户端可见）。
type SubscriptionNode struct {
	ID            uint      `gorm:"primaryKey" json:"id"`                                 // 主键
	TokenID       uint      `gorm:"index;uniqueIndex:idx_token_inbound" json:"tokenId"`   // 所属订阅用户
	InboundID     uint      `gorm:"index;uniqueIndex:idx_token_inbound" json:"inboundId"` // 所属入站
	InboundUserID uint      `gorm:"index" json:"inboundUserId"`                           // 该入站内的用户记录
	CreatedAt     time.Time `json:"createdAt"`                                            // 创建时间
}

// UserTraffic 按用户累计流量（键为内核计数器 tag：用户名），用于列表里展示每个节点用了多少。
//
// 仅作展示：用户被删除（取消勾选入站）时该行随之消失，不影响配额。
// 配额与到期的权威计数在 TokenTraffic（按订阅用户累计），见下。
type UserTraffic struct {
	Tag           string    `gorm:"primaryKey;size:128" json:"tag"` // 用户名（内核计数器 tag）
	TokenID       uint      `gorm:"index" json:"tokenId"`           // 归属订阅用户（0 表示无订阅的手动用户）
	UploadBytes   int64     `json:"uploadBytes"`                    // 上行累计字节数
	DownloadBytes int64     `json:"downloadBytes"`                  // 下行累计字节数
	UpdatedAt     time.Time `json:"updatedAt"`                      // 更新时间
}

// TokenTraffic 订阅用户累计流量（键为订阅用户 ID），配额与到期的唯一依据。
//
// 之所以不直接按用户名汇总：用户的入站身份会因取消/重新勾选而重建（用户名、行都会变），
// 若配额跟着用户名走，一次误操作或一次删除就会把已用流量清零或串到别人头上。
type TokenTraffic struct {
	TokenID       uint      `gorm:"primaryKey" json:"tokenId"` // 订阅用户 ID
	UploadBytes   int64     `json:"uploadBytes"`               // 上行累计字节数
	DownloadBytes int64     `json:"downloadBytes"`             // 下行累计字节数
	UpdatedAt     time.Time `json:"updatedAt"`                 // 更新时间
}

// AIProvider AI 供应商配置（OpenAI 兼容接口），一个供应商下挂多个模型（AIModel）。
type AIProvider struct {
	ID        uint      `gorm:"primaryKey" json:"id"`        // 主键
	Name      string    `gorm:"size:64" json:"name"`         // 供应商名称（UI 显示名）
	BaseURL   string    `gorm:"size:256" json:"baseUrl"`     // OpenAI 兼容接口地址（base-url）
	APIKey    string    `gorm:"size:256" json:"-"`           // API Key，不回显
	Enabled   bool      `gorm:"default:true" json:"enabled"` // 是否启用（停用后不进对话路由）
	Sort      int       `json:"sort"`                        // 排序，值小者靠前
	CreatedAt time.Time `json:"createdAt"`                   // 创建时间
	UpdatedAt time.Time `json:"updatedAt"`                   // 更新时间
}

// AIModel 供应商下的模型：ModelID 即发往上游的模型名，跨供应商全局唯一
// （前端 @veltra/ai 的 ChatModel 要求模型 id 全局唯一，对话代理据此路由）。
type AIModel struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`                 // 主键
	ProviderID            uint      `gorm:"index" json:"providerId"`              // 所属供应商
	ModelID               string    `gorm:"uniqueIndex;size:128" json:"modelId"`  // 模型 id（全局唯一，即上游模型名）
	Label                 string    `gorm:"size:128" json:"label"`                // 显示名，空时前端取 id
	Description           string    `gorm:"size:256" json:"description"`          // 一句话描述
	ReasoningLevels       string    `gorm:"type:text" json:"reasoningLevels"`     // 推理等级列表（JSON 数组 [{value,label}]）
	DefaultReasoningLevel string    `gorm:"size:32" json:"defaultReasoningLevel"` // 默认推理等级（须落在列表内，空为不指定）
	ContextWindow         int       `json:"contextWindow"`                        // 上下文窗口（token），0 为未知
	Enabled               bool      `gorm:"default:true" json:"enabled"`          // 是否启用
	CreatedAt             time.Time `json:"createdAt"`                            // 创建时间
	UpdatedAt             time.Time `json:"updatedAt"`                            // 更新时间
}
