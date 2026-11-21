package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// sptr 取字符串指针（tokenReq 备注字段用）。
func sptr(s string) *string { return &s }

// TestDecodeToolArgs 工具参数解码：空参数保持零值、典型输入正确解码、非法输入报错。
func TestDecodeToolArgs(t *testing.T) {
	// 空参数：保持目标零值
	var arg idArg
	if err := decodeToolArgs(nil, &arg); err != nil {
		t.Fatalf("空参数应通过: %v", err)
	}
	if arg.ID != 0 {
		t.Errorf("空参数应保持零值, got %d", arg.ID)
	}

	// 典型输入：解码到目标字段，模型多给的未知字段忽略
	if err := decodeToolArgs(json.RawMessage(`{"id": 7, "extra": "x"}`), &arg); err != nil {
		t.Fatalf("合法参数应通过: %v", err)
	}
	if arg.ID != 7 {
		t.Errorf("应解码出 id=7, got %d", arg.ID)
	}

	// 类型不符
	if err := decodeToolArgs(json.RawMessage(`{"id": "abc"}`), &arg); err == nil {
		t.Errorf("id 为字符串应报错")
	}
	// 非法 JSON
	if err := decodeToolArgs(json.RawMessage(`{invalid`), &arg); err == nil {
		t.Errorf("非法 JSON 应报错")
	}
}

