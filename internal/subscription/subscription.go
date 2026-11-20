// Package subscription 组装订阅数据并渲染 sing-box / mihomo 客户端配置，
// 提供匿名 token 订阅端点。
package subscription

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// ErrInvalidToken 匿名令牌不存在或已停用。
var ErrInvalidToken = errors.New("订阅令牌不存在或已停用")

// ErrUnavailable 订阅已到期或流量用尽。
var ErrUnavailable = errors.New("订阅已到期或流量已用尽")

// ErrNoNodes 该订阅没有任何可用节点（未勾选入站、入站被停用或用户被暂停）。
var ErrNoNodes = errors.New("订阅下没有可用节点")

// Format 订阅输出格式。
type Format string

// 支持的订阅输出格式。
const (
	FormatSingBox Format = "sing-box" // sing-box 客户端配置（默认）
	FormatClash   Format = "clash"    // clash(mihomo) 客户端配置
)

// DetectFormat 选择订阅输出格式：查询参数 format 优先于 UA；
// UA 含 clash/mihomo/stash 判为 clash，含 sing-box/singbox/SFI/SFM 判为 sing-box。
func DetectFormat(param, ua string) Format {
	switch strings.ToLower(strings.TrimSpace(param)) {
	case "clash", "mihomo", "clashmeta", "clash.meta":
		return FormatClash
	case "singbox", "sing-box", "sing":
		return FormatSingBox
	}
	lower := strings.ToLower(ua)
	if strings.Contains(lower, "clash") || strings.Contains(lower, "mihomo") || strings.Contains(lower, "stash") {
		return FormatClash
	}
	for _, kw := range []string{"sing-box", "singbox", "sfi", "sfm"} {
		if strings.Contains(lower, kw) {
			return FormatSingBox
		}
	}
	return FormatSingBox
}

// Service 订阅服务：校验匿名令牌、组装订阅数据并按格式渲染下发。
type Service struct {
	db *gorm.DB
}

// New 构造订阅服务。
func New(database *gorm.DB) *Service { return &Service{db: database} }

