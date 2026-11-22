// Package subscription 组装订阅数据并渲染 sing-box / mihomo 客户端配置，
// 提供匿名 token 订阅端点。
package subscription

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// ErrInvalidToken 匿名令牌不存在或已停用（预览按 ID 查不到订阅用户也复用此错误）。
var ErrInvalidToken = errors.New("订阅令牌不存在或已停用")

// ErrUnavailable 订阅已到期或流量用尽。
var ErrUnavailable = errors.New("订阅已到期或流量已用尽")

// ErrNoNodes 该订阅没有任何可用节点（未勾选入站、入站被停用或用户被停用/暂停）；
// 具体原因以 %w 附在错误消息里。
var ErrNoNodes = errors.New("订阅下没有可用节点")

// Format 订阅输出格式。
type Format string

// 支持的订阅输出格式。
const (
	FormatSingBox Format = "sing-box" // sing-box 客户端配置（默认）
	FormatClash   Format = "clash"    // clash(mihomo) 客户端配置
)

// ParseFormat 解析显式给出的格式参数（接受 sing-box / mihomo 及其常见别名）；
// 不认识的值返回 false，供需要严格校验格式的调用方（如预览接口）区别于 DetectFormat 的兜底默认。
func ParseFormat(param string) (Format, bool) {
	switch strings.ToLower(strings.TrimSpace(param)) {
	case "clash", "mihomo", "clashmeta", "clash.meta":
		return FormatClash, true
	case "singbox", "sing-box", "sing":
		return FormatSingBox, true
	}
	return "", false
}

// DetectFormat 选择订阅输出格式：查询参数 format 优先于 UA；
// UA 含 clash/mihomo/stash 判为 clash，其余默认 sing-box。
func DetectFormat(param, ua string) Format {
	if f, ok := ParseFormat(param); ok {
		return f
	}
	lower := strings.ToLower(ua)
	if strings.Contains(lower, "clash") || strings.Contains(lower, "mihomo") || strings.Contains(lower, "stash") {
		return FormatClash
	}
	return FormatSingBox
}

// Service 订阅服务：校验匿名令牌、组装订阅数据并按格式渲染下发。
type Service struct {
	db      *gorm.DB
	dataDir string // 面板数据目录，用于读取托管证书判定是否自签
}

// New 构造订阅服务。
func New(database *gorm.DB, dataDir string) *Service {
	return &Service{db: database, dataDir: dataDir}
}

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
	title := sanitizeHeaderTitle(data.Title)
	c.Header("Cache-Control", "no-store")
	c.Header("Profile-Title", "base64:"+base64.StdEncoding.EncodeToString([]byte(title)))
	c.Header("Content-Disposition", contentDisposition(title))
	if data.UserInfo != nil {
		c.Header("Subscription-Userinfo", formatUserInfo(data.UserInfo))
	}
	if format == FormatClash {
		c.Data(http.StatusOK, "text/yaml; charset=utf-8", content)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", content)
}

// Fetch 校验匿名令牌并组装订阅数据：节点取该用户在各入站下的用户身份，
// 服务器端口为入站端口（同入站下所有用户共享），服务器地址取请求 Host（反代天然适配）。
// 保持原语义供匿名订阅端点使用；预览走 Preview（共用 assemble，差异只在校验）。
func (s *Service) Fetch(token, requestHost string) (Data, error) {
	var record db.SubscriptionToken
	if err := s.db.First(&record, "token = ?", token).Error; err != nil {
		return Data{}, ErrInvalidToken
	}
	if record.Revoked {
		return Data{}, ErrInvalidToken
	}
	// 配额与到期判定必须统计该用户全部节点（含被停用的），否则停用后用量归零会重新放行
	up, down, err := s.trafficBytes(record.ID)
	if err != nil {
		return Data{}, err
	}
	used := up + down
	if record.ExpiresAt != nil && time.Now().After(*record.ExpiresAt) {
		return Data{}, ErrUnavailable
	}
	if record.TrafficLimitBytes > 0 && used >= record.TrafficLimitBytes {
		return Data{}, ErrUnavailable
	}
	now := time.Now()
	s.db.Model(&record).Update("last_used_at", now) // 最近使用时间非关键数据，失败忽略
	data, err := s.assemble(record, requestHost)
	if err != nil {
		return Data{}, err
	}
	info := &UserInfo{Upload: up, Download: down, Total: record.TrafficLimitBytes}
	if record.ExpiresAt != nil {
		info.Expire = record.ExpiresAt.Unix()
	}
	data.UserInfo = info
	return data, nil
}

