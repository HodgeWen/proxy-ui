package api

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// 自签证书密钥类型。
const (
	KeyTypeP256    = "p256"    // ECDSA P-256（默认：体积小、握手快）
	KeyTypeRSA2048 = "rsa2048" // RSA 2048（兼容老客户端）
	KeyTypeEd25519 = "ed25519" // Ed25519（最新一代，需 TLS 1.3）
)

// SupportedCertKeyTypes 自签密钥类型候选。
var SupportedCertKeyTypes = []string{KeyTypeP256, KeyTypeRSA2048, KeyTypeEd25519}

// certReq 创建/更新证书请求；managed 模式传 PEM 内容（上传或粘贴），path 模式传服务器本地路径。
type certReq struct {
	Name     string `json:"name"`               // 名称备注
	Mode     string `json:"mode"`               // 引用方式：managed / path
	CertPem  string `json:"certPem,omitempty"`  // managed：证书 PEM 内容
	KeyPem   string `json:"keyPem,omitempty"`   // managed：私钥 PEM 内容
	CertPath string `json:"certPath,omitempty"` // path：证书绝对路径
	KeyPath  string `json:"keyPath,omitempty"`  // path：私钥绝对路径
}

// selfSignReq 一键自签请求：主域名作 CN 并并入 SAN，SANs 为附加域名/IP，days 为有效期天数。
type selfSignReq struct {
	Name    string   `json:"name"`              // 名称备注（留空取主域名）
	Domain  string   `json:"domain"`            // 主域名
	SANs    []string `json:"sans,omitempty"`    // 附加 SAN（域名或 IP）
	Days    int      `json:"days"`              // 有效期天数
	KeyType string   `json:"keyType,omitempty"` // 密钥类型（缺省 p256）
}

// certView 证书视图：库记录之上补充从证书 PEM 解析出的域名与到期信息，
// 解析失败时 NotAfter / DaysLeft 为 null、CertError 给出可读原因（不中断整张列表）。
type certView struct {
	db.Certificate
	CommonName   string     `json:"commonName"`          // 证书 CN
	Domains      []string   `json:"domains"`             // CN + SAN 域名/IP 汇总（展示用）
	NotAfter     *time.Time `json:"notAfter"`            // 到期时间
	DaysLeft     *int       `json:"daysLeft"`            // 剩余整天数（已过期为负）
	CertError    string     `json:"certError,omitempty"` // 证书解析失败的可读原因
	ReferencedBy []string   `json:"referencedBy"`        // 引用该证书的入站名称（AI 工具视图填充）
}

// HandleCertsList GET /api/certs 证书列表（含到期时间与剩余天数）。
func (a *API) HandleCertsList(c *gin.Context) {
	views, err := a.certViews()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询证书列表失败: "+err.Error())
		return
	}
	OK(c, views)
}

// certViews 查询全部证书并组装视图（含被入站引用情况之外的基础信息）。
func (a *API) certViews() ([]certView, error) {
	var certs []db.Certificate
	if err := a.db.Order("id").Find(&certs).Error; err != nil {
		return nil, err
	}
	views := make([]certView, len(certs))
	for i, cert := range certs {
		views[i] = a.certViewOf(cert)
	}
	return views, nil
}

// HandleCertSelfSigned POST /api/certs/self-signed 一键生成自签证书（managed 存储口径落 certs/）。
func (a *API) HandleCertSelfSigned(c *gin.Context) {
	var req selfSignReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	view, err := a.createSelfSigned(req)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if !a.applyOrFail(c) {
		return
	}
	OK(c, view)
}

