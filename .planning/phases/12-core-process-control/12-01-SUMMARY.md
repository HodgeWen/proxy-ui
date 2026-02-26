---
phase: 12-core-process-control
plan: 01
subsystem: api
tags: [go, sing-box, lifecycle, process-control]

# Dependency graph
requires:
  - phase: 11-ui-animation-foundation
    provides: UI animation foundation and polling-based interaction baseline
provides:
  - Core four-state lifecycle source of truth with failure context snapshot
  - Semantic start/stop/restart APIs with structured error codes
  - Logs tail endpoint for error-state troubleshooting
  - Regression tests for lifecycle and control API semantics
affects: [12-02, core-status-ui, process-control-actions]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Lifecycle snapshot pattern (state + actions + lastError)
    - Semantic process control error mapping (code/message/detail)

key-files:
  created:
    - internal/core/lifecycle_state.go
    - internal/core/lifecycle_state_test.go
    - internal/api/core_test.go
  modified:
    - internal/core/process.go
    - internal/api/core.go
    - internal/api/routes.go

key-decisions:
  - "State priority fixed to not_installed > running > error > stopped to avoid misleading control hints."
  - "Process control methods return semantic codes instead of silent success for missing binary or invalid lifecycle transitions."
  - "Status API keeps legacy running field while introducing state/actions/lastError for compatibility-first migration."

patterns-established:
  - "Core lifecycle APIs should expose action hints directly from backend state instead of front-end inference."
  - "Control endpoints should return normalized {code,message,detail} payloads for deterministic UI branching."

requirements-completed: [CORE-01, CORE-02, CORE-03, CORE-04]

# Metrics
duration: 5 min
completed: 2026-02-26
---

# Phase 12 Plan 01: Core Process Control Summary

**Shipped a four-state sing-box lifecycle model with semantic control endpoints, startup-failure context capture, and logs-tail diagnostics for UI-driven process control.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-26T01:23:19Z
- **Completed:** 2026-02-26T01:29:01Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- Added `not_installed/stopped/running/error` lifecycle resolution with deterministic action matrix.
- Reworked process management into explicit `Start/Stop/Restart` semantics and startup health recheck to classify `CORE_START_FAILED`.
- Upgraded core APIs with state-driven actions, structured errors, and `GET /api/core/logs` tail reading for failure triage.

## Task Commits

Each task was committed atomically:

1. **Task 1: 实现四态生命周期模型与语义化进程控制** - `d6c09a4` (feat)
2. **Task 2: 改造 Core API 为状态驱动动作接口** - `164044d` (feat)
3. **Task 3: 增补后端测试覆盖四态与动作语义** - `6ebc792` (test)

**Plan metadata:** pending docs commit

## Files Created/Modified
- `internal/core/lifecycle_state.go` - Four-state lifecycle snapshot and action matrix with last-failure context memory.
- `internal/core/process.go` - Semantic lifecycle control (`Start/Stop/Restart`) and startup health validation window.
- `internal/api/core.go` - State/action/lastError status output, structured control errors, and logs endpoint.
- `internal/api/routes.go` - Registration for `/api/core/start`, `/api/core/stop`, `/api/core/restart`, `/api/core/logs`.
- `internal/core/lifecycle_state_test.go` - Lifecycle priority, action matrix, and failure snapshot tests.
- `internal/api/core_test.go` - API behavior tests for semantic error codes, error-state actions, and logs paths.

## Decisions Made
- Used backend action matrix as the single source of truth (`running→stop/restart`, `stopped→start`, `not_installed→install`, `error→retry_start/view_logs`) to prevent UI misguidance.
- Preserved `running` in status response while adding `state/actions/lastError` to keep existing callers stable during migration.
- Standardized control errors to `{code,message,detail}` and mapped process semantic errors to deterministic HTTP statuses.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Backend lifecycle and control contracts are ready for Phase `12-02` frontend wiring.
- No blockers identified for continuing this phase.

## Self-Check: PASSED
- Verified summary file exists on disk.
- Verified all task commit hashes are present in git history.

---
*Phase: 12-core-process-control*
*Completed: 2026-02-26*
