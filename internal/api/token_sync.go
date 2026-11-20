package api

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"

	"proxy-ui/internal/db"
)

// syncTokenNodes 把订阅用户的节点集合对齐到勾选的入站列表：
// 新增勾选 → 在该入站下为这个用户创建 InboundUser（独立凭据）；
// 取消勾选 → 删除该用户身份（节点展示用量随之消失，配额计数在 TokenTraffic 上不受影响）；
// 保留项不动（不重建凭据）。
func (a *API) syncTokenNodes(token *db.SubscriptionToken, inboundIDs []uint) error {
	var bindings []db.SubscriptionNode
	if err := a.db.Where("token_id = ?", token.ID).Find(&bindings).Error; err != nil {
		return err
	}
	want := make(map[uint]bool, len(inboundIDs))
	for _, id := range inboundIDs {
		want[id] = true
	}
	for _, b := range bindings {
		if want[b.InboundID] {
			continue
		}
		if err := a.removeTokenNode(b); err != nil {
			return err
		}
	}
	have := make(map[uint]bool, len(bindings))
	for _, b := range bindings {
		have[b.InboundID] = true
	}
	for _, inboundID := range inboundIDs {
		if have[inboundID] {
			continue
		}
		if err := a.addTokenNode(token, inboundID); err != nil {
			return err
		}
	}
	return nil
}

// addTokenNode 为订阅用户在某入站下创建用户身份并建立绑定。
func (a *API) addTokenNode(token *db.SubscriptionToken, inboundID uint) error {
	var in db.Inbound
	if err := a.db.First(&in, inboundID).Error; err != nil {
		return fmt.Errorf("入站不存在: %d", inboundID)
	}
	name := a.tokenUserName(token, in)
	user, err := a.createUser(&in, name, nil)
	if err != nil {
		return fmt.Errorf("为入站 %s 创建用户失败: %w", in.Name, err)
	}
	binding := db.SubscriptionNode{TokenID: token.ID, InboundID: in.ID, InboundUserID: user.ID}
	if err := a.db.Create(&binding).Error; err != nil {
		return fmt.Errorf("建立订阅绑定失败: %w", err)
	}
	return nil
}

// removeTokenNode 解除绑定并删除对应入站用户，但**保留其已用流量**。
//
// 用量按 TokenID 汇总，所以取消/重新勾选入站不会把配额清零（否则一次误操作即可重置配额）。
func (a *API) removeTokenNode(binding db.SubscriptionNode) error {
	return a.db.Transaction(func(tx *gorm.DB) error {
		var user db.InboundUser
		found := tx.First(&user, binding.InboundUserID).Error == nil
		if err := tx.Where("id = ?", binding.ID).Delete(&db.SubscriptionNode{}).Error; err != nil {
			return err
		}
		if !found {
			return nil
		}
		// 流量行改挂到订阅用户上（tag 保留原用户名，重建同名用户后继续累加）
		if err := tx.Model(&db.UserTraffic{}).Where("tag = ?", user.Name).
			Update("token_id", binding.TokenID).Error; err != nil {
			return err
		}
		return tx.Delete(&db.InboundUser{}, user.ID).Error
	})
}

// tokenUserName 生成该订阅用户在某入站下的用户名：{订阅备注|user}-{入站名}，冲突追加序号。
//
// 用户名与入站 tag 共用流量表的键，必须全局不同；与入站同名时追加数字序号（而不是报错），
// 否则「订阅备注恰好等于入站名」这类正常输入会整个创建失败。
func (a *API) tokenUserName(token *db.SubscriptionToken, in db.Inbound) string {
	base := strings.TrimSpace(token.Name)
	if base == "" {
		base = "user"
	}
	base = sanitizeTag(base + "-" + in.Name)
	if !a.userNameTaken(in.ID, base) {
		return base
	}
	for i := 2; ; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		if !a.userNameTaken(in.ID, name) {
			return name
		}
	}
}

// userNameTaken 判断用户名是否已被同一入站内的用户或任何入站 tag 占用。
func (a *API) userNameTaken(inboundID uint, name string) bool {
	var userDup int64
	a.db.Model(&db.InboundUser{}).Where("inbound_id = ? AND name = ?", inboundID, name).Count(&userDup)
	if userDup > 0 {
		return true
	}
	var inboundDup int64
	a.db.Model(&db.Inbound{}).Where("name = ?", name).Count(&inboundDup)
	return inboundDup > 0
}

// sanitizeTag 把任意备注清洗为可用作内核 tag 的字符串：保留字母（含中文）、数字与 ._-，
// 空白与其它符号转 -，避免内核 tag 或客户端展示出现问题。
func sanitizeTag(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "user"
	}
	// 按 rune 截断，避免切坏多字节字符
	if runes := []rune(out); len(runes) > 60 {
		out = string(runes[:60])
	}
	return out
}

// usedBytesByToken 按订阅用户取已用流量（键为订阅用户 ID）。
//
// 唯一依据是 TokenTraffic：用户的入站身份会因取消/重新勾选而重建，
// 按用户名汇总会让配额随之清零或串号。
func (a *API) usedBytesByToken() (map[uint]int64, error) {
	var rows []db.TokenTraffic
	if err := a.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	byToken := make(map[uint]int64, len(rows))
	for _, r := range rows {
		byToken[r.TokenID] = r.UploadBytes + r.DownloadBytes
	}
	return byToken, nil
}

// usedBytes 查询单个订阅用户的已用流量。
func (a *API) usedBytes(tokenID uint) (int64, error) {
	var row db.TokenTraffic
	err := a.db.First(&row, "token_id = ?", tokenID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return row.UploadBytes + row.DownloadBytes, nil
}

// ReconcileQuota 按配额、到期与停用刷新各入站用户的暂停状态与生效开关，
// 返回是否有变化（调用方据此重建内核配置）。管理员的「手动停用」不会被覆盖。
func (a *API) ReconcileQuota() (bool, error) {
	var tokens []db.SubscriptionToken
	if err := a.db.Find(&tokens).Error; err != nil {
		return false, err
	}
	// 循环每 30 秒跑一次：节点与用量一轮批量查询，不随订阅用户数放大查询次数
	nodesByToken, err := a.tokenNodesByToken()
	if err != nil {
		return false, err
	}
	usedByToken, err := a.usedBytesByToken()
	if err != nil {
		return false, err
	}
	changed := false
	for _, token := range tokens {
		nodes := nodesByToken[token.ID]
		used := usedByToken[token.ID]
		expired := token.ExpiresAt != nil && time.Now().After(*token.ExpiresAt)
		over := token.TrafficLimitBytes > 0 && used >= token.TrafficLimitBytes
		shouldSuspend := expired || over || token.Revoked
		for _, n := range nodes {
			// 生效状态 = 未被管理员手动停用 且 未被系统暂停
			if n.Suspended == shouldSuspend && n.Enabled == (!n.Disabled && !shouldSuspend) {
				continue
			}
			if err := a.db.Model(&db.InboundUser{}).Where("id = ?", n.UserID).
				Updates(map[string]any{"suspended": shouldSuspend, "enabled": !n.Disabled && !shouldSuspend}).Error; err != nil {
				return changed, err
			}
			changed = true
		}
	}
	return changed, nil
}
