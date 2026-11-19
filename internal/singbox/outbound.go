package singbox

import (
	"encoding/json"
	"fmt"
	"strings"
)

// outboundJSON 把出站条目映射为 sing-box outbound JSON 对象；
// Config 为 type/tag 之外的出站字段（JSON 对象，direct 出站留空）。
func outboundJSON(e OutboundEntry) (map[string]any, error) {
	fields := map[string]any{}
	if strings.TrimSpace(e.Config) != "" {
		if err := json.Unmarshal([]byte(e.Config), &fields); err != nil {
			return nil, fmt.Errorf("出站 %s: 配置 JSON 不合法: %w", e.Tag, err)
		}
	}
	// type/tag 以记录为准，忽略配置内同名字段
	delete(fields, "type")
	delete(fields, "tag")
	fields["type"] = e.Type
	fields["tag"] = e.Tag
	return fields, nil
}
