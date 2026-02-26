---
phase: 13-core-update-progress
plan: 01
subsystem: api
tags: [sse, trylock, core-update, go]

requires:
  - phase: 12-core-process-control
    provides: lifecycle state and semantic core control APIs
provides:
  - async core update trigger with conflict-safe TryLock guard
  - SSE progress stream with snapshot-first delivery and flush semantics
  - real download progress behavior tests for known/unknown content-length
affects: [13-02 frontend-update-stream, core-update-ui]

tech-stack:
  added: []
  patterns: [trigger-stream split, global update snapshot source]

key-files:
  created: [.planning/phases/13-core-update-progress/13-01-SUMMARY.md]
  modified:
    - internal/core/update_progress_state.go
    - internal/core/update_progress_state_test.go
    - internal/core/updater_progress_test.go
    - internal/api/core.go
    - internal/api/routes.go
    - internal/api/core_test.go

key-decisions:
  - "POST /api/core/update returns 202 immediately and runs update in background goroutine."
  - "GET /api/core/update/stream uses SSE with no-buffer headers and snapshot-first subscription."
  - "409 conflict is enforced by backend TryLock regardless of UI button state."

patterns-established:
  - "Pattern: state.Begin -> background runner -> state.Publish -> state.Finish lifecycle."
  - "Pattern: SSE loop must flush each event and exit on r.Context().Done() for cleanup."

requirements-completed: [UPDT-01, UPDT-02]

duration: 3 min
completed: 2026-02-26
---

# Phase 13 Plan 01: Core Update Progress Summary

**Core update now runs as a conflict-safe async workflow with SSE snapshot streaming and real download percentage continuity.**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-26T03:23:31Z
- **Completed:** 2026-02-26T03:26:56Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- Added backend conflict guard for duplicate update triggers with stable HTTP 409 semantics.
- Added `/api/core/update/stream` SSE endpoint with required no-buffer headers, first snapshot push, and flush loop.
- Strengthened updater progress tests to enforce monotonic intermediate percentages and deterministic fallback.

## Task Commits

Each task was committed atomically:

1. **Task 1: 建立更新进度协调器与 TryLock 并发保护** - `42d377f` (fix)
2. **Task 2: 改造下载流程以输出真实百分比** - `794424b` (test)
3. **Task 3: 暴露更新触发与 SSE 流接口并补齐 API 测试** - `d393f56` (feat)

**Plan metadata:** pending (will be captured in docs commit for summary/state/roadmap).

## Files Created/Modified
- `internal/core/update_progress_state.go` - Unsubscribe only deregisters subscriber to avoid close/broadcast races.
- `internal/core/update_progress_state_test.go` - Adds unsubscribe broadcast-stop coverage.
- `internal/core/updater_progress_test.go` - Tightens assertions for monotonic and intermediate download percentages.
- `internal/api/core.go` - Implements async update trigger, 409 conflict handling, and SSE stream handler.
- `internal/api/routes.go` - Registers `/api/core/update/stream`.
- `internal/api/core_test.go` - Adds conflict and SSE snapshot/header tests.

## Decisions Made
- Update trigger remains lightweight and returns `202 Accepted` while update runs in background.
- Progress stream source is centralized in global update progress state to support refresh snapshot continuity.
- SSE protocol response is strictly unbuffered for reverse-proxy compatibility and real-time behavior.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed channel close in Unsubscribe to avoid send-on-closed panic race**
- **Found during:** Task 1 (进度协调器并发行为收敛)
- **Issue:** Broadcast and unsubscribe can race; closing subscriber channel allows panic on concurrent publish.
- **Fix:** Unsubscribe now removes subscriber from registry only; broadcasts target active subscribers.
- **Files modified:** `internal/core/update_progress_state.go`, `internal/core/update_progress_state_test.go`
- **Verification:** `go test ./internal/core -run 'UpdateProgress|TryLock' -count=1`
- **Committed in:** `42d377f`

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Deviation improved runtime safety without expanding scope; all plan goals remain intact.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
Backend contract for progress streaming and conflict semantics is ready for `13-02` frontend integration.

---
*Phase: 13-core-update-progress*
*Completed: 2026-02-26*