// createSelfSigned 生成自签证书并落库（内核热应用由调用方负责）。
func (a *API) createSelfSigned(req selfSignReq) (certView, error) {
	certPEM, keyPEM, err := generateSelfSigned(req.Domain, req.SANs, req.Days, req.KeyType)
	if err != nil {
		return certView{}, err
	}
	name := req.Name
	if name == "" {
		name = req.Domain
	}
	var count int64
	if err := a.db.Model(&db.Certificate{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return certView{}, fmt.Errorf("查询证书失败: %w", err)
	}
	if count > 0 {
		return certView{}, fmt.Errorf("名称「%s」已存在，请换一个名称", name)
	}
	certName, keyName, err := a.writeCertFiles(certPEM, keyPEM)
	if err != nil {
		return certView{}, err
	}
	cert := db.Certificate{Name: name, Mode: singbox.CertModeManaged, CertPath: certName, KeyPath: keyName}
	if err := a.db.Create(&cert).Error; err != nil {
		_ = os.Remove(filepath.Join(a.certDir(), certName))
		_ = os.Remove(filepath.Join(a.certDir(), keyName))
		return certView{}, fmt.Errorf("创建证书失败: %w", err)
	}
	return a.certViewOf(cert), nil
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
	OK(c, a.certViewOf(cert))
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
	OK(c, a.certViewOf(cert))
}

// HandleCertDelete DELETE /api/certs/:id 删除证书（被入站 TLS 或 ECH 密钥引用时拒绝）。
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
	if refs := a.certReferencedBy(cert); len(refs) > 0 {
		Fail(c, http.StatusConflict, fmt.Sprintf(
			"证书已被入站 %s 引用（TLS 证书或 ECH 密钥），请先在对应入站中解除引用", strings.Join(refs, "、")))
		return
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

// certReferencedBy 返回引用该证书的入站名称，覆盖全部引用点：
// 入站 TLS 的面板存储证书 ID、入站 TLS / ECH 密钥的服务器路径（path 模式证书按文件路径命中）。
func (a *API) certReferencedBy(cert db.Certificate) []string {
	var files []string
	if cert.Mode == singbox.CertModeManaged {
		files = []string{
			cleanAbsPath(filepath.Join(a.certDir(), cert.CertPath)),
			cleanAbsPath(filepath.Join(a.certDir(), cert.KeyPath)),
		}
	} else {
		files = []string{cleanAbsPath(cert.CertPath), cleanAbsPath(cert.KeyPath)}
	}
	var inbounds []db.Inbound
	if err := a.db.Find(&inbounds).Error; err != nil {
		return nil // 查询失败时不拦截删除；删除后的热应用仍会兜底暴露问题
	}
	var refs []string
	for _, in := range inbounds {
		cfg, err := singbox.ParseInboundConfig(in.Protocol, in.Config)
		if err != nil {
			continue
		}
		tls := singbox.InboundTLS(cfg)
		if tls == nil {
			continue
		}
		used := (tls.CertMode == singbox.CertModeManaged && tls.CertID == cert.ID) ||
			contains(files, cleanAbsPath(tls.CertPath)) ||
			contains(files, cleanAbsPath(tls.KeyPath)) ||
			contains(files, cleanAbsPath(tls.ECH.KeyPath))
		if used {
			refs = append(refs, in.Name)
		}
	}
	return refs
}

// cleanAbsPath 规范化为绝对路径用于比较；空串原样返回（不参与比较）。
func cleanAbsPath(p string) string {
	if p == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return abs
}

// certViewOf 为证书记录补充从 PEM 解析出的域名与到期信息；解析失败只置 CertError。
func (a *API) certViewOf(cert db.Certificate) certView {
	v := certView{Certificate: cert, Domains: []string{}, ReferencedBy: []string{}}
	pemText, err := a.readCertPEM(cert)
	if err != nil {
		v.CertError = err.Error()
		return v
	}
	meta, err := parseCertPEM(pemText)
	if err != nil {
		v.CertError = err.Error()
		return v
	}
	v.CommonName = meta.commonName
	v.Domains = meta.domains
	notAfter := meta.notAfter
	v.NotAfter = &notAfter
	days := daysUntil(notAfter)
	v.DaysLeft = &days
	return v
}

// readCertPEM 读取证书 PEM 文本：managed 读面板证书目录内文件，path 读服务器绝对路径。
func (a *API) readCertPEM(cert db.Certificate) ([]byte, error) {
	p := cert.CertPath
	if cert.Mode == singbox.CertModeManaged {
		p = filepath.Join(a.certDir(), cert.CertPath)
	}
	if p == "" {
		return nil, fmt.Errorf("未配置证书路径")
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("证书文件不可读: %s", p)
	}
	return raw, nil
}

// certMeta 从证书 PEM 解析出的展示信息。
type certMeta struct {
	commonName string
	domains    []string
	notAfter   time.Time
}

// parseCertPEM 解析证书 PEM 文本（取首个 CERTIFICATE 块），提取 CN、域名列表与到期时间。
func parseCertPEM(pemText []byte) (certMeta, error) {
	block, _ := pem.Decode(pemText)
	if block == nil || block.Type != "CERTIFICATE" {
		return certMeta{}, fmt.Errorf("证书文件不是合法的 PEM")
	}
	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return certMeta{}, fmt.Errorf("证书内容解析失败: %v", err)
	}
	domains := make([]string, 0, 1+len(parsed.DNSNames)+len(parsed.IPAddresses))
	seen := map[string]bool{}
	for _, d := range append([]string{parsed.Subject.CommonName}, parsed.DNSNames...) {
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		domains = append(domains, d)
	}
	for _, ip := range parsed.IPAddresses {
		if !seen[ip.String()] {
			domains = append(domains, ip.String())
		}
	}
	return certMeta{commonName: parsed.Subject.CommonName, domains: domains, notAfter: parsed.NotAfter}, nil
}

// daysUntil 距 t 还剩的整天数（向零截断：不足一天按 0，已过期为负）。
// 是否已过期由前端比对 notAfter 判定，不依赖这里的符号。
func daysUntil(t time.Time) int {
	return int(t.Sub(time.Now()).Hours() / 24)
}

// generateSelfSigned 用标准库生成自签服务器证书与私钥 PEM。
// 主域名作为 CN 并并入 SAN；SANs 中的 IP 入 IPAddresses，其余并入 DNSNames。
func generateSelfSigned(domain string, sans []string, days int, keyType string) (certPEM, keyPEM string, err error) {
	if domain == "" {
		return "", "", fmt.Errorf("域名不能为空")
	}
	if net.ParseIP(domain) != nil {
		return "", "", fmt.Errorf("主域名不能是 IP，IP 请填入附加 SAN")
	}
	if days < 1 || days > 36500 {
		return "", "", fmt.Errorf("有效期须在 1-36500 天之间")
	}
	if keyType == "" {
		keyType = KeyTypeP256
	}
	var (
		signer       crypto.Signer
		keyDER       []byte
		keyBlockType string
	)
	switch keyType {
	case KeyTypeP256:
		key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			return "", "", fmt.Errorf("生成私钥失败: %w", e)
		}
		signer = key
		keyDER, e = x509.MarshalECPrivateKey(key)
		if e != nil {
			return "", "", fmt.Errorf("序列化私钥失败: %w", e)
		}
		keyBlockType = "EC PRIVATE KEY"
	case KeyTypeRSA2048:
		key, e := rsa.GenerateKey(rand.Reader, 2048)
		if e != nil {
			return "", "", fmt.Errorf("生成私钥失败: %w", e)
		}
		signer = key
		keyDER = x509.MarshalPKCS1PrivateKey(key)
		keyBlockType = "RSA PRIVATE KEY"
	case KeyTypeEd25519:
		_, priv, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return "", "", fmt.Errorf("生成私钥失败: %w", e)
		}
		signer = priv
		keyDER, e = x509.MarshalPKCS8PrivateKey(priv)
		if e != nil {
			return "", "", fmt.Errorf("序列化私钥失败: %w", e)
		}
		keyBlockType = "PRIVATE KEY"
	default:
		return "", "", fmt.Errorf("密钥类型须为 %s 之一", strings.Join(SupportedCertKeyTypes, " / "))
	}
	now := time.Now()
	serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if e != nil {
		return "", "", fmt.Errorf("生成序列号失败: %w", e)
	}
	tpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: domain},
		NotBefore:             now.Add(-time.Hour), // 容忍服务器与客户端时钟偏差
		NotAfter:              now.Add(time.Duration(days) * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	tpl.DNSNames = []string{domain}
	for _, san := range sans {
		san = strings.TrimSpace(san)
		if san == "" || san == domain || contains(tpl.DNSNames, san) {
			continue
		}
		if ip := net.ParseIP(san); ip != nil {
			tpl.IPAddresses = append(tpl.IPAddresses, ip)
		} else {
			tpl.DNSNames = append(tpl.DNSNames, san)
		}
	}
	der, e := x509.CreateCertificate(rand.Reader, tpl, tpl, signer.Public(), signer)
	if e != nil {
		return "", "", fmt.Errorf("生成证书失败: %w", e)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: keyBlockType, Bytes: keyDER}))
	return certPEM, keyPEM, nil
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
