package singbox

import "testing"

// TestUserCounter 只接受 user 维度计数器（入站维度不参与按用户统计，避免与用户同名互串）。
func TestUserCounter(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"用户上行", "user>>>alice>>>traffic>>>uplink", "alice", true},
		{"用户下行", "user>>>alice>>>traffic>>>downlink", "alice", true},
		{"带中文用户名", "user>>>手机-美国>>>traffic>>>uplink", "手机-美国", true},
		{"入站维度被排除", "inbound>>>vless-in>>>traffic>>>uplink", "", false},
		{"空串", "", "", false},
		{"段数不足", "user>>>alice>>>uplink", "", false},
		{"段数过多", "user>>>alice>>>traffic>>>uplink>>>extra", "", false},
		{"非 traffic 段", "user>>>alice>>>speed>>>uplink", "", false},
		{"分隔符不符", "user.alice.traffic.uplink", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := userCounter(tt.in)
			if ok != tt.ok {
				t.Fatalf("userCounter(%q) ok = %v, want %v", tt.in, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("userCounter(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestIsUplink 上行计数器判定。
func TestIsUplink(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"user>>>alice>>>traffic>>>uplink", true},
		{"inbound>>>vless-in>>>traffic>>>uplink", true},
		{"user>>>alice>>>traffic>>>downlink", false},
		{"", false},
		{"uplink", false},
	}
	for _, tt := range tests {
		if got := isUplink(tt.in); got != tt.want {
			t.Errorf("isUplink(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// TestMinConfig 最小配置形状：无入站、direct 兜底、统计与缓存。
func TestMinConfig(t *testing.T) {
	cfg := MinConfig()
	if len(cfg.Inbounds) != 0 {
		t.Errorf("最小配置不应包含入站: %v", cfg.Inbounds)
	}
	if len(cfg.Outbounds) != 1 || cfg.Outbounds[0]["tag"] != "direct" || cfg.Outbounds[0]["type"] != "direct" {
		t.Errorf("最小配置应仅有 direct 兜底出站: %v", cfg.Outbounds)
	}
	if cfg.Log == nil || cfg.Log.Level != "info" || !cfg.Log.Timestamp {
		t.Errorf("最小配置日志段不符: %+v", cfg.Log)
	}
	if cfg.Experimental == nil || cfg.Experimental.V2RayAPI == nil || cfg.Experimental.V2RayAPI.Stats == nil {
		t.Fatalf("最小配置缺少统计服务: %+v", cfg.Experimental)
	}
	if !cfg.Experimental.V2RayAPI.Stats.Enabled {
		t.Error("最小配置应启用统计")
	}
	if cfg.Experimental.V2RayAPI.Listen != StatsListen() {
		t.Errorf("统计监听地址 = %q, want %q", cfg.Experimental.V2RayAPI.Listen, StatsListen())
	}
	if cfg.Experimental.CacheFile == nil || !cfg.Experimental.CacheFile.Enabled {
		t.Error("最小配置应启用缓存文件")
	}
	if cfg.Route != nil || cfg.DNS != nil {
		t.Errorf("最小配置不应含 route/dns 段: %+v %+v", cfg.Route, cfg.DNS)
	}
}