// Handle GET /sub/:token 匿名订阅端点（无会话）：按 format 参数或 UA 自适应格式。
func (s *Service) Handle(c *gin.Context) {
	data, err := s.Fetch(c.Param("token"), c.Request.Host)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnavailable):
			c.String(http.StatusForbidden, "订阅已到期或流量已用尽，请联系管理员")
		case errors.Is(err, ErrNoNodes):
			c.String(http.StatusForbidden, "订阅下没有可用节点，请联系管理员检查入站与配额")
		default:
			c.String(http.StatusNotFound, "订阅不存在或已失效")
		}
		return
	}
	format := DetectFormat(c.Query("format"), c.Request.UserAgent())
	content, err := Render(data, format)
	if err != nil {
		c.String(http.StatusInternalServerError, "生成订阅失败: "+err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	if format == FormatClash {
		c.Header("Content-Disposition", `attachment; filename="config.yaml"`)
		c.Data(http.StatusOK, "text/yaml; charset=utf-8", content)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="config.json"`)
	c.Data(http.StatusOK, "application/json; charset=utf-8", content)
}

// Fetch 校验匿名令牌并组装订阅数据：节点取该用户在各入站下的用户身份，
// 服务器端口为入站端口（同入站下所有用户共享），服务器地址取请求 Host（反代天然适配）。
func (s *Service) Fetch(token, requestHost string) (Data, error) {
	var record db.SubscriptionToken
	if err := s.db.First(&record, "token = ?", token).Error; err != nil {
		return Data{}, ErrInvalidToken
	}
	if record.Revoked {
		return Data{}, ErrInvalidToken
	}
	// 配额与到期判定必须统计该用户全部节点（含被停用的），否则停用后用量归零会重新放行
	used, err := s.usedBytes(record.ID)
	if err != nil {
		return Data{}, err
	}
	if record.ExpiresAt != nil && time.Now().After(*record.ExpiresAt) {
		return Data{}, ErrUnavailable
	}
	if record.TrafficLimitBytes > 0 && used >= record.TrafficLimitBytes {
		return Data{}, ErrUnavailable
	}
	now := time.Now()
	s.db.Model(&record).Update("last_used_at", now) // 最近使用时间非关键数据，失败忽略

	specs, err := s.nodes(record.ID)
	if err != nil {
		return Data{}, err
	}
	if len(specs) == 0 {
		// 没有任何可用节点时，下发的只会是一份"全部直连"的空配置，客户端会以为订阅正常。
		// 直接报错让用户知道该去找管理员（未勾选入站 / 入站被停用 / 用户被暂停）。
		return Data{}, ErrNoNodes
	}
	var groups []db.PolicyGroup
	if err := s.db.Where("enabled = ?", true).Order("id").Find(&groups).Error; err != nil {
		return Data{}, err
	}
	groupEntries := make([]singbox.GroupEntry, 0, len(groups))
	for _, g := range groups {
		members := []string{}
		if g.Members != "" {
			if err := json.Unmarshal([]byte(g.Members), &members); err != nil {
				return Data{}, fmt.Errorf("策略组成员配置不合法: %w", err)
			}
		}
		groupEntries = append(groupEntries, singbox.GroupEntry{Name: g.Name, Mode: g.Mode, Members: members})
	}
	var rules []db.Rule
	if err := s.db.Where("enabled = ?", true).Order("`order`").Find(&rules).Error; err != nil {
		return Data{}, err
	}
	ruleEntries, err := s.ruleEntries(rules)
	if err != nil {
		return Data{}, err
	}
	dns, err := s.dnsConfig()
	if err != nil {
		return Data{}, err
	}
	return Assemble(hostOnly(requestHost), specs, groupEntries, ruleEntries,
		getSetting(s.db, db.SettingRouteFinal), dns)
}

// usedBytes 统计订阅用户的已用流量（唯一依据是 TokenTraffic，与当前用户身份无关）。
func (s *Service) usedBytes(tokenID uint) (int64, error) {
	var row db.TokenTraffic
	err := s.db.First(&row, "token_id = ?", tokenID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return row.UploadBytes + row.DownloadBytes, nil
}

// nodes 组装该用户生效中的节点（入站启用 + 用户启用且未暂停）。
func (s *Service) nodes(tokenID uint) ([]NodeSpec, error) {
	var bindings []db.SubscriptionNode
	if err := s.db.Where("token_id = ?", tokenID).Order("inbound_id, id").Find(&bindings).Error; err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return nil, nil
	}
	var inbounds []db.Inbound
	if err := s.db.Where("enabled = ?", true).Find(&inbounds).Error; err != nil {
		return nil, err
	}
	inMeta := make(map[uint]db.Inbound, len(inbounds))
	for _, in := range inbounds {
		inMeta[in.ID] = in
	}
	specs := make([]NodeSpec, 0, len(bindings))
	for _, b := range bindings {
		in, ok := inMeta[b.InboundID]
		if !ok {
			continue // 入站已停用
		}
		var user db.InboundUser
		if err := s.db.First(&user, b.InboundUserID).Error; err != nil {
			continue
		}
		if !user.Enabled || user.Suspended {
			continue
		}
		specs = append(specs, NodeSpec{
			Tag: user.Name, Protocol: in.Protocol, Port: in.ListenPort,
			Config: in.Config, CredJSON: user.Credential,
		})
	}
	return specs, nil
}

// ruleEntries 把规则记录解析为带规则集下载源的条目（跳过无可用源的规则集规则）。
func (s *Service) ruleEntries(rules []db.Rule) ([]singbox.RuleEntry, error) {
	var sets []db.RuleSet
	if err := s.db.Find(&sets).Error; err != nil {
		return nil, err
	}
	var items []db.RuleSetItem
	if err := s.db.Where("enabled = ?", true).Find(&items).Error; err != nil {
		return nil, err
	}
	setName := make(map[uint]string, len(sets))
	for _, set := range sets {
		setName[set.ID] = set.Name
	}
	active := make(map[uint]db.RuleSetItem, len(items))
	for _, it := range items {
		if _, ok := active[it.RuleSetID]; !ok {
			active[it.RuleSetID] = it
		}
	}
	out := make([]singbox.RuleEntry, 0, len(rules))
	for _, r := range rules {
		entry := singbox.RuleEntry{
			MatchType: r.MatchType, MatchMethod: r.MatchMethod, Value: r.Value, Outbound: r.Outbound,
		}
		if r.MatchType == db.RuleMatchRuleSet {
			item, ok := active[r.RuleSetID]
			if !ok {
				continue // 规则集无启用项，客户端无法下载，跳过该规则
			}
			entry.RuleSet = &singbox.RuleSetSource{
				Tag: setName[r.RuleSetID], URL: item.SingBoxURL,
				MihomoURL: item.MihomoURL, MihomoBehavior: item.MihomoBehavior,
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

// dnsConfig 读取面板 DNS 设置（未设置时用默认国内外分流配置）。
func (s *Service) dnsConfig() (*singbox.DNSConfig, error) {
	raw := getSetting(s.db, db.SettingDNS)
	if raw == "" {
		return singbox.DefaultDNSConfig(), nil
	}
	var cfg singbox.DNSConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("DNS 配置不合法: %w", err)
	}
	return &cfg, nil
}

// Render 按格式渲染订阅内容。
func Render(d Data, format Format) ([]byte, error) {
	switch format {
	case FormatClash:
		return RenderClash(d)
	default:
		cfg, err := RenderSingBox(d)
		if err != nil {
			return nil, err
		}
		return marshalJSON(cfg)
	}
}

// hostOnly 取 Host 头的主机名部分（去端口，IPv6 兼容）。
func hostOnly(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return strings.Trim(host, "[]")
}

// getSetting 读取面板设置值，不存在或出错返回空字符串。
func getSetting(database *gorm.DB, key string) string {
	var s db.Setting
	if err := database.First(&s, "key = ?", key).Error; err != nil {
		return ""
	}
	return s.Value
}

// marshalJSON 序列化 sing-box 配置（缩进两格）。
func marshalJSON(v any) ([]byte, error) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化 sing-box 配置失败: %w", err)
	}
	return out, nil
}
