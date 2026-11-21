package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
//
// 全部写操作走调用方给出的事务 tx：中途任一步失败整体回滚，不留半应用状态。
func (a *API) syncTokenNodes(tx *gorm.DB, token *db.SubscriptionToken, inboundIDs []uint) error {
	var bindings []db.SubscriptionNode
	if err := tx.Where("token_id = ?", token.ID).Find(&bindings).Error; err != nil {
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
		if err := a.removeTokenNode(tx, b); err != nil {
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
		if err := a.addTokenNode(tx, token, inboundID); err != nil {
			return err
		}
	}
	return nil
}

// addTokenNode 为订阅用户在某入站下创建用户身份并建立绑定。
func (a *API) addTokenNode(tx *gorm.DB, token *db.SubscriptionToken, inboundID uint) error {
	var in db.Inbound
	if err := tx.First(&in, inboundID).Error; err != nil {
		return fmt.Errorf("入站不存在: %d", inboundID)
	}
	name := a.tokenUserName(tx, token, in)
	user, err := a.createUser(tx, &in, name)
	if err != nil {
		return fmt.Errorf("为入站 %s 创建用户失败: %w", in.Name, err)
	}
	binding := db.SubscriptionNode{TokenID: token.ID, InboundID: in.ID, InboundUserID: user.ID}
	if err := tx.Create(&binding).Error; err != nil {
		return fmt.Errorf("建立订阅绑定失败: %w", err)
	}
	return nil
}

// removeTokenNode 解除绑定并删除对应入站用户，但**保留其已用流量**。
//
// 用量按 TokenID 汇总，所以取消/重新勾选入站不会把配额清零（否则一次误操作即可重置配额）。
func (a *API) removeTokenNode(tx *gorm.DB, binding db.SubscriptionNode) error {
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
}

// tokenUserName 生成该订阅用户在某入站下的用户名：{订阅备注|user}-{入站名}，冲突追加序号。
//
// 用户名与入站 tag 共用流量表的键，必须全局不同；与入站同名时追加数字序号（而不是报错），
// 否则「订阅备注恰好等于入站名」这类正常输入会整个创建失败。
func (a *API) tokenUserName(tx *gorm.DB, token *db.SubscriptionToken, in db.Inbound) string {
	base := strings.TrimSpace(token.Name)
	if base == "" {
		base = "user"
	}
	base = sanitizeTag(base + "-" + in.Name)
	if !a.userNameTaken(tx, base) {
		return base
	}
	for i := 2; ; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		if !a.userNameTaken(tx, name) {
			return name
		}
	}
}

// userNameTaken 判断用户名是否已被任何用户或任何入站 tag 占用（全局口径，与 checkUserName 一致）。
//
// 用户名是流量统计与节点展示的全局键：此前只查本入站，跨入站撞名
// （token「a」+入站「b-c」与 token「a-b」+入站「c」同名）会拖到 createUser 才失败，
// 让节点同步中途留下半应用状态。
func (a *API) userNameTaken(tx *gorm.DB, name string) bool {
	var userDup int64
	tx.Model(&db.InboundUser{}).Where("name = ?", name).Count(&userDup)
	if userDup > 0 {
		return true
	}
	var inboundDup int64
	tx.Model(&db.Inbound{}).Where("name = ?", name).Count(&inboundDup)
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
	return a.usedBytesByTokens(nil)
}

// usedBytesByTokens 按给定订阅用户取已用流量（键为订阅用户 ID）；tokenIDs 为空表示全部。
func (a *API) usedBytesByTokens(tokenIDs []uint) (map[uint]int64, error) {
	query := a.db
	if len(tokenIDs) > 0 {
		query = query.Where("token_id IN ?", tokenIDs)
	}
	var rows []db.TokenTraffic
	if err := query.Find(&rows).Error; err != nil {
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
// 返回是否有变化（调用方据此重建内核配置）。
func (a *API) ReconcileQuota() (bool, error) {
	var tokens []db.SubscriptionToken
	if err := a.db.Find(&tokens).Error; err != nil {
		return false, err
	}
	return a.reconcileTokens(tokens)
}

// ReconcileQuotaTokens 只核对给定订阅用户的配额与到期：生效状态有变化时即时热应用内核。
// 由统计采集在流量落库成功后回调（只传本轮有流量的 token），把超限/到期用户的放行窗口
// 从周期核对的一个间隔缩短到一个落库窗口；失败只告警，兜底交给周期 QuotaLoop。
// 线程安全：跑在统计采集 goroutine 上，与写路径经 applyMu 串行。
func (a *API) ReconcileQuotaTokens(tokenIDs []uint) {
	a.reconcileQuotaTokens(context.Background(), tokenIDs)
}

func (a *API) reconcileQuotaTokens(ctx context.Context, tokenIDs []uint) {
	if len(tokenIDs) == 0 || ctx.Err() != nil {
		return
	}
	var tokens []db.SubscriptionToken
	if err := a.db.Where("id IN ?", tokenIDs).Find(&tokens).Error; err != nil {
		slog.Warn("即时配额核对查询订阅用户失败", "err", err)
		return
	}
	changed, err := a.reconcileTokens(tokens)
	if err != nil {
		slog.Warn("即时配额核对失败", "err", err)
		return
	}
	if changed {
		if err := a.applyKernelConfig(ctx); err != nil {
			if ctx.Err() != nil || errors.Is(err, errKernelStopped) {
				return
			}
			slog.Warn("即时配额核对后热应用内核配置失败", "err", err)
		}
	}
}

// reconcileTokens 对一批订阅用户核对配额、到期与停用，刷新其入站用户的暂停状态与生效
// 开关，返回是否有变化。节点与用量按这批用户一轮批量查询，查询量与核对范围成正比。
func (a *API) reconcileTokens(tokens []db.SubscriptionToken) (bool, error) {
	if len(tokens) == 0 {
		return false, nil
	}
	ids := make([]uint, 0, len(tokens))
	for _, t := range tokens {
		ids = append(ids, t.ID)
	}
	var nodes []db.SubscriptionNode
	if err := a.db.Where("token_id IN ?", ids).Find(&nodes).Error; err != nil {
		return false, err
	}
	if len(nodes) == 0 {
		return false, nil // 名下没有入站身份：无生效开关可翻
	}
	userIDs := make([]uint, 0, len(nodes))
	for _, n := range nodes {
		userIDs = append(userIDs, n.InboundUserID)
	}
	var users []db.InboundUser
	if err := a.db.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return false, err
	}
	userByID := make(map[uint]db.InboundUser, len(users))
	for _, u := range users {
		userByID[u.ID] = u
	}
	usedByToken, err := a.usedBytesByTokens(ids)
	if err != nil {
		return false, err
	}
	changed := false
	for _, token := range tokens {
		used := usedByToken[token.ID]
		expired := token.ExpiresAt != nil && time.Now().After(*token.ExpiresAt)
		over := token.TrafficLimitBytes > 0 && used >= token.TrafficLimitBytes
		shouldSuspend := expired || over || token.Revoked
		for _, n := range nodes {
			if n.TokenID != token.ID {
				continue
			}
			u, ok := userByID[n.InboundUserID]
			if !ok {
				continue
			}
			// 生效状态 = 未被系统暂停
			if u.Suspended == shouldSuspend && u.Enabled == !shouldSuspend {
				continue
			}
			if err := a.db.Model(&db.InboundUser{}).Where("id = ?", u.ID).
				Updates(map[string]any{"suspended": shouldSuspend, "enabled": !shouldSuspend}).Error; err != nil {
				return changed, err
			}
			changed = true
		}
	}
	return changed, nil
}
