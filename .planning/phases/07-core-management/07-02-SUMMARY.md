---
phase: 07-core-management
plan: 02
subsystem: ui
tags: [react, dashboard, sing-box, core, version, update, rollback]

# Dependency graph
requires:
  - phase: 07-01
    provides: GET /api/core/versions, POST /api/core/update, POST /api/core/rollback
provides:
  - Dashboard CoreVersionCard: current + latest, update/rollback with confirm, version list, new-version badge
affects: [07-core-management]

# Tech tracking
tech-stack:
  added: []
  patterns: [useQuery + useMutation for core APIs, confirm dialog pattern]

key-files:
  created: []
  modified: [web/src/pages/Dashboard.tsx]

key-decisions:
  - "Dashboard card badge only for new version (simpler than Sidebar)"
  - "latestStable = first prerelease=false; latestOverall = first release"

patterns-established:
  - "Pattern: Update/rollback with confirm dialog, Loader2 spinner during mutation"

# Metrics
duration: ~5min
completed: 2026-02-12
---

# Phase 07 Plan 02: Dashboard Core Version Management Summary

**Dashboard CoreVersionCard with current/latest display, update and rollback confirm dialogs, version list modal, and new-version badge**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-02-12T07:54:24Z
- **Completed:** 2026-02-12T08:00:00Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments

- Extended sing-box card to "状态与版本": current version, latest version, restart, update, rollback
- Update button opens confirm dialog; on confirm POST /api/core/update with Loader2 spinner
- Rollback button opens confirm dialog; on confirm POST /api/core/rollback
- Helpful message when SINGBOX_BINARY_PATH not set
- "查看所有版本" link opens modal with releases list; each row: tag + Badge stable (green) / pre-release (amber)
- "有新版本" badge on card header when update available

## Task Commits

Each task was committed atomically:

1. **Task 1: Core version card + update/rollback** - `cc18254` (feat)
2. **Task 2: Version list + new version badge** - `abf349f` (feat)

## Files Created/Modified

- `web/src/pages/Dashboard.tsx` - CoreVersionCard: fetch versions, current/latest display, update/rollback with confirm dialogs, version list modal, new-version badge

## Decisions Made

- Dashboard card badge only for "有新版本" (per CONTEXT: Sidebar or Dashboard; chose simpler)
- latestStable = first release with prerelease=false; latestOverall = first release
- Singleton SINGBOX_BINARY_PATH error surfaced via toast with extended duration

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no additional setup beyond 07-01 (SINGBOX_BINARY_PATH for update/rollback).

## Next Phase Readiness

- Phase 07 complete; core management backend + frontend delivered
- Admin can update and rollback sing-box from Dashboard
- All versions visible with stable/pre-release markers
- New version badge guides admin to update

## Self-Check: PASSED

- web/src/pages/Dashboard.tsx (modified): FOUND
- Commits cc18254, abf349f: FOUND

---
*Phase: 07-core-management*
*Completed: 2026-02-12*
