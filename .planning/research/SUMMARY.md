# Project Research Summary

**Project:** s-ui — sing-box proxy management panel  
**Domain:** Proxy management / network infrastructure  
**Researched:** 2026-02-11  
**Confidence:** HIGH

---

## Executive Summary

s-ui is a sing-box proxy management panel targeting the sing-box ecosystem. Unlike Xray-based panels (3x-ui, Marzban), sing-box has no management API—the panel controls it via JSON config file and process lifecycle. Experts build these panels as a control-plane/data-plane split: the panel (Go + React) persists config to SQLite, generates sing-box JSON, validates with `sing-box check`, then restarts the binary. The data plane (sing-box) runs as an external process.

The recommended approach is Go 1.24+ backend with Chi router, GORM + SQLite, and React 18/19 frontend with Vite, shadcn/ui, and Tailwind v4. Chi aligns with sing-box (which uses Chi internally); SQLite enables single-binary deployment. The frontend uses modal-driven forms, smart defaults, and field tooltips to differentiate from dated 3x-ui UX. Scope v1 to VLESS + Hysteria2 only; defer traffic stats, REST API, Telegram bot, and multi-protocol sprawl.

Key risks: (1) Panel auth bypass and HTTPS→HTTP redirect (3x-ui issues); (2) sing-box config schema drift and protocol-specific mistakes (Hysteria2 port range `-` vs `:`); (3) V2Ray API requires custom sing-box build—stats may need Phase 2 research. Mitigate with no default credentials, respect `X-Forwarded-Proto`, run `sing-box check` before every apply, and atomic config writes.

---

## Stack Recommendations

(From STACK.md)

### Backend (Go)

| Technology | Version | Purpose | Rationale |
|------------|---------|---------|------------|
| **Go** | 1.24+ | Runtime | sing-box 1.13.0 requires Go 1.24 minimum; same language enables single-binary deployment |
| **Chi** | v5.x | HTTP router | sing-box uses Chi; lighter than Gin; stdlib-compatible |
| **GORM** | v1.31+ | ORM | Industry standard; 3x-ui uses GORM + SQLite; migrations, soft deletes |
| **embed** | stdlib | Static assets | Bundle React build into single binary; no CGO |
| **certmagic** | v0.25+ | ACME TLS | Same library sing-box uses; panel HTTPS |

### Frontend (React)

| Technology | Version | Purpose | Rationale |
|------------|---------|---------|------------|
| **React** | 18.x or 19.x | UI framework | Stable; shadcn supports both |
| **Vite** | 6.x | Build tool | CRA deprecated; fast HMR, ESM |
| **TypeScript** | 5.x | Type safety | Required for shadcn; API types shared with Go |
| **shadcn/ui** | latest | Component library | Copy-paste; dark theme; `new-york` style |
| **Tailwind CSS** | v4 | Styling | shadcn native |
| **TanStack Query** | v5 | Server state | Caching, invalidation, loading/error states |
| **react-bits** | — | Micro-interactions | 110+ animated components; PROJECT.md requirement |
| **Sonner** | — | Toast | shadcn-recommended |
| **React Hook Form** | v7 | Forms | shadcn Forms + Zod validation |
| **React Router** | v7 | Routing | SPA routing |

### Database & sing-box Integration

- **SQLite** — Primary store; single file, portable; GORM driver. Optional PostgreSQL for multi-instance.
- **sing-box** — Config-file driven; no gRPC. Panel: read config → modify → validate (`sing-box check`) → write → restart.

### Avoid

- Create React App, Gin (use Chi), Fiber, PostgreSQL as default, Next.js, Redux, direct sing-box API.

---

## Feature Landscape

(From FEATURES.md)

### Table Stakes (Must Have)

| Category | Features |
|----------|----------|
| **Inbound** | Add/Edit/Delete inbound; VLESS + Hysteria2; port, TLS, tag config; modal forms |
| **User** | Create/Edit/Delete user; assign to inbound(s); traffic cap; expiry; UUID generation |
| **Traffic & Stats** | Per-user and per-inbound traffic stats; online client count |
| **Subscription** | Subscription URL per user; Base64 format; QR code; Sing-box client compatible |
| **Certificate** | Manual cert path; auto-apply cert to inbound |
| **System** | sing-box config apply/restart; basic auth; HTTPS for panel |