// Preview 按订阅用户 ID 与格式渲染预览内容：跳过停用、配额与到期校验，
// 与匿名端点共用同一 assemble / Render，保证预览即真实下发内容；无可用节点时返回带原因的 ErrNoNodes。
func (s *Service) Preview(id uint, requestHost string, format Format) ([]byte, error) {
	var record db.SubscriptionToken
	if err := s.db.First(&record, id).Error; err != nil {
		return nil, ErrInvalidToken
	}
	data, err := s.assemble(record, requestHost)
	if err != nil {
		return nil, err
	}
	return Render(data, format)
}

// assemble 组装订阅数据（节点、策略组、规则与 DNS），匿名下发与预览共用。
func (s *Service) assemble(record db.SubscriptionToken, requestHost string) (Data, error) {
	specs, err := s.nodes(record.ID)
	if err != nil {
		return Data{}, err
	}
	if len(specs) == 0 {
		// 没有任何可用节点时，下发的只会是一份"全部直连"的空配置，客户端会以为订阅正常。
		// 直接报错并带上原因（未勾选入站 / 入站被停用 / 用户被停用或暂停）。
		return Data{}, fmt.Errorf("%w：%s", ErrNoNodes, s.noNodeReason(record.ID))
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
	data, err := Assemble(hostOnly(requestHost), specs, groupEntries, ruleEntries,
		getSetting(s.db, db.SettingRouteFinal), dns)
	if err != nil {
		return Data{}, err
	}
	data.Title = subscriptionTitle(record.Name, requestHost)
	return data, nil
}

// noNodeReason 无可用节点的原因：要么没勾过入站，要么勾选的入站已全部停用，
// 或该用户在入站下的节点已全部被停用/暂停（入站全停时节点状态已无关紧要，优先报入站）。
func (s *Service) noNodeReason(tokenID uint) string {
	var bindings []db.SubscriptionNode
	if err := s.db.Where("token_id = ?", tokenID).Find(&bindings).Error; err != nil || len(bindings) == 0 {
		return "该用户未勾选任何入站"
	}
	inboundIDs := make([]uint, 0, len(bindings))
	for _, b := range bindings {
		inboundIDs = append(inboundIDs, b.InboundID)
	}
	var enabledInbounds int64
	if err := s.db.Model(&db.Inbound{}).Where("id IN ? AND enabled = ?", inboundIDs, true).Count(&enabledInbounds).Error; err != nil || enabledInbounds == 0 {
		return "勾选的入站已全部停用"
	}
	return "该用户的节点已全部停用/暂停"
}

// trafficBytes 查询订阅用户的上行与下行累计流量（唯一依据是 TokenTraffic，与当前用户身份无关）。
func (s *Service) trafficBytes(tokenID uint) (upload, download int64, err error) {
	var row db.TokenTraffic
	err = s.db.First(&row, "token_id = ?", tokenID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	return row.UploadBytes, row.DownloadBytes, nil
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
	// 用户一次 IN 查询取回，不随节点数放大查询次数
	userIDs := make([]uint, 0, len(bindings))
	for _, b := range bindings {
		userIDs = append(userIDs, b.InboundUserID)
	}
	var users []db.InboundUser
	if err := s.db.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	userMeta := make(map[uint]db.InboundUser, len(users))
	for _, u := range users {
		userMeta[u.ID] = u
	}
	var inbounds []db.Inbound
	if err := s.db.Where("enabled = ?", true).Find(&inbounds).Error; err != nil {
		return nil, err
	}
	inMeta := make(map[uint]db.Inbound, len(inbounds))
	for _, in := range inbounds {
		inMeta[in.ID] = in
	}
	var certs []db.Certificate
	if err := s.db.Find(&certs).Error; err != nil {
		return nil, err
	}
	certMeta := make(map[uint]db.Certificate, len(certs))
	for _, c := range certs {
		certMeta[c.ID] = c
	}
	selfSignedCache := make(map[string]bool)
	specs := make([]NodeSpec, 0, len(bindings))
	for _, b := range bindings {
		in, ok := inMeta[b.InboundID]
		if !ok {
			continue // 入站已停用
		}
		user, ok := userMeta[b.InboundUserID]
		if !ok {
			continue
		}
		if !user.Enabled || user.Suspended {
			continue
		}
		specs = append(specs, NodeSpec{
			Tag: user.Name, Protocol: in.Protocol, Port: in.ListenPort,
			Config: in.Config, CredJSON: user.Credential,
			Insecure: s.inboundInsecure(in.Protocol, in.Config, certMeta, selfSignedCache),
		})
	}
	return specs, nil
}

// inboundInsecure 判断入站是否使用自签 TLS 证书（Reality 节点恒为 false）。
func (s *Service) inboundInsecure(protocol, rawConfig string, certs map[uint]db.Certificate, cache map[string]bool) bool {
	cfg, err := singbox.ParseInboundConfig(protocol, rawConfig)
	if err != nil {
		return false
	}
	if vc, ok := cfg.(*singbox.VLESSConfig); ok && vc.Reality.Enabled {
		return false
	}
	tlsCfg := singbox.InboundTLS(cfg)
	if tlsCfg == nil {
		return false
	}
	tlsActive := tlsCfg.Enabled || protocol == singbox.ProtoHysteria2 || protocol == singbox.ProtoTUIC ||
		protocol == singbox.ProtoAnyTLS || protocol == singbox.ProtoTrojan
	if !tlsActive {
		return false
	}
	path := s.certFilePath(tlsCfg, certs)
	if path == "" {
		return false
	}
	if cached, ok := cache[path]; ok {
		return cached
	}
	selfSigned := isSelfSignedPEMFile(path)
	cache[path] = selfSigned
	return selfSigned
}

// certFilePath 解析入站 TLS 引用的证书文件路径。
func (s *Service) certFilePath(tlsCfg *singbox.TLSConfig, certs map[uint]db.Certificate) string {
	switch tlsCfg.CertMode {
	case singbox.CertModeManaged:
		if tlsCfg.CertID == 0 {
			return ""
		}
		cert, ok := certs[tlsCfg.CertID]
		if !ok {
			return ""
		}
		if cert.Mode == singbox.CertModeManaged {
			if cert.CertPath == "" || s.dataDir == "" {
				return ""
			}
			return filepath.Join(s.dataDir, "certs", cert.CertPath)
		}
		if cert.Mode == singbox.CertModePath {
			return cert.CertPath
		}
	case singbox.CertModePath:
		return tlsCfg.CertPath
	}
	return ""
}

// isSelfSignedPEMFile 判断 PEM 文件中的首个证书是否为自签证书。
func isSelfSignedPEMFile(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for len(raw) > 0 {
		var block *pem.Block
		block, raw = pem.Decode(raw)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return false
		}
		// 面板自签叶子证书不带 IsCA/KeyUsageCertSign，CheckSignatureFrom 会按 CA 约束拒绝；
		// 用 CheckSignature 直接校验证书自身公钥签名。
		return bytes.Equal(cert.RawIssuer, cert.RawSubject) &&
			cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature) == nil
	}
	return false
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

// subscriptionTitle 解析订阅显示名：优先用户备注，其次请求域名，最后 proxy-ui。
func subscriptionTitle(name, requestHost string) string {
	if t := strings.TrimSpace(name); t != "" {
		return t
	}
	if h := strings.TrimSpace(hostOnly(requestHost)); h != "" {
		return h
	}
	return "proxy-ui"
}

// sanitizeHeaderTitle 清洗响应头使用的订阅名：移除控制字符并替换引号/分隔符等破坏头结构的字符。
func sanitizeHeaderTitle(raw string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r < 0x20 || r == 0x7f:
			return -1
		case r == '"' || r == '\\' || r == ';' || r == '/':
			return '-'
		default:
			return r
		}
	}, strings.TrimSpace(raw))
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return "proxy-ui"
	}
	return cleaned
}

// contentDisposition 生成 Content-Disposition 响应头（不附带 config.* 与扩展名，避免客户端展示为文件名）：
// 统一使用 RFC 5987 无引号格式 `filename*=UTF-8”...`，避免部分客户端（如 Clash Verge）把双引号解析进配置文件名。
func contentDisposition(title string) string {
	return "attachment; filename*=UTF-8''" + url.PathEscape(title)
}

// formatUserInfo 生成 Subscription-Userinfo 响应头值。
func formatUserInfo(info *UserInfo) string {
	parts := []string{
		fmt.Sprintf("upload=%d", info.Upload),
		fmt.Sprintf("download=%d", info.Download),
	}
	if info.Total > 0 {
		parts = append(parts, fmt.Sprintf("total=%d", info.Total))
	}
	if info.Expire > 0 {
		parts = append(parts, fmt.Sprintf("expire=%d", info.Expire))
	}
	return strings.Join(parts, "; ")
}

// marshalJSON 序列化 sing-box 配置（缩进两格）。
func marshalJSON(v any) ([]byte, error) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化 sing-box 配置失败: %w", err)
	}
	return out, nil
}
