// Package subscription 组装订阅数据并渲染 sing-box / clash(mihomo) 客户端配置，
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

// ErrInvalidToken 匿名令牌不存在或已吊销。
var ErrInvalidToken = errors.New("订阅令牌不存在或已吊销")

// Format 订阅输出格式。
type Format string

// 支持的订阅输出格式。
const (
	FormatSingBox Format = "sing-box" // sing-box 客户端配置（默认）
	FormatClash   Format = "clash"    // clash(mihomo) 客户端配置
)

// DetectFormat 选择订阅输出格式：查询参数 format（clash/mihomo 或 singbox/sing-box）
// 优先于 UA；UA 含 clash/mihomo 判为 clash，含 sing-box/singbox/SFI/SFM 判为 sing-box，
// 其余默认 sing-box。
func DetectFormat(param, ua string) Format {
	switch strings.ToLower(strings.TrimSpace(param)) {
	case "clash", "mihomo":
		return FormatClash
	case "singbox", "sing-box", "sing":
		return FormatSingBox
	}
	lower := strings.ToLower(ua)
	if strings.Contains(lower, "clash") || strings.Contains(lower, "mihomo") {
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
		c.String(http.StatusNotFound, "订阅不存在或已失效")
		return
	}
	format := DetectFormat(c.Query("format"), c.Request.UserAgent())
	content, err := Render(data, format)
	if err != nil {
		c.String(http.StatusInternalServerError, "生成订阅失败: "+err.Error())
		return
	}
	if format == FormatClash {
		c.Header("Content-Disposition", `attachment; filename="config.yaml"`)
		c.Data(http.StatusOK, "text/yaml; charset=utf-8", content)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="config.json"`)
	c.Data(http.StatusOK, "application/json; charset=utf-8", content)
}

// Fetch 校验匿名令牌并组装订阅数据；节点服务器地址取客户端拉取订阅的请求 Host
// （去端口），反代或改端口部署天然适配，成功时更新令牌最近使用时间。
func (s *Service) Fetch(token, requestHost string) (Data, error) {
	var record db.SubscriptionToken
	if err := s.db.First(&record, "token = ?", token).Error; err != nil {
		return Data{}, ErrInvalidToken
	}
	if record.Revoked {
		return Data{}, ErrInvalidToken
	}
	now := time.Now()
	s.db.Model(&record).Update("last_used_at", now) // 最近使用时间非关键数据，失败忽略

	var inbounds []db.Inbound
	if err := s.db.Where("enabled = ?", true).Order("id").Find(&inbounds).Error; err != nil {
		return Data{}, err
	}
	var rules []db.Rule
	if err := s.db.Where("enabled = ?", true).Order("`order`").Find(&rules).Error; err != nil {
		return Data{}, err
	}
	var groups []db.PolicyGroup
	if err := s.db.Where("enabled = ?", true).Order("id").Find(&groups).Error; err != nil {
		return Data{}, err
	}
	specs := make([]InboundSpec, 0, len(inbounds))
	for _, in := range inbounds {
		specs = append(specs, InboundSpec{Tag: in.Name, Protocol: in.Protocol, Port: in.ListenPort, Config: in.Config})
	}
	groupEntries := make([]singbox.GroupEntry, 0, len(groups))
	for _, g := range groups {
		members := []string{}
		if g.Members != "" {
			if err := unmarshalMembers(g.Members, &members); err != nil {
				return Data{}, err
			}
		}
		groupEntries = append(groupEntries, singbox.GroupEntry{Name: g.Name, Mode: g.Mode, Members: members})
	}
	ruleEntries := make([]singbox.RuleEntry, 0, len(rules))
	for _, r := range rules {
		ruleEntries = append(ruleEntries, singbox.RuleEntry{Type: r.Type, Value: r.Value, Outbound: r.Outbound})
	}
	server := hostOnly(requestHost)
	return Assemble(server, specs, groupEntries, ruleEntries,
		getSetting(s.db, db.SettingRouteFinal), getSetting(s.db, db.SettingAntiADSource))
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
	return host
}

// unmarshalMembers 解析策略组成员 JSON 数组。
func unmarshalMembers(raw string, dst *[]string) error {
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		return fmt.Errorf("策略组成员配置不合法: %w", err)
	}
	return nil
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
