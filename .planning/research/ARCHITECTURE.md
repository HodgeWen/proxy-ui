# Architecture Research: s-ui

**Project:** sing-box proxy management panel  
**Researched:** 2026-02-11  
**Mode:** Ecosystem — architecture dimension  
**Downstream:** Roadmap phase structure

---

## System Overview

s-ui uses a **control-plane / data-plane separation** architecture. The panel (control plane) manages sing-box as an **external process** via config file and process lifecycle. sing-box (data plane) runs as a separate binary; the panel does not embed sing-box as a library.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Browser (User)                                    │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  Frontend (React SPA)          │  Backend API (Go)                        │
│  - shadcn/ui + Tailwind        │  - Chi router, REST endpoints            │
│  - Modal forms, TanStack Query │  - Session auth                          │
│  - Dark theme                  │  - Serves /api/*, embeds static assets    │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  sing-box Integration Layer                                              │
│  - ConfigGenerator (DB → JSON)                                            │
│  - ProcessManager (start/stop/restart)                                     │
│  - StatsClient (V2Ray API gRPC → traffic)                                  │
└─────────────────────────────────────────────────────────────────────────┘
           │                    │                    │
           ▼                    ▼                    ▼
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│  SQLite (GORM)   │  │  sing-box binary  │  │  V2Ray API        │
│  - Inbounds      │  │  - config.json   │  │  - stats.users    │
│  - Users         │  │  - systemd/exec  │  │  - stats.inbounds │
│  - Certs         │  │                  │  │  (gRPC, optional) │
└──────────────────┘  └──────────────────┘  └──────────────────┘
```

**Key principle:** Control plane remains available when sing-box is restarted or misconfigured. Config changes are persisted to DB first, then applied to sing-box.

---

## Components

### 1. Frontend (React SPA)

| Responsibility | Details |
|----------------|---------|
| **UI** | Modal-driven forms (no page navigation for CRUD), shadcn/ui components, dark theme |
| **State** | TanStack Query for server state; local state for modals |
| **Auth** | Session-based (cookie); login page, protect routes |
| **API** | `fetch` or axios to `/api/*`; no direct sing-box access |

**Boundaries:** Talks only to Backend API. No WebSocket required for MVP (polling for stats is acceptable).

### 2. Backend API (Go)

| Responsibility | Details |
|----------------|---------|
| **HTTP** | Chi router, CORS, static file serving (embedded React build) |
| **Auth** | Session middleware (e.g. alexedwards/scs); login/logout endpoints |
| **Routes** | `/api/inbounds`, `/api/users`, `/api/certs`, `/api/stats`, `/api/sub/:token`, `/api/core` (start/stop/restart) |
| **Services** | InboundService, UserService, CertService, SubscriptionService, CoreService |

**Boundaries:** Orchestrates Integration Layer; never directly reads/writes sing-box config file outside Integration Layer.

### 3. sing-box Integration Layer

Central abstraction for all sing-box interaction. Three sub-components:

| Sub-component | Responsibility | Implementation |
|---------------|----------------|----------------|
| **ConfigGenerator** | Build sing-box JSON from DB models | Inbound models → VLESS/Hysteria2 `users` arrays; TLS, log, route, experimental (V2Ray API) |
| **ProcessManager** | Start/stop/restart sing-box | `sing-box run -c config.json`; systemd or `exec.Command`; `sing-box check` before apply |
| **StatsClient** | Fetch traffic stats | V2Ray API gRPC (if built with `with_v2ray_api`); periodic poll, aggregate to DB |

**Config flow:**
1. ConfigGenerator produces full config JSON from DB
2. Write to temp file → `sing-box check -c temp.json` → if OK, move to `config.json`
3. ProcessManager restarts sing-box so it picks up new config

**Boundaries:** Only layer that touches sing-box config file, binary, or API. All other code goes through this layer.

### 4. Database Layer

| Responsibility | Details |
|----------------|---------|
| **ORM** | GORM + SQLite |
| **Models** | Inbound, User, UserInbound (many-to-many), Certificate, TrafficStat |
| **Migrations** | Auto-migrate on startup |

**Schema highlights:**
- `inbounds`: protocol, tag, listen, port, tls_config, settings (JSON for protocol-specific)
- `users`: name, uuid (VLESS), password (H2), traffic_limit, expire_at
- `user_inbounds`: user_id, inbound_id (assignment)
- `certificates`: path, domain, acme_config (JSON)

### 5. Certificate Manager

| Responsibility | Details |
|----------------|---------|
| **Manual** | Store cert path; reference in inbound TLS config |
| **ACME** | Optional: certmagic for panel HTTPS; for inbound certs, two options: (a) sing-box built-in ACME in TLS config, or (b) external certbot/acme.sh + path |

**Recommendation:** Start with manual path + optional certbot instructions. Add ACME automation in a later phase. sing-box supports `acme` in TLS config (see [TLS docs](https://sing-box.sagernet.org/configuration/shared/tls/)); requires `with_acme` build tag.

### 6. Subscription Service

| Responsibility | Details |
|----------------|---------|
| **URL** | `/sub/:token` — token maps to user; returns Base64-encoded proxy list |
| **Format** | One `vless://` or `hysteria2://` URI per assigned inbound; newline-separated; Base64-encode entire string |
| **Auth** | Token in URL; validate against user (or dedicated subscription token) |

**Boundaries:** Reads from UserService + InboundService; no sing-box dependency.

---

## Data Flow

### Config Update Flow

```
User edits inbound in UI
    → POST /api/inbounds/:id
    → InboundService.Update()
    → DB transaction: save inbound + user assignments
    → ConfigGenerator.Generate() from DB
    → Write temp config → sing-box check
    → If OK: atomic move to config.json
    → ProcessManager.Restart()
    → Commit DB transaction
```

**Rollback:** If `sing-box check` fails or restart fails, DB transaction rolls back; previous config.json remains.

### Traffic Stats Flow

```
Cron (e.g. every 10s)
    → StatsClient.FetchStats()
    → V2Ray API gRPC: GetStats() for stats.users, stats.inbounds
    → Map API response to user/inbound IDs (via tag → inbound mapping)
    → TrafficStatService.Save(interval_stats)
    → Frontend GET /api/stats → from DB
```

**Fallback:** If V2Ray API unavailable (sing-box not built with `with_v2ray_api` or not configured), stats remain at 0 or use log parsing (defer to later phase).

---

## sing-box Integration

### Config Generation

sing-box config is JSON with this structure ([config reference](https://sing-box.sagernet.org/configuration/)):

| Section | Panel responsibility |
|---------|----------------------|
| `log` | Level, output path |
| `inbounds` | One object per DB inbound; VLESS/Hysteria2 `users` from UserService |
| `outbounds` | Minimal: `direct`, `block` |
| `route` | Simple rules; default direct |
| `experimental.v2ray_api` | If stats needed: `listen`, `stats.enabled`, `stats.users`, `stats.inbounds` |

**VLESS inbound** ([ref](https://sing-box.sagernet.org/configuration/inbound/vless/)):
```json
{
  "type": "vless",
  "tag": "vless-in-1",
  "listen": "::",
  "listen_port": 443,
  "users": [
    { "name": "user1", "uuid": "...", "flow": "xtls-rprx-vision" }
  ],
  "tls": { "enabled": true, "server_name": "example.com", "certificate_path": "...", "key_path": "..." }
}
```

**Hysteria2 inbound** ([ref](https://sing-box.sagernet.org/configuration/inbound/hysteria2/)):
```json
{
  "type": "hysteria2",
  "tag": "h2-in-1",
  "listen": "::",
  "listen_port": 8080,
  "users": [
    { "name": "user1", "password": "..." }
  ],
  "tls": { "enabled": true, "server_name": "example.com", "certificate_path": "...", "key_path": "..." }
}
```

**Tag convention:** Use deterministic `{protocol}-in-{id}` (e.g. `vless-in-1`) so StatsClient can map API response back to DB inbound.

### Process Management

| Action | Implementation |
|--------|----------------|
| **Start** | `exec.Command("sing-box", "run", "-c", configPath)` or `systemctl start sing-box` |
| **Stop** | `systemctl stop sing-box` or `Process.Kill()` |
| **Restart** | Stop → (optional brief wait) → Start |
| **Status** | Check process existence; optional: `sing-box check` on config |

**Important:** sing-box has **no hot-reload**. Config changes require a full restart. Connection drops during restart; keep restart time minimal.

**Config validation:** Always run `sing-box check -c /path/to/config.json` before replacing production config. Use temp file + atomic move.

### Traffic Stats Collection

**V2Ray API** ([ref](https://sing-box.sagernet.org/configuration/experimental/v2ray-api/)):

- **Type:** gRPC
- **Build tag:** `with_v2ray_api` (not in default build; requires custom compile)
- **Config:**
```json
"experimental": {
  "v2ray_api": {
    "listen": "127.0.0.1:8080",
    "stats": {
      "enabled": true,
      "inbounds": ["vless-in-1"],
      "outbounds": ["direct"],
      "users": ["user1"]
    }
  }
}
```

- **Stats:** Per-user and per-inbound uplink/downlink bytes via gRPC `GetStats()`.

**Alternative — Clash API:** RESTful, but focused on selector outbound switching. Traffic stats via Clash API are less standardized for per-user tracking. Prefer V2Ray API for stats.

**If V2Ray API unavailable:** Official sing-box releases may not include `with_v2ray_api` by default ([V2Ray API docs](https://sing-box.sagernet.org/configuration/experimental/v2ray-api/) state "not included by default"). Panel can either: (a) document custom build requirement, or (b) defer traffic stats to Phase 2. Log parsing is possible but complex.

---

## API Design

### REST Endpoints (recommended)

| Method | Path | Purpose |
|--------|------|---------|
| POST | /api/login | Session auth |
| POST | /api/logout | Clear session |
| GET | /api/inbounds | List inbounds |
| POST | /api/inbounds | Create inbound |
| PUT | /api/inbounds/:id | Update inbound |
| DELETE | /api/inbounds/:id | Delete inbound |
| GET | /api/users | List users |
| POST | /api/users | Create user |
| PUT | /api/users/:id | Update user |
| DELETE | /api/users/:id | Delete user |
| GET | /api/certs | List certs |
| POST | /api/certs | Add cert (path) |
| GET | /api/stats | Traffic stats (per user, per inbound) |
| GET | /api/core/status | sing-box running? |
| POST | /api/core/restart | Restart sing-box |
| GET | /sub/:token | Subscription (no auth; token = user token) |

### Response Format

```json
{ "data": {...}, "error": null }
{ "data": null, "error": "message" }
```

---

## Suggested Build Order

Recommended phase ordering based on dependencies:

| Phase | Components | Rationale |
|-------|-------------|----------|
| **1. Foundation** | DB, Auth, ConfigGenerator (minimal), ProcessManager | Need DB and core integration before features |
| **2. Inbound Management** | InboundService, Inbound API, ConfigGenerator (full) | Base for users and subscriptions |
| **3. Certificate Management** | CertService, Cert API | Required for TLS inbounds |
| **4. User Management** | UserService, User API, User-Inbound assignment | Users attach to inbounds |
| **5. Subscription Service** | SubscriptionService, /sub/:token | Delivers value; depends on users + inbounds |
| **6. Traffic Stats** | StatsClient, V2Ray API config, StatsJob | Depends on running sing-box with v2ray_api |

**Dependency chain:**
```
Foundation → Inbound → Cert (parallel possible after Inbound)
         → User (after Inbound)
         → Subscription (after User)
         → Stats (after core running)
```

---

## Key Architecture Decisions

| Decision | Rationale |
|----------|-----------|
| **External sing-box process** | Aligns with PROJECT.md; avoids forking/embedding sing-box; simpler upgrades |
| **Config file as contract** | sing-box has no management API; JSON config is the only control surface |
| **ProcessManager owns restart** | Config changes require restart; single place to handle failure modes |
| **V2Ray API for stats** | Per-user stats require `stats.users`; V2Ray API provides this |
| **Subscription as separate endpoint** | No auth; token in path; high traffic; keep off main API |
| **DB-first config updates** | ConfigGenerator reads from DB; never from config file. DB is source of truth |
| **Chi over Gin** | STACK.md; aligns with sing-box ecosystem |

---

## Sources

| Source | Confidence |
|--------|------------|
| [sing-box configuration](https://sing-box.sagernet.org/configuration/) | HIGH |
| [sing-box VLESS inbound](https://sing-box.sagernet.org/configuration/inbound/vless/) | HIGH |
| [sing-box Hysteria2 inbound](https://sing-box.sagernet.org/configuration/inbound/hysteria2/) | HIGH |
| [sing-box experimental (V2Ray API, Clash API)](https://sing-box.sagernet.org/configuration/experimental/) | HIGH |
| [sing-box build tags](https://sing-box.sagernet.org/installation/build-from-source/) | HIGH |
| [sing-box TLS (ACME)](https://sing-box.sagernet.org/configuration/shared/tls/) | HIGH |
| [DeepWiki s-ui system architecture](https://deepwiki.com/alireza0/s-ui/1.1-system-architecture) | MEDIUM (alireza0/s-ui uses embedded sing-box; different integration model) |
| [3x-ui xray package](https://pkg.go.dev/github.com/mhsanaei/3x-ui/v2/xray) | MEDIUM |

---

## Confidence Assessment

| Area | Level | Notes |
|------|-------|-------|
| Component boundaries | HIGH | Standard 3-tier; integration layer clearly scoped |
| sing-box config format | HIGH | Official docs |
| Process management | HIGH | No hot-reload; restart required |
| V2Ray API for stats | MEDIUM | Requires custom sing-box build; verify default build tags |
| Build order | HIGH | Follows FEATURES.md dependency chain |
