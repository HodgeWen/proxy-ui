---
phase: 12-core-process-control
plan: 02
subsystem: ui
tags: [react, tanstack-query, lifecycle-ui, state-matrix]

# Dependency graph
requires:
  - phase: 12-core-process-control
    provides: Backend state/actions/logs APIs from 12-01
provides:
  - Four-state Core page rendering with contextual lifecycle guidance
  - State-matrix lifecycle controls mapped to start/stop/restart/retry/logs
  - Error-state log viewer dialog backed by /api/core/logs
affects: [13-core-update-progress, core-management-ui]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Frontend lifecycle contract module (state constants + state metadata + action matrix)
    - State-driven action rendering with backend action intersection for safe UI controls

key-files:
  created:
    - web/src/lib/core-status.ts
  modified:
    - web/src/pages/Core.tsx

key-decisions:
  - "Centralized Core lifecycle state helpers in web/src/lib/core-status.ts to avoid scattered if/else state handling."
  - "Lifecycle buttons are rendered from state matrix and backend-provided actions intersection so error state cannot surface restart."
  - "View Logs uses on-demand API fetch with dialog loading/empty/error states instead of silent failure."

patterns-established:
  - "Core status pages should map state metadata and actions through a shared contract module."
  - "Lifecycle mutations must always invalidate ['core','status'] after success and show semantic toast feedback."

requirements-completed: [CORE-01, CORE-02, CORE-03, CORE-04]

# Metrics
duration: 4 min
completed: 2026-02-26
---

# Phase 12 Plan 02: Core Process Control Summary

**Delivered a fully state-driven Core management UI with four-state rendering, lifecycle action matrix, install guidance, and error-log diagnostics.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-26T01:31:47Z
- **Completed:** 2026-02-26T01:36:45Z
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments
- Added frontend lifecycle contract definitions (`CORE_STATES`, `ACTIONS_BY_STATE`, `getStateMeta`) for unified status semantics.
- Refactored `Core.tsx` to render running/stopped/not_installed/error specific messaging and matrix-based action buttons.
- Implemented error-state `View Logs` flow via `/api/core/logs` with explicit loading, empty, and error UI states.

## Task Commits

Each task was committed atomically:

1. **Task 1: 更新前端状态契约并接入 start/stop/restart API** - `1345806` (feat)
2. **Task 2: 实现四态差异化 UI 与动作矩阵渲染** - `c44d24e` (feat)
3. **Task 3: 补齐错误态日志查看与交互反馈** - `2cb26cd` (feat)

**Plan metadata:** pending docs commit

## Files Created/Modified
- `web/src/lib/core-status.ts` - 定义前端核心四态常量、状态元数据与动作矩阵。
- `web/src/pages/Core.tsx` - 按状态驱动生命周期动作渲染，接入 start/stop/restart/retry 与日志查看对话框。

## Decisions Made
- 采用前端状态契约模块集中维护状态文案、样式和动作映射，降低后续扩展成本。
- 生命周期动作区以状态矩阵为主、后端 actions 为约束，防止 error 状态误露出 Restart。
- 日志查看改为按需读取并显式反馈 loading/empty/error，确保错误态具备可诊断路径。

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 12 frontend goals are complete and aligned with backend contracts from 12-01.
- Ready to proceed to Phase 13 (Core Update Progress).

## Self-Check: PASSED
- Verified summary file exists on disk.
- Verified task commit hashes (`1345806`, `c44d24e`, `2cb26cd`) exist in git history.