### Differentiators (Should Have)

| Category | Features |
|----------|----------|
| **Inbound** | Smart defaults; field tooltips; modal-driven forms; advanced transport (WebSocket first) |
| **User** | User search/filter |
| **Subscription** | Subscription info page (usage/expiry) |
| **Certificate** | One-click Let's Encrypt; auto-renewal |
| **System** | Docker / bash / single-binary deploy (all three required) |

### Defer to v2+

- Traffic statistics (Phase 2—need sing-box API research)
- Bulk user actions
- REST API
- Clash/ClashMeta subscription formats
- Periodic traffic limits
- Database backup/restore
- External subscription merge

### Anti-Features (Do Not Build)

- System resource monitoring (CPU/RAM)
- Multi-language/i18n (Chinese-only)
- Protocols beyond VLESS + Hysteria2
- Telegram bot
- Mobile app
- Multiple admin roles
- v2board-style SaaS (plans, payments)
- CLI tool

---

## Architecture Overview

(From ARCHITECTURE.md)

**Control-plane / data-plane separation.** Panel manages sing-box as an external process via config file. DB is source of truth; ConfigGenerator reads from DB and produces sing-box JSON.

### Major Components

1. **Frontend (React SPA)** — Modal forms, shadcn/ui, TanStack Query, session auth. Talks only to Backend API.
2. **Backend API (Go)** — Chi router, session middleware (SCS), `/api/*` routes, embedded static assets.
3. **sing-box Integration Layer** — ConfigGenerator (DB → JSON), ProcessManager (start/stop/restart), StatsClient (V2Ray API gRPC, optional).
4. **Database (GORM + SQLite)** — Inbound, User, UserInbound, Certificate, TrafficStat models.
5. **Certificate Manager** — Manual path + optional certbot/ACME.
6. **Subscription Service** — `/sub/:token` endpoint; Base64-encoded proxy list; token maps to user.

### Key Data Flow

- **Config update:** User edit → API → DB transaction → ConfigGenerator → temp file → `sing-box check` → atomic move → ProcessManager.Restart()
- **Subscription:** GET `/sub/:token` → validate token → fetch user + inbounds → build URIs → Base64 encode → return

### sing-box Constraints

- No hot-reload; config changes require full restart.
- Always run `sing-box check` before applying config.
- Use temp file + atomic rename for config writes.
- Tag convention: `{protocol}-in-{id}` for stats mapping.

---

## Key Risks & Mitigations

(From PITFALLS.md)

### Critical

