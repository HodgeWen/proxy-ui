# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-11)

**Core value:** 提供一个界面美观、体验流畅、部署简单的 sing-box 管理面板

**Current focus:** Phase 10 — Sidebar Pages & Config Fix

## Current Position

Phase: 10 of 10 (Sidebar Pages & Config Fix)
Plan: 0/0 complete
Current Plan: —
Total Plans in Phase: 0
Status: Not planned yet
Last activity: 2026-02-19 — Phase 10 added

Progress: [░░░░░░░░░░] 0% (Phase 10)

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
| Phase 06-traffic-statistics P03 | 2 | 2 tasks | 6 files |
| Phase 07 P01 | 5 | 2 tasks | 4 files |
| Phase 07-core-management P02 | 5 | 2 tasks | 1 files |
| Phase 08-deployment-production P03 | 12 | 2 tasks | 1 file |
| Phase 08 P01 | 6 | 4 tasks | 11 files |
| Phase 08 P04 | 4 | 2 tasks | 3 files |
| Phase 08 P02 | 8 | 2 tasks | 4 files |
| Phase 09-certificate-config-sync P02 | 5 | 1 task | 1 file |
| Phase 09-certificate-config-sync P03 | 2 | 1 task | 1 file |

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
- 06-03: formatBytes shared in lib; InboundTable/UserTable 上行/下行 columns; sort dropdown; 超限 in status badge
- 07-01: binaryPath: SINGBOX_BINARY_PATH or dataDir/bin/sing-box; CoreUpdater ListReleases, Update, Rollback; single .backup; GET /versions, POST /update, POST /rollback
- 07-02: Dashboard CoreVersionCard: current+latest, update/rollback with confirm, version list modal, new-version badge
- [Phase 07-core-management]: Dashboard card badge only for new version; latestStable=first prerelease=false
- 08-01: glebarez/sqlite CGO-free; internal sqlStore; LoadConfig with CONFIG_PATH; FORCE_HTTPS for Cookie.Secure
- 08-02: Dockerfile multi-stage with s-ui + sing-box; docker-compose bind mount ./data:/data; sing-box 1.12.21 from GitHub releases
- 08-03: install.sh with install/update/uninstall; GITHUB_REPO=HodgeWen/proxy-ui; INSTALL_DIR=/usr/local/s-ui
- 08-04: GitHub Actions release on tag push v*; build-release for linux/amd64 and arm64; action-gh-release
- 09-01: UpdateCertificateHandler with Generate/Apply/Restart chain; rollback on ApplyConfig failure; routes pass panelCfg
- 09-02: CertificateFormModal checkError display for ApplyConfig 400 failure; match InboundFormModal pattern
- 09-03: 09-VERIFICATION.md with executable steps for Cert update -> Generate -> Apply -> Restart; closes v1.0-MILESTONE-AUDIT audit gap

### Pending Todos

None yet.

### Roadmap Evolution

- Phase 10 added: Sidebar Pages & Config Fix — 订阅/流量/核心独立页面 + 配置路径修复

### Blockers/Concerns

None yet.

## Session Continuity

Last session: 2026-02-19
Stopped at: Phase 10 context gathered
Resume file: .planning/phases/10-sidebar-pages-config-fix/10-CONTEXT.md
