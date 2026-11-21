package server

import (
	"net/http"
	"strings"
	"testing"

	"proxy-ui/internal/db"
)

// TestUserNameGloballyUnique 用户名是流量统计的键：必须全局唯一，且不得等于任何入站 tag。
// 入站用户统一由订阅勾选生成，同名订阅在同一入站下必须得到自动加序号的不同用户名。
func TestUserNameGloballyUnique(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	in1 := createInbound(t, client, srv.URL, map[string]any{"name": "in-a", "protocol": "shadowsocks"})
	in2 := createInbound(t, client, srv.URL, map[string]any{"name": "in-b", "protocol": "shadowsocks"})

	// 同名订阅用户勾选同一入站：用户名按「备注-入站名」生成，冲突时自动追加序号
	first := createToken(t, client, srv.URL, map[string]any{"name": "dup", "nodeInbounds": []uint{in1.ID}})
	second := createToken(t, client, srv.URL, map[string]any{"name": "dup", "nodeInbounds": []uint{in1.ID}})
	if len(first.Nodes) != 1 || len(second.Nodes) != 1 {
		t.Fatalf("两个订阅应各有 1 个节点: %+v %+v", first.Nodes, second.Nodes)
	}
	if first.Nodes[0].Name == second.Nodes[0].Name {
		t.Fatalf("同名订阅的用户名必须不同（否则用量互相覆盖）: %q", first.Nodes[0].Name)
	}
	if want := "dup-in-a"; first.Nodes[0].Name != want {
		t.Errorf("首个用户名 = %q, want %q", first.Nodes[0].Name, want)
	}

	// 入站改名为已有用户名同样要拒绝（入站名与用户名共用流量表的键）
	code, env := doJSON(t, client, http.MethodPut,
		srv.URL+"/api/inbounds/"+itoa(in2.ID), map[string]any{"name": first.Nodes[0].Name})
	if code != http.StatusBadRequest {
		t.Fatalf("入站改名为已有用户名应被拒绝, code=%d msg=%s", code, env.Msg)
	}
}

// TestQuotaUsesTokenTraffic 配额计数绑定在订阅用户上：取消/重新勾选入站（用户被重建）
// 不得把已用流量清零，清零操作必须真正生效。
func TestQuotaUsesTokenTraffic(t *testing.T) {
	srv, dbPath := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	in := createInbound(t, client, srv.URL, map[string]any{"name": "q-in", "protocol": "shadowsocks"})
	const limit int64 = 1 << 20
	view := createToken(t, client, srv.URL, map[string]any{
		"name": "tu", "nodeInbounds": []uint{in.ID}, "trafficLimitBytes": limit,
	})

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	if err := database.Create(&db.TokenTraffic{
		TokenID: view.ID, UploadBytes: limit, DownloadBytes: limit,
	}).Error; err != nil {
		t.Fatalf("写入流量失败: %v", err)
	}

	// 取消勾选：用量必须保留（早先删掉用户即丢了用量，导致配额被重置）
	code, env := doJSON(t, client, http.MethodPut,
		srv.URL+"/api/subscription/tokens/"+itoa(view.ID), map[string]any{"nodeInbounds": []uint{}})
	if code != http.StatusOK {
		t.Fatalf("取消勾选失败: %s", env.Msg)
	}
	var after tokenResp
	decodeData(t, env, &after)
	if after.UsedBytes < limit*2 {
		t.Fatalf("取消勾选后用量不应清零: %d", after.UsedBytes)
	}

	// 重新勾选：用量仍然保留
	code, env = doJSON(t, client, http.MethodPut,
		srv.URL+"/api/subscription/tokens/"+itoa(view.ID), map[string]any{"nodeInbounds": []uint{in.ID}})
	if code != http.StatusOK {
		t.Fatalf("重新勾选失败: %s", env.Msg)
	}
	decodeData(t, env, &after)
	if after.UsedBytes < limit*2 || !after.OverQuota {
		t.Fatalf("重新勾选后用量应保留且仍超额: used=%d over=%v", after.UsedBytes, after.OverQuota)
	}
	if len(after.Nodes) != 1 || !after.Nodes[0].Suspended {
		t.Fatalf("超额用户应被暂停: %+v", after.Nodes)
	}

	// 清零：必须真正解除暂停（早先按用户名删流量，取消勾选后就删不到东西）
	code, env = doJSON(t, client, http.MethodPost,
		srv.URL+"/api/subscription/tokens/"+itoa(view.ID)+"/reset-traffic", map[string]any{})
	if code != http.StatusOK {
		t.Fatalf("清零流量失败: %s", env.Msg)
	}
	decodeData(t, env, &after)
	if after.UsedBytes != 0 || after.OverQuota {
		t.Fatalf("清零后用量应为 0: used=%d over=%v", after.UsedBytes, after.OverQuota)
	}
	if len(after.Nodes) != 1 || after.Nodes[0].Suspended || !after.Nodes[0].Enabled {
		t.Fatalf("清零后用户应恢复: %+v", after.Nodes)
	}
}

