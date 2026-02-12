---
phase: 09-certificate-config-sync
plan: 02
subsystem: ui
tags: [certificate, checkError, ApplyConfig, sing-box, CertificateFormModal]

# Dependency graph
requires:
  - phase: 09-01
    provides: 400 JSON error response on ApplyConfig failure
provides:
  - CertificateFormModal checkError display for ApplyConfig failure (match InboundFormModal)
affects: [certificate-management]

# Tech tracking
tech-stack:
  added: []
  patterns: [checkError UI: 400 + data.error display, no toast]

key-files:
  created: []
  modified: [web/src/components/certificates/CertificateFormModal.tsx]

key-decisions:
  - "checkError block placed between DialogHeader and form, same as InboundFormModal"

patterns-established:
  - "ApplyConfig 400 failure: setCheckError(data.error), no toast; show inline in Modal"

# Metrics
duration: ~5min
completed: 2026-02-12
---

# Phase 09 Plan 02: Certificate Config Sync Summary

**CertificateFormModal checkError display for ApplyConfig failure, matching InboundFormModal behavior**

## Performance

- **Duration:** ~5 min
- **Completed:** 2026-02-12
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- CertificateFormModal shows sing-box check error output when ApplyConfig fails (400)
- checkError state and reset on open/certificate change
- On 400 + data.error: setCheckError, no toast; other errors use toast.error
- Inline error block with destructive styling (rounded-lg bg-destructive/10 text-destructive)

## Task Commits

Each task was committed atomically:

1. **Task 1: CertificateFormModal 增加 checkError 显示** - `2b97288` (feat)

## Files Created/Modified

- `web/src/components/certificates/CertificateFormModal.tsx` - Add checkError state, 400 error handling, error display block

## Decisions Made

None - followed plan as specified

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Certificate update ApplyConfig failure now shows detailed error in Modal
- 09-03 plan can proceed

## Self-Check: PASSED

- [x] web/src/components/certificates/CertificateFormModal.tsx exists
- [x] 2b97288 commit exists

---
*Phase: 09-certificate-config-sync*
*Completed: 2026-02-12*
