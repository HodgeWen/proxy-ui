---
phase: 05-subscription-system
plan: 02
subsystem: api
tags: [subscription, reset-token, admin-api, users]

# Dependency graph
requires:
  - phase: 05-01
    provides: SubscriptionToken, GetNodeLinks, GetUserBySubscriptionToken
provides:
  - POST /api/users/:id/reset-subscription (admin token reset)
  - GetUser response: subscription_url, subscription_nodes (for UserSubscriptionCard)
affects: [05-03]

# Tech tracking
tech-stack:
  added: []
  patterns: [buildSubscriptionURL with SUB_URL_PREFIX, userFromDB includeNodes flag]

key-files:
  created: []
  modified: [internal/api/users.go, internal/api/routes.go, internal/db/user.go, internal/db/db.go]

key-decisions:
  - "subscription_url: path by default; full URL when SUB_URL_PREFIX env set (per RESEARCH Open Question 1)"
  - "ListUsers: subscription_url only; GetUser: subscription_url + subscription_nodes (lighter list payload)"

patterns-established:
  - "Reset token invalidates old link immediately (UpdateUser overwrites)"
  - "subscription_nodes from core.GetNodeLinks for per-node copy in admin UI"

# Metrics
duration: ~5min
completed: 2026-02-12
---

# Phase 05 Plan 02: Admin API Reset Token & Subscription Info Summary

**POST /api/users/:id/reset-subscription for admin token reset; GetUser and ListUsers extended with subscription_url and subscription_nodes for admin panel display.**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-02-12
- **Completed:** 2026-02-12
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- ResetSubscriptionHandler: require auth, parse id, generate new token via db.GenerateSubscriptionToken, update user; return subscription_url (path or full URL when SUB_URL_PREFIX set)
- userItem: SubscriptionURL, SubscriptionNodes (Name, Link); userFromDB sets SubscriptionURL; GetUser includes nodes, ListUsers excludes nodes for lighter payload
- Route POST /api/users/:id/reset-subscription under users group

## Task Commits

Each task was committed atomically:

1. **Task 1: Reset subscription token endpoint** - `cc31e88` (feat)
2. **Task 2: Extend GetUser with subscription_url and subscription_nodes** - `b81177a` (feat)

## Files Created/Modified

- `internal/api/users.go` - ResetSubscriptionHandler, buildSubscriptionURL; userItem SubscriptionURL/SubscriptionNodes; userFromDB(includeNodes)
- `internal/api/routes.go` - r.Post("/{id}/reset-subscription", ResetSubscriptionHandler(sm))
- `internal/db/user.go` - Export GenerateSubscriptionToken (was generateSubscriptionToken)
- `internal/db/db.go` - Use GenerateSubscriptionToken in backfill

## Decisions Made

- SUB_URL_PREFIX env: when set, subscription_url is full URL; else path only (frontend builds from origin)
- ListUsers returns subscription_url only; GetUser returns subscription_url and subscription_nodes (detail modal needs nodes for per-node copy)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 03: UserSubscriptionCard in user detail modal, QR code, copy buttons
- Reset endpoint and GetUser subscription fields ready for admin UI wiring

## Self-Check: PASSED

- Files: internal/api/users.go, internal/api/routes.go, internal/db/user.go, internal/db/db.go
- Commits: cc31e88, b81177a

---

_Phase: 05-subscription-system_
_Completed: 2026-02-12_
