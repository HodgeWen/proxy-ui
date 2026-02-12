---
phase: 06-traffic-statistics
verified: 2026-02-12T00:00:00Z
status: passed
score: 12/12 must-haves verified
---

# Phase 6: Traffic Statistics Verification Report

**Phase Goal:** 面板展示每个用户和每个入站的流量使用量

**Verified:** 2026-02-12

**Status:** passed

**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1   | Panel can connect to V2Ray API and fetch stats | ✓ VERIFIED | StatsClient in internal/core/stats.go: gRPC connect, QueryStats, delta aggregation |
| 2   | Panel persists traffic deltas to DB (Inbound and User) | ✓ VERIFIED | stats.go applyInboundStat/applyUserStat call db.UpdateInbound/UpdateUser |
| 3   | ConfigGenerator injects v2ray_api when stats enabled | ✓ VERIFIED | generator.go v2rayAPIEnabled, v2rayAPIBlock; stats.inbounds, stats.users from DB |
| 4   | Stats poll runs periodically (cron) | ✓ VERIFIED | cmd/server/main.go: robfig/cron, FetchAndPersist @every 60s when V2RAY_API_ENABLED=true |
| 5   | ListInbounds returns traffic_uplink, traffic_downlink per inbound | ✓ VERIFIED | inbounds.go inboundItem, inboundFromDB; db.ListInbounds(sort) |
| 6   | ListInbounds supports ?sort=traffic_asc|traffic_desc | ✓ VERIFIED | inbounds.go: sort := r.URL.Query().Get("sort"); db.ListInbounds(sort); db/inbound.go Order by traffic |
| 7   | ListUsers/GetUser returns traffic_uplink, traffic_downlink | ✓ VERIFIED | users.go userItem, userFromDB sets TrafficUplink, TrafficDownlink from u |
| 8   | reset_traffic clears TrafficUplink, TrafficDownlink, TrafficUsed | ✓ VERIFIED | users.go BatchUsersHandler case "reset_traffic": updated.TrafficUplink=0, TrafficDownlink=0, TrafficUsed=0 |
| 9   | Inbound table shows uplink and downlink traffic columns | ✓ VERIFIED | InboundTable.tsx: TableHead 上行/下行, formatBytes(ib.traffic_uplink), formatBytes(ib.traffic_downlink) |
| 10  | Inbound list supports sort by traffic (asc/desc) | ✓ VERIFIED | Inbounds.tsx: sort state, Select, fetchInbounds with ?sort=, queryKey ["inbounds", sort] |
| 11  | User table shows uplink and downlink traffic columns | ✓ VERIFIED | UserTable.tsx: TableHead 上行/下行, formatBytes(user.traffic_uplink), formatBytes(user.traffic_downlink) |
| 12  | User traffic over-limit status is visible | ✓ VERIFIED | UserTable.tsx getStatusBadge: overLimit = traffic_used >= traffic_limit → Badge 超限 |

**Score:** 12/12 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/statsproto/command.proto` | QueryStats types | ✓ VERIFIED | QueryStatsRequest, Stat, QueryStatsResponse, StatsService |
| `internal/core/stats.go` | StatsClient | ✓ VERIFIED | 130 lines; FetchAndPersist, QueryStats, delta → db.UpdateInbound/UpdateUser |
| `internal/db/inbound.go` | TrafficUplink, TrafficDownlink | ✓ VERIFIED | Columns present; ListInbounds(sort) with traffic_asc/traffic_desc |
| `internal/db/user.go` | TrafficUplink, TrafficDownlink | ✓ VERIFIED | Columns present; GetUserByName for StatsClient |
| `internal/core/generator.go` | v2ray_api injection | ✓ VERIFIED | v2rayAPIEnabled, v2rayAPIBlock; stats.inbounds, stats.users |
| `internal/api/inbounds.go` | Traffic in list, sort | ✓ VERIFIED | inboundItem TrafficUplink/Downlink; sort param from query |
| `internal/api/users.go` | traffic in response | ✓ VERIFIED | userItem TrafficUplink/Downlink; reset_traffic clears all three |
| `web/src/lib/format.ts` | formatBytes | ✓ VERIFIED | formatBytes(bytes) |
| `web/src/components/inbounds/InboundTable.tsx` | Traffic columns | ✓ VERIFIED | traffic_uplink, traffic_downlink columns, formatBytes |
| `web/src/components/users/UserTable.tsx` | Traffic columns | ✓ VERIFIED | traffic_uplink, traffic_downlink columns, formatBytes, getStatusBadge |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| internal/core/stats.go | internal/db | UpdateInbound, UpdateUser | ✓ WIRED | applyInboundStat/applyUserStat call db.UpdateInbound/UpdateUser |
| internal/core/stats.go | internal/statsproto | QueryStats | ✓ WIRED | c.client.QueryStats(ctx, &statsproto.QueryStatsRequest{Reset_: false}) |
| internal/core/generator.go | internal/db | ListInbounds, GetUsersForInbound | ✓ WIRED | v2rayAPIBlock uses db.ListInbounds(""), db.GetUsersForInbound |
| internal/api/inbounds.go | internal/db | ListInbounds(sort) | ✓ WIRED | sort := r.URL.Query().Get("sort"); db.ListInbounds(sort) |
| web/src/pages/Inbounds.tsx | /api/inbounds | ?sort= param | ✓ WIRED | fetchInbounds with ?sort=${sort} when sort !== "created" |
| web/src/components/inbounds/InboundTable.tsx | formatBytes | @/lib/format | ✓ WIRED | import formatBytes from "@/lib/format" |
| web/src/components/users/UserTable.tsx | formatBytes | @/lib/format | ✓ WIRED | import formatBytes from "@/lib/format" |
| internal/core/subscription.go | User.TrafficUplink, TrafficDownlink | BuildUserinfoHeader | ✓ WIRED | upload = u.TrafficUplink, download = u.TrafficDownlink |

### Requirements Coverage

| Requirement | Status | Blocking Issue |
| ----------- | ------ | -------------- |
| TRF-01: 系统记录并展示每个用户的上行/下行流量使用量 | ✓ SATISFIED | StatsClient records; API exposes; UserTable displays |
| TRF-02: 系统记录并展示每个入站节点的总流量使用量 | ✓ SATISFIED | StatsClient records; API exposes; InboundTable displays |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | - | - | - | - |

### Human Verification Required

| Test | Expected | Why human |
| ---- | -------- | --------- |
| Visual: Inbound table traffic columns | 上行/下行 columns display correctly | Visual appearance |
| Visual: User table traffic columns | 上行/下行 columns, 超限 badge when over limit | Visual appearance |
| Flow: Sort dropdown on Inbounds page | Select changes sort; table refetches with new order | User flow |
| Live: Traffic refresh | 60s refetch interval; traffic updates when V2Ray API enabled | Requires running sing-box with v2ray_api |
| Live: Stats with V2Ray API | V2RAY_API_ENABLED=true + sing-box with v2ray_api → traffic populates | sing-box build with `with_v2ray_api` required |

### Gaps Summary

None. All backend collection, API exposure, and frontend display are delivered. Phase goal achieved.

---

_Verified: 2026-02-12_
_Verifier: Claude (gsd-verifier)_
