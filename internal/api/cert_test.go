package api

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// testCertModel 构造证书记录（视图测试用）。
func testCertModel(mode, certPath, keyPath string) db.Certificate {
	return db.Certificate{Name: "test", Mode: mode, CertPath: certPath, KeyPath: keyPath}
}

// publicKeysEqual 按密钥类型比较两个公钥（私钥与证书配对校验用）。
func publicKeysEqual(a, b any) bool {
	switch x := a.(type) {
	case *ecdsa.PublicKey:
		y, ok := b.(*ecdsa.PublicKey)
		return ok && x.Equal(y)
	case *rsa.PublicKey:
		y, ok := b.(*rsa.PublicKey)
		return ok && x.Equal(y)
	case ed25519.PublicKey:
		y, ok := b.(ed25519.PublicKey)
		return ok && x.Equal(y)
	}
	return false
}

// parseGenerated 解析 generateSelfSigned 的产物，返回证书与私钥公钥（配对校验用）。
func parseGenerated(t *testing.T, certPEM, keyPEM string) (*x509.Certificate, any) {
	t.Helper()
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("证书应可解析为 CERTIFICATE PEM 块")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("证书 DER 解析失败: %v", err)
	}
	keyBlock, _ := pem.Decode([]byte(keyPEM))
	if keyBlock == nil {
		t.Fatal("私钥应为合法 PEM 块")
	}
	var pub any
	switch keyBlock.Type {
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
		if err != nil {
			t.Fatalf("解析 EC 私钥失败: %v", err)
		}
		pub = &key.PublicKey
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			t.Fatalf("解析 RSA 私钥失败: %v", err)
		}
		pub = &key.PublicKey
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err != nil {
			t.Fatalf("解析 PKCS8 私钥失败: %v", err)
		}
		switch k := key.(type) {
		case ed25519.PrivateKey:
			pub = k.Public()
		default:
			t.Fatalf("PKCS8 私钥应为 Ed25519: %T", key)
		}
	default:
		t.Fatalf("私钥 PEM 类型不符: %s", keyBlock.Type)
	}
	return cert, pub
}

// TestGenerateSelfSigned 自签生成：证书可解析、CN/SAN 正确、有效期正确、私钥可配对。
func TestGenerateSelfSigned(t *testing.T) {
	const domain = "example.com"
	const days = 90
	sans := []string{"www.example.com", "10.0.0.8", " example.com ", "10.0.0.8"} // 含 IP、需去空白、主域名与 IP 重复项
	genLower := time.Now().Add(-2 * time.Hour)
	certPEM, keyPEM, err := generateSelfSigned(domain, sans, days, KeyTypeP256)
	if err != nil {
		t.Fatalf("生成自签证书失败: %v", err)
	}
	cert, pub := parseGenerated(t, certPEM, keyPEM)

	if cert.Subject.CommonName != domain {
		t.Errorf("CN = %q, want %q", cert.Subject.CommonName, domain)
	}
	if want := []string{domain, "www.example.com"}; len(cert.DNSNames) != 2 || cert.DNSNames[0] != want[0] || cert.DNSNames[1] != want[1] {
		t.Errorf("DNS SAN = %v, want %v", cert.DNSNames, want)
	}
	if len(cert.IPAddresses) != 1 || cert.IPAddresses[0].String() != "10.0.0.8" {
		t.Errorf("IP SAN = %v, want [10.0.0.8]", cert.IPAddresses)
	}
	if cert.NotBefore.Before(genLower) || cert.NotBefore.After(time.Now()) {
		t.Errorf("NotBefore = %v, 应在生成时刻前 1 小时（容忍时钟偏差）", cert.NotBefore)
	}
	wantNotAfter := time.Now().Add(days * 24 * time.Hour)
	if diff := cert.NotAfter.Sub(wantNotAfter); diff < -time.Hour || diff > time.Hour {
		t.Errorf("NotAfter = %v, 应为生成时刻 + %d 天（偏差 %v）", cert.NotAfter, days, diff)
	}
	if !publicKeysEqual(pub, cert.PublicKey) {
		t.Error("私钥应与证书公钥配对")
	}
}

