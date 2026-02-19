---
phase: 02-inbound-management
plan: 05
subsystem: ui
tags: [react-hook-form, zod, shadcn, vless, hysteria2, sing-box]

# Dependency graph
requires:
  - phase: 02-inbound-management
    provides: Inbound list page, REST API, ConfigGenerator
provides:
  - InboundFormModal for add/edit VLESS and Hysteria2
  - config_json build/parse for API compatibility
  - Edit flow with GET /api/inbounds/:id returning config_json
affects: [02-inbound-management, 04-user-management]

# Tech tracking
tech-stack:
  added: [react-hook-form, @hookform/resolvers, zod, shadcn select, label, radio-group]
  patterns: [form modal with check error in top, FieldLabel with info tooltip]

key-files:
  created: [web/src/components/inbounds/InboundFormModal.tsx, web/src/components/ui/label.tsx, web/src/components/ui/select.tsx, web/src/components/ui/radio-group.tsx]
  modified: [web/package.json, web/src/pages/Inbounds.tsx, internal/api/inbounds.go]

key-decisions:
  - "GET /api/inbounds/:id returns config_json for edit form population"
  - "window.confirm for delete (per plan: AlertDialog or window.confirm)"

patterns-established:
  - "buildConfigJson: frontend builds config_json matching ConfigGenerator/API structure"
  - "Check error shown in Modal top, form stays open per CONTEXT"

# Metrics
duration: ~25min
completed: 2026-02-11
---

# Phase 2 Plan 5: Inbound Form Modal Summary

**InboundFormModal with unified add/edit for VLESS and Hysteria2, react-hook-form + zod, sections (基本设置/TLS/传输), TLS dropdown (VLESS: 无/TLS/Reality; Hysteria2: TLS only), transport radio, info tooltips, check error in Modal top**

## Performance

- **Duration:** ~25 min
- **Tasks:** 3
- **Files modified:** 10 (created 4, modified 6)

## Accomplishments

- InboundFormModal with protocol select, conditional TLS/transport fields
- VLESS: 无 TLS / TLS / Reality; TLS cert paths; Reality handshake fields
- Hysteria2: TLS only; up_mbps, down_mbps, obfs_password
- Transport: TCP / WebSocket / gRPC / HTTP/2 with conditional path/service_name/host
- buildConfigJson and parseConfigFromJson for API round-trip
- Info tooltips on all fields (label + Info icon)
- Submit: POST/PUT to /api/inbounds; 400 shows checkError in Modal, does not close
- Edit: fetch GET /api/inbounds/:id (with config_json), populate form
- Delete: window.confirm, DELETE /api/inbounds/:id, invalidate query

## Task Commits

1. **Task 1: Form shell and dependencies** - `b926d6c` (feat)
2. **Task 2: TLS and transport fields** - `af5d581` (feat)
3. **Task 3: Smart defaults, info tooltips, submit flow** - `872110a` (feat)

## Files Created/Modified

- `web/src/components/inbounds/InboundFormModal.tsx` - Add/edit form modal
- `web/src/components/ui/label.tsx` - shadcn Label
- `web/src/components/ui/select.tsx` - shadcn Select
- `web/src/components/ui/radio-group.tsx` - shadcn RadioGroup
- `web/src/pages/Inbounds.tsx` - Wire add/edit/delete, fetch inbound for edit
- `internal/api/inbounds.go` - GET /api/inbounds/:id returns config_json for edit

## Decisions Made

- GET /api/inbounds/:id extended to return config_json for edit form population (Rule 2 - critical for edit)
- window.confirm for delete confirmation (per plan: "AlertDialog or window.confirm")

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] GET /api/inbounds/:id returns config_json**
- **Found during:** Task 3 (edit flow)
- **Issue:** Edit form needs config_json to populate TLS/transport fields; list endpoint didn't include it
- **Fix:** Added inboundDetail struct with config_json, GetInboundHandler returns it
- **Files modified:** internal/api/inbounds.go
- **Committed in:** 872110a (Task 3)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Essential for edit to work. No scope creep.

## Issues Encountered

None - shadcn CLI add failed (network) so label/select/radio-group were manually created from registry patterns.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 2 complete. Inbound add/edit/delete wired; add VLESS/Hysteria2 via modal.
- Ready for Phase 3 (cert management) or Phase 4 (user allocation).

## Self-Check: PASSED

- All created files exist
- All task commits exist (b926d6c, af5d581, 872110a)

---
*Phase: 02-inbound-management*
*Completed: 2026-02-11*
