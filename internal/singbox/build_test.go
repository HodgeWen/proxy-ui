package singbox

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestBuildConfigAggregate(t *testing.T) {
	cfg, err := BuildConfig(
		[]InboundEntry{
			{Tag: "hy2-in", Protocol: ProtoHysteria2, ListenPort: 8443,
				Config: `{"password":"pw","tls":{"enabled":true,"certMode":"managed","certId":7}}`, Cert: stubResolver},
			{Tag: "direct-in", Protocol: ProtoDirect, ListenPort: 1080, Config: `{}`},
		},
		[]OutboundEntry{
			{Tag: "my-proxy", Type: "http", Config: `{"server":"127.0.0.1","server_port":3128,"type":"ignored","tag":"ignored"}`},
		},
	)
	if err != nil {
		t.Fatalf("聚合生成失败: %v", err)
	}
	if len(cfg.Inbounds) != 2 {
		t.Fatalf("应有 2 条入站, got %d", len(cfg.Inbounds))
	}
	// 自定义出站字段合并且 type/tag 以记录为准，另补 direct 兜底
	if len(cfg.Outbounds) != 2 {
		t.Fatalf("应有 2 条出站（自定义 + direct 兜底）, got %d", len(cfg.Outbounds))
	}
	custom := cfg.Outbounds[0]
	if custom["type"] != "http" || custom["tag"] != "my-proxy" || custom["server"] != "127.0.0.1" || custom["server_port"] != float64(3128) {
		t.Errorf("自定义出站字段不符: %v", custom)
	}
	fallback := cfg.Outbounds[1]
	if fallback["type"] != "direct" || fallback["tag"] != "direct" {
		t.Errorf("direct 兜底出站不符: %v", fallback)
	}
	// 序列化为合法 JSON 且包含日志与 clash_api 基线
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	for _, want := range []string{`"inbounds"`, `"hy2-in"`, `"experimental"`, `"clash_api"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("聚合配置缺少 %s: %s", want, data)
		}
	}
}

func TestBuildConfigDirectOutboundNoFallback(t *testing.T) {
	cfg, err := BuildConfig(nil, []OutboundEntry{{Tag: "direct", Type: "direct", Config: "{}"}})
	if err != nil {
		t.Fatalf("聚合生成失败: %v", err)
	}
	if len(cfg.Outbounds) != 1 || cfg.Outbounds[0]["tag"] != "direct" {
		t.Errorf("已有 direct 出站时不应重复兜底: %v", cfg.Outbounds)
	}
}

func TestBuildConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		inbound InboundEntry
	}{
		{
			name:    "不支持的协议",
			inbound: InboundEntry{Tag: "bad", Protocol: "socks", Config: "{}"},
		},
		{
			name:    "配置 JSON 非法",
			inbound: InboundEntry{Tag: "bad", Protocol: ProtoTrojan, Config: "not-json"},
		},
		{
			name: "启用 TLS 但未配置证书",
			inbound: InboundEntry{Tag: "no-cert", Protocol: ProtoTrojan,
				Config: `{"password":"pw","tls":{"enabled":true}}`},
		},
		{
			name: "managed 证书缺少解析器",
			inbound: InboundEntry{Tag: "no-resolver", Protocol: ProtoHysteria2,
				Config: `{"password":"pw","tls":{"enabled":true,"certMode":"managed","certId":7}}`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildConfig([]InboundEntry{tt.inbound}, nil)
			if err == nil || !strings.Contains(err.Error(), tt.inbound.Tag) {
				t.Errorf("错误应带上入站 tag %q, got %v", tt.inbound.Tag, err)
			}
		})
	}
}

func TestBuildConfigCertResolveError(t *testing.T) {
	failed := func(uint) (string, string, error) { return "", "", fmt.Errorf("证书文件不存在") }
	_, err := BuildConfig([]InboundEntry{{
		Tag: "hy2-in", Protocol: ProtoHysteria2, ListenPort: 8443,
		Config: `{"password":"pw","tls":{"enabled":true,"certMode":"managed","certId":7}}`, Cert: failed,
	}}, nil)
	if err == nil || !strings.Contains(err.Error(), "证书文件不存在") {
		t.Errorf("应透传证书解析错误, got %v", err)
	}
}

func TestOutboundJSON(t *testing.T) {
	_, err := BuildConfig(nil, []OutboundEntry{{Tag: "bad", Type: "vmess", Config: `[1,2]`}})
	if err == nil || !strings.Contains(err.Error(), "bad") {
		t.Errorf("非对象出站配置应报错, got %v", err)
	}
	// direct 出站忽略附加配置
	ob, err := outboundJSON(OutboundEntry{Tag: "direct", Type: "direct", Config: ""})
	if err != nil || len(ob) != 2 || ob["type"] != "direct" || ob["tag"] != "direct" {
		t.Errorf("direct 出站映射不符: %v %v", ob, err)
	}
}
