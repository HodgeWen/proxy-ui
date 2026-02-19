---
phase: 05-subscription-system
plan: 03
subsystem: ui
tags: [subscription, qrcode, react, shadcn, modal]

# Dependency graph
requires:
  - phase: 05-subscription-system
    provides: GET /sub/{token}, subscription_url, subscription_nodes in GetUser, reset-subscription API
provides:
  - UserSubscriptionCard component (subscription info, nodes, QR, copy, reset)
  - UserSubscriptionModal (dedicated dialog for subscription info)
  - Subscription button in UserTable actions column
affects: []

# Tech tracking
tech-stack:
  added: [qrcode.react]
  patterns: [separate modal for detail view, fallbackHost from request Host header]

key-files:
  created: [web/src/components/users/UserSubscriptionCard.tsx, web/src/components/users/UserSubscriptionModal.tsx]
  modified: [web/src/components/users/UserFormModal.tsx, web/src/components/users/UserTable.tsx, web/src/pages/Users.tsx, internal/core/subscription.go, internal/api/subscription.go, internal/api/users.go]

key-decisions:
  - "Subscription info in separate modal, not inside edit form modal"
  - "FallbackHost from request Host header when tls.server_name absent"
  - "Support VLESS without TLS (security=none in share link)"

patterns-established:
  - "Dedicated modals for detail views (subscription, etc.) to keep edit forms focused"
  - "Link icon button in table actions for subscription access"

# Metrics
duration: ~20min
completed: 2026-02-12
---

# Phase 05 Plan 03: UserSubscriptionCard + Modal Summary

**UserSubscriptionCard with QR code, per-node copy, reset token in dedicated subscription modal; Link button in user table for quick access.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-02-12
- **Completed:** 2026-02-12
- **Tasks:** 3 (2 auto + 1 human-verify checkpoint)
- **Files modified:** 10

## Accomplishments

- UserSubscriptionCard: subscription link + copy, per-node links + copy, QR code (hidden by default), reset token with confirmation
- UserSubscriptionModal: dedicated dialog wrapping UserSubscriptionCard, fetches user detail for subscription_nodes
- Link icon button in UserTable actions column for direct subscription access
- Bug fixes: fallbackHost for inbounds without tls.server_name, SubscriptionToken preserved on UpdateUser, VLESS non-TLS share links

## Task Commits

Each task was committed atomically:

1. **Task 1: Install qrcode.react and create UserSubscriptionCard** - `af0df1b` (feat)
2. **Task 2: Integrate UserSubscriptionCard into UserFormModal** - `b34dcd4` (feat)
3. **Bug fixes: subscription link empty, token lost, missing nodes** - `be0f9be` (fix)
4. **Refactor: extract subscription into separate modal** - `5fc51db` (refactor)

**Checkpoint:** Human verification — approved

## Files Created/Modified

- `web/src/components/users/UserSubscriptionCard.tsx` - Subscription info display with QR, copy, reset
- `web/src/components/users/UserSubscriptionModal.tsx` - Dedicated dialog for subscription info
- `web/src/components/users/UserFormModal.tsx` - Removed subscription card (now in separate modal)
- `web/src/components/users/UserTable.tsx` - Added Link button + onSubscription callback
- `web/src/pages/Users.tsx` - Subscription modal state management
- `internal/core/subscription.go` - fallbackHost parameter, isTLSEnabled, VLESS non-TLS support
- `internal/api/subscription.go` - Extract request hostname as fallbackHost
- `internal/api/users.go` - SubscriptionToken preserved on update, extractRequestHost, fallbackHost threading

## Decisions Made

- Moved subscription card from edit form modal to dedicated modal (user feedback: edit modal too crowded)
- Added fallbackHost from request Host header for inbounds without tls.server_name
- VLESS without TLS uses security=none in share links

## Deviations from Plan

### Auto-fixed Issues

**1. [Bug Fix] Subscription link empty for inbounds without tls.server_name**
- **Found during:** Human verification
- **Issue:** extractHostFromInbound returned empty when no tls.server_name, causing empty subscription body
- **Fix:** Added fallbackHost parameter using request Host header
- **Committed in:** be0f9be

**2. [Bug Fix] SubscriptionToken lost on user update**
- **Found during:** Human verification
- **Issue:** UpdateUserHandler didn't copy SubscriptionToken from old user
- **Fix:** Added SubscriptionToken: old.SubscriptionToken
- **Committed in:** be0f9be

**3. [UX] Subscription info moved to separate modal**
- **Found during:** Human verification
- **Issue:** Edit modal too crowded with subscription info causing scrollbars
- **Fix:** Created UserSubscriptionModal, added Link button in table
- **Committed in:** 5fc51db

---

**Total deviations:** 3 (2 bug fixes, 1 UX refactor)
**Impact on plan:** Bug fixes essential for functionality; UX refactor improves usability. No scope creep.

## Issues Encountered

- Edit modal scrollbars due to subscription card content — resolved by extracting to separate modal

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 5 complete: subscription token, /sub endpoint, admin API, UI with QR and reset
- Ready for Phase 6: Traffic Statistics

## Self-Check: PASSED

- Files: UserSubscriptionCard.tsx, UserSubscriptionModal.tsx, UserTable.tsx, subscription.go
- Commits: af0df1b, b34dcd4, be0f9be, 5fc51db

---
*Phase: 05-subscription-system*
*Completed: 2026-02-12*
