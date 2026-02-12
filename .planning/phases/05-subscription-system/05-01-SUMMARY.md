---
phase: 05-subscription-system
plan: 01
subsystem: api
tags: [subscription, vless, hysteria2, clash, base64, yaml]

# Dependency graph
requires:
  - phase: 04-user-management
    provides: User model, UserInbound, GetUsersForInbound, Inbound with ConfigJSON
provides:
  - User.SubscriptionToken column and GetUserBySubscriptionToken
  - SubscriptionGenerator: Base64 and ClashMeta formats
  - GET /sub/{token} public endpoint (no auth)
affects: [05-02, 05-03]

# Tech tracking
tech-stack:
  added: [gopkg.in/yaml.v3]
  patterns: [subscription endpoint outside /api, host from tls.server_name]

key-files:
  created: [internal/core/subscription.go, internal/api/subscription.go, internal/db/user_test.go, internal/core/subscription_test.go, internal/api/subscription_test.go]
  modified: [internal/db/user.go, internal/db/db.go, internal/api/routes.go, go.mod]

key-decisions:
  - "Short token 16 chars a-z0-9 via crypto/rand (per RESEARCH)"
  - "subscription-userinfo header lowercase (per RESEARCH Pitfall 2)"
  - "Host from tls.server_name, fallback config[host] (per RESEARCH Pitfall 1)"

patterns-established:
  - "Subscription route outside /api, no auth"
  - "Format: UA contains 'clash' or ?format=clash -> YAML; else Base64"

# Metrics
duration: ~15min
completed: 2026-02-12
---

# Phase 05 Plan 01: Subscription Token & Handler Summary

**Subscription token on User model; SubscriptionGenerator for Base64 and ClashMeta; GET /sub/{token} public endpoint with UA/query format detection and 403 for disabled/expired/over-limit.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-02-12T05:44:04Z
- **Completed:** 2026-02-12
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments

- User.SubscriptionToken (16-char a-z0-9) with CreateUser auto-generation and backfill for existing users
- SubscriptionGenerator: BuildUserinfoHeader, GenerateBase64, GenerateClash, GetNodeLinks; host from tls.server_name
- GET /sub/{token} handler: 404 invalid token, 403 disabled/expired/over-limit; format UA/query; subscription-userinfo header

## Task Commits

Each task was committed atomically:

1. **Task 1: User model subscription token** - `b35745e` (feat)
2. **Task 2: SubscriptionGenerator (Base64 + Clash)** - `19a72dc` (feat)
3. **Task 3: GET /sub/{token} handler and route** - `e03a746` (feat)

**Plan metadata:** `1290896` (docs: complete 05-01 plan)

## Files Created/Modified

- `internal/db/user.go` - Added SubscriptionToken, generateSubscriptionToken, GetUserBySubscriptionToken; CreateUser auto-gen
- `internal/db/db.go` - backfillSubscriptionTokens in Init
- `internal/db/user_test.go` - TestCreateUser_SubscriptionToken, GetUserBySubscriptionToken
- `internal/core/subscription.go` - BuildUserinfoHeader, GenerateBase64, GenerateClash, GetNodeLinks; host from tls.server_name
- `internal/core/subscription_test.go` - GenerateBase64, GenerateClash, BuildUserinfoHeader tests
- `internal/api/subscription.go` - SubscriptionHandler
- `internal/api/subscription_test.go` - 404, 200 Base64, Clash UA, 403 disabled
- `internal/api/routes.go` - r.Get("/sub/{token}", SubscriptionHandler)

## Decisions Made

- None beyond plan—followed CONTEXT and RESEARCH decisions.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 02: Admin UI subscription info display (UserFormModal, UserSubscriptionCard, QR, copy)
- Plan 03: Token reset endpoint
- All must-haves verified: build passes, tests pass, subscription endpoint returns Base64/Clash per format

## Self-Check: PASSED

- Files: internal/db/user.go, internal/core/subscription.go, internal/api/subscription.go, internal/api/routes.go
- Commits: b35745e, 19a72dc, e03a746

---
*Phase: 05-subscription-system*
*Completed: 2026-02-12*
