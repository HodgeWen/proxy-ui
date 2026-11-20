package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// HandleKernelApply POST /api/kernel/apply 应用配置：聚合全部启用入站与用户重新生成并
// 重建内核实例（内置内核下配置变更即重建，无需单独的"校验-写入-重启"三步）。
func (a *API) HandleKernelApply(c *gin.Context) {
	// 应用前按配额/到期刷新用户状态，保证写入内核的用户都在有效期内
	if _, err := a.ReconcileQuota(); err != nil {
		Fail(c, http.StatusInternalServerError, "刷新配额状态失败: "+err.Error())
		return
	}
	result, status, err := a.applyKernel(c.Request.Context())
	if err != nil {
		Fail(c, status, err.Error())
		return
	}
	OK(c, result)
}

// applyKernel 聚合并应用内核配置，返回结果载荷与错误对应的 HTTP 状态码。
func (a *API) applyKernel(ctx context.Context) (gin.H, int, error) {
	cfg, err := a.buildKernelConfig()
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	if err := a.core.CheckConfig(cfg); err != nil {
		return nil, http.StatusBadRequest, err
	}
	// 已运行时按新配置重建；未运行时只保存待启动（避免偷偷拉起内核）
	applied := false
	if a.core.Status().Running {
		if err := a.core.Restart(cfg); err != nil {
			return nil, http.StatusBadRequest, err
		}
		applied = true
	}
	a.configMu.Lock()
	a.pendingConfig = cfg
	a.configMu.Unlock()
	return gin.H{"applied": applied, "status": a.kernelStatusOf(ctx)}, http.StatusOK, nil
}

// buildKernelConfig 从数据库聚合启用中的入站与用户/出站/规则/策略组/DNS 生成完整内核配置。
func (a *API) buildKernelConfig() (singbox.Config, error) {
	var inbounds []db.Inbound
	if err := a.db.Where("enabled = ?", true).Order("id").Find(&inbounds).Error; err != nil {
		return singbox.Config{}, fmt.Errorf("查询入站失败: %w", err)
	}
	var users []db.InboundUser
	if err := a.db.Where("enabled = ? AND suspended = ?", true, false).Order("id").Find(&users).Error; err != nil {
		return singbox.Config{}, fmt.Errorf("查询入站用户失败: %w", err)
	}
	usersByInbound := make(map[uint][]db.InboundUser, len(inbounds))
	for _, u := range users {
		usersByInbound[u.InboundID] = append(usersByInbound[u.InboundID], u)
	}
	inEntries := make([]singbox.InboundEntry, 0, len(inbounds))
	for _, in := range inbounds {
		boundUsers := usersByInbound[in.ID]
		if len(boundUsers) == 0 {
			continue // 无启用用户的入站无法启动，跳过
		}
		entries := make([]singbox.InboundUserEntry, 0, len(boundUsers))
		for _, u := range boundUsers {
			cred, err := singbox.ParseCredential(in.Protocol, u.Credential)
			if err != nil {
				return singbox.Config{}, fmt.Errorf("用户 %s 凭据不合法: %w", u.Name, err)
			}
			entries = append(entries, singbox.InboundUserEntry{Name: u.Name, Credential: cred})
		}
		inEntries = append(inEntries, singbox.InboundEntry{
			Tag: in.Name, Protocol: in.Protocol, Listen: in.Listen, Port: in.ListenPort,
			Config: in.Config, Users: entries, Cert: a.resolveCertPaths,
		})
	}
	var outbounds []db.Outbound
	if err := a.db.Where("enabled = ?", true).Order("id").Find(&outbounds).Error; err != nil {
		return singbox.Config{}, fmt.Errorf("查询出站失败: %w", err)
	}
	outEntries := make([]singbox.OutboundEntry, 0, len(outbounds))
	for _, ob := range outbounds {
		outEntries = append(outEntries, singbox.OutboundEntry{Tag: ob.Name, Type: ob.Type, Config: ob.Config})
	}
	routing, err := a.buildRouting()
	if err != nil {
		return singbox.Config{}, err
	}
	dns, err := a.buildDNS()
	if err != nil {
		return singbox.Config{}, err
	}
	return singbox.BuildConfig(inEntries, outEntries, routing, dns)
}

// buildRouting 聚合启用中的规则（解析规则集引用）、策略组与兜底出站。
func (a *API) buildRouting() (*singbox.Routing, error) {
	var rules []db.Rule
	if err := a.db.Where("enabled = ?", true).Order("`order`").Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("查询规则失败: %w", err)
	}
	routing := &singbox.Routing{Final: a.getSetting(db.SettingRouteFinal)}
	sources := map[uint]*singbox.RuleSetSource{}
	for _, r := range rules {
		entry := singbox.RuleEntry{
			MatchType: r.MatchType, MatchMethod: r.MatchMethod, Value: r.Value, Outbound: r.Outbound,
		}
		if r.MatchType == db.RuleMatchRuleSet {
			src, ok := sources[r.RuleSetID]
			if !ok {
				resolved, err := a.ruleSetSourceOf(r.RuleSetID)
				if err != nil {
					return nil, fmt.Errorf("规则引用异常: %w", err)
				}
				src = resolved
				sources[r.RuleSetID] = src
			}
			entry.RuleSet = src
		}
		routing.Rules = append(routing.Rules, entry)
	}
	var groups []db.PolicyGroup
	if err := a.db.Where("enabled = ?", true).Order("id").Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("查询策略组失败: %w", err)
	}
	for _, g := range groups {
		members := []string{}
		if g.Members != "" {
			if err := json.Unmarshal([]byte(g.Members), &members); err != nil {
				return nil, fmt.Errorf("策略组 %s 成员配置不合法: %w", g.Name, err)
			}
		}
		routing.Groups = append(routing.Groups, singbox.GroupEntry{Name: g.Name, Mode: g.Mode, Members: members})
	}
	return routing, nil
}

// buildDNS 读取面板 DNS 设置（未设置时用默认国内外分流配置）。
func (a *API) buildDNS() (*singbox.DNSConfig, error) {
	raw := a.getSetting(db.SettingDNS)
	if raw == "" {
		return singbox.DefaultDNSConfig(), nil
	}
	var cfg singbox.DNSConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("DNS 配置不合法: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// resolveCertPaths 把面板存储证书 ID 解析为证书/私钥绝对路径（含存在性校验）。
// 必须返回绝对路径：sing-box 对配置内的相对路径按其工作目录解析，相对路径会指错位置。
func (a *API) resolveCertPaths(certID uint) (string, string, error) {
	var cert db.Certificate
	if err := a.db.First(&cert, certID).Error; err != nil {
		return "", "", fmt.Errorf("证书 %d 不存在", certID)
	}
	certPath, err := filepath.Abs(filepath.Join(a.certDir(), cert.CertPath))
	if err != nil {
		return "", "", fmt.Errorf("解析证书路径失败: %w", err)
	}
	keyPath, err := filepath.Abs(filepath.Join(a.certDir(), cert.KeyPath))
	if err != nil {
		return "", "", fmt.Errorf("解析私钥路径失败: %w", err)
	}
	for _, p := range []string{certPath, keyPath} {
		if _, err := os.Stat(p); err != nil {
			return "", "", fmt.Errorf("证书 %s 的文件不存在: %s", cert.Name, p)
		}
	}
	return certPath, keyPath, nil
}
