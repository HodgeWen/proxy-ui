package singbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeClashAPI 模拟 sing-box Clash API：/traffic 输出一行速率样本，
// /connections 返回 connectionsJSON 快照（secret 非空时校验 Bearer 认证）。
func fakeClashAPI(t *testing.T, secret, connectionsJSON string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/traffic", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"up":1024,"down":2048}` + "\n"))
	})
	mux.HandleFunc("/connections", func(w http.ResponseWriter, r *http.Request) {
		if secret != "" && r.Header.Get("Authorization") != "Bearer "+secret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(connectionsJSON))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// managerWithClash 构造 Manager 并把 config.json 的 clash_api 指向测试服务。
func managerWithClash(t *testing.T, addr, secret string) *Manager {
	t.Helper()
	m := New(t.TempDir())
	cfg := MinConfig()
	cfg.Experimental.ClashAPI.ExternalController = addr
	cfg.Experimental.ClashAPI.Secret = secret
	if err := m.WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig 失败: %v", err)
	}
	return m
}

func TestClashClientTraffic(t *testing.T) {
	server := fakeClashAPI(t, "", `{}`)
	m := managerWithClash(t, server.Listener.Addr().String(), "")
	client := m.ClashClient()
	if client == nil {
		t.Fatal("应能从 config.json 解析 Clash API 端点")
	}
	sample, err := client.Traffic(context.Background())
	if err != nil {
		t.Fatalf("Traffic 失败: %v", err)
	}
	if sample.Up != 1024 || sample.Down != 2048 {
		t.Errorf("速率样本不符: %+v", sample)
	}
}

func TestClashClientConnections(t *testing.T) {
	payload := `{
		"downloadTotal": 5000,
		"uploadTotal": 4000,
		"memory": 1048576,
		"connections": [
			{
				"id": "conn-1",
				"metadata": {
					"network": "tcp",
					"type": "hysteria2/hy-in",
					"sourceIP": "1.2.3.4",
					"destinationIP": "5.6.7.8",
					"sourcePort": "52000",
					"destinationPort": "443",
					"host": "example.com"
				},
				"upload": 100,
				"download": 900,
				"start": "2026-11-19T08:00:00Z",
				"chains": ["hy-in", "direct"],
				"rule": "geosite-cn => direct"
			},
			{
				"id": "conn-2",
				"metadata": {
					"network": "udp",
					"type": "direct",
					"sourceIP": "1.2.3.4",
					"destinationIP": "9.9.9.9",
					"sourcePort": "53000",
					"destinationPort": "53",
					"host": ""
				},
				"upload": 50,
				"download": 250,
				"start": "2026-11-19T08:00:01Z",
				"chains": ["direct"],
				"rule": "final"
			}
		]
	}`
	server := fakeClashAPI(t, "s3cret", payload)
	m := managerWithClash(t, server.Listener.Addr().String(), "s3cret")
	client := m.ClashClient()

	snap, err := client.Connections(context.Background())
	if err != nil {
		t.Fatalf("Connections 失败: %v", err)
	}
	if snap.UploadTotal != 4000 || snap.DownloadTotal != 5000 || snap.Memory != 1048576 {
		t.Errorf("快照总量不符: %+v", snap)
	}
	if len(snap.Connections) != 2 {
		t.Fatalf("连接数不符: %d", len(snap.Connections))
	}
	first := snap.Connections[0]
	if first.Inbound != "hy-in" || first.Network != "tcp" {
		t.Errorf("首连接解析不符: %+v", first)
	}
	if first.Source != "1.2.3.4:52000" || first.Destination != "example.com:443" || first.Host != "example.com" {
		t.Errorf("首连接地址解析不符: %+v", first)
	}
	if first.Upload != 100 || first.Download != 900 || first.Chain[1] != "direct" {
		t.Errorf("首连接计数不符: %+v", first)
	}
	second := snap.Connections[1]
	if second.Inbound != "" || second.Destination != "9.9.9.9:53" {
		t.Errorf("无 tag 连接解析不符: %+v", second)
	}
}

func TestClashClientAuth(t *testing.T) {
	server := fakeClashAPI(t, "s3cret", `{"connections":[]}`)

	// 密钥错误：401 应转为错误
	wrong := managerWithClash(t, server.Listener.Addr().String(), "wrong")
	if _, err := wrong.ClashClient().Connections(context.Background()); err == nil {
		t.Error("错误密钥应被拒绝")
	}

	// 密钥正确：认证通过
	right := managerWithClash(t, server.Listener.Addr().String(), "s3cret")
	if _, err := right.ClashClient().Connections(context.Background()); err != nil {
		t.Errorf("正确密钥不应报错: %v", err)
	}

	// WriteConfig 后按新配置重新解析端点
	cfg := MinConfig()
	cfg.Experimental.ClashAPI.ExternalController = server.Listener.Addr().String()
	if err := right.WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig 失败: %v", err)
	}
	client := right.ClashClient()
	if client == nil || client.secret != "" {
		t.Fatalf("WriteConfig 后应按新配置解析: %+v", client)
	}
	if _, err := client.Connections(context.Background()); err == nil {
		t.Error("新配置无密钥，请求带认证的服务应失败")
	}
}

func TestClashClientUnavailable(t *testing.T) {
	if client := New(t.TempDir()).ClashClient(); client != nil {
		t.Errorf("配置缺失时应返回 nil, got %+v", client)
	}
	m := managerWithClash(t, "127.0.0.1:1", "")
	client := m.ClashClient()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := client.Traffic(ctx); err == nil {
		t.Error("不可达的 Clash API 应报错")
	}
	if _, err := client.Connections(ctx); err == nil {
		t.Error("不可达的 Clash API 应报错")
	}
}

func TestInboundTagOf(t *testing.T) {
	if got := inboundTagOf("vless/my-tag"); got != "my-tag" {
		t.Errorf(`inboundTagOf("vless/my-tag") = %q`, got)
	}
	if got := inboundTagOf("direct"); got != "" {
		t.Errorf("无 tag 的类型应返回空, got %q", got)
	}
}

// 确保 ConnectionsSnapshot 可直接 JSON 序列化（API 透传依赖）。
func TestSnapshotJSONKeys(t *testing.T) {
	snap := ConnectionsSnapshot{UploadTotal: 1, Memory: 2, Connections: []Connection{{ID: "x", Inbound: "in"}}}
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	for _, key := range []string{"uploadTotal", "downloadTotal", "memory", "connections"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("序列化缺少字段 %q: %s", key, data)
		}
	}
}
