# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-11)

**Core value:** 提供一个界面美观、体验流畅、部署简单的 sing-box 管理面板

**Current focus:** Phase 6 — Traffic Statistics (Plan 2 complete)

## Current Position

Phase: 6 of 8 (Traffic Statistics)
Plan: 2/3 complete
Current Plan: 2
Total Plans in Phase: 3
Status: Plan 2 complete — ready for Plan 3
Last activity: 2026-02-12 — Phase 6 Plan 2 executed

Progress: [██████░░░░] 67% (Phase 6)

## Performance Metrics

**Velocity:**
- Total plans completed: 0
- Average duration: —
- Total execution time: —

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**
- Last 5 plans: —
- Trend: —
| Phase 01-foundation P01 | 70 | 3 tasks | 22 files |
| Phase 01-foundation P02 | 15 | 3 tasks | 5 files |
| Phase 01-foundation P03 | 5 | 3 tasks | 20+ files |
| Phase 02-inbound-management P01 | 5 | 2 tasks | 4 files |
| Phase 02 P02 | 1 | 2 tasks | 1 files |
| Phase 03-certificate-management P01 | 2 | 2 tasks | 3 files |
| Phase 03-certificate-management P02 | 5 | 2 tasks | 8 files |
| Phase 03-certificate-management P03 | 3 | 1 task | 1 file |
| Phase 04-user-management P02 | 3 | 2 tasks | 4 files |
| Phase 04-user-management P03 | 8 | 2 tasks | 8 files |
| Phase 05-subscription-system P01 | 15 | 3 tasks | 9 files |
| Phase 05-subscription-system P02 | 5 | 2 tasks | 3 files |
| Phase 05-subscription-system P03 | 20 | 3 tasks | 10 files |
| Phase 06-traffic-statistics P02 | 102 | 2 tasks | 6 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Roadmap: 8 phases derived from 42 requirements; Foundation → Inbound → Cert → User → Subscription → Traffic → Core → Deployment
- 01-01: SCS sqlite3store as separate module; sessions table in session.Init; SPA fallback via ServeContent to avoid FileServer 301
- 01-02: Config path from SINGBOX_CONFIG_PATH env; pgrep/pkill for process lifecycle; check output returned on ApplyConfig failure for frontend Modal
- 01-03: Custom ThemeProvider (defaultTheme=dark); Vite proxy /api for dev; shadcn sidebar + Dashboard status/restart
- 02-01: ConfigJSON uses gorm.io/datatypes.JSON; Inbound CRUD follows Admin pattern
- 02-02: ConfigGenerator reads from DB via ListInbounds; produces full sing-box JSON; VLESS/Hysteria2 inbound mapping
- 02-03: Inbound REST API with rollback on ApplyConfig failure; check error returned to frontend Modal
- 02-04: Edit inline, Delete in dropdown per CONTEXT; onEdit/onDelete no-op for Plan 05 to wire
- 02-05: GET /api/inbounds/:id returns config_json for edit form; InboundFormModal with react-hook-form, zod, info tooltips
- 02-06: Protocol sync when editing Hysteria2 inbound; FieldLabel for protocol and TLS type
- [Phase 02]: ConfigGenerator reads from DB via ListInbounds; produces full sing-box JSON; VLESS/Hysteria2 inbound mapping
- 03-01: Certificate model with CRUD; ConfigGenerator resolves certificate_id to paths; backward compat with inline paths
- 03-02: Cert CRUD API; Certificates page with table and form; delete blocked when cert in use
- 03-03: InboundFormModal cert selector; 选择证书 vs 手动输入; certificate_id in config_json
- 04-01: User model with UserInbound many-to-many; ConfigGenerator derives users from DB, not config_json
- [Phase 04-user-management]: Batch rollback uses snapshot-and-revert (Generate reads committed state)
- [Phase 04-user-management]: Traffic limit: user enters GB in form; convert to bytes for API (0 = unlimited)
- [Phase 04-user-management]: Expire_at: date input YYYY-MM-DD; send ISO midnight for API
- 04-04: Search query in TanStack Query key ["users", searchQ] to preserve filter after batch actions (RESEARCH Pitfall 5)
- 05-01: Short token 16 chars a-z0-9 via crypto/rand; subscription-userinfo lowercase; host from tls.server_name
- 05-02: SUB_URL_PREFIX env for full subscription URL; ListUsers subscription_url only; GetUser includes subscription_nodes
- 05-03: Subscription info in separate modal (not edit form); fallbackHost from request Host header; VLESS non-TLS security=none
- 06-01: Minimal stats proto in internal/statsproto; V2RAY_API_ENABLED gates ConfigGenerator v2ray_api and cron; delta aggregation for traffic
- [Phase 06]: ListInbounds(sort string): traffic_asc/traffic_desc order by (traffic_uplink+traffic_downlink); default created_at DESC
- [Phase 06]: reset_traffic clears TrafficUplink, TrafficDownlink, TrafficUsed

### Pending Todos

None yet.

### Blockers/Concerns

None yet.

## Session Continuity

Last session: 2026-02-12
Stopped at: Completed 06-02-PLAN.md (API Traffic Exposure)
Resume file: None
