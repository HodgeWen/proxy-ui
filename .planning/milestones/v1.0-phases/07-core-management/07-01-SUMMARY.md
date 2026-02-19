---
phase: 07-core-management
plan: 01
subsystem: api
tags: [sing-box, core, updater, github, tar, gzip]

# Dependency graph
requires: []
provides:
  - CoreUpdater: fetch releases, download, extract, backup, atomic replace, restart
  - ProcessManager with binary path support (SINGBOX_BINARY_PATH)
  - GET /api/core/versions, POST /api/core/update, POST /api/core/rollback
affects: [07-core-management]

# Tech tracking
tech-stack:
  added: []
  patterns: [Stop→backup→replace→start flow, atomic binary replace, GitHub Releases API]

key-files:
  created: [internal/core/updater.go]
  modified: [internal/core/process.go, internal/api/core.go, internal/api/routes.go]

key-decisions:
  - "binaryPath: SINGBOX_BINARY_PATH or dataDir/bin/sing-box"
  - "Single .backup backup per RESEARCH; no version list on disk"

patterns-established:
  - "Pattern: Stop → backup → download → extract → atomic replace → verify → start"
  - "Pattern: Rollback swaps backup with current via atomic rename"

# Metrics
duration: ~5min
completed: 2026-02-12
---

# Phase 07 Plan 01: Core Management Backend Summary

**CoreUpdater with GitHub releases fetch, binary path support, atomic update/rollback, and REST API for versions/update/rollback**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-02-12T07:39:20Z
- **Completed:** 2026-02-12T07:46:00Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- ProcessManager binary path: `SINGBOX_BINARY_PATH` env; `dataDir/bin/sing-box` fallback; `Available()`/`Version()`/`Check()`/`Restart()` use explicit path when set
- CoreUpdater: `ListReleases()` from GitHub API, `assetForPlatform()` for GOOS/GOARCH, `Update()` stop→backup→download→extract→atomic replace→verify→start, `Rollback()` swap backup with current
- API: GET `/api/core/versions`, POST `/api/core/update`, POST `/api/core/rollback` (RequireAuth)

## Task Commits

Each task was committed atomically:

1. **Task 1: ProcessManager binary path + CoreUpdater** - `5d5ccfe` (feat)
2. **Task 2: API routes for versions, update, rollback** - `7349457` (feat)

## Files Created/Modified

- `internal/core/updater.go` - CoreUpdater: ListReleases, assetForPlatform, Update, Rollback
- `internal/core/process.go` - binaryPath field, NewProcessManagerWithBinary; Available/Version/Check/Restart use binaryPath
- `internal/api/core.go` - binaryPath(), VersionsHandler, UpdateHandler, RollbackHandler
- `internal/api/routes.go` - GET /versions, POST /update, POST /rollback under /api/core

## Decisions Made

- binaryPath resolution: `SINGBOX_BINARY_PATH` env when set; else `filepath.Join(dataDir, "bin", "sing-box")`; empty when neither is used (backward compat)
- Single backup file: `binaryPath+".backup"` per RESEARCH; no on-disk version list

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

For core update/rollback to work:

- Set `SINGBOX_BINARY_PATH` to the path where the sing-box binary is installed (e.g. `/opt/sing-box/bin/sing-box`), or
- Use default `dataDir/bin/sing-box` (ensure `DATA_DIR` is set or default `./data`)

Without `SINGBOX_BINARY_PATH` and no binary at `dataDir/bin/sing-box`, Update/rollback return "请设置 SINGBOX_BINARY_PATH 以启用核心更新".

## Next Phase Readiness

- Backend ready for Phase 07 Plan 02 (frontend CoreVersionCard, UpdateConfirmDialog)
- Versions API returns GitHub releases; Update and Rollback APIs implemented end-to-end

## Self-Check: PASSED

- internal/core/updater.go: FOUND
- internal/core/process.go (modified): FOUND
- internal/api/core.go (modified): FOUND
- internal/api/routes.go (modified): FOUND
- Commits 5d5ccfe, 7349457: FOUND

---
*Phase: 07-core-management*
*Completed: 2026-02-12*
