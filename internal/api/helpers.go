package api

import (
	"encoding/json"
	"unicode"

	"proxy-ui/internal/db"
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
