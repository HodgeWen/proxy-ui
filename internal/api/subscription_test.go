package api

import (
	"testing"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// TestDeleteTokenClearsTraffic 删除订阅用户时一并清理 TokenTraffic、UserTraffic、绑定与入站用户，
// 不影响其它订阅用户的流量与记录。
func TestDeleteTokenClearsTraffic(t *testing.T) {
	a := newInboundTestAPI(t)
	in1, err := a.createInbound(inboundReq{Name: "in-del-1", Protocol: singbox.ProtoVLESS})
	if err != nil {
		t.Fatalf("创建入站失败: %v", err)
	}
	in2, err := a.createInbound(inboundReq{Name: "in-del-2", Protocol: singbox.ProtoShadowsocks})
	if err != nil {
		t.Fatalf("创建入站失败: %v", err)
	}

	target, err := a.createToken(tokenReq{Name: new("alice"), NodeInbounds: &[]uint{in1.ID, in2.ID}})
	if err != nil {
		t.Fatalf("创建目标订阅用户失败: %v", err)
	}
	other, err := a.createToken(tokenReq{Name: new("bob"), NodeInbounds: &[]uint{in1.ID}})
	if err != nil {
		t.Fatalf("创建对照订阅用户失败: %v", err)
	}

	if err := a.db.Create(&db.TokenTraffic{TokenID: target.ID, UploadBytes: 100, DownloadBytes: 200}).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.db.Create(&db.TokenTraffic{TokenID: other.ID, UploadBytes: 300, DownloadBytes: 400}).Error; err != nil {
		t.Fatal(err)
	}
	for _, n := range target.Nodes {
		if err := a.db.Create(&db.UserTraffic{Tag: n.Name, TokenID: target.ID, UploadBytes: 10, DownloadBytes: 20}).Error; err != nil {
			t.Fatal(err)
		}
	}
	otherTag := other.Nodes[0].Name
	if err := a.db.Create(&db.UserTraffic{Tag: otherTag, TokenID: other.ID, UploadBytes: 30, DownloadBytes: 40}).Error; err != nil {
		t.Fatal(err)
	}

	if err := a.deleteToken(target.ID); err != nil {
		t.Fatalf("删除订阅用户失败: %v", err)
	}

	// 目标 TokenTraffic、UserTraffic、SubscriptionNode、InboundUser、SubscriptionToken 全部清除
	assertCount := func(model any, query string, args ...any) int64 {
		t.Helper()
		var n int64
		if err := a.db.Model(model).Where(query, args...).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := assertCount(&db.TokenTraffic{}, "token_id = ?", target.ID); n != 0 {
		t.Errorf("删除后 TokenTraffic 应已清除, got %d", n)
	}
	if n := assertCount(&db.SubscriptionNode{}, "token_id = ?", target.ID); n != 0 {
		t.Errorf("删除后 SubscriptionNode 应已清除, got %d", n)
	}
	if n := assertCount(&db.SubscriptionToken{}, "id = ?", target.ID); n != 0 {
		t.Errorf("删除后 SubscriptionToken 应已清除, got %d", n)
	}
	for _, node := range target.Nodes {
		if n := assertCount(&db.UserTraffic{}, "tag = ?", node.Name); n != 0 {
			t.Errorf("删除后节点流量 %s 应已清除, got %d", node.Name, n)
		}
		if n := assertCount(&db.InboundUser{}, "id = ?", node.UserID); n != 0 {
			t.Errorf("删除后入站用户 %d 应已清除, got %d", node.UserID, n)
		}
	}

	// 对照用户不受影响
	if n := assertCount(&db.TokenTraffic{}, "token_id = ?", other.ID); n != 1 {
		t.Errorf("对照 TokenTraffic 不应被误删, got %d", n)
	}
	if n := assertCount(&db.UserTraffic{}, "tag = ?", otherTag); n != 1 {
		t.Errorf("对照 UserTraffic 不应被误删, got %d", n)
	}
}

// TestTokenNodesScopedQuery 单用户 tokenNodes 只返回该用户节点及对应流量，空节点用户返回空切片。
func TestTokenNodesScopedQuery(t *testing.T) {
	a := newInboundTestAPI(t)
	in1, err := a.createInbound(inboundReq{Name: "in-s-1", Protocol: singbox.ProtoVLESS})
	if err != nil {
		t.Fatal(err)
	}
	in2, err := a.createInbound(inboundReq{Name: "in-s-2", Protocol: singbox.ProtoShadowsocks})
	if err != nil {
		t.Fatal(err)
	}
	t1, err := a.createToken(tokenReq{Name: new("u1"), NodeInbounds: &[]uint{in1.ID, in2.ID}})
	if err != nil {
		t.Fatal(err)
	}
	t2, err := a.createToken(tokenReq{Name: new("u2"), NodeInbounds: &[]uint{in1.ID}})
	if err != nil {
		t.Fatal(err)
	}
	tEmpty, err := a.createToken(tokenReq{Name: new("empty")})
	if err != nil {
		t.Fatal(err)
	}

	if err := a.db.Create(&db.UserTraffic{Tag: t1.Nodes[0].Name, TokenID: t1.ID, UploadBytes: 111, DownloadBytes: 222}).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.db.Create(&db.UserTraffic{Tag: t2.Nodes[0].Name, TokenID: t2.ID, UploadBytes: 999, DownloadBytes: 888}).Error; err != nil {
		t.Fatal(err)
	}

	got1, err := a.tokenNodes(t1.ID)
	if err != nil {
		t.Fatalf("查询 t1 节点失败: %v", err)
	}
	if len(got1) != 2 {
		t.Fatalf("t1 应有 2 个节点, got %d", len(got1))
	}
	if got1[0].UploadBytes != 111 || got1[0].DownloadBytes != 222 {
		t.Errorf("t1 首个节点流量不符: %+v", got1[0])
	}
	if got1[1].InboundName != in2.Name || got1[1].UploadBytes != 0 {
		t.Errorf("t1 第二个节点不符: %+v", got1[1])
	}

	gotEmpty, err := a.tokenNodes(tEmpty.ID)
	if err != nil {
		t.Fatalf("查询空节点用户失败: %v", err)
	}
	if gotEmpty == nil || len(gotEmpty) != 0 {
		t.Errorf("空节点用户应返回非 nil 空切片, got %#v", gotEmpty)
	}
}

// TestDeleteInboundClearsUsersTraffic 删除入站时批量清理其下全部用户的 UserTraffic。
func TestDeleteInboundClearsUsersTraffic(t *testing.T) {
	a := newInboundTestAPI(t)
	in1, err := a.createInbound(inboundReq{Name: "in-rm", Protocol: singbox.ProtoVLESS})
	if err != nil {
		t.Fatal(err)
	}
	in2, err := a.createInbound(inboundReq{Name: "in-keep", Protocol: singbox.ProtoVLESS})
	if err != nil {
		t.Fatal(err)
	}
	t1, err := a.createToken(tokenReq{Name: new("a"), NodeInbounds: &[]uint{in1.ID, in2.ID}})
	if err != nil {
		t.Fatal(err)
	}
	t2, err := a.createToken(tokenReq{Name: new("b"), NodeInbounds: &[]uint{in1.ID}})
	if err != nil {
		t.Fatal(err)
	}

	var rmTags []string
	var keepTag string
	for _, n := range append(t1.Nodes, t2.Nodes...) {
		if err := a.db.Create(&db.UserTraffic{Tag: n.Name, UploadBytes: 50, DownloadBytes: 60}).Error; err != nil {
			t.Fatal(err)
		}
		if n.InboundID == in1.ID {
			rmTags = append(rmTags, n.Name)
		} else {
			keepTag = n.Name
		}
	}

	if err := a.deleteInbound(in1.ID); err != nil {
		t.Fatalf("删除入站失败: %v", err)
	}

	var count int64
	a.db.Model(&db.UserTraffic{}).Where("tag IN ?", rmTags).Count(&count)
	if count != 0 {
		t.Errorf("被删入站下的用户流量应全部清理, got %d", count)
	}
	a.db.Model(&db.UserTraffic{}).Where("tag = ?", keepTag).Count(&count)
	if count != 1 {
		t.Errorf("其它入站的用户流量不应受影响, got %d", count)
	}
}
