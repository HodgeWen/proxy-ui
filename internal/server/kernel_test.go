package server

import (
	"net/http"
	"testing"
	"time"
)

// kernelStatusResp 内核状态视图（测试断言用）。
type kernelStatusResp struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Embedded  bool   `json:"embedded"`
	Version   string `json:"version"`
	Uptime    int64  `json:"uptimeSeconds"`
}

// waitKernelRunning 轮询内核状态直到运行中（启动与热应用都是异步后台完成的）。
func waitKernelRunning(t *testing.T, client *http.Client, srvURL string) kernelStatusResp {
	t.Helper()
	var status kernelStatusResp
	for i := 0; i < 100; i++ {
		code, env := doJSON(t, client, http.MethodGet, srvURL+"/api/kernel/status", nil)
		if code != http.StatusOK {
			t.Fatalf("查询内核状态失败: code=%d msg=%s", code, env.Msg)
		}
		decodeData(t, env, &status)
		if status.Running {
			return status
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("内核未在预期时间内运行: %+v", status)
	return status
}

// TestKernelAutoStarts 面板启动后无需任何操作内核即处于运行状态，且库里没有入站时以最小配置启动。
func TestKernelAutoStarts(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	status := waitKernelRunning(t, client, srv.URL)
	if !status.Installed || !status.Embedded || status.Version == "" {
		t.Errorf("内置内核状态不符: %+v", status)
	}
}

// TestKernelHotApplyOnWrite 写操作成功后内核按新配置重建，无需任何「应用配置」调用。
// 同时确认四个内核启停/应用接口已不存在。
func TestKernelHotApplyOnWrite(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)
	waitKernelRunning(t, client, srv.URL)

	// 新建入站（带一个用户）→ 请求成功即说明新配置已能启动
	certID := createTestCert(t, client, srv.URL)
	in := createInbound(t, client, srv.URL, map[string]any{
		"name": "hot-apply-in", "protocol": "trojan",
		"config": map[string]any{
			"tls": map[string]any{"enabled": true, "certMode": "managed", "certId": certID},
		},
	})
	createToken(t, client, srv.URL, map[string]any{"name": "hot-user", "nodeInbounds": []uint{in.ID}})

	status := waitKernelRunning(t, client, srv.URL)
	if status.Uptime < 0 {
		t.Errorf("运行时长不应为负: %+v", status)
	}

	// 内核日志里应有每次重建的启动记录（证明真的按新配置重建过）
	code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/kernel/logs?limit=50", nil)
	if code != http.StatusOK {
		t.Fatalf("查询内核日志失败: %s", env.Msg)
	}

	// 四个已删除的内核控制接口一律 404
	for _, tt := range []struct{ method, path string }{
		{http.MethodPost, "/api/kernel/start"},
		{http.MethodPost, "/api/kernel/stop"},
		{http.MethodPost, "/api/kernel/restart"},
		{http.MethodPost, "/api/kernel/apply"},
	} {
		if code, _ := doJSON(t, client, tt.method, srv.URL+tt.path, map[string]any{}); code != http.StatusNotFound {
			t.Errorf("%s %s 应 404（接口已删除）, got %d", tt.method, tt.path, code)
		}
	}
}

// TestKernelApplyFailureReportsReason 新配置无法启动时该次保存请求返回失败原因，
// 且内核回到上一份可用配置继续运行（数据库改动保留）。
func TestKernelApplyFailureReportsReason(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)
	waitKernelRunning(t, client, srv.URL)

	// Hysteria2 强制 TLS：不给证书 → 聚合配置失败。
	// 新建的入站还没有用户时不会进入内核配置，因此先建入站、再加用户，让失败发生在加用户这一步。
	in := createInbound(t, client, srv.URL, map[string]any{
		"name": "broken-hy2", "protocol": "hysteria2",
		"config": map[string]any{
			"tls": map[string]any{"enabled": true, "certMode": "managed", "certId": 0},
		},
	})
	code, env := doJSON(t, client, http.MethodPost,
		srv.URL+"/api/inbounds/"+itoa(in.ID)+"/users", map[string]any{"name": "broken-hy2-user"})
	if code == http.StatusOK {
		t.Fatal("缺少证书的 Hysteria2 入站应因内核重建失败而返回错误")
	}
	if env.Msg == "" {
		t.Error("失败响应应带上原因")
	}

	// 内核仍在运行（回滚到上一份可用配置 / 未受影响）
	status := waitKernelRunning(t, client, srv.URL)
	if !status.Running {
		t.Errorf("重建失败后内核应仍运行: %+v", status)
	}
}
