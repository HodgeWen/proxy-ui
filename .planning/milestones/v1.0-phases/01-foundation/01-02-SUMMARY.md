---
phase: 01-foundation
plan: 02
subsystem: core
tags: [sing-box, process, config, chi, rest]

requires:
  - phase: 01-foundation
    plan: 01
    provides: auth, session, routes
provides:
  - sing-box ProcessManager (Version, Check, IsRunning, Restart)
  - Atomic config write (temp + check + rename)
  - GET /api/core/status, POST /api/core/restart, POST /api/core/config
affects: [02-inbound, 06-traffic]

tech-stack:
  added: []
  patterns: [ProcessManager via exec, atomic config write, RequireAuth middleware]

key-files:
  created: [internal/core/process.go, internal/core/config.go, internal/api/core.go]
  modified: [internal/api/routes.go]

key-decisions:
  - "Config path from SINGBOX_CONFIG_PATH env, default ./config.json"
  - "IsRunning/Restart use pgrep/pkill (Linux standard)"
  - "ApplyConfig returns full check output on failure for frontend Modal"

patterns-established:
  - "Pattern: temp file + sing-box check + os.Rename for config writes"
  - "Pattern: RequireAuth middleware for protected API routes"

duration: 15min
completed: 2026-02-11
---

# Phase 1 Plan 2: sing-box Integration Summary

**sing-box process integration: version detection, config validation, atomic apply, restart via ProcessManager and REST API**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-02-11T08:47:44Z
- **Completed:** 2026-02-11
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments

- ProcessManager with Version(), Check(), IsRunning(), Restart()
- Atomic config write: temp file → sing-box check → rename on success
- Core API: GET /api/core/status, POST /api/core/restart, POST /api/core/config
- RequireAuth middleware for /api/core/* routes

## Task Commits

Each task was committed atomically:

1. **Task 1: sing-box ProcessManager** - `cd7634d` (feat)
2. **Task 2: Config write (temp + atomic rename)** - `22451c6` (feat)
3. **Task 3: Core API handlers** - `1be5b25` (feat)

## Files Created/Modified

- `internal/core/process.go` - ProcessManager: Version, Check, IsRunning, Restart
- `internal/core/config.go` - ApplyConfig (temp + check + rename)
- `internal/api/core.go` - StatusHandler, RestartHandler, ConfigHandler, RequireAuth
- `internal/api/routes.go` - /api/core/* routes with RequireAuth

## Decisions Made

- Config path from SINGBOX_CONFIG_PATH env, default ./config.json
- IsRunning/Restart use pgrep/pkill (Linux); Version uses sing-box version -n
- Check failure returns full stderr in error for frontend Modal display

## Deviations from Plan

None - plan executed exactly as written

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- sing-box integration complete; panel can detect version, apply config, restart
- Phase 2 (Inbound) can use ApplyConfig + Restart for config application
- Requires sing-box binary in PATH for full functionality

## Self-Check: PASSED

Verification performed:
- `go build ./...` succeeds
- GET /api/core/status returns `{"running":false,"version":""}` when sing-box not installed
- POST /api/core/config returns check error with executable-not-found message when sing-box missing
- Auth required: unauthenticated requests to /api/core/* return 401
