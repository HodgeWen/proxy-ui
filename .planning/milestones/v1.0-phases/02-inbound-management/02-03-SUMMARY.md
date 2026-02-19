---
phase: 02-inbound-management
plan: 03
subsystem: api
tags: [chi, rest, inbounds, config-generator, apply-config, sing-box]

# Dependency graph
requires:
  - phase: 02-01
    provides: Inbound model, db CRUD
  - phase: 02-02
    provides: ConfigGenerator.Generate
provides:
  - REST API for inbound CRUD: GET list, GET :id, POST create, PUT update, DELETE
  - Config apply flow: Generate -> ApplyConfig -> Restart on mutate
  - Check failure returns 400 with error (frontend Modal display)
affects: [02-04, 02-05]

# Tech tracking
tech-stack:
  added: []
  patterns: [DB-first mutate flow, rollback on ApplyConfig failure]

key-files:
  created: [internal/api/inbounds.go]
  modified: [internal/api/routes.go, internal/db/inbound.go]

key-decisions:
  - "Create/Update/Delete rollback DB on ApplyConfig failure; return 400 with check error"
  - "Restart on success is best-effort; config persisted even if restart fails"

patterns-established:
  - "Inbound mutate: persist -> Generate -> ApplyConfig; on failure rollback, return error"

# Metrics
duration: 3min
completed: 2026-02-11
---

# Phase 02 Plan 03: Inbound REST API Summary

**Full CRUD REST API for inbounds with ConfigGenerator apply flow, rollback on check failure, and error returned to frontend**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-02-11T10:05:46Z
- **Completed:** 2026-02-11T10:14:43Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- ListInboundsHandler and GetInboundHandler with tls_type/transport_type derived from config_json
- CreateInboundHandler: tag uniqueness, db.Create, Generate, ApplyConfig, Restart; rollback on failure
- UpdateInboundHandler: fetch-before-save, same apply flow, revert on failure
- DeleteInboundHandler: delete, Generate, ApplyConfig, Restart; rollback on failure
- Check failure returns 400 with {"error": err.Error()} for frontend Modal display

## Task Commits

Each task was committed atomically:

1. **Task 1: List and Get handlers** - `d8d4794` (feat)
2. **Task 2: Create, Update, Delete with config apply flow** - `313607a` (feat)

**Plan metadata:** (pending docs commit)

## Files Created/Modified
- `internal/api/inbounds.go` - List, Get, Create, Update, Delete handlers; tls_type/transport_type derivation
- `internal/api/routes.go` - /api/inbounds routes with RequireAuth
- `internal/db/inbound.go` - InboundExistsByTag for tag uniqueness validation

## Decisions Made
- Rollback DB on ApplyConfig failure; return 400 with check error per CONTEXT (Modal display)
- Restart after apply is best-effort; config file persisted even if restart fails
- Tag uniqueness validated before Create; on Update, only if tag changed

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- sing-box not installed in dev environment; verification confirmed ApplyConfig failure returns 400 with error and DB rollback

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Inbound REST API complete; frontend can consume CRUD endpoints
- 02-04 (Inbounds page UI) and 02-05 (Inbound form) can proceed

## Self-Check: PASSED

- [ -f internal/api/inbounds.go ] && echo FOUND || echo MISSING
- git log --oneline | grep d8d4794
- git log --oneline | grep 313607a

---
*Phase: 02-inbound-management*
*Completed: 2026-02-11*
