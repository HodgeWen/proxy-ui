---
phase: 09-certificate-config-sync
plan: 01
subsystem: api
tags: [certificate, config-sync, sing-box, rollback, generate-apply-restart]

# Dependency graph
requires:
  - phase: 03-certificate-management
    provides: Certificate CRUD, ConfigGenerator certificate_id resolution
provides:
  - UpdateCertificateHandler with Mutate -> Generate -> Apply -> Restart chain
  - Certificate update rollback on ApplyConfig failure; 400 JSON error response
affects: [certificate-management, sing-box-config]

# Tech tracking
tech-stack:
  added: []
  patterns: [rollback on ApplyConfig failure, best-effort Restart]

key-files:
  created: []
  modified: [internal/api/certs.go, internal/api/routes.go]

key-decisions:
  - "Certificate update follows same Generate/Apply/Restart pattern as UpdateInboundHandler"
  - "ApplyConfig failure returns 400 with JSON error; Generate failure returns 500"

patterns-established:
  - "Certificate update: db.UpdateCertificate -> Generate -> ApplyConfig -> Restart; rollback on Generate/Apply failure"

# Metrics
duration: ~5min
completed: 2026-02-12
---

# Phase 09 Plan 01: Certificate Config Sync Summary

**UpdateCertificateHandler with Generate -> ApplyConfig -> Restart chain; rollback on ApplyConfig failure; routes pass panelCfg**

## Performance

- **Duration:** ~5 min
- **Completed:** 2026-02-12T09:47:06Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- UpdateCertificateHandler now accepts `panelCfg *config.Config` and runs Generate -> ApplyConfig -> Restart after db.UpdateCertificate
- Save old certificate before update for rollback on Generate or ApplyConfig failure
- ApplyConfig failure: rollback DB, return 400 with JSON `{"error": err.Error()}`
- Restart is best-effort; failure is logged but not returned
- routes.go passes cfg to UpdateCertificateHandler

## Task Commits

Each task was committed atomically:

1. **Task 1: UpdateCertificateHandler 接入 Generate/Apply/Restart** - `2d7b66e` (feat)
2. **Task 2: routes.go 传入 panelCfg** - `9584a3d` (feat)

## Files Created/Modified

- `internal/api/certs.go` - UpdateCertificateHandler signature + Generate/Apply/Restart chain; rollback on failure; imports config, core
- `internal/api/routes.go` - UpdateCertificateHandler(sm, cfg) call

## Decisions Made

- None - followed plan as specified

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Certificate update path now triggers config regeneration and sing-box restart; inbounds referencing the cert will use new paths immediately
- 09-02 plan can proceed (certificate sync verification)

## Self-Check: PASSED

- [x] internal/api/certs.go exists
- [x] internal/api/routes.go exists
- [x] 2d7b66e commit exists
- [x] 9584a3d commit exists

---
*Phase: 09-certificate-config-sync*
*Completed: 2026-02-12*