// TestGenerateSelfSignedKeyTypes 三种密钥类型均可生成且私钥可配对。
func TestGenerateSelfSignedKeyTypes(t *testing.T) {
	for _, keyType := range SupportedCertKeyTypes {
		t.Run(keyType, func(t *testing.T) {
			certPEM, keyPEM, err := generateSelfSigned("example.com", nil, 30, keyType)
			if err != nil {
				t.Fatalf("生成失败: %v", err)
			}
			cert, pub := parseGenerated(t, certPEM, keyPEM)
			if !publicKeysEqual(pub, cert.PublicKey) {
				t.Error("私钥应与证书公钥配对")
			}
		})
	}
	// 缺省密钥类型为 p256
	_, keyPEM, err := generateSelfSigned("example.com", nil, 30, "")
	if err != nil {
		t.Fatalf("缺省密钥类型生成失败: %v", err)
	}
	block, _ := pem.Decode([]byte(keyPEM))
	if key, err := x509.ParseECPrivateKey(block.Bytes); err != nil || key.Curve != elliptic.P256() {
		t.Errorf("缺省密钥应为 ECDSA P-256: %v", err)
	}
	_, _, err = generateSelfSigned("example.com", nil, 30, "bad-type")
	if err == nil || !strings.Contains(err.Error(), "密钥类型") {
		t.Errorf("非法密钥类型应报错, got %v", err)
	}
}

// TestGenerateSelfSignedValidation 域名与有效期校验。
func TestGenerateSelfSignedValidation(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		days   int
		want   string
	}{
		{"域名为空", "", 30, "域名"},
		{"主域名为 IP", "1.2.3.4", 30, "IP"},
		{"有效期为零", "example.com", 0, "有效期"},
		{"有效期超长", "example.com", 36501, "有效期"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := generateSelfSigned(tt.domain, nil, tt.days, KeyTypeP256)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("应报含 %q 的错误, got %v", tt.want, err)
			}
		})
	}
}

// TestParseCertPEMAndDaysUntil 到期计算：剩余天数向零截断、已过期为负、非证书 PEM 报可读错误。
func TestParseCertPEMAndDaysUntil(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		offset   time.Duration
		wantDays int
	}{
		{"刚生成 3 天余 2", 3 * 24 * time.Hour, 2},
		{"不足一天余 0", 23 * time.Hour, 0},
		{"已过期半天为 0", -12 * time.Hour, 0},
		{"已过期两天为 -2", -2*24*time.Hour - time.Hour, -2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := daysUntil(now.Add(tt.offset)); got != tt.wantDays {
				t.Errorf("daysUntil = %d, want %d", got, tt.wantDays)
			}
		})
	}

	// 用真实自签证书走一遍 parseCertPEM
	certPEM, _, err := generateSelfSigned("example.com", nil, 365, KeyTypeP256)
	if err != nil {
		t.Fatalf("生成证书失败: %v", err)
	}
	meta, err := parseCertPEM([]byte(certPEM))
	if err != nil {
		t.Fatalf("解析证书失败: %v", err)
	}
	if meta.commonName != "example.com" || len(meta.domains) != 1 || meta.domains[0] != "example.com" {
		t.Errorf("解析结果不符: %+v", meta)
	}
	if days := daysUntil(meta.notAfter); days < 364 || days > 365 {
		t.Errorf("一年期证书剩余天数 = %d, want 364-365", days)
	}

	// 非证书内容给出可读错误而不是 panic
	if _, err := parseCertPEM([]byte("not a pem")); err == nil || !strings.Contains(err.Error(), "PEM") {
		t.Errorf("非 PEM 内容应报可读错误, got %v", err)
	}
	if _, err := parseCertPEM(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1}})); err == nil {
		t.Error("私钥 PEM 不应被当作证书解析成功")
	}
}

