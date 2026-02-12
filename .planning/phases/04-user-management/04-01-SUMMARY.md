---
phase: 04-user-management
plan: 01
subsystem: database, core
tags: gorm, user, user-inbound, sing-box, config-generator, uuid

# Dependency graph
requires:
  - phase: 02-inbound-management
    provides: Inbound model, ConfigGenerator, inbound-to-sing-box mapping
  - phase: 03-certificate-management
    provides: Certificate model, resolveCertInTLS
provides:
  - User model with UUID, Password, TrafficLimit, TrafficUsed, ExpireAt, Enabled
  - UserInbound many-to-many linking User to Inbound
  - GetUsersForInbound: valid users (enabled, under limit, not expired)
  - ConfigGenerator derives sing-box users from DB, not config_json
affects: 04-02 user API, 04-03 frontend, 04-04 batch ops

# Tech tracking
tech-stack:
  added: github.com/google/uuid (direct dep for CreateUser)
  patterns: GORM many2many user_inbounds, GetUsersForInbound filter

key-files:
  created: internal/db/user.go
  modified: internal/db/db.go, internal/core/generator.go, go.mod, go.sum

key-decisions:
  - "User model is source of truth; config_json users ignored per RESEARCH"
  - "GetUsersForInbound filters in memory after JOINS (enabled, under limit, not expired, UTC)"
  - "VLESS: name+uuid+flow; Hysteria2: name+password per sing-box spec"

patterns-established:
  - "User-Inbound many-to-many via user_inbounds; Preload Inbounds for CRUD"
  - "ConfigGenerator: db.GetUsersForInbound(ib.ID) per inbound; never cfg[\"users\"]"

# Metrics
duration: ~5min
completed: 2026-02-12
---

# Phase 4 Plan 1: User Model and ConfigGenerator User Derivation Summary

**User model with UserInbound many-to-many; ConfigGenerator derives sing-box users from DB and filters disabled/over-limit/expired users**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-02-12T02:14:30Z
- **Completed:** 2026-02-12T02:19:00Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- User model with Name, Remark, UUID, Password, TrafficLimit, TrafficUsed, ExpireAt, Enabled
- UserInbound many-to-many via GORM; users and user_inbounds tables
- CRUD: ListUsers(keyword), GetUserByID, CreateUser, UpdateUser, DeleteUser
- CreateUser auto-generates UUID and Password via uuid.NewString() when empty
- GetUsersForInbound(inboundID): returns only valid users (enabled, under limit, not expired)
- ConfigGenerator: vlessToSingBox and hysteria2ToSingBox call db.GetUsersForInbound; no users from config_json

## Task Commits

Each task was committed atomically:

1. **Task 1: User model and UserInbound** - `2f9e4db` (feat)
2. **Task 2: ConfigGenerator user derivation** - `9a95485` (feat)

## Self-Check: PASSED

- internal/db/user.go: FOUND
- 04-01-SUMMARY.md: FOUND
- Commits 2f9e4db, 9a95485: FOUND
- go build ./...: PASSED

## Files Created/Modified

- `internal/db/user.go` - User model, UserInbound (many2many), CRUD, GetUsersForInbound
- `internal/db/db.go` - Add User to AutoMigrate
- `internal/core/generator.go` - vlessToSingBox/hysteria2ToSingBox derive users from DB; remove config_json users
- `go.mod` / `go.sum` - go mod tidy (google/uuid direct)

## Decisions Made

None - followed plan as specified. User model and ConfigGenerator behavior match RESEARCH and CONTEXT.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- User model and ConfigGenerator foundation complete
- Ready for 04-02: User REST API, ListUsers with filters, CreateUser with inbound_ids
- User CRUD API can wire to frontend; ConfigGenerator will include users when UserInbound links exist

---
*Phase: 04-user-management*
*Completed: 2026-02-12*
