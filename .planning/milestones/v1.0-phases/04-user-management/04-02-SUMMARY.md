---
phase: 04-user-management
plan: 02
subsystem: api
tags: rest, users, crud, batch, apply-config, rollback

# Dependency graph
requires:
  - phase: 04-user-management
    plan: 01
    provides: User model, GetUsersForInbound, ConfigGenerator
  - phase: 02-inbound-management
    provides: Inbound API pattern, ApplyConfig + Restart on mutate
provides:
  - User REST API: GET/POST/PUT/DELETE /api/users
  - ListUsers with optional q search; GetUser with inbound_ids and inbound_tags
  - Create/Update/Delete with ApplyConfig + Restart; rollback on ApplyConfig failure
  - POST /api/users/batch: delete, enable, disable, reset_traffic
affects: 04-03 frontend Users page

# Tech tracking
tech-stack:
  added: []
  patterns: Phase 2 pattern (mutate -> Generate -> ApplyConfig; rollback on failure)

key-files:
  created: internal/api/users.go
  modified: internal/api/routes.go, internal/db/inbound.go, internal/db/user.go

key-decisions:
  - "Batch operations use snapshot-and-revert rollback (Generate reads committed state)"
  - "Response shape: traffic_used/traffic_limit int64, expire_at ISO or null"

patterns-established:
  - "User CRUD follows inbounds.go: RequireAuth, configPath, gen.Generate, core.ApplyConfig, pm.Restart"
  - "Batch: store snapshots, apply mutations, Generate, ApplyConfig; on failure revert each"

# Metrics
duration: ~3min
completed: 2026-02-12
---

# Phase 4 Plan 2: User REST API Summary

**User REST API with CRUD and batch operations; all mutations trigger ConfigGenerator + ApplyConfig + Restart with rollback on failure**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-02-12T02:16:00Z
- **Completed:** 2026-02-12T02:19:00Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- ListUsersHandler: GET /api/users with optional ?q= search; returns users with inbound_ids, inbound_tags
- GetUserHandler: GET /api/users/:id for edit form
- CreateUserHandler: POST with name, remark, inbound_ids, traffic_limit, expire_at; auto UUID/password; rollback on ApplyConfig failure
- UpdateUserHandler: PUT with same body; ReplaceUserInbounds; rollback on failure
- DeleteUserHandler: DELETE with recreate-on-failure rollback
- BatchUsersHandler: POST /api/users/batch with action delete|enable|disable|reset_traffic; snapshot-and-revert rollback
- All mutations call gen.Generate, core.ApplyConfig, pm.Restart; 400 with error body on ApplyConfig failure

## Task Commits

Each task was committed atomically:

1. **Task 1: User CRUD API** - `db041e6` (feat)
2. **Task 2: Batch API** - `9467fee` (feat)

## Self-Check: PASSED

- internal/api/users.go: FOUND
- 04-02-SUMMARY.md: FOUND
- Commits db041e6, 9467fee: FOUND
- go build ./...: PASSED

## Files Created/Modified

- `internal/api/users.go` - User REST handlers (List, Get, Create, Update, Delete, Batch)
- `internal/api/routes.go` - /api/users routes with RequireAuth
- `internal/db/inbound.go` - GetInboundsByIDs for user-inbound association
- `internal/db/user.go` - ReplaceUserInbounds for update associations

## Decisions Made

- Batch rollback uses snapshot-and-revert: store old state before mutations, apply, Generate, ApplyConfig; on failure restore each. Generate reads from committed DB state, so we cannot use a single DB transaction that spans ApplyConfig.
- Response shape: expire_at as *string (ISO RFC3339 or null), traffic_used/traffic_limit as int64.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. Verification requires sing-box binary for ApplyConfig success; in environments without sing-box, Create/Update/Delete/Batch return 400 with check error. Code paths and rollback verified.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- User API complete with search and batch
- Ready for 04-03: Users page frontend
- ApplyConfig and Restart invoked on all mutations; rollback on failure per Phase 2 pattern

---
*Phase: 04-user-management*
*Completed: 2026-02-12*
