package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// newInboundTestAPI 以临时数据库与内核管理器构造 API（不启动内核实例）。
func newInboundTestAPI(t *testing.T) *API {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	return New(database, nil, singbox.New(t.TempDir()), nil, nil, t.TempDir())
}

// TestHandleInboundTemplate 模板端点：七个协议各返回非空默认配置（VLESS 含预生成的
// Reality 密钥材料，前端据此删掉手写 X25519），非法协议返回 400。
func TestHandleInboundTemplate(t *testing.T) {
	a := newInboundTestAPI(t)
	for _, protocol := range singbox.SupportedProtocols {
		c, w := ginCtx(http.MethodGet, "/api/inbounds/template/"+protocol, "",
			gin.Params{{Key: "protocol", Value: protocol}})
		a.HandleInboundTemplate(c)
		if w.Code != http.StatusOK {
			t.Fatalf("%s 模板应返回 200, got %d: %s", protocol, w.Code, w.Body.String())
		}
		var resp struct {
			Data struct {
				Config map[string]json.RawMessage `json:"config"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析 %s 模板响应失败: %v", protocol, err)
		}
		if len(resp.Data.Config) == 0 {
			t.Errorf("%s 模板配置不应为空", protocol)
		}
		if protocol != singbox.ProtoVLESS {
			continue
		}
		var reality struct {
			PrivateKey string `json:"privateKey"`
			PublicKey  string `json:"publicKey"`
			ShortID    string `json:"shortId"`
		}
		if err := json.Unmarshal(resp.Data.Config["reality"], &reality); err != nil {
			t.Fatalf("解析 Reality 段失败: %v", err)
		}
		if reality.PrivateKey == "" || reality.PublicKey == "" || reality.ShortID == "" {
			t.Errorf("VLESS 模板应含预生成的 Reality 密钥材料: %s", w.Body.String())
		}
	}
	c, w := ginCtx(http.MethodGet, "/api/inbounds/template/bogus", "",
		gin.Params{{Key: "protocol", Value: "bogus"}})
	a.HandleInboundTemplate(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法协议应返回 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCreateInboundCompletesRealityKeys 部分配置（Reality 启用但缺私钥，如 AI 建站工具
// 给出的最小配置）保存时应补齐密钥材料，且聚合出的内核配置能通过校验。
func TestCreateInboundCompletesRealityKeys(t *testing.T) {
	a := newInboundTestAPI(t)
	view, err := a.createInbound(inboundReq{
		Name:     "reality-partial",
		Protocol: singbox.ProtoVLESS,
		Config:   json.RawMessage(`{"reality":{"enabled":true,"handshakeServer":"www.microsoft.com","handshakePort":443},"tls":{"enabled":true,"serverName":"www.microsoft.com"}}`),
	})
	if err != nil {
		t.Fatalf("创建入站失败: %v", err)
	}

	var in db.Inbound
	if err := a.db.First(&in, view.ID).Error; err != nil {
		t.Fatalf("查询入站失败: %v", err)
	}
	var saved struct {
		Reality struct {
			PrivateKey string `json:"privateKey"`
			PublicKey  string `json:"publicKey"`
			ShortID    string `json:"shortId"`
		} `json:"reality"`
	}
	if err := json.Unmarshal([]byte(in.Config), &saved); err != nil {
		t.Fatalf("解析落库配置失败: %v", err)
	}
	if saved.Reality.PrivateKey == "" || saved.Reality.PublicKey == "" || saved.Reality.ShortID == "" {
		t.Errorf("落库配置应含补齐的密钥材料: %s", in.Config)
	}

	// 有用户时该入站会进内核配置：补齐后的配置必须能通过内核校验（修复前此处为 invalid private key）
	cred, err := singbox.NewCredential(singbox.ProtoVLESS)
	if err != nil {
		t.Fatal(err)
	}
	rawCred, _ := json.Marshal(cred)
	if err := a.db.Create(&db.InboundUser{InboundID: in.ID, Name: "u1", Credential: string(rawCred), Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	cfg, err := a.buildKernelConfig()
	if err != nil {
		t.Fatalf("聚合内核配置失败: %v", err)
	}
	if err := a.core.CheckConfig(cfg); err != nil {
		t.Fatalf("补齐密钥后的配置应通过内核校验: %v", err)
	}
}