// TestModifyToolReportsKernelApplyFailure 写工具「落库成功 + 热应用失败」的回报语义：
// 错误信息携带「已保存，但内核按新配置重建失败」，且数据库改动保留（修正后重试即可）。
func TestModifyToolReportsKernelApplyFailure(t *testing.T) {
	a := newInboundTestAPI(t)
	// 制造无法聚合的存量配置：启用用户的凭据损坏 → buildKernelConfig 失败
	in, err := a.createInbound(inboundReq{Name: "broken", Protocol: singbox.ProtoVLESS})
	if err != nil {
		t.Fatalf("创建入站失败: %v", err)
	}
	if err := a.db.Create(&db.InboundUser{
		InboundID: in.ID, Name: "bad-cred", Credential: "not-json", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	_, err = a.ai.Call(context.Background(), "create_outbound",
		json.RawMessage(`{"type":"socks","name":"via-tool"}`))
	if err == nil || !strings.Contains(err.Error(), "已保存，但内核按新配置重建失败") {
		t.Fatalf("热应用失败应回报已保存语义, got %v", err)
	}
	var count int64
	a.db.Model(&db.Outbound{}).Where("name = ?", "via-tool").Count(&count)
	if count != 1 {
		t.Errorf("热应用失败时改动应已落库, count=%d", count)
	}
}

// TestModifyToolReportsSaveFailure 写工具落库（校验）失败的回报语义：
// 只回报业务错误，不带「已保存」前缀，且数据库无残留。
func TestModifyToolReportsSaveFailure(t *testing.T) {
	a := newInboundTestAPI(t)
	// selector/urltest 类型保留给策略组：应被业务校验拒绝
	_, err := a.ai.Call(context.Background(), "create_outbound",
		json.RawMessage(`{"type":"selector","name":"s"}`))
	if err == nil || strings.Contains(err.Error(), "已保存") {
		t.Fatalf("落库失败不应带已保存语义, got %v", err)
	}
	if !strings.Contains(err.Error(), "保留给策略组") {
		t.Errorf("应回报业务校验错误, got %v", err)
	}
	var count int64
	a.db.Model(&db.Outbound{}).Count(&count)
	if count != 0 {
		t.Errorf("失败的操作不应落库, count=%d", count)
	}
}

// TestListInboundUsersToolHidesCredential list_inbound_users 工具返回脱敏视图：
// 不含 Credential 字段与凭据明文，其余字段保留。
func TestListInboundUsersToolHidesCredential(t *testing.T) {
	a := newInboundTestAPI(t)
	in, err := a.createInbound(inboundReq{Name: "in-1", Protocol: singbox.ProtoVLESS})
	if err != nil {
		t.Fatalf("创建入站失败: %v", err)
	}
	if err := a.db.Create(&db.InboundUser{
		InboundID: in.ID, Name: "u-plain", Credential: `{"uuid":"secret-uuid"}`, Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	result, err := a.ai.Call(context.Background(), "list_inbound_users", nil)
	if err != nil {
		t.Fatalf("查询入站用户失败: %v", err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "credential") || strings.Contains(string(raw), "secret-uuid") {
		t.Errorf("工具结果不应含凭据: %s", raw)
	}
	if !strings.Contains(string(raw), "u-plain") || !strings.Contains(string(raw), `"inboundId"`) {
		t.Errorf("其余字段应保留: %s", raw)
	}
}

// TestCreateTokenCrossInboundNameCollision D5：用户名占用为全局口径。
// token「a」勾选入站「b-c」生成用户 a-b-c；token「a-b」勾选入站「c」撞同名时
// 应自动追加序号（a-b-c-2），而不是在同步中途因 createUser 全局查重失败。
func TestCreateTokenCrossInboundNameCollision(t *testing.T) {
	a := newInboundTestAPI(t)
	bc, err := a.createInbound(inboundReq{Name: "b-c", Protocol: singbox.ProtoVLESS})
	if err != nil {
		t.Fatalf("创建入站失败: %v", err)
	}
	inc, err := a.createInbound(inboundReq{Name: "c", Protocol: singbox.ProtoVLESS})
	if err != nil {
		t.Fatalf("创建入站失败: %v", err)
	}
	if _, err := a.createToken(tokenReq{Name: sptr("a"), NodeInbounds: &[]uint{bc.ID}}); err != nil {
		t.Fatalf("创建首个订阅用户失败: %v", err)
	}
	if !a.userNameTaken(a.db, "a-b-c") {
		t.Errorf("跨入站同名用户应视为已占用（全局口径）")
	}

	if _, err := a.createToken(tokenReq{Name: sptr("a-b"), NodeInbounds: &[]uint{inc.ID}}); err != nil {
		t.Fatalf("跨入站撞名应追加序号而不是失败: %v", err)
	}
	var users []db.InboundUser
	if err := a.db.Where("inbound_id = ?", inc.ID).Find(&users).Error; err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Name != "a-b-c-2" {
		t.Errorf("撞名应生成 a-b-c-2, got %+v", users)
	}
}

// TestUpdateTokenRollsBackOnSyncFailure D5：token 更新的事务整体回滚。
// 改名同时勾选不存在的入站：节点同步失败后，改名也应一并回滚，不留半应用状态。
func TestUpdateTokenRollsBackOnSyncFailure(t *testing.T) {
	a := newInboundTestAPI(t)
	in, err := a.createInbound(inboundReq{Name: "n1", Protocol: singbox.ProtoVLESS})
	if err != nil {
		t.Fatalf("创建入站失败: %v", err)
	}
	view, err := a.createToken(tokenReq{Name: sptr("old"), NodeInbounds: &[]uint{in.ID}})
	if err != nil {
		t.Fatalf("创建订阅用户失败: %v", err)
	}

	_, err = a.updateToken(view.ID, tokenReq{
		Name: sptr("renamed"), NodeInbounds: &[]uint{in.ID, 9999},
	})
	if err == nil || !strings.Contains(err.Error(), "入站不存在") {
		t.Fatalf("勾选不存在的入站应失败: %v", err)
	}
	var record db.SubscriptionToken
	if err := a.db.First(&record, view.ID).Error; err != nil {
		t.Fatal(err)
	}
	if record.Name != "old" {
		t.Errorf("同步失败后改名应整体回滚, got %s", record.Name)
	}
	var users []db.InboundUser
	if err := a.db.Where("inbound_id = ?", in.ID).Find(&users).Error; err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Name != "old-n1" {
		t.Errorf("原有节点不应被改动, got %+v", users)
	}
}

// TestUpdateTokenRenameUsesNewPrefix 漂移一：改名后新增节点用新名称做前缀。
// 此前工具路径未在更新后重读记录，沿用旧前缀。
func TestUpdateTokenRenameUsesNewPrefix(t *testing.T) {
	a := newInboundTestAPI(t)
	in1, err := a.createInbound(inboundReq{Name: "i1", Protocol: singbox.ProtoVLESS})
	if err != nil {
		t.Fatalf("创建入站失败: %v", err)
	}
	in2, err := a.createInbound(inboundReq{Name: "i2", Protocol: singbox.ProtoVLESS})
	if err != nil {
		t.Fatalf("创建入站失败: %v", err)
	}
	view, err := a.createToken(tokenReq{Name: sptr("old"), NodeInbounds: &[]uint{in1.ID}})
	if err != nil {
		t.Fatalf("创建订阅用户失败: %v", err)
	}

	if _, err := a.updateToken(view.ID, tokenReq{
		Name: sptr("new"), NodeInbounds: &[]uint{in1.ID, in2.ID},
	}); err != nil {
		t.Fatalf("改名并勾选新入站失败: %v", err)
	}
	var users []db.InboundUser
	if err := a.db.Where("inbound_id = ?", in2.ID).Find(&users).Error; err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Name != "new-i2" {
		t.Errorf("新增节点应用新前缀 new-i2, got %+v", users)
	}
}

// TestResetTokenTrafficClearsCounters 漂移二：清零流量（共用方法内含事务）后
// 配额计数与节点展示用量一并归零。
func TestResetTokenTrafficClearsCounters(t *testing.T) {
	a := newInboundTestAPI(t)
	in, err := a.createInbound(inboundReq{Name: "r1", Protocol: singbox.ProtoVLESS})
	if err != nil {
		t.Fatalf("创建入站失败: %v", err)
	}
	view, err := a.createToken(tokenReq{Name: sptr("t"), NodeInbounds: &[]uint{in.ID}})
	if err != nil {
		t.Fatalf("创建订阅用户失败: %v", err)
	}
	var users []db.InboundUser
	if err := a.db.Where("inbound_id = ?", in.ID).Find(&users).Error; err != nil {
		t.Fatal(err)
	}
	tag := users[0].Name
	if err := a.db.Create(&db.TokenTraffic{TokenID: view.ID, UploadBytes: 100, DownloadBytes: 50}).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.db.Create(&db.UserTraffic{Tag: tag, UploadBytes: 10, DownloadBytes: 5}).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := a.resetTokenTraffic(view.ID); err != nil {
		t.Fatalf("清零流量失败: %v", err)
	}
	var quota, display int64
	a.db.Model(&db.TokenTraffic{}).Where("token_id = ?", view.ID).Count(&quota)
	a.db.Model(&db.UserTraffic{}).Where("tag = ?", tag).Count(&display)
	if quota != 0 || display != 0 {
		t.Errorf("流量应清零, quota=%d display=%d", quota, display)
	}
}
