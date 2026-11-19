package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// setupAdmin 完成首次设置并返回带会话的客户端。
func setupAdmin(t *testing.T, srvURL string) *http.Client {
	t.Helper()
	client := newClient(t)
	if code, env := doJSON(t, client, http.MethodPost, srvURL+"/api/setup",
		map[string]string{"username": "admin", "password": "password123"}); code != http.StatusOK {
		t.Fatalf("setup 失败: %s", env.Msg)
	}
	return client
}

// kernelConfigJSON 应用配置后读取内核生成的 config.json。
func kernelConfigJSON(t *testing.T, dataDir string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dataDir, "core", "config.json"))
	if err != nil {
		t.Fatalf("读取内核配置失败: %v", err)
	}
	cfg := map[string]any{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("解析内核配置失败: %v", err)
	}
	return cfg
}

// findOutbound 按 tag 取出站对象。
func findOutbound(t *testing.T, cfg map[string]any, tag string) map[string]any {
	t.Helper()
	for _, ob := range cfg["outbounds"].([]any) {
		if m := ob.(map[string]any); m["tag"] == tag {
			return m
		}
	}
	t.Fatalf("出站 %s 不存在: %v", tag, cfg["outbounds"])
	return nil
}

// findRule 按 rule_set 引用取路由规则对象。
func findRule(t *testing.T, cfg map[string]any, ruleSetTag string) map[string]any {
	t.Helper()
	for _, r := range cfg["route"].(map[string]any)["rules"].([]any) {
		m := r.(map[string]any)
		for _, rs := range m["rule_set"].([]any) {
			if rs == ruleSetTag {
				return m
			}
		}
	}
	t.Fatalf("引用规则集 %s 的规则不存在: %v", ruleSetTag, cfg["route"])
	return nil
}

// findRuleSet 按 tag 取规则集定义。
func findRuleSet(t *testing.T, cfg map[string]any, tag string) map[string]any {
	t.Helper()
	for _, rs := range cfg["route"].(map[string]any)["rule_set"].([]any) {
		if m := rs.(map[string]any); m["tag"] == tag {
			return m
		}
	}
	t.Fatalf("规则集 %s 不存在: %v", tag, cfg["route"])
	return nil
}

// applyKernel 一键应用配置。
func applyKernel(t *testing.T, client *http.Client, srvURL string) {
	t.Helper()
	if code, env := doJSON(t, client, http.MethodPost, srvURL+"/api/kernel/apply", nil); code != http.StatusOK {
		t.Fatalf("应用配置失败: code=%d msg=%s", code, env.Msg)
	}
}

func TestRuleTemplateApply(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "panel.db")
	dataDir := t.TempDir()
	srv := httptest.NewServer(mustNewEngine(dbPath, dataDir))
	defer srv.Close()
	client := setupAdmin(t, srv.URL)

	// 准备一个自定义出站供策略组引用
	if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/outbounds",
		map[string]any{"name": "hk", "type": "trojan", "config": map[string]any{"server": "hk.example.com", "server_port": 443}}); code != http.StatusOK {
		t.Fatalf("创建出站失败: %s", env.Msg)
	}
	// 模板列表可见
	if code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/rule-templates", nil); code != http.StatusOK {
		t.Fatalf("模板列表失败: %s", env.Msg)
	}

	// 一键应用「国内直连+海外代理+广告拦截」模板并聚合生成内核配置
	if code, env := doJSON(t, client, http.MethodPost, srv.URL+"/api/rule-templates/cn-direct-adblock/apply", nil); code != http.StatusOK {
		t.Fatalf("应用模板失败: %s", env.Msg)
	}
	applyKernel(t, client, srv.URL)
	cfg := kernelConfigJSON(t, dataDir)

	// route 规则：anti-AD 拦截、国内域名/Ip 直连
	if r := findRule(t, cfg, "anti-ad"); r["action"] != "reject" {
		t.Errorf("anti-AD 规则应为 reject: %v", r)
	}
	if r := findRule(t, cfg, "geosite-cn"); r["outbound"] != "direct" {
		t.Errorf("国内域名规则应直连: %v", r)
	}
	if r := findRule(t, cfg, "geoip-cn"); r["outbound"] != "direct" {
		t.Errorf("国内 IP 规则应直连: %v", r)
	}
	// anti-AD rule_set 默认官方源
	if rs := findRuleSet(t, cfg, "anti-ad"); rs["url"] != "https://anti-ad.net/anti-ad-sing-box.srs" {
		t.Errorf("anti-AD 规则集应默认官方源: %v", rs)
	}
	// 策略组出站与兜底
	group := findOutbound(t, cfg, "proxy")
	if group["type"] != "urltest" || group["outbounds"].([]any)[0] != "hk" {
		t.Errorf("策略组应为自动选择且包含出站成员: %v", group)
	}
	if final := cfg["route"].(map[string]any)["final"]; final != "proxy" {
		t.Errorf("兜底出站应为 proxy: %v", final)
	}

	// 应用模板后修改任一规则并重新应用成功
	var rules []map[string]any
	if code, env := doJSON(t, client, http.MethodGet, srv.URL+"/api/rules", nil); code != http.StatusOK {
		t.Fatalf("规则列表失败: %s", env.Msg)
	} else if err := json.Unmarshal(env.Data, &rules); err != nil {
		t.Fatal(err)
	}
	var ruleID int
	for _, r := range rules {
		if r["type"] == "geosite" {
			ruleID = int(r["id"].(float64))
		}
	}
	if code, env := doJSON(t, client, http.MethodPut,
		srv.URL+"/api/rules/"+strconv.Itoa(ruleID),
		map[string]any{"type": "geosite", "value": "cn,private", "outbound": "direct"}); code != http.StatusOK {
		t.Fatalf("修改规则失败: %s", env.Msg)
	}
	applyKernel(t, client, srv.URL)
	cfg = kernelConfigJSON(t, dataDir)
	r := findRule(t, cfg, "geosite-private")
	if r["outbound"] != "direct" {
		t.Errorf("修改后的规则应生效: %v", r)
	}

	// anti-AD 源地址更换后配置随之更新
	if code, env := doJSON(t, client, http.MethodPut, srv.URL+"/api/anti-ad",
		map[string]string{"url": "https://mirror.example.com/anti-ad.srs"}); code != http.StatusOK {
		t.Fatalf("更换 anti-AD 源失败: %s", env.Msg)
	}
	applyKernel(t, client, srv.URL)
	cfg = kernelConfigJSON(t, dataDir)
	if rs := findRuleSet(t, cfg, "anti-ad"); rs["url"] != "https://mirror.example.com/anti-ad.srs" {
		t.Errorf("anti-AD 源更换后配置应更新: %v", rs)
	}
}
