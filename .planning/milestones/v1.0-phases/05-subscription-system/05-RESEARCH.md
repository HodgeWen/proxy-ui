# Phase 5: Subscription System - Research

**Researched:** 2026-02-12
**Domain:** Proxy subscription links (V2Ray/ClashMeta formats), QR codes, UA detection
**Confidence:** HIGH

## Summary

Phase 5 implements a subscription system where each user has a unique shareable link. The link returns either Base64-encoded V2Ray share links (`vless://`, `hysteria2://`) or ClashMeta YAML. Format is chosen by UA detection (Clash clients get YAML) or by `?format=clash` query param. The `subscription-userinfo` header displays traffic/expiry in clients. Subscription info and QR codes appear in the admin panel's user detail modal only.

**Primary recommendation:** Use `crypto/rand` for short tokens; `encoding/base64` for subscription body; `qrcode.react` for frontend QR; extract host from inbound `tls.server_name`; match UA containing `clash` for format detection.

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **订阅链接设计:** 使用短 token（如 `/sub/abc123def`），非 UUID，需额外生成并存储在数据库；格式切换采用双模式：默认 UA 自动检测（Clash 客户端返回 YAML，其他返回 Base64），同时支持 `?format=clash` query param 强制指定；管理员可重置用户的订阅 token，重置后旧链接立即失效；订阅链接无额外认证，知道链接即可访问
- **信息展示:** 订阅信息仅在管理后台展示，不作为公开信息页；展示内容丰富：用户名、剩余流量、到期时间、节点列表、各节点状态、每节点一键复制链接；暗色卡片风格，响应式布局
- **QR 码:** 订阅链接级别生成一个 QR 码；展示位置：用户详情弹窗内；默认隐藏，点击按钮弹出/展开；QR 码旁附带一键复制订阅链接功能
- **订阅内容生成:** 禁用或过期用户访问订阅链接时返回 HTTP 403；Base64 格式：标准 V2ray 分享链接；Clash 格式：仅 ClashMeta 格式（支持 VLESS 和 Hysteria2）；响应携带 `subscription-userinfo` header

### Claude's Discretion

- 短 token 的具体长度和字符集
- 订阅 API 的具体路由结构
- QR 码的尺寸和样式细节
- 用户详情弹窗中订阅信息区域的具体布局
- UA 检测的具体规则（哪些 UA 匹配 Clash）

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope

</user_constraints>

## Standard Stack

### Core

| Library           | Version | Purpose                         | Why Standard                                             |
| ----------------- | ------- | ------------------------------- | -------------------------------------------------------- |
| `encoding/base64` | stdlib  | Base64 encode subscription body | V2Ray subscription spec; stdlib, no deps                 |
| `crypto/rand`     | stdlib  | Short token generation          | Cryptographically secure; URL-safe                       |
| `qrcode.react`    | v4.x    | React QR code component         | SVG rendering, TypeScript, zero deps; active maintenance |

### Supporting

| Library            | Version | Purpose              | When to Use                                      |
| ------------------ | ------- | -------------------- | ------------------------------------------------ |
| `gopkg.in/yaml.v3` | v3      | Clash YAML serialize | Go backend only; when generating Clash format    |
| `net/url`          | stdlib  | Query param parsing  | `?format=clash` detection                        |
| `strings`          | stdlib  | UA contains check    | `strings.Contains(strings.ToLower(ua), "clash")` |

### Alternatives Considered

| Instead of       | Could Use        | Tradeoff                                          |
| ---------------- | ---------------- | ------------------------------------------------- |
| qrcode.react     | react-qr-code    | More downloads; qrcode.react has SVG, better a11y |
| crypto/rand      | uuid.NewString   | UUID is 36 chars; user wants short token          |
| gopkg.in/yaml.v3 | gopkg.in/yaml.v2 | v3 is maintained; v2 has security issues          |

**Installation:**

```bash
# Frontend
cd web && pnpm add qrcode.react

# Backend (Go) - no extra deps for Base64; YAML if needed:
go get gopkg.in/yaml.v3
```

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── api/
│   ├── subscription.go   # Subscription handler (no auth)
│   └── routes.go         # Add /sub/{token} route
├── core/
│   └── subscription.go   # SubscriptionGenerator: build Base64/Clash content
└── db/
    └── user.go           # Add SubscriptionToken column
