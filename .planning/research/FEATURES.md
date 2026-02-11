# Features Research: s-ui

**Domain:** sing-box proxy management panel  
**Researched:** 2026-02-11  
**Mode:** Ecosystem — feature landscape

---

## Existing Panel Analysis

| Panel | Core | Stars | Focus | Key Characteristics |
|-------|------|-------|-------|---------------------|
| **3x-ui** | Xray | 30K | Multi-protocol, multi-user | Most popular; VMess/VLESS/Trojan/SS/Wireguard; traffic limits; Telegram bot; geofile updates; DB export/import; known for functional but dated UI |
| **s-ui (alireza0)** | sing-box | 7.4K | sing-box native | Multi-protocol (VLESS, VMess, Trojan, SS, Hysteria/H2, Naive, TUIC); routing UI; subscription JSON+info; dark/light theme; API; Windows support |
| **Marzban** | Xray | 6K | Commercial-friendly | Python+React; REST API; multi-node; periodic traffic limits; Clash/V2ray subscription; Telegram bot; CLI; webhook notifications |
| **Hiddify** | Xray + sing-box | 8.3K | Anti-censorship, multi-core | 20+ protocols; smart proxy routing; sing-box + Xray; user management; dedicated client; DoH; auto CDN; WARP |
| **v2board** | Multi-agent | — | Subscription business | PHP/Laravel; MySQL/Redis; plans, orders, payments; multi-agent (XrayR, etc.); not sing-box native |

**Insight:** s-ui (this project) targets the sing-box subset. 3x-ui and Marzban use Xray; v2board is a full SaaS stack. The gap: sing-box panels with modern UX, modal-driven forms, and smart defaults.

---

## Feature Categories

### Inbound Management

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Add/Edit/Delete inbound | Core CRUD; every panel has it | Low | Modal forms preferred (per PROJECT.md) |
| Protocol selection (VLESS, Hysteria2) | Users expect at least 2 protocols | Low | s-ui v1: VLESS + Hysteria2 only |
| Port configuration | Required for any inbound | Low | Listen address, port |
| TLS configuration | HTTPS for production; clients expect it | Medium | Enable/disable, cert path or auto |
| Tag/name for inbound | Routing and identification | Low | sing-box tag field |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Smart defaults (best-practice presets) | Minimize user input; fewer mistakes | Low | Pre-fill flow, transport, sane ports |
| Field tooltips (info icon + hover) | Lower barrier; explain VLESS flow, TLS options | Low | PROJECT.md explicitly requires |
| Modal-driven forms (no page nav) | Single-page feel; faster workflow | Low | Differentiates from 3x-ui page-based UI |
| Advanced transport options (WebSocket, gRPC, HTTP/2) | Power users need flexibility | Medium | Start with WS; add gRPC later |

---

### User Management

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Create/Edit/Delete user | All panels have user CRUD | Low | Username, UUID (VLESS) or password (H2) |
| Assign user to inbound(s) | User must map to nodes | Low | One-to-many or many-to-many |
| Traffic limit (cap) | Essential for paid/resource control | Medium | Per-user bytes; reset options |
| Expiration date | Users expect time-bound access | Low | Expiry date per user |
| UUID generation | VLESS requires UUID | Low | Auto-generate, copy |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Bulk actions (delete, activate, reset) | Hiddify v11; saves time at scale | Medium | Defer to v2 |
| Periodic traffic limits (daily/weekly) | Marzban; subscription-style usage | High | Defer; not in s-ui scope |
| User search/filter | 3x-ui has global search | Low | Table stakes for 10+ users |

---

### Traffic & Statistics

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Per-user traffic stats | Users expect to see consumption | Medium | Requires sing-box API or log parsing |
| Per-inbound traffic stats | Operators need node-level visibility | Medium | Same data source |
| Online client count | Basic operational awareness | Low | If sing-box API supports |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Real-time dashboard / charts | Marzban, s-ui; better UX | Medium | Optional; defer if complex |
| System resource monitoring (CPU/RAM) | 3x-ui, Marzban; PROJECT.md explicitly out of scope | Medium | **Anti-feature** for s-ui — keep panel focused |

---

### Subscription System

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Subscription URL per user | Clients (Sing-box, V2rayNG) expect this | Medium | Base64-encoded proxy list |
| Base64 format (V2ray subscription) | Standard; Sing-box clients accept | Low | `vless://`, `hysteria2://` etc. |
| QR code generation | Mobile setup; common expectation | Low | Per-node or per-subscription |
| Compatible with Sing-box clients | Target ecosystem is sing-box | Low | Use sing-box URI format |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Multi-format (Clash, ClashMeta) | Marzban; broader client support | High | Defer; VLESS/H2 clients prefer V2ray format |
| External subscription merge | s-ui allows adding external links | Medium | Defer to v2 |
| Subscription info page | s-ui shows usage/expiry on sub URL | Low | Nice UX; table stakes for some |