// TestCertViewOf 证书视图：managed / path 两种来源补充到期字段，读不到文件时给出可读字段。
func TestCertViewOf(t *testing.T) {
	dataDir := t.TempDir()
	a := &API{dataDir: dataDir}
	certPEM, keyPEM, err := generateSelfSigned("example.com", []string{"www.example.com"}, 30, KeyTypeP256)
	if err != nil {
		t.Fatalf("生成证书失败: %v", err)
	}
	certName, keyName, err := a.writeCertFiles(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("写入证书文件失败: %v", err)
	}

	// managed：从面板证书目录读取并解析
	view := a.certViewOf(testCertModel("managed", certName, keyName))
	if view.CertError != "" {
		t.Fatalf("managed 解析不应出错: %s", view.CertError)
	}
	if view.CommonName != "example.com" {
		t.Errorf("CN = %q", view.CommonName)
	}
	if len(view.Domains) != 2 {
		t.Errorf("Domains = %v, want CN+SAN 两个域名", view.Domains)
	}
	if view.NotAfter == nil || view.DaysLeft == nil || *view.DaysLeft < 29 || *view.DaysLeft > 30 {
		t.Errorf("到期字段不符: notAfter=%v daysLeft=%v", view.NotAfter, view.DaysLeft)
	}

	// path：直接读服务器路径
	pathCert := filepath.Join(dataDir, "srv-cert.pem")
	if err := os.WriteFile(pathCert, []byte(certPEM), 0o600); err != nil {
		t.Fatal(err)
	}
	view = a.certViewOf(testCertModel("path", pathCert, pathCert))
	if view.CertError != "" || view.NotAfter == nil {
		t.Fatalf("path 解析结果不符: %+v", view)
	}

	// 文件不可读：给可读字段而非报错
	view = a.certViewOf(testCertModel("managed", "missing.pem", "missing.pem"))
	if view.CertError == "" || view.NotAfter != nil || view.DaysLeft != nil {
		t.Errorf("不可读证书应有 CertError 且无到期字段: %+v", view)
	}
}

// TestHandleCertDelete 删除证书：先删库记录、成功后再删文件，两处都生效；
// 证书文件已缺失时删除仍成功（孤儿文件场景可接受）。
func TestHandleCertDelete(t *testing.T) {
	a := newInboundTestAPI(t)

	certPEM, keyPEM, err := generateSelfSigned("example.com", nil, 30, KeyTypeP256)
	if err != nil {
		t.Fatalf("生成证书失败: %v", err)
	}
	certName, keyName, err := a.writeCertFiles(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("写入证书文件失败: %v", err)
	}
	cert := db.Certificate{Name: "del", Mode: singbox.CertModeManaged, CertPath: certName, KeyPath: keyName}
	if err := a.db.Create(&cert).Error; err != nil {
		t.Fatalf("创建证书记录失败: %v", err)
	}
	id := strconv.FormatUint(uint64(cert.ID), 10)
	c, w := ginCtx(http.MethodDelete, "/api/certs/"+id, "", gin.Params{{Key: "id", Value: id}})
	a.HandleCertDelete(c)
	if w.Code != http.StatusOK {
		t.Fatalf("删除证书应成功, got %d: %s", w.Code, w.Body.String())
	}
	var count int64
	if err := a.db.Model(&db.Certificate{}).Where("id = ?", cert.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Error("证书记录应已删除")
	}
	for _, name := range []string{certName, keyName} {
		if _, err := os.Stat(filepath.Join(a.certDir(), name)); !os.IsNotExist(err) {
			t.Errorf("证书文件应已删除: %s (%v)", name, err)
		}
	}

	// 文件已缺失（如被手动清掉）时只剩记录也能删干净，不应报错
	missing := db.Certificate{Name: "no-files", Mode: singbox.CertModeManaged, CertPath: "gone-cert.pem", KeyPath: "gone-key.pem"}
	if err := a.db.Create(&missing).Error; err != nil {
		t.Fatalf("创建证书记录失败: %v", err)
	}
	id = strconv.FormatUint(uint64(missing.ID), 10)
	c, w = ginCtx(http.MethodDelete, "/api/certs/"+id, "", gin.Params{{Key: "id", Value: id}})
	a.HandleCertDelete(c)
	if w.Code != http.StatusOK {
		t.Fatalf("文件缺失时删除证书仍应成功, got %d: %s", w.Code, w.Body.String())
	}
	if err := a.db.Model(&db.Certificate{}).Where("id = ?", missing.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Error("文件缺失时证书记录也应删除")
	}
}
