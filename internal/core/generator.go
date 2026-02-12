package core

import (
	"encoding/json"
	"os"

	"github.com/s-ui/s-ui/internal/db"
)

// ConfigGenerator produces full sing-box JSON config from DB inbounds.
type ConfigGenerator struct{}

// Generate reads all inbounds from DB and builds full sing-box config JSON.
func (g *ConfigGenerator) Generate() ([]byte, error) {
	inbounds, err := db.ListInbounds("")
	if err != nil {
		return nil, err
	}

	raw := make([]map[string]any, 0, len(inbounds))
	for i := range inbounds {
		raw = append(raw, g.inboundToSingBox(&inbounds[i]))
	}

	cfg := map[string]any{
		"log": map[string]any{"level": "info"},
		"inbounds": raw,
		"outbounds": []map[string]any{
			{"type": "direct", "tag": "direct"},
			{"type": "block", "tag": "block"},
		},
		"route": map[string]any{"rules": []any{}},
	}

	if g.v2rayAPIEnabled() {
		cfg["experimental"] = g.v2rayAPIBlock(inbounds)
	}

	return json.MarshalIndent(cfg, "", "  ")
}

func (g *ConfigGenerator) v2rayAPIEnabled() bool {
	return os.Getenv("V2RAY_API_ENABLED") == "true"
}

func (g *ConfigGenerator) v2rayAPIListen() string {
	if s := os.Getenv("V2RAY_API_LISTEN"); s != "" {
		return s
	}
	return "127.0.0.1:8080"
}

func (g *ConfigGenerator) v2rayAPIBlock(inbounds []db.Inbound) map[string]any {
	tags := make([]string, 0, len(inbounds))
	userSet := make(map[string]struct{})
	for _, ib := range inbounds {
		tags = append(tags, ib.Tag)
		users, _ := db.GetUsersForInbound(ib.ID)
		for _, u := range users {
			userSet[u.Name] = struct{}{}
		}
	}
	users := make([]string, 0, len(userSet))
	for name := range userSet {
		users = append(users, name)
	}
	return map[string]any{
		"v2ray_api": map[string]any{
			"listen": g.v2rayAPIListen(),
			"stats": map[string]any{
				"enabled":   true,
				"inbounds":  tags,
				"users":     users,
				"outbounds": []string{"direct"},
			},
		},
	}
}

// inboundToSingBox converts db.Inbound to sing-box inbound JSON object.
func (g *ConfigGenerator) inboundToSingBox(ib *db.Inbound) map[string]any {
	switch ib.Protocol {
	case "hysteria2":
		return g.hysteria2ToSingBox(ib)
	default:
		return g.vlessToSingBox(ib)
	}
}

// vlessToSingBox produces VLESS inbound map for sing-box.
// Users are derived from User+UserInbound (valid only); config_json users ignored.
func (g *ConfigGenerator) vlessToSingBox(ib *db.Inbound) map[string]any {
	users, _ := db.GetUsersForInbound(ib.ID)
	userArr := make([]any, 0, len(users))
	for _, u := range users {
		userArr = append(userArr, map[string]any{
			"name": u.Name,
			"uuid": u.UUID,
			"flow": "xtls-rprx-vision",
		})
	}

	out := map[string]any{
		"type":         "vless",
		"tag":          ib.Tag,
		"listen":       ib.Listen,
		"listen_port":  ib.ListenPort,
		"users":        userArr,
	}

	if len(ib.ConfigJSON) > 0 {
		var cfg map[string]any
		if err := json.Unmarshal(ib.ConfigJSON, &cfg); err == nil {
			if tls, ok := cfg["tls"]; ok && tls != nil {
				if t, ok := tls.(map[string]any); ok && len(t) > 0 {
					resolveCertInTLS(t)
					out["tls"] = t
				}
			}
			if transport, ok := cfg["transport"]; ok && transport != nil {
				if tr, ok := transport.(map[string]any); ok && len(tr) > 0 {
					out["transport"] = tr
				}
			}
		}
	}

	return out
}

// hysteria2ToSingBox produces Hysteria2 inbound map for sing-box.
// Users are derived from User+UserInbound (valid only); config_json users ignored.
func (g *ConfigGenerator) hysteria2ToSingBox(ib *db.Inbound) map[string]any {
	users, _ := db.GetUsersForInbound(ib.ID)
	userArr := make([]any, 0, len(users))
	for _, u := range users {
		userArr = append(userArr, map[string]any{
			"name":     u.Name,
			"password": u.Password,
		})
	}

	out := map[string]any{
		"type":         "hysteria2",
		"tag":          ib.Tag,
		"listen":       ib.Listen,
		"listen_port":  ib.ListenPort,
		"users":        userArr,
		"tls": map[string]any{
			"enabled":          true,
			"server_name":      "",
			"certificate_path": "",
			"key_path":         "",
		},
	}

	if len(ib.ConfigJSON) > 0 {
		var cfg map[string]any
		if err := json.Unmarshal(ib.ConfigJSON, &cfg); err == nil {
			if up, ok := cfg["up_mbps"]; ok {
				if f, ok := toFloat(up); ok && f > 0 {
					out["up_mbps"] = int(f)
				}
			}
			if down, ok := cfg["down_mbps"]; ok {
				if f, ok := toFloat(down); ok && f > 0 {
					out["down_mbps"] = int(f)
				}
			}
			if obfs, ok := cfg["obfs"]; ok && obfs != nil {
				if o, ok := obfs.(map[string]any); ok && len(o) > 0 {
					out["obfs"] = o
				}
			}
			if tls, ok := cfg["tls"]; ok && tls != nil {
				if t, ok := tls.(map[string]any); ok && len(t) > 0 {
					resolveCertInTLS(t)
					out["tls"] = t
				}
			}
		}
	}

	return out
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		return 0, false
	}
}

// toUint converts float64, int, int64, json.Number to uint; returns (uint, ok).
func toUint(v any) (uint, bool) {
	switch x := v.(type) {
	case float64:
		if x >= 0 && x == float64(uint(x)) {
			return uint(x), true
		}
		return 0, false
	case int:
		if x >= 0 {
			return uint(x), true
		}
		return 0, false
	case int64:
		if x >= 0 {
			return uint(x), true
		}
		return 0, false
	case json.Number:
		n, err := x.Int64()
		if err != nil || n < 0 {
			return 0, false
		}
		return uint(n), true
	default:
		return 0, false
	}
}

// resolveCertInTLS resolves certificate_id in tls map to certificate_path and key_path.
// Modifies tls in-place. Deletes certificate_id before emitting (sing-box does not know it).
func resolveCertInTLS(tls map[string]any) {
	certID, ok := tls["certificate_id"]
	if !ok || certID == nil {
		return
	}
	id, ok := toUint(certID)
	if !ok {
		delete(tls, "certificate_id")
		return
	}
	cert, err := db.GetCertificateByID(id)
	if err != nil {
		delete(tls, "certificate_id")
		return
	}
	tls["certificate_path"] = cert.FullchainPath
	tls["key_path"] = cert.PrivkeyPath
	delete(tls, "certificate_id")
}