---

### Certificate Management

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Manual cert path (existing cert) | Operators often have certs | Low | Path to fullchain + privkey |
| Auto-apply cert to inbound | TLS must use cert | Low | Link cert to inbound config |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| One-click Let's Encrypt (Certbot) | 3x-ui, Marzban; reduces friction | Medium | Certbot integration; standalone domain |
| Auto-renewal (cron/certbot) | Prevents expiry | Low | Certbot renews by default |
| ACME (acme.sh) | Alternative to certbot | Medium | Hiddify uses acme.sh; equivalent outcome |

---

### System & Operations

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| sing-box config apply/restart | Panel must push config to core | Medium | Edit JSON, reload sing-box |
| Basic auth for panel | Prevent unauthorized access | Low | Username/password |
| HTTPS for panel | Production security | Low | Use cert from cert management |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Docker / bash / single-binary deploy | PROJECT.md requires all three | Medium | Core deployment requirement |
| REST API | 3x-ui, Marzban, s-ui; automation | High | Defer to v2 |
| Telegram bot | 3x-ui, Marzban; remote management | High | **Anti-feature** for v1 — scope creep |
| CLI tool | Marzban; scripting | Medium | **Anti-feature** for v1 |
| Database backup/restore | 3x-ui, Hiddify; disaster recovery | Medium | Defer; use SQLite file backup initially |

---

## Anti-Features

Things to deliberately NOT build (aligned with PROJECT.md and research).

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **System resource monitoring** (CPU/RAM/bandwidth) | Not core; keeps panel focused | Omit; users use `htop` or external monitoring |
| **Multi-language/i18n** | PROJECT.md: Chinese-only; reduces complexity | Single locale only |
| **Protocols beyond VLESS + Hysteria2** | v1 scope; 3x-ui/s-ui bloat from protocol sprawl | Add later if validated |
| **Telegram bot** | Scope creep; 3x-ui/Marzban have it but s-ui targets simplicity | Omit for v1 |
| **Full v2board-style SaaS** (plans, payments, orders) | Different product category; PHP stack | Stay personal/small-group panel |
| **Mobile app** | PROJECT.md: Web-first | Web responsive only |
| **Multiple admin roles** | Marzban has multi-admin; adds complexity | Single admin for v1 |

---

## Feature Dependencies

```
Inbound Management ──┬──► User Management (users attach to inbounds)
                     └──► Subscription System (sub links include inbound configs)

Certificate Management ──► Inbound Management (TLS inbounds need certs)

User Management ──► Traffic & Statistics (stats are per-user)
User Management ──► Subscription System (each user gets sub link)

Traffic & Statistics ──► sing-box API or log parsing (external dependency)
```

**Recommended build order:**

1. **Inbound Management** — foundation
2. **Certificate Management** — needed for TLS inbounds
3. **User Management** — core value
4. **Subscription System** — delivers value to end users
5. **Traffic & Statistics** — operational visibility

---

## Complexity Assessment

| Feature Area | Overall Complexity | Risk |
|-------------|-------------------|------|
| Inbound Management | Low–Medium | sing-box JSON schema; modal forms are straightforward |
| User Management | Low | Standard CRUD + assignment logic |
| Traffic & Statistics | Medium | Depends on sing-box API capabilities; may need log parsing |
| Subscription System | Low–Medium | Base64 encoding; URI format; well-documented |
| Certificate Management | Medium | Certbot/acme.sh integration; certificate lifecycle |
| System & Operations | Low | Config file editing, process restart |

---

## MVP Recommendation

**Prioritize (table stakes):**

1. Inbound CRUD (VLESS, Hysteria2) via modal forms
2. User CRUD with traffic cap + expiry
3. Subscription link generation (Base64, sing-box compatible)
4. TLS certificate management (manual path + optional Certbot)
5. Smart defaults + field tooltips
6. Modern dark theme UI
7. Docker / bash / single-binary deployment

**Defer:**

- Traffic statistics (Phase 2 — need sing-box API/research)
- Bulk user actions
- REST API
- Clash/ClashMeta subscription formats
- Multi-protocol beyond VLESS + Hysteria2

---

## Sources

- [3x-ui GitHub](https://github.com/MHSanaei/3x-ui) — features, README
- [alireza0/s-ui GitHub](https://github.com/alireza0/s-ui) — sing-box panel features
- [Marzban GitHub](https://github.com/Gozargah/Marzban) — features, README
- [Hiddify-Manager GitHub](https://github.com/hiddify/Hiddify-Manager) — protocols, features
- [v2board docs](https://docs.v2board.com/)
- [sing-box VLESS inbound](https://sing-box.sagernet.org/configuration/inbound/vless/)
- [Subscription format (Base64)](https://xray-checker.kutovoy.dev/configuration/subscription/)
- PROJECT.md (s-ui requirements, out of scope)
