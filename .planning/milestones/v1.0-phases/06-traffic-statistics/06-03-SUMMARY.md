---
phase: 06-traffic-statistics
plan: 03
subsystem: ui
tags: [inbounds, users, traffic, formatBytes, sort, Badge]

# Dependency graph
requires:
  - phase: 06-traffic-statistics
    provides: 06-02 API traffic_uplink, traffic_downlink; ListInbounds sort param
provides:
  - formatBytes shared util in web/src/lib/format.ts
  - InboundTable: 上行/下行 columns, sort dropdown (默认/流量升序/流量降序)
  - UserTable: 上行/下行 columns; getStatusBadge shows 超限
  - 60s refetchInterval for traffic freshness
affects: [06-traffic-statistics]

# Tech tracking
tech-stack:
  added: []
  patterns: [shared format util, sort dropdown with queryKey]

key-files:
  created: [web/src/lib/format.ts]
  modified: [web/src/components/inbounds/InboundTable.tsx, web/src/pages/Inbounds.tsx, web/src/components/users/UserTable.tsx, web/src/components/users/UserSubscriptionCard.tsx, web/src/pages/Users.tsx]

key-decisions:
  - "Replace 流量 column with 上行/下行 (per CONTEXT); 超限 in status badge"
  - "formatBytes extracted to shared lib; used by InboundTable, UserTable, UserSubscriptionCard"

patterns-established:
  - "Traffic display: same 上行/下行 style across Inbound and User tables"

# Metrics
duration: 2min
completed: 2026-02-12
---

# Phase 6 Plan 3: Traffic Columns UI Summary

**formatBytes shared util; InboundTable and UserTable with 上行/下行 columns; inbound sort by traffic; over-limit badge**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-02-12T06:46:00Z
- **Completed:** 2026-02-12T06:48:09Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- formatBytes shared in web/src/lib/format.ts (0 B, KB, MB, GB, TB)
- InboundTable: traffic_uplink, traffic_downlink columns; formatBytes from lib
- Inbounds page: sort dropdown (默认/流量升序/流量降序); queryKey [inbounds, sort]; refetchInterval 60000
- UserTable: replaced 流量 with 上行/下行 columns; formatBytes from lib; getStatusBadge shows 超限
- UserSubscriptionCard: formatBytes from lib (removed local)
- Users page: refetchInterval 60000

## Task Commits

Each task was committed atomically:

1. **Task 1: formatBytes util + InboundTable traffic + sort** - `ab1b689` (feat)
2. **Task 2: UserTable traffic columns + UserSubscriptionCard** - `477a003` (feat)

## Files Created/Modified

- `web/src/lib/format.ts` - formatBytes shared util
- `web/src/components/inbounds/InboundTable.tsx` - traffic columns, formatBytes import
- `web/src/pages/Inbounds.tsx` - sort state, Select dropdown, refetchInterval
- `web/src/components/users/UserTable.tsx` - 上行/下行 columns, formatBytes from lib; removed formatTraffic
- `web/src/components/users/UserSubscriptionCard.tsx` - formatBytes from lib
- `web/src/pages/Users.tsx` - refetchInterval 60000

## Decisions Made

- Replaced User 流量 column with 上行/下行 columns (per CONTEXT: 与入站保持一致的展示风格)
- Over-limit status: getStatusBadge already shows 超限 when traffic_used >= traffic_limit
- No separate "已用/上限" column; badge sufficient per CONTEXT

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 6 traffic statistics complete: DB columns, API exposure, and UI display all in place
- Inbounds and Users pages show traffic and refresh every 60s

## Self-Check: PASSED

- web/src/lib/format.ts: FOUND
- web/src/components/inbounds/InboundTable.tsx: FOUND
- web/src/components/users/UserTable.tsx: FOUND
- web/src/components/users/UserSubscriptionCard.tsx: FOUND
- Commits ab1b689, 477a003: FOUND

---
*Phase: 06-traffic-statistics*
*Plan: 03*
*Completed: 2026-02-12*
