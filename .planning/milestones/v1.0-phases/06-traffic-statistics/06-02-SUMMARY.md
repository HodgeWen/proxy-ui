---
phase: 06-traffic-statistics
plan: 02
subsystem: api
tags: [inbounds, users, traffic, sort, reset_traffic, subscription-userinfo]

# Dependency graph
requires:
  - phase: 06-traffic-statistics
    provides: 06-01 TrafficUplink/TrafficDownlink on Inbound and User
provides:
  - ListInbounds returns traffic_uplink, traffic_downlink; ?sort=traffic_asc|traffic_desc
  - ListUsers/GetUser returns traffic_uplink, traffic_downlink
  - reset_traffic clears TrafficUplink, TrafficDownlink, TrafficUsed
  - BuildUserinfoHeader uses real UL/DL instead of TrafficUsed/2 placeholder
affects: [06-traffic-statistics, frontend]

# Tech tracking
tech-stack:
  added: []
  patterns: [query param sort for list endpoints]

key-files:
  created: []
  modified: [internal/api/inbounds.go, internal/api/users.go, internal/db/inbound.go, internal/core/subscription.go, internal/core/generator.go, internal/db/certificate.go]

key-decisions:
  - "ListInbounds(sort string): traffic_asc/traffic_desc order by (traffic_uplink+traffic_downlink); default created_at DESC"
  - "reset_traffic clears all three: TrafficUplink, TrafficDownlink, TrafficUsed"

patterns-established:
  - "subscription-userinfo upload/download = TrafficUplink, TrafficDownlink (real values from Phase 6 stats)"

# Metrics
duration: 2min
completed: 2026-02-12
---

# Phase 6 Plan 2: API Traffic Exposure Summary

**ListInbounds with traffic columns + sort param; ListUsers/GetUser with traffic_uplink, traffic_downlink; reset_traffic clears all traffic; subscription uses real UL/DL**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-02-12T06:41:50Z
- **Completed:** 2026-02-12T06:43:32Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- inboundItem: TrafficUplink, TrafficDownlink in API response; inboundFromDB maps from db.Inbound
- ListInboundsHandler: parses ?sort= (traffic_asc, traffic_desc); default created_at desc
- db.ListInbounds(sort string): optional Order by (traffic_uplink+traffic_downlink) ASC/DESC
- userItem: TrafficUplink, TrafficDownlink; userFromDB maps from db.User
- reset_traffic: sets TrafficUplink=0, TrafficDownlink=0, TrafficUsed=0
- BuildUserinfoHeader: upload=TrafficUplink, download=TrafficDownlink (replaces TrafficUsed/2)

## Task Commits

Each task was committed atomically:

1. **Task 1: Inbound API traffic + sort** - `8c930e4` (feat)
2. **Task 2: User API traffic + reset_traffic** - `99b221f` (feat)

## Files Created/Modified

- `internal/api/inbounds.go` - inboundItem traffic fields, sort param, inboundFromDB
- `internal/api/users.go` - userItem traffic fields, userFromDB, reset_traffic clears all three
- `internal/db/inbound.go` - ListInbounds(sort string) with traffic ordering
- `internal/core/subscription.go` - BuildUserinfoHeader uses TrafficUplink, TrafficDownlink
- `internal/core/generator.go` - ListInbounds("") call site
- `internal/db/certificate.go` - ListInbounds("") call site

## Decisions Made

None - followed plan as specified.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- API returns traffic in list/get responses; frontend (06-03) can display traffic columns and sort controls
- reset_traffic clears all traffic fields as required
- subscription-userinfo now reports real upload/download from stats

## Self-Check: PASSED

- internal/api/inbounds.go: FOUND
- internal/api/users.go: FOUND
- internal/db/inbound.go: FOUND
- internal/core/subscription.go: FOUND
- Commits 8c930e4, 99b221f: FOUND

---
*Phase: 06-traffic-statistics*
*Plan: 02*
*Completed: 2026-02-12*
