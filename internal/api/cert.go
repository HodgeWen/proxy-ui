package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// certReq 创建/更新证书请求；managed 模式传 PEM 内容（上传或粘贴），path 模式传服务器本地路径。
type certReq struct {
	Name     string `json:"name"`               // 名称备注
	Mode     string `json:"mode"`               // 引用方式：managed / path
	CertPem  string `json:"certPem,omitempty"`  // managed：证书 PEM 内容
	KeyPem   string `json:"keyPem,omitempty"`   // managed：私钥 PEM 内容
	CertPath string `json:"certPath,omitempty"` // path：证书绝对路径
	KeyPath  string `json:"keyPath,omitempty"`  // path：私钥绝对路径
}

// HandleCertsList GET /api/certs 证书列表。
func (a *API) HandleCertsList(c *gin.Context) {
	var certs []db.Certificate
	if err := a.db.Order("id").Find(&certs).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "查询证书列表失败: "+err.Error())
		return
	}
	OK(c, certs)
}

// HandleCertCreate POST /api/certs 新增证书（上传/粘贴存储或本地路径引用）。
func (a *API) HandleCertCreate(c *gin.Context) {
	var req certReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	cert := db.Certificate{Name: req.Name, Mode: req.Mode}
	if err := a.applyCertReq(&cert, req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.db.Create(&cert).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "创建证书失败: "+err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	OK(c, cert)
}

// HandleCertUpdate PUT /api/certs/:id 更新证书（managed 模式可整体替换 PEM，path 模式可更换路径）。
func (a *API) HandleCertUpdate(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req certReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var cert db.Certificate
	if err := a.db.First(&cert, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "证书不存在")
		return
	}
	if err := a.applyCertReq(&cert, req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.db.Save(&cert).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "保存证书失败: "+err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	OK(c, cert)
}

// HandleCertDelete DELETE /api/certs/:id 删除证书（被入站引用时拒绝）。
func (a *API) HandleCertDelete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var cert db.Certificate
	if err := a.db.First(&cert, id).Error; err != nil {
		Fail(c, http.StatusNotFound, "证书不存在")
		return
	}
	var inbounds []db.Inbound
	if err := a.db.Find(&inbounds).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "查询入站失败: "+err.Error())
		return
	}
	for _, in := range inbounds {
		cfg, err := singbox.ParseInboundConfig(in.Protocol, in.Config)
		if err != nil {
			continue
		}
		if tls := singbox.InboundTLS(cfg); tls != nil && tls.CertMode == singbox.CertModeManaged && tls.CertID == id {
			Fail(c, http.StatusConflict, fmt.Sprintf("证书已被入站 %s 引用，请先在入站表单中解除引用", in.Name))
			return
		}
	}
	if cert.Mode == singbox.CertModeManaged {
		_ = os.Remove(filepath.Join(a.certDir(), cert.CertPath))
		_ = os.Remove(filepath.Join(a.certDir(), cert.KeyPath))
	}
	if err := a.db.Delete(&db.Certificate{}, id).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "删除证书失败: "+err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	OK(c, gin.H{"deleted": id})
}

// applyCertReq 把请求字段应用到证书记录并校验：managed 模式 PEM 落盘，path 模式校验路径存在。
func (a *API) applyCertReq(cert *db.Certificate, req certReq) error {
	if req.Name != "" {
		cert.Name = req.Name
	}
	if req.Mode != "" {
		if cert.Mode != "" && cert.Mode != req.Mode {
			return fmt.Errorf("证书引用方式创建后不可变更，请删除后重建")
		}
		cert.Mode = req.Mode
	}
	if cert.Name == "" {
		return fmt.Errorf("名称不能为空")
	}
	switch cert.Mode {
	case singbox.CertModeManaged:
		if req.CertPem == "" && req.KeyPem == "" {
			return nil // 仅改名的更新
		}
		if req.CertPem == "" || req.KeyPem == "" {
			return fmt.Errorf("证书与私钥 PEM 需成对提供")
		}
		if !validPEM(req.CertPem) || !validPEM(req.KeyPem) {
			return fmt.Errorf("证书或私钥 PEM 内容不合法")
		}
		certName, keyName, err := a.writeCertFiles(req.CertPem, req.KeyPem)
		if err != nil {
			return err
		}
		// 替换内容时删除旧文件
		if cert.CertPath != "" {
			_ = os.Remove(filepath.Join(a.certDir(), cert.CertPath))
			_ = os.Remove(filepath.Join(a.certDir(), cert.KeyPath))
		}
		cert.CertPath, cert.KeyPath = certName, keyName
	case singbox.CertModePath:
		if req.CertPath == "" && req.KeyPath == "" {
			return nil
		}
		if req.CertPath == "" || req.KeyPath == "" {
			return fmt.Errorf("证书与私钥路径需成对提供")
		}
		for _, p := range []string{req.CertPath, req.KeyPath} {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("路径不可访问: %s", p)
			}
		}
		cert.CertPath, cert.KeyPath = req.CertPath, req.KeyPath
	default:
		return fmt.Errorf("证书引用方式须为 managed 或 path")
	}
	return nil
}

// writeCertFiles 把证书/私钥 PEM 写入证书目录（0600），返回相对文件名。
func (a *API) writeCertFiles(certPem, keyPem string) (string, string, error) {
	name := randFileName()
	dir := a.certDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("创建证书目录失败: %w", err)
	}
	certName, keyName := name+"-cert.pem", name+"-key.pem"
	if err := os.WriteFile(filepath.Join(dir, certName), []byte(certPem), 0o600); err != nil {
		return "", "", fmt.Errorf("写入证书失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, keyName), []byte(keyPem), 0o600); err != nil {
		return "", "", fmt.Errorf("写入私钥失败: %w", err)
	}
	return certName, keyName, nil
}

// validPEM 校验 PEM 文本能解析出至少一个块。
func validPEM(s string) bool {
	block, _ := pem.Decode([]byte(s))
	return block != nil
}

// randFileName 生成证书文件随机基名。
func randFileName() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		panic(err) // 系统随机源不可用属致命错误
	}
	return hex.EncodeToString(buf)
}
