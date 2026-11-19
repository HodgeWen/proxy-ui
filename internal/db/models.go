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

// Inbound 入站配置；协议特定参数（密码/UUID/Reality/TLS/证书引用等）统一存 Config JSON。
type Inbound struct {
	ID         uint      `gorm:"primaryKey" json:"id"`          // 主键
	Name       string    `gorm:"size:64" json:"name"`           // 名称
	Protocol   string    `gorm:"size:32;index" json:"protocol"` // 协议：vless/hysteria2/tuic/anytls/trojan/vmess/shadowsocks/direct
	ListenPort int       `gorm:"uniqueIndex" json:"listenPort"` // 监听端口
	Enabled    bool      `gorm:"default:true" json:"enabled"`   // 是否启用
	Config     string    `gorm:"type:text" json:"config"`       // 协议特定配置（JSON）
	CreatedAt  time.Time `json:"createdAt"`                     // 创建时间
	UpdatedAt  time.Time `json:"updatedAt"`                     // 更新时间
}

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

// Rule 全局路由规则。
type Rule struct {
	ID        uint      `gorm:"primaryKey" json:"id"`        // 主键
	Type      string    `gorm:"size:32" json:"type"`         // 类型：domain/domain_suffix/domain_keyword/ip_cidr/geosite/geoip 等
	Value     string    `gorm:"size:256" json:"value"`       // 匹配值
	Outbound  string    `gorm:"size:64" json:"outbound"`     // 动作出站 tag：direct、block、策略组或自定义出站
	Order     int       `json:"order"`                       // 排序，值小者靠前
	Enabled   bool      `gorm:"default:true" json:"enabled"` // 是否启用
	CreatedAt time.Time `json:"createdAt"`                   // 创建时间
	UpdatedAt time.Time `json:"updatedAt"`                   // 更新时间
}

// SubscriptionToken 订阅匿名令牌，供客户端免登录拉取订阅。
type SubscriptionToken struct {
	ID         uint       `gorm:"primaryKey" json:"id"`             // 主键
	Token      string     `gorm:"uniqueIndex;size:64" json:"token"` // 匿名令牌（随机生成）
	Name       string     `gorm:"size:64" json:"name"`              // 备注
	Revoked    bool       `json:"revoked"`                          // 是否已吊销
	CreatedAt  time.Time  `json:"createdAt"`                        // 创建时间
	LastUsedAt *time.Time `json:"lastUsedAt"`                       // 最近一次使用时间
}

// InboundTraffic 按入站累计流量（面板重启后不丢失）。
type InboundTraffic struct {
	InboundID     uint      `gorm:"primaryKey" json:"inboundId"` // 入站 ID
	UploadBytes   int64     `json:"uploadBytes"`                 // 上行累计字节数
	DownloadBytes int64     `json:"downloadBytes"`               // 下行累计字节数
	UpdatedAt     time.Time `json:"updatedAt"`                   // 更新时间
}

// AIProvider AI 供应商配置（OpenAI 兼容接口）。
type AIProvider struct {
	ID              uint      `gorm:"primaryKey" json:"id"`           // 主键
	BaseURL         string    `gorm:"size:256" json:"baseUrl"`        // 接口地址（base-url）
	APIKey          string    `gorm:"size:256" json:"-"`              // API Key，不回显
	Model           string    `gorm:"size:64" json:"model"`           // 模型名
	ReasoningEffort string    `gorm:"size:16" json:"reasoningEffort"` // 推理强度：low/medium/high
	UpdatedAt       time.Time `json:"updatedAt"`                      // 更新时间
}
