package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"gorm.io/gorm"

	"proxy-ui/internal/db"
	"proxy-ui/internal/singbox"
)

// validTag 校验 tag 类名称：允许字母（含中文等 Unicode 字母）、数字、下划线、点与短横线。
// 内核 tag 与客户端节点名允许中文，因此不能用纯 ASCII 限制。
func validTag(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// marshalMembers 序列化成员列表；nil 视为空数组。
func marshalMembers(members []string) string {
	if members == nil {
		members = []string{}
	}
	raw, _ := json.Marshal(members)
	return string(raw)
}

// dedupeMembers 去重并保持顺序（内核拒绝同组内重复 tag，客户端里重复项也没有意义）。
func dedupeMembers(members []string) []string {
	seen := make(map[string]bool, len(members))
	out := make([]string, 0, len(members))
	for _, m := range members {
		m = strings.TrimSpace(m)
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

// getSetting 读取设置值，不存在返回空字符串。
func (a *API) getSetting(key string) string {
	var s db.Setting
	if err := a.db.First(&s, "key = ?", key).Error; err != nil {
		return ""
	}
	return s.Value
}

// setSetting 写入设置值（upsert）。
func (a *API) setSetting(key, value string) error {
	return a.db.Save(&db.Setting{Key: key, Value: value}).Error
}

// tagRefCount 统计某个出站 tag 被引用的处数：启用中的路由规则出站、启用中的策略组成员
// 与兜底出站、DNS 服务器的 detour。
//
// 只统计真正会进内核配置的引用（停用的规则与策略组不参与聚合），否则会拦住本该允许的
// 删除/停用；这些引用都只存 tag 字符串，改名或删除时必须一并处理，任一处悬空都会让
// 内核配置聚合失败（面板从此每次保存都报错，且回滚在旧配置上运行）。
func (a *API) tagRefCount(tx *gorm.DB, name string) (int, error) {
	var ruleCount int64
	if err := tx.Model(&db.Rule{}).Where("outbound = ? AND enabled = ?", name, true).
		Count(&ruleCount).Error; err != nil {
		return 0, err
	}
	var groups []db.PolicyGroup
	if err := tx.Find(&groups).Error; err != nil {
		return 0, err
	}
	total := int(ruleCount)
	for _, g := range groups {
		for _, m := range groupEntryOf(g).Members {
			if m == name {
				total++
			}
		}
	}
	var final db.Setting
	if err := tx.First(&final, "key = ?", db.SettingRouteFinal).Error; err == nil && final.Value == name {
		total++
	}
	detours, err := a.dnsDetours(tx)
	if err != nil {
		return 0, err
	}
	for _, d := range detours {
		if d == name {
			total++
		}
	}
	return total, nil
}

// renameTagRefs 把出站 tag 的全部引用由 oldName 改为 newName（规则出站、策略组成员、
// 兜底出站与 DNS detour）。与 tagRefCount 必须保持同一口径，否则改名后仍会留下悬空引用。
func (a *API) renameTagRefs(tx *gorm.DB, oldName, newName string) error {
	if err := tx.Model(&db.Rule{}).Where("outbound = ?", oldName).
		Update("outbound", newName).Error; err != nil {
		return err
	}
	var groups []db.PolicyGroup
	if err := tx.Find(&groups).Error; err != nil {
		return err
	}
	for _, g := range groups {
		entry := groupEntryOf(g)
		changed := false
		for i, m := range entry.Members {
			if m == oldName {
				entry.Members[i] = newName
				changed = true
			}
		}
		if !changed {
			continue
		}
		// 改名后可能出现重复成员（原有 hk 与新名 hk2 同时存在），一并去重
		if err := tx.Model(&db.PolicyGroup{}).Where("id = ?", g.ID).
			Update("members", marshalMembers(dedupeMembers(entry.Members))).Error; err != nil {
			return err
		}
	}
	if err := tx.Model(&db.Setting{}).Where("key = ? AND value = ?", db.SettingRouteFinal, oldName).
		Update("value", newName).Error; err != nil {
		return err
	}
	return a.renameDNSDetours(tx, oldName, newName)
}

// dnsDetours 取面板 DNS 配置里各服务器显式指定的 detour 出站 tag。
func (a *API) dnsDetours(tx *gorm.DB) ([]string, error) {
	cfg, err := a.dnsConfigFrom(tx)
	if err != nil || cfg == nil {
		return nil, err
	}
	out := make([]string, 0, len(cfg.Servers))
	for _, s := range cfg.Servers {
		if s.Detour != "" {
			out = append(out, s.Detour)
		}
	}
	return out, nil
}

// renameDNSDetours 把 DNS 配置里指向 oldName 的 detour 改为 newName。
func (a *API) renameDNSDetours(tx *gorm.DB, oldName, newName string) error {
	cfg, err := a.dnsConfigFrom(tx)
	if err != nil || cfg == nil {
		return err
	}
	changed := false
	for i := range cfg.Servers {
		if cfg.Servers[i].Detour == oldName {
			cfg.Servers[i].Detour = newName
			changed = true
		}
	}
	if !changed {
		return nil
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return tx.Save(&db.Setting{Key: db.SettingDNS, Value: string(raw)}).Error
}

// dnsConfigFrom 读取面板 DNS 设置（未设置返回 nil，非法 JSON 报错）。
func (a *API) dnsConfigFrom(tx *gorm.DB) (*singbox.DNSConfig, error) {
	var s db.Setting
	if err := tx.First(&s, "key = ?", db.SettingDNS).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if strings.TrimSpace(s.Value) == "" {
		return nil, nil
	}
	var cfg singbox.DNSConfig
	if err := json.Unmarshal([]byte(s.Value), &cfg); err != nil {
		return nil, fmt.Errorf("DNS 配置不合法: %w", err)
	}
	return &cfg, nil
}