web/src/components/users/
├── UserFormModal.tsx     # Extend: add subscription info section
└── UserSubscriptionCard.tsx  # New: dark card, nodes, QR, copy
```

### Pattern 1: Subscription Endpoint (No Auth)

**What:** Subscription route is outside `/api/**`; no session middleware.

**When to use:** Client compatibility; subscriptions fetched by clients with no cookies.

**Example:**

```go
// routes.go: register outside api group
r.Get("/sub/{token}", SubscriptionHandler)

// subscription.go
func SubscriptionHandler(w http.ResponseWriter, r *http.Request) {
    token := chi.URLParam(r, "token")
    user, err := db.GetUserBySubscriptionToken(token)
    if err != nil || user == nil {
        http.Error(w, "not found", http.StatusNotFound)
        return
    }
    if !user.Enabled || isExpired(user) {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    if !isValid(user) {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    format := detectFormat(r)
    body := generateSubscription(user, format)
    w.Header().Set("Subscription-Userinfo", buildUserinfoHeader(user))
    w.Write(body)
}
```

### Pattern 2: Userinfo Header

**What:** `subscription-userinfo` header with upload, download, total, expire.

**Example (from Clash Verge Rev docs):**

```
subscription-userinfo:upload=1234; download=2234; total=1024000; expire=2218532293
```

- `upload`, `download`, `total`: bytes
- `expire`: Unix timestamp (seconds); omit if no expiry

**Example (Go):**

```go
func buildUserinfoHeader(u *db.User) string {
    parts := []string{
        fmt.Sprintf("upload=%d", u.TrafficUsed/2),  // approximate; Phase 6 has real stats
        fmt.Sprintf("download=%d", u.TrafficUsed/2),
        fmt.Sprintf("total=%d", u.TrafficLimit),
    }
    if u.ExpireAt != nil {
        parts = append(parts, fmt.Sprintf("expire=%d", u.ExpireAt.Unix()))
    }
    return strings.Join(parts, "; ")
}
```

### Anti-Patterns to Avoid

- **Don't put subscription under `/api/`:** Keeps it separate from auth; no CORS issues for clients.
- **Don't use User UUID as subscription token:** User decided short token; UUID is long and exposes user identity.
- **Don't hand-roll QR generation:** Use qrcode.react; SVG is scalable and accessible.

## Don't Hand-Roll

| Problem        | Don't Build          | Use Instead                | Why                                        |
| -------------- | -------------------- | -------------------------- | ------------------------------------------ |
| Random token   | `math/rand`          | `crypto/rand`              | Predictable tokens; security risk          |
| URL-safe chars | Custom base64        | `crypto/rand` + base32/hex | `rand.Text()` or custom base64url alphabet |
| QR code        | Canvas/SVG manual    | qrcode.react               | Encoding, error correction, size handling  |
| Clash YAML     | Manual string concat | gopkg.in/yaml.v3           | Escaping, structure, indentation           |

**Key insight:** Subscription formats are finicky; clients expect exact schemes. VLESS and Hysteria2 URI schemes have specific query params; ClashMeta YAML has required fields. Use verified formats and libraries.

## Common Pitfalls

### Pitfall 1: Wrong Host in Share Links

**What goes wrong:** `vless://uuid@0.0.0.0:443` or `vless://uuid@::1:443` — clients cannot connect.

**Why it happens:** Inbound `listen` is bind address (`::`, `0.0.0.0`); not the client-facing host.

**How to avoid:** Use `tls.server_name` from inbound ConfigJSON as host. If absent, use `host` field from config or require it in inbound setup. Fallback: document that TLS must have server_name.

**Warning signs:** Subscription links work in some clients but not others; localhost in share link.

### Pitfall 2: subscription-userinfo Header Case

**What goes wrong:** `Subscription-Userinfo` vs `subscription-userinfo` — some clients case-sensitive.

**Why it happens:** HTTP headers are case-insensitive per spec, but some Clash forks may expect lowercase.

**How to avoid:** Use `subscription-userinfo` (lowercase) as in Clash Verge Rev docs. Source: https://clashvergerev.com/en/guide/url_schemes

### Pitfall 3: Clash UA Detection Too Broad

**What goes wrong:** Non-Clash clients (e.g. generic HTTP libs with "clash" in UA) get YAML; they expect Base64.

**Why it happens:** Overly broad `strings.Contains(ua, "clash")`.

**How to avoid:** Only return YAML when (1) `?format=clash` is present, or (2) UA contains `clash` (case-insensitive). Default to Base64 for unknown clients. Document: `Clash`, `Clash Verge`, `ClashMeta`, `Stash`, etc. all use "clash" in UA.

### Pitfall 4: Hysteria2 Auth Format

**What goes wrong:** `hysteria2://user:pass@host` — wrong for userpass; password alone is typical.

**Why it happens:** Hysteria2 URI: `hysteria2://[auth@]hostname[:port]/?params`. Auth can be `password` or `user:password` for userpass.

**How to avoid:** Use `hysteria2://{password}@{host}:{port}/?sni={host}`. Per Hysteria2 URI spec: auth is password for single-user. Source: https://v2.hysteria.network/docs/developers/URI-Scheme

## Code Examples

### VLESS Share Link (Base64 Subscription)

```go
// vless://uuid@host:port?params#name
// params: type=tcp, security=tls, flow=xtls-rprx-vision
func buildVLessLink(u *db.User, host string, port uint, name string) string {
    params := url.Values{
        "type":     {"tcp"},
        "security": {"tls"},
        "flow":     {"xtls-rprx-vision"},
    }.Encode()
    raw := fmt.Sprintf("vless://%s@%s:%d?%s#%s",
        u.UUID, host, port, params, url.PathEscape(name))
    return raw
}
```

### Hysteria2 Share Link

```go
// hysteria2://password@host:port/?sni=host
func buildHysteria2Link(password, host string, port uint, name string) string {
    raw := fmt.Sprintf("hysteria2://%s@%s:%d/?sni=%s#%s",
        url.PathEscape(password), host, port, host, url.PathEscape(name))
    return raw
}
```

### Base64 Subscription Body

```go
// V2Ray format: newline-separated links, Base64 encoded
var links []string
for _, node := range nodes {
    links = append(links, buildLink(node))
}
body := base64.StdEncoding.EncodeToString([]byte(strings.Join(links, "\n")))
w.Header().Set("Content-Type", "text/plain; charset=utf-8")
w.Write([]byte(body))
```

### ClashMeta Proxy Entry (VLESS)

```yaml
# From wiki.metacubex.one/config/proxies/vless
- name: "vless-tcp"
  type: vless
  server: example.com
  port: 443
  uuid: uuid
  network: tcp
  servername: example.com
  flow: xtls-rprx-vision
  tls: true
```

### ClashMeta Proxy Entry (Hysteria2)

```yaml
# From wiki.metacubex.one/config/proxies/hysteria2
- name: "hysteria2"
  type: hysteria2
  server: example.com
  port: 443
  password: yourpassword
  sni: example.com
```

### Short Token Generation

```go
// 16 chars, URL-safe (a-z0-9)
func generateSubscriptionToken() string {
    const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        panic(err)
    }
    for i := range b {
        b[i] = charset[int(b[i])%len(charset)]
    }
    return string(b)
}
```

### UA Detection (Claude's Discretion)

```go
func wantClashFormat(r *http.Request) bool {
    if r.URL.Query().Get("format") == "clash" {
        return true
    }
    ua := strings.ToLower(r.Header.Get("User-Agent"))
    return strings.Contains(ua, "clash")
}
```

## State of the Art

| Old Approach       | Current Approach      | When Changed          | Impact                                 |
| ------------------ | --------------------- | --------------------- | -------------------------------------- |
| VMess share links  | VLESS + XTLS Vision   | Xray/V2Ray 5          | VLESS is stateless; no time dependency |
| Base64 only        | Base64 + Clash YAML   | ClashMeta clients     | ClashMeta supports VLESS/H2 natively   |
| No userinfo header | subscription-userinfo | Clash Verge Rev v1.6+ | Clients display traffic/expiry         |

**Deprecated/outdated:**

- VMess for new configs: VLESS preferred; VMess has time sync issues.
- Plain `tcp` without flow: VLESS XTLS Vision (`xtls-rprx-vision`) is standard for sing-box.

## Open Questions

1. **Subscription URL base**
   - What we know: Panel serves from request Host; frontend can use `window.location.origin + '/sub/' + token`.
   - What's unclear: Reverse proxy with different public URL (e.g. panel at `:8080`, public at `https://panel.example.com`).
   - Recommendation: Add optional env `SUB_URL_PREFIX` (e.g. `https://panel.example.com`) for API-returned full URL. Fallback: relative path; frontend builds URL from origin.

2. **Traffic split (upload/download)**
   - What we know: `subscription-userinfo` needs upload, download; Phase 6 has traffic stats.
   - What's unclear: Phase 5 has only `TrafficUsed` (no split).
   - Recommendation: Use `TrafficUsed/2` for both upload and download as placeholder; Phase 6 will supply real values.

## Sources

### Primary (HIGH confidence)

- [Clash Verge Rev - subscription-userinfo](https://clashvergerev.com/en/guide/url_schemes) — header format
- [Hysteria2 URI Scheme](https://v2.hysteria.network/docs/developers/URI-Scheme) — hysteria2:// format
- [ClashMeta VLESS proxy](https://wiki.metacubex.one/en/config/proxies/vless) — YAML structure
- [ClashMeta Hysteria2 proxy](https://wiki.metacubex.one/en/config/proxies/hysteria2) — YAML structure
- [XTLS/VLESS outbound](https://xtls.github.io/en/config/outbounds/vless.html) — flow, uuid

### Secondary (MEDIUM confidence)

- [V2Ray subscription spec](https://www.v2fly.org/en_US/v5/config/service/subscription.html) — Base64 container format
- [qrcode.react npm](https://www.npmjs.com/package/qrcode.react) — React QR library

### Tertiary (LOW confidence)

- WebSearch: Clash UA patterns — multiple clients use "clash" in UA; verified via Clash Verge Rev docs (response headers only when UA contains "clash")

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — stdlib + qrcode.react; official docs
- Architecture: HIGH — ARCHITECTURE.md + existing patterns; Clash Verge Rev docs
- Pitfalls: HIGH — documented host/header/UA issues

**Research date:** 2026-02-12
**Valid until:** 2026-03-12 (30 days; stable domain)
