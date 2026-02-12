---
phase: 06-traffic-statistics
plan: 01
subsystem: database, api, infra
tags: [grpc, v2ray-api, traffic-stats, sing-box, cron]

# Dependency graph
requires:
  - phase: 05-subscription-system
    provides: User model, GetUsersForInbound, subscription flow
  - phase: 02-inbound-management
    provides: Inbound model, ConfigGenerator, ListInbounds
provides:
  - StatsClient gRPC client for V2Ray API traffic stats
  - TrafficUplink/TrafficDownlink columns on Inbound and User
  - ConfigGenerator v2ray_api injection when V2RAY_API_ENABLED=true
  - Periodic stats polling via robfig/cron every 60s
affects: [06-traffic-statistics, api-inbounds, api-users]

# Tech tracking
tech-stack:
  added: [google.golang.org/grpc, google.golang.org/protobuf, github.com/robfig/cron/v3]
  patterns: [delta-based traffic aggregation, minimal proto copy to avoid v2ray-core]

key-files:
  created: [internal/statsproto/command.proto, internal/statsproto/command.pb.go, internal/statsproto/command_grpc.pb.go, internal/core/stats.go]
  modified: [internal/db/inbound.go, internal/db/user.go, internal/core/generator.go, cmd/server/main.go]

key-decisions:
  - "Minimal proto copy: QueryStatsRequest, Stat, QueryStatsResponse, StatsService only; avoid v2ray-core import"
  - "Delta aggregation: API returns cumulative; persist deltas to survive sing-box restarts"
  - "V2RAY_API_ENABLED gates both ConfigGenerator v2ray_api and cron; default false"

patterns-established:
  - "StatsClient: connect once, QueryStats with Reset=false, parse inbound>>>tag>>>traffic>>>uplink|downlink and user>>>name>>>traffic>>>uplink|downlink"
  - "ConfigGenerator v2ray_api: stats.inbounds from ListInbounds tags, stats.users from GetUsersForInbound (dedupe)"

# Metrics
duration: ~15min
completed: 2026-02-12
---

# Phase 6 Plan 1: Traffic Stats Backend Summary

**V2Ray API gRPC client, DB traffic columns, ConfigGenerator v2ray_api injection, and 60s cron polling**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-02-12T06:33:59Z
- **Completed:** 2026-02-12T06:50:00Z
- **Tasks:** 3
- **Files modified:** 10

## Accomplishments

- Inbound and User models: TrafficUplink, TrafficDownlink columns (default 0)
- ConfigGenerator: experimental.v2ray_api block when V2RAY_API_ENABLED=true; stats.inbounds and stats.users from DB
- Minimal stats proto (QueryStats, Stat, StatsService) and generated Go gRPC client
- StatsClient: FetchAndPersist polls API, computes deltas, persists to Inbound/User; graceful on connect failure
- Cron runs FetchAndPersist every 60s (or V2RAY_STATS_INTERVAL) when stats enabled

## Task Commits

Each task was committed atomically:

1. **Task 1: DB columns + ConfigGenerator v2ray_api** - `54fd412` (feat)
2. **Task 2: Minimal proto + StatsClient** - `c14a1c7` (feat)
3. **Task 3: Cron + main wiring** - `c44fe4b` (feat)

## Files Created/Modified

- `internal/statsproto/command.proto` - Minimal StatsService proto (QueryStats, Stat)
- `internal/statsproto/command.pb.go` - Generated protobuf messages
- `internal/statsproto/command_grpc.pb.go` - Generated gRPC client
- `internal/core/stats.go` - StatsClient with delta aggregation and DB persistence
- `internal/db/inbound.go` - TrafficUplink, TrafficDownlink
- `internal/db/user.go` - TrafficUplink, TrafficDownlink, GetUserByName
- `internal/core/generator.go` - v2ray_api injection when V2RAY_API_ENABLED
- `cmd/server/main.go` - Stats cron when stats enabled

## Decisions Made

- Minimal proto in internal/statsproto; go_package matches module path
- GetUserByName added for user stat name mapping
- V2RAY_API_LISTEN default 127.0.0.1:8080; V2RAY_STATS_INTERVAL default 60

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. To enable stats:
- Set `V2RAY_API_ENABLED=true`
- Optionally `V2RAY_API_LISTEN` (default 127.0.0.1:8080), `V2RAY_STATS_INTERVAL` (default 60)
- sing-box must be built with `with_v2ray_api`; ConfigGenerator injects v2ray_api when enabled

## Next Phase Readiness

- Traffic columns and StatsClient ready for 06-02 (API exposure) and 06-03 (frontend table)
- DB AutoMigrate will add new columns on next Init

## Self-Check: PASSED

- internal/statsproto/command.proto: FOUND
- internal/core/stats.go: FOUND
- internal/db/inbound.go TrafficUplink: FOUND
- Commits 54fd412, c14a1c7, c44fe4b: FOUND

---
*Phase: 06-traffic-statistics*
*Plan: 01*
*Completed: 2026-02-12*
