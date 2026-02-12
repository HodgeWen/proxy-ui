package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/s-ui/s-ui/internal/db"
	"gopkg.in/yaml.v3"
)

// BuildUserinfoHeader returns subscription-userinfo header (lowercase).
// upload/download: TrafficUsed/2 each (placeholder until Phase 6); total: TrafficLimit (omit if 0); expire: Unix sec (omit if nil).
func BuildUserinfoHeader(u *db.User) string {
	upload := u.TrafficUsed / 2
	download := u.TrafficUsed / 2
	parts := []string{
		fmt.Sprintf("upload=%d", upload),
		fmt.Sprintf("download=%d", download),
	}
	if u.TrafficLimit > 0 {
		parts = append(parts, fmt.Sprintf("total=%d", u.TrafficLimit))
	}
	if u.ExpireAt != nil {
		parts = append(parts, fmt.Sprintf("expire=%d", u.ExpireAt.Unix()))
	}
	return strings.Join(parts, "; ")
}

// extractHostFromInbound extracts host from inbound ConfigJSON for share links.
// Uses tls.server_name; falls back to config["host"]. Returns empty string if absent.
func extractHostFromInbound(ib *db.Inbound) string {
	if len(ib.ConfigJSON) == 0 {
		return ""
	}
	var cfg map[string]any
	if err := json.Unmarshal(ib.ConfigJSON, &cfg); err != nil {
		return ""
	}
	if tls, ok := cfg["tls"]; ok && tls != nil {
		if t, ok := tls.(map[string]any); ok {
			if s, ok := t["server_name"].(string); ok && s != "" {
				return s
			}
		}
	}
	if host, ok := cfg["host"].(string); ok && host != "" {
		return host
	}
	return ""
}

// NodeLink holds a name and link for a single node (used by admin UI).
type NodeLink struct {
	Name string
	Link string
}

// GetNodeLinks returns per-node links for admin UI per-node copy.
func GetNodeLinks(u *db.User) []NodeLink {
	links := make([]NodeLink, 0, len(u.Inbounds))
	for _, ib := range u.Inbounds {
		host := extractHostFromInbound(&ib)
		if host == "" {
			continue
		}
		if ib.Protocol == "vless" {
			params := url.Values{
				"type":     {"tcp"},
				"security": {"tls"},
				"flow":     {"xtls-rprx-vision"},
			}
			raw := fmt.Sprintf("vless://%s@%s:%d?%s#%s",
				u.UUID, host, ib.ListenPort, params.Encode(), url.PathEscape(ib.Tag))
			links = append(links, NodeLink{Name: ib.Tag, Link: raw})
		} else if ib.Protocol == "hysteria2" {
			raw := fmt.Sprintf("hysteria2://%s@%s:%d/?sni=%s#%s",
				url.PathEscape(u.Password), host, ib.ListenPort, host, url.PathEscape(ib.Tag))
			links = append(links, NodeLink{Name: ib.Tag, Link: raw})
		}
	}
	return links
}

// GenerateBase64 returns Base64-encoded subscription body (V2Ray format).
// For each inbound: VLESS or Hysteria2 links; join with newline; Base64 encode.
func GenerateBase64(u *db.User) ([]byte, error) {
	nodeLinks := GetNodeLinks(u)
	if len(nodeLinks) == 0 {
		return nil, nil
	}
	lines := make([]string, len(nodeLinks))
	for i, nl := range nodeLinks {
		lines[i] = nl.Link
	}
	body := strings.Join(lines, "\n")
	return []byte(base64.StdEncoding.EncodeToString([]byte(body))), nil
}

// clashProxy represents a ClashMeta proxy entry.
type clashProxy struct {
	Name      string `yaml:"name"`
	Type      string `yaml:"type"`
	Server    string `yaml:"server"`
	Port      uint   `yaml:"port"`
	UUID      string `yaml:"uuid,omitempty"`
	Network   string `yaml:"network,omitempty"`
	Servername string `yaml:"servername,omitempty"`
	Flow      string `yaml:"flow,omitempty"`
	TLS       bool   `yaml:"tls,omitempty"`
	Password  string `yaml:"password,omitempty"`
	SNI       string `yaml:"sni,omitempty"`
}

// GenerateClash returns ClashMeta YAML bytes.
func GenerateClash(u *db.User) ([]byte, error) {
	nodeLinks := GetNodeLinks(u)
	if len(nodeLinks) == 0 {
		return nil, nil
	}

	proxies := make([]clashProxy, 0, len(u.Inbounds))
	for _, ib := range u.Inbounds {
		host := extractHostFromInbound(&ib)
		if host == "" {
			continue
		}
		if ib.Protocol == "vless" {
			proxies = append(proxies, clashProxy{
				Name:       ib.Tag,
				Type:       "vless",
				Server:     host,
				Port:       ib.ListenPort,
				UUID:       u.UUID,
				Network:    "tcp",
				Servername: host,
				Flow:       "xtls-rprx-vision",
				TLS:        true,
			})
		} else if ib.Protocol == "hysteria2" {
			proxies = append(proxies, clashProxy{
				Name:     ib.Tag,
				Type:     "hysteria2",
				Server:   host,
				Port:     ib.ListenPort,
				Password: u.Password,
				SNI:      host,
			})
		}
	}
	if len(proxies) == 0 {
		return nil, nil
	}

	cfg := map[string]any{
		"proxies": proxies,
	}
	return yaml.Marshal(cfg)
}
