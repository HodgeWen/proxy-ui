---
phase: 02-inbound-management
plan: 02
subsystem: core
tags: [sing-box, config, generator, vless, hysteria2, json]

# Dependency graph
requires:
  - phase: 02-inbound-management
    plan: 01
    provides: Inbound model, ListInbounds, ConfigJSON
provides:
  - ConfigGenerator.Generate() producing full sing-box JSON from DB inbounds
  - VLESS inbound mapping (type, tag, listen, listen_port, users, tls, transport)
  - Hysteria2 inbound mapping (type, tag, listen, listen_port, users, tls, up_mbps, down_mbps, obfs)
affects: [02-03, 02-04, 02-05]

# Tech tracking
tech-stack:
  added: []
  patterns: [DB-first config generation, inboundToSingBox protocol dispatch]

key-files:
  created: [internal/core/generator.go]
  modified: []

key-decisions:
  - "ConfigGenerator reads from DB via ListInbounds; no raw config edit"
  - "config_json stores protocol-specific fields (tls, transport, users, up_mbps, down_mbps, obfs)"

patterns-established:
  - "inboundToSingBox dispatches by protocol to vlessToSingBox / hysteria2ToSingBox"

# Metrics
duration: 1min
completed: 2026-02-11
---

# Phase 2 Plan 2: ConfigGenerator Summary

**ConfigGenerator reads inbounds from DB and produces full sing-box JSON with VLESS and Hysteria2 inbound mapping**

## Performance

- **Duration:** ~1 min
- **Started:** 2026-02-11T10:02:46Z
- **Completed:** 2026-02-11T10:03:49Z
- **Tasks:** 2 (combined into single implementation)
- **Files modified:** 1 created

## Accomplishments

- ConfigGenerator struct with Generate() returning full sing-box config byte slice
- db.ListInbounds() integration; inboundToSingBox protocol dispatch
- VLESS: type, tag, listen, listen_port, users, tls, transport from config_json or defaults
- Hysteria2: type, tag, listen, listen_port, users, tls (required), up_mbps, down_mbps, obfs from config_json
- Full config structure: log, inbounds, outbounds, route per sing-box schema

## Task Commits

Each task was committed atomically:

1. **Task 1: ConfigGenerator skeleton and VLESS mapping** - `cbbebc1` (feat)
2. **Task 2: Hysteria2 inbound mapping** - `cbbebc1` (same commit; combined implementation)

**Plan metadata:** `075232b` (docs: complete plan)

_Note: Both tasks implemented in single commit as Hysteria2 was required for build to succeed_

## Files Created/Modified

- `internal/core/generator.go` - ConfigGenerator with Generate(), vlessToSingBox(), hysteria2ToSingBox(), toFloat()

## Decisions Made

- Use github.com/s-ui/s-ui import path (fixed from s-ui/internal/db per Rule 3)
- Hysteria2 default tls with enabled:true and empty cert paths; config_json overrides when present
- Omit up_mbps/down_mbps when zero or not set per RESEARCH

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed import path**
- **Found during:** Task 1 (go build)
- **Issue:** Used `s-ui/internal/db` but module is `github.com/s-ui/s-ui`
- **Fix:** Changed to `github.com/s-ui/s-ui/internal/db`
- **Files modified:** internal/core/generator.go
- **Verification:** go build ./internal/core/... passes
- **Committed in:** cbbebc1

**2. [Rule 3 - Blocking] Added hysteria2ToSingBox stub**
- **Found during:** Task 1 (go build)
- **Issue:** inboundToSingBox called hysteria2ToSingBox but it didn't exist
- **Fix:** Implemented full hysteria2ToSingBox in same commit (Task 2 scope)
- **Files modified:** internal/core/generator.go
- **Verification:** go build passes
- **Committed in:** cbbebc1

---

**Total deviations:** 2 auto-fixed (both blocking)
**Impact on plan:** Import fix required for correctness. Combined Task 1+2 for efficiency since hysteria2 was needed for build.

## Issues Encountered

None - plan executed with minor fixes.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- ConfigGenerator ready for ApplyConfig integration (02-03)
- Generate() output can be piped to `sing-box check -c /dev/stdin` when sing-box is installed
- Inbound API can call ConfigGenerator before ApplyConfig on create/update/delete

## Self-Check: PASSED

- [x] internal/core/generator.go exists
- [x] Commit cbbebc1 exists
- [x] go build ./... passes

---
*Phase: 02-inbound-management*
*Completed: 2026-02-11*
