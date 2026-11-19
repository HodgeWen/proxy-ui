package singbox

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMinConfigJSON(t *testing.T) {
	data, err := json.Marshal(MinConfig())
	if err != nil {
		t.Fatalf("序列化最小配置失败: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("最小配置不是合法 JSON: %v", err)
	}
	log, _ := cfg["log"].(map[string]any)
	if log == nil || log["level"] != "info" || log["timestamp"] != true {
		t.Errorf("log 配置不符: %v", cfg["log"])
	}
	outbounds, _ := cfg["outbounds"].([]any)
	if len(outbounds) != 1 {
		t.Fatalf("outbounds 应仅 direct 兜底, got %v", cfg["outbounds"])
	}
	direct, _ := outbounds[0].(map[string]any)
	if direct["type"] != "direct" || direct["tag"] != "direct" {
		t.Errorf("兜底出站应为 direct, got %v", outbounds[0])
	}
	exp, _ := cfg["experimental"].(map[string]any)
	clash, _ := exp["clash_api"].(map[string]any)
	if clash == nil || clash["external_controller"] != "127.0.0.1:19090" {
		t.Errorf("clash_api 预留配置不符: %v", cfg["experimental"])
	}
	// 预留字段未启用时不应输出空对象
	if strings.Contains(string(data), "\"dns\"") || strings.Contains(string(data), "\"inbounds\"") {
		t.Errorf("未启用的预留字段不应输出: %s", data)
	}
}
