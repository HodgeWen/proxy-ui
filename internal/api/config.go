package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// HandleKernelApply POST /api/kernel/apply 应用配置：聚合全部启用入站/出站重新生成
// config.json；内核运行中则按新配置重启，未运行时仅写入待下次启动生效。
func (a *API) HandleKernelApply(c *gin.Context) {
	cfg, err := a.buildKernelConfig()
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.core.WriteConfig(cfg); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.core.CheckConfig(c.Request.Context()); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	applied := false
	if a.core.Status().Running {
		if err := a.core.Restart(); err != nil {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		applied = true
	}
	OK(c, gin.H{"applied": applied, "status": a.kernelStatus(c)})
}

// buildKernelConfig 从数据库聚合启用中的入站/出站/规则/策略组生成完整内核配置。
func (a *API) buildKernelConfig() (singbox.Config, error) {
	var inbounds []db.Inbound
	if err := a.db.Where("enabled = ?", true).Order("id").Find(&inbounds).Error; err != nil {
		return singbox.Config{}, fmt.Errorf("查询入站失败: %w", err)
	}
	var outbounds []db.Outbound
	if err := a.db.Where("enabled = ?", true).Order("id").Find(&outbounds).Error; err != nil {
		return singbox.Config{}, fmt.Errorf("查询出站失败: %w", err)
	}
	var rules []db.Rule
	if err := a.db.Where("enabled = ?", true).Order("`order`").Find(&rules).Error; err != nil {
		return singbox.Config{}, fmt.Errorf("查询规则失败: %w", err)
	}
	var groups []db.PolicyGroup
	if err := a.db.Where("enabled = ?", true).Order("id").Find(&groups).Error; err != nil {
		return singbox.Config{}, fmt.Errorf("查询策略组失败: %w", err)
	}
	inEntries := make([]singbox.InboundEntry, 0, len(inbounds))
	for _, in := range inbounds {
		inEntries = append(inEntries, singbox.InboundEntry{
			Tag: in.Name, Protocol: in.Protocol, ListenPort: in.ListenPort,
			Config: in.Config, Cert: a.resolveCertPaths,
		})
	}
	outEntries := make([]singbox.OutboundEntry, 0, len(outbounds))
	for _, ob := range outbounds {
		outEntries = append(outEntries, singbox.OutboundEntry{Tag: ob.Name, Type: ob.Type, Config: ob.Config})
	}
	routing := &singbox.Routing{Final: a.getSetting(settingRouteFinal), AntiAD: a.getSetting(settingAntiAD)}
	for _, r := range rules {
		routing.Rules = append(routing.Rules, singbox.RuleEntry{Type: r.Type, Value: r.Value, Outbound: r.Outbound})
	}
	for _, g := range groups {
		members := []string{}
		if g.Members != "" {
			if err := json.Unmarshal([]byte(g.Members), &members); err != nil {
				return singbox.Config{}, fmt.Errorf("策略组 %s 成员配置不合法: %w", g.Name, err)
			}
		}
		routing.Groups = append(routing.Groups, singbox.GroupEntry{Name: g.Name, Mode: g.Mode, Members: members})
	}
	return singbox.BuildConfig(inEntries, outEntries, routing)
}

// resolveCertPaths 把面板存储证书 ID 解析为证书/私钥绝对路径（含存在性校验）。
func (a *API) resolveCertPaths(certID uint) (string, string, error) {
	var cert db.Certificate
	if err := a.db.First(&cert, certID).Error; err != nil {
		return "", "", fmt.Errorf("证书 %d 不存在", certID)
	}
	certPath := filepath.Join(a.certDir(), cert.CertPath)
	keyPath := filepath.Join(a.certDir(), cert.KeyPath)
	for _, p := range []string{certPath, keyPath} {
		if _, err := os.Stat(p); err != nil {
			return "", "", fmt.Errorf("证书 %s 的文件不存在: %s", cert.Name, p)
		}
	}
	return certPath, keyPath, nil
}