// TestShadowsocksInboundGetsPSK 从界面创建的 Shadowsocks 入站必须自动获得入站 PSK，
// 且方法不受支持时明确报错（多用户只支持两种 AES 方法）。
func TestShadowsocksInboundGetsPSK(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	// 模拟界面提交的配置（不含 PSK）
	code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/inbounds", map[string]any{
		"protocol": "shadowsocks", "name": "ss-in",
		"config": map[string]any{"method": "2022-blake3-aes-128-gcm"},
	})
	if code != http.StatusOK {
		t.Fatalf("创建 Shadowsocks 入站失败: %s", env.Msg)
	}
	var in struct {
		Config struct {
			Method string `json:"method"`
			PSK    string `json:"psk"`
		} `json:"config"`
	}
	decodeData(t, env, &in)
	if in.Config.PSK == "" {
		t.Fatal("入站应自动获得 PSK（否则客户端无法用 psk:userkey 认证、内核拒绝启动）")
	}

	// 不支持多用户的方法必须拒绝
	code, env = doJSON(t, client, http.MethodPost, srv.URL+"/api/inbounds", map[string]any{
		"protocol": "shadowsocks", "name": "ss-bad",
		"config": map[string]any{"method": "2022-blake3-chacha20-poly1305"},
	})
	if code != http.StatusBadRequest {
		t.Fatalf("chacha20 方法应被拒绝, code=%d msg=%s", code, env.Msg)
	}
}

// TestSubscriptionWithoutNodes 没有可用节点时订阅应明确报错，
// 而不是下发一份"全部直连"的空配置让客户端误以为订阅正常。
func TestSubscriptionWithoutNodes(t *testing.T) {
	srv, _ := newTestServer(t)
	client := setupAdmin(t, srv.URL)

	in := createInbound(t, client, srv.URL, map[string]any{"name": "n-in", "protocol": "shadowsocks"})
	view := createToken(t, client, srv.URL, map[string]any{
		"name": "no-node", "nodeInbounds": []uint{in.ID},
	})
	// 取消全部勾选 → 该订阅不再有节点
	if code, env := doJSON(t, client, http.MethodPut,
		srv.URL+"/api/subscription/tokens/"+itoa(view.ID), map[string]any{"nodeInbounds": []uint{}}); code != http.StatusOK {
		t.Fatalf("取消勾选失败: %s", env.Msg)
	}
	code, _, body := getSub(t, srv.URL+"/sub/"+view.Token, "SFM")
	if code != http.StatusForbidden {
		t.Fatalf("无节点订阅状态码 = %d, want 403: %s", code, body)
	}
	if !strings.Contains(body, "没有可用节点") {
		t.Errorf("应说明没有可用节点: %s", body)
	}
}
