package api

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// 入站用户没有独立的管理接口：统一在「订阅用户」页创建与删除（勾选入站即生成、取消勾选即删除），
// 这里只保留订阅绑定所需的用户创建与查询辅助。

// createUser 在入站下新建用户并生成该协议凭据；name 为空按「u-协议-序号」兜底。
// 落库与查重走调用方给出的事务 tx（订阅节点同步要求整体提交或回滚）。
func (a *API) createUser(tx *gorm.DB, in *db.Inbound, name string, enabled *bool) (db.InboundUser, error) {
	cfg, err := singbox.ParseInboundConfig(in.Protocol, in.Config)
	if err != nil {
		return db.InboundUser{}, err
	}
	cred, err := a.newCredential(in.Protocol, cfg)
	if err != nil {
		return db.InboundUser{}, err
	}
	rawCred, err := json.Marshal(cred)
	if err != nil {
		return db.InboundUser{}, fmt.Errorf("序列化凭据失败: %w", err)
	}
	if name == "" {
		name = a.uniqueUserName(tx, in.Protocol)
	}
	if err := a.checkUserName(tx, name, 0); err != nil {
		return db.InboundUser{}, err
	}
	user := db.InboundUser{
		InboundID: in.ID, Name: name, Credential: string(rawCred), Enabled: true,
	}
	if enabled != nil {
		user.Enabled = *enabled
	}
	if err := tx.Create(&user).Error; err != nil {
		return db.InboundUser{}, fmt.Errorf("创建用户失败: %w", err)
	}
	return user, nil
}

// newCredential 生成协议凭据；Shadowsocks 需要按入站方法生成定长密钥。
func (a *API) newCredential(protocol string, cfg any) (singbox.Credential, error) {
	if protocol == singbox.ProtoShadowsocks {
		method := "2022-blake3-aes-128-gcm"
		if c, ok := cfg.(*singbox.ShadowsocksConfig); ok && c.Method != "" {
			method = c.Method
		}
		key, err := singbox.NewSSKey(method)
		if err != nil {
			return singbox.Credential{}, err
		}
		return singbox.Credential{Password: key}, nil
	}
	return singbox.NewCredential(protocol)
}

// uniqueUserName 生成未占用的用户名（u-协议-序号）。
//
// 前缀 u- 是为了与入站 tag（协议-序号）区分：入站名与用户名共用流量表的 tag 键，
// 撞名会让两者的用量互相覆盖。
func (a *API) uniqueUserName(tx *gorm.DB, protocol string) string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("u-%s-%d", protocol, i)
		if a.checkUserName(tx, name, 0) == nil {
			return name
		}
	}
}

// checkUserName 校验用户名合法且**全局**唯一（同时不得等于任何入站 tag）。
//
// 用户名是流量统计与节点展示的键，两个入站下同名用户会让用量混在一起，
// 因此唯一性必须是全局的，不能只在单个入站内检查。
func (a *API) checkUserName(tx *gorm.DB, name string, excludeID uint) error {
	if !validTag(name) {
		return fmt.Errorf("用户名只能包含字母、数字、下划线、点与短横线: %s", name)
	}
	var dup int64
	tx.Model(&db.InboundUser{}).Where("name = ? AND id <> ?", name, excludeID).Count(&dup)
	if dup > 0 {
		return fmt.Errorf("用户名 %s 已被占用，请换一个", name)
	}
	var clash int64
	tx.Model(&db.Inbound{}).Where("name = ?", name).Count(&clash)
	if clash > 0 {
		return fmt.Errorf("用户名 %s 与入站名冲突，请换一个", name)
	}
	return nil
}

// trafficByTag 查询全部累计流量（键为用户名/入站 tag）。
func (a *API) trafficByTag() (map[string]db.UserTraffic, error) {
	var rows []db.UserTraffic
	if err := a.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	byTag := make(map[string]db.UserTraffic, len(rows))
	for _, r := range rows {
		byTag[r.Tag] = r
	}
	return byTag, nil
}

// bindingMap 用户 ID -> 订阅绑定，以及订阅 ID -> 备注。
func (a *API) bindingMap() (map[uint]db.SubscriptionNode, map[uint]string, error) {
	var nodes []db.SubscriptionNode
	if err := a.db.Find(&nodes).Error; err != nil {
		return nil, nil, err
	}
	byUser := make(map[uint]db.SubscriptionNode, len(nodes))
	for _, n := range nodes {
		byUser[n.InboundUserID] = n
	}
	var tokens []db.SubscriptionToken
	if err := a.db.Find(&tokens).Error; err != nil {
		return nil, nil, err
	}
	names := make(map[uint]string, len(tokens))
	for _, t := range tokens {
		names[t.ID] = t.Name
	}
	return byUser, names, nil
}