| Risk | Mitigation |
|------|------------|
| **Panel auth bypass / weak auth** | No default credentials; bcrypt/argon2; session timeout; login rate limiting; secure cookie flags |
| **HTTPS→HTTP redirect** (3x-ui #2857) | Respect `X-Forwarded-Proto`; generate URLs from config, not request headers; test behind reverse proxy |
| **sing-box CVE-2023-43644** | Pin sing-box ≥ 1.4.5; check version on startup; document minimum version |
| **Listen address over-exposure** | Default panel to `127.0.0.1`; provide sensible inbound listen defaults |
| **Subscription token exposure** | Opaque token; support rotation; avoid logging; no Referer leakage |

### sing-box Integration

| Risk | Mitigation |
|------|------------|
| **Deprecated/unknown field errors** | Run `sing-box check` before every apply; track deprecated list; test against latest stable |
| **Protocol-specific mistakes** (Hysteria2 port `:` vs `-`) | Protocol-specific validation; field tooltips; smart defaults |
| **Config file corruption/race** | Temp file + atomic rename; file lock; single writer; validate before restart |

### Certificate & Deployment

| Risk | Mitigation |
|------|------------|
| **ACME rate limits** | Use staging for dev; backoff; cache success; one account per deployment |
| **Cert renewal failure** | Renew 30 days before expiry; log attempts; in-app warning when &lt; 14 days |
| **Docker volume permissions** | chown in Dockerfile; document paths and permissions |

### UX

| Risk | Mitigation |
|------|------------|
| **Form validation too late** | Client-side format validation; field-level server errors; preserve form state |
| **Filter reset on navigation** | Persist filter in URL query or sessionStorage |

---

## Recommended Build Order

### Phase 1: Foundation

**Rationale:** DB, auth, and core integration before features.  
**Delivers:** GORM + SQLite, session auth, ConfigGenerator (minimal), ProcessManager, basic frontend shell.  
**Avoids:** 1.1 (auth), 1.4 (listen), 6.2 (Docker permissions).

### Phase 2: Inbound Management

**Rationale:** Base for users and subscriptions.  
**Delivers:** Inbound CRUD (VLESS, Hysteria2), modal forms, smart defaults, field tooltips.  
**Avoids:** 1.3 (CVE), 2.1 (deprecated), 2.2 (protocol), 2.3 (config race).

### Phase 3: Certificate Management

**Rationale:** Required for TLS inbounds.  
**Delivers:** Manual cert path, Cert API, optional Certbot integration.  
**Avoids:** 4.1 (rate limit), 4.2 (proxy), 4.3 (renewal).

### Phase 4: User Management

**Rationale:** Users attach to inbounds; core value.  
**Delivers:** User CRUD, traffic cap, expiry, user-inbound assignment, search/filter.  
**Avoids:** 1.5 (token), 7.1 (validation), 7.2 (filter).

### Phase 5: Subscription System

**Rationale:** Delivers value to end users; depends on users + inbounds.  
**Delivers:** `/sub/:token`, Base64 proxy list, QR code, subscription info page.  
**Avoids:** 1.5 (token exposure).

### Phase 6: Traffic & Statistics

**Rationale:** Operational visibility; depends on running sing-box with V2Ray API.  
**Delivers:** StatsClient, per-user/per-inbound stats, dashboard.  
**⚠️ Needs research:** V2Ray API requires `with_v2ray_api` build tag; verify default sing-box build. Fallback: defer or log parsing.

### Phase 7: Deployment & Polish

**Rationale:** Production readiness.  
**Delivers:** Docker, bash install script, single-binary with embed; HTTPS; reverse proxy testing.  
**Avoids:** 1.2 (HTTPS redirect), 6.1, 6.3 (Docker TUN).

### Dependency Chain

```
Foundation → Inbound → Cert (parallel after Inbound)
         → User (after Inbound)
         → Subscription (after User)
         → Stats (after core running)
```

---

## Open Questions

1. **Traffic stats source:** V2Ray API requires custom sing-box build. Is `with_v2ray_api` in default releases? Document custom build requirement or defer stats to Phase 2.
2. **react-bits compatibility:** Verify Tailwind v4 + shadcn compatibility with react-bits micro-interactions.
3. **ACME for inbound certs:** sing-box supports `acme` in TLS config with `with_acme` build tag. Prefer built-in or external certbot/acme.sh?
4. **Subscription token design:** Per-user UUID vs opaque token? Rotation without changing user identity?
5. **Config merge:** If user edits config.json manually, document overwrite or implement merge (higher complexity)?

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | sing-box changelog, Chi usage, Vite default, shadcn docs |
| Features | HIGH | Ecosystem analysis; 3x-ui, Marzban, s-ui feature sets |
| Architecture | HIGH | Official sing-box config docs; standard 3-tier pattern |
| Pitfalls | HIGH | 3x-ui GitHub issues; CVE; sing-box deprecation list |

**Overall confidence:** HIGH

### Gaps to Address

- **Traffic stats:** V2Ray API build tag availability—validate during Phase 6 planning.
- **ACME integration:** Panel HTTPS vs inbound certs—clarify certmagic scope.
- **react-bits:** Verify shadcn v4 + Tailwind v4 compatibility.

---

## Sources

### Primary (HIGH confidence)

- sing-box changelog, go.mod, configuration docs
- 3x-ui go.mod, GitHub issues (#2857, #2806)
- shadcn/ui installation, changelog (Tailwind v4)
- CVE-2023-43644, sing-box deprecated list
- Let's Encrypt rate limits

### Secondary (MEDIUM confidence)

- react-bits (DavidHDev)
- DeepWiki s-ui architecture
- Docker TUN limitations

---
*Research completed: 2026-02-11*  
*Ready for roadmap: yes*
