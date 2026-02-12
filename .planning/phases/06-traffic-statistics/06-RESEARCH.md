# Phase 6: Traffic Statistics - Research

**Researched:** 2026-02-12
**Domain:** sing-box V2Ray API traffic stats, Go gRPC client, frontend table integration
**Confidence:** HIGH

## Summary

Phase 6 adds per-user and per-inbound traffic display (uplink/downlink) to the existing Inbounds and Users tables. Traffic data comes from sing-box's optional V2Ray API (gRPC), which requires a custom sing-box build with `with_v2ray_api`. The panel must poll the API periodically, apply deltas to persisted totals, and expose stats via existing list APIs. The frontend adds traffic columns to both tables and supports sort-by-traffic for inbounds.

**Key constraint:** V2Ray API is **not included in default sing-box builds**. Deployment must use a custom-built sing-box or document the requirement. When unavailable, stats display as 0; panel remains functional.

**Primary recommendation:** Implement StatsClient (gRPC) + ConfigGenerator v2ray_api injection + Inbound/User traffic columns + frontend table updates. Use delta-based aggregation (API returns cumulative since process start; persist deltas to survive restarts).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### 入站流量展示
- 流量数据直接内嵌在现有 Inbounds 列表表格中，增加上行/下行流量列
- 每个入站仅展示上行总量 + 下行总量，纯数字展示，无图表
- 入站列表支持按流量升降序排序

#### 用户流量展示
- 用户列表表格中增加流量列（上行/下行），与入站保持一致的展示风格
- 流量超限与状态提示由 Claude 决定合理方案

### Claude's Discretion
- 数据更新体验：自动轮询 vs 手动刷新、刷新频率
- 后端采集频率与实时性策略（根据 sing-box API 能力决定）
- 用户流量重置与入站统计的联动关系
- 流量数字的格式化（B/KB/MB/GB 自动换算）
- 用户接近或超过流量限制时的界面提示方式

### Deferred Ideas (OUT OF SCOPE)
- None — discussion stayed within phase scope
</user_constraints>

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| google.golang.org/grpc | v1.x | gRPC client for V2Ray API | sing-box StatsService is gRPC; standard Go client |
| protobuf (generated) | — | StatsService proto | v2fly/v2ray-core and sing-box share compatible proto; copy minimal proto to avoid heavy v2ray-core dep |
| robfig/cron/v3 | v3 | Periodic stats fetch | STACK.md; used for scheduled tasks |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| GORM | existing | Traffic columns on Inbound, User | Auto-migrate new columns |
| formatBytes | existing | B/KB/MB/GB | UserTable already has it; extract to shared util |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| V2Ray API gRPC | Log parsing | Log parsing is complex, fragile; prefer API when available |
| v2fly/v2ray-core dep | Copy minimal proto | v2ray-core is heavy; minimal proto avoids ~20 transitive deps |
| Separate /api/stats | Embed in list APIs | CONTEXT: embed in tables; less API surface |

**Installation:**
```bash
go get google.golang.org/grpc
go get google.golang.org/protobuf
# Proto: copy minimal stats service from v2fly/v2ray-core or sing-box, generate with protoc-gen-go-grpc
```

---

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── core/
│   ├── generator.go       # Add v2ray_api to config when stats enabled
│   ├── stats.go           # StatsClient: gRPC connect, QueryStats, delta aggregation
│   └── process.go         # unchanged
├── db/
│   ├── inbound.go         # Add TrafficUplink, TrafficDownlink (or separate table)
│   ├── user.go            # Add TrafficUplink, TrafficDownlink; TrafficUsed = UL+DL for limit
│   └── db.go              # AutoMigrate new models/columns
├── api/
│   ├── inbounds.go        # Include traffic in list response; support ?sort=traffic_asc|traffic_desc
│   └── users.go           # Include traffic_uplink, traffic_downlink
web/src/
├── components/
│   ├── inbounds/InboundTable.tsx   # Add uplink/downlink columns; sortable
│   └── users/UserTable.tsx        # Split traffic column to uplink/downlink; status badge
└── lib/
    └── format.ts          # formatBytes shared util
```

### Pattern 1: StatsClient Delta Aggregation

**What:** Poll V2Ray API; API returns cumulative bytes since process start. Compute delta from last poll; add delta to DB totals. On sing-box restart, API counters reset to 0.

**When to use:** Always; V2Ray API has no persistent storage.

**Example:**
```go
// Pseudocode: StatsClient.FetchAndPersist()
func (c *StatsClient) FetchAndPersist() error {
    resp, err := c.client.QueryStats(ctx, &QueryStatsRequest{}) // empty = all stats
    // resp.Stat = []*Stat{ {Name: "user>>>user1>>>traffic>>>uplink", Value: 12345}, ... }
    for _, s := range resp.Stat {
        delta := s.Value - c.lastSeen[s.Name] // c.lastSeen in-memory
        if delta < 0 { delta = s.Value }      // restart: counter reset
        c.lastSeen[s.Name] = s.Value
        // Map name -> User or Inbound, add delta to DB
    }
}
```

### Pattern 2: ConfigGenerator v2ray_api Injection

**What:** Add `experimental.v2ray_api` to sing-box config when stats enabled. Include all inbound tags and all user names (from users assigned to inbounds) in stats config.

**When to use:** Each config generation; stats only work if configured.

**Example:**
```json
"experimental": {
  "v2ray_api": {
    "listen": "127.0.0.1:8080",
    "stats": {
      "enabled": true,
      "inbounds": ["vless-in-1", "h2-in-1"],
      "outbounds": ["direct"],
      "users": ["user1", "user2"]
    }
  }
}
```

**Critical:** stats.users must use User.Name (exactly as in sing-box users array); stats.inbounds must use Inbound.Tag. ConfigGenerator already uses db.Inbound.Tag and User.Name.

### Pattern 3: Stat Name Mapping

**What:** sing-box uses fixed name format (source: sing-box experimental/v2rayapi/stats.go).

**Inbound:** `inbound>>>{tag}>>>traffic>>>uplink` / `downlink`  
**User:** `user>>>{name}>>>traffic>>>uplink` / `downlink`

Map: inbound tag → db.Inbound (GetInboundByTag); user name → db.User (by Name, unique per user).

### Anti-Patterns to Avoid

- **Reset=true on QueryStats:** Don't reset counters; that clears sing-box's in-memory stats. Use reset=false, compute delta client-side.
- **Storing API raw values:** API values are cumulative since start; store deltas in DB to survive restarts.
- **Heavy v2ray-core import:** Use minimal proto copy; v2ray-core pulls many transitive deps.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| gRPC client | Manual HTTP/JSON | google.golang.org/grpc + generated client | StatsService is gRPC; standard client is correct |
| Proto generation | Hand-written structs | protoc + protoc-gen-go-grpc | Message format must match sing-box exactly |
| Traffic formatting | Custom logic | formatBytes (B/KB/MB/GB) | UserTable already has this; reuse |
| Cron/scheduler | time.Sleep loop | robfig/cron/v3 | STACK.md; idiomatic; supports configurable interval |

**Key insight:** sing-box's StatsService uses the same proto as v2ray-core; service name is `v2ray.core.app.stats.command.StatsService`. Copy the minimal proto (GetStats, QueryStats, Stat types) and generate client; avoid importing full v2ray-core.

---

## Common Pitfalls

### Pitfall 1: V2Ray API Unavailable

**What goes wrong:** sing-box default build has no V2Ray API; gRPC connect fails; stats stay 0.

**Why it happens:** `with_v2ray_api` is not a default build tag.

**How to avoid:** Document custom build requirement; gracefully handle connect failure (log, don't crash); when API unavailable, list APIs still return 0 for traffic.

**Warning signs:** Connection refused or "connection reset" on stats port.

### Pitfall 2: Stats Double-Count or Restart Loss

**What goes wrong:** Traffic wrong after sing-box restart; negative deltas; double-counting.

**Why it happens:** API counters reset on restart; if delta = current - lastSeen and current=0, delta negative. Or lastSeen not persisted across panel restart.

**How to avoid:** If delta < 0, treat as restart: use current as delta (or skip). Persist lastSeen in DB or accept small loss on panel restart. Never use reset=true.

**Warning signs:** User.TrafficUsed decreases; inbound traffic goes negative.

### Pitfall 3: User/Inbound Name Mismatch

**What goes wrong:** Stats not attributed to correct user/inbound.

**Why it happens:** stats.users must match User.Name exactly; stats.inbounds must match Inbound.Tag. Case-sensitive.

**How to avoid:** ConfigGenerator gets user names from GetUsersForInbound (User.Name); inbound tags from db.Inbound.Tag. No transformation.

### Pitfall 4: User Traffic Reset vs Inbound Stats

**What goes wrong:** Admin resets user traffic; unclear if inbound totals should change.

**Why it happens:** User traffic = sum across assigned inbounds; reset_traffic clears User.TrafficUsed (and UL/DL). Inbound totals are independent (all users on that inbound).

**How to avoid:** reset_traffic affects only User; inbound stats are never reset by user action. Clear separation.

---

## Code Examples

### V2Ray API QueryStats (empty patterns = all)

```go
// Source: sing-box experimental/v2rayapi/stats.go, v2fly/v2ray-core
resp, err := statsClient.QueryStats(ctx, &command.QueryStatsRequest{
    Reset: false,  // never reset
})
// resp.Stat = []*Stat{ {Name: "user>>>user1>>>traffic>>>uplink", Value: 12345}, ... }
for _, s := range resp.Stat {
    if strings.HasPrefix(s.Name, "user>>>") {
        // Parse: user>>>{name}>>>traffic>>>uplink|downlink
    }
    if strings.HasPrefix(s.Name, "inbound>>>") {
        // Parse: inbound>>>{tag}>>>traffic>>>uplink|downlink
    }
}
```

### formatBytes (existing in UserTable)

```typescript
// Source: web/src/components/users/UserTable.tsx
function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}
```

### Table Sort (Inbound by traffic)

```typescript
// Inbounds page: ?sort=traffic_asc | traffic_desc
// Backend: ORDER BY (traffic_uplink + traffic_downlink) ASC|DESC
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Log parsing for stats | V2Ray API gRPC | sing-box added v2rayapi | Per-user stats possible; requires custom build |
| Inbound-only stats | User + Inbound stats | Issue #185 (SagerNet/sing-box) | User stats supported |

**Deprecated/outdated:**
- Relying on default sing-box binary for stats: V2Ray API not in default build.

---

## Open Questions

1. **gRPC dependency size**
   - What we know: v2fly/v2ray-core is heavy; sing-box has minimal proto.
   - What's unclear: Whether a standalone proto + generated client adds acceptable deps.
   - Recommendation: Copy minimal proto (GetStats, QueryStats, Stat, StatsService) into `internal/statsproto/`; generate with `protoc`; add only `google.golang.org/grpc` and `google.golang.org/protobuf`. Avoid v2ray-core.

2. **User.TrafficUsed vs TrafficUplink/Downlink**
   - What we know: User has TrafficUsed (limit check); subscription needs upload/download.
   - What's unclear: Store TrafficUplink + TrafficDownlink and derive TrafficUsed = UL+DL, or keep TrafficUsed and add UL/DL?
   - Recommendation: Add TrafficUplink, TrafficDownlink. TrafficUsed = TrafficUplink + TrafficDownlink (computed or stored). Reset clears all three. Subscription uses UL/DL directly.

3. **Inbound traffic storage**
   - What we know: Inbound model has no traffic fields.
   - What's unclear: Add columns to Inbound vs separate table.
   - Recommendation: Add TrafficUplink, TrafficDownlink to Inbound. Simpler; no join.

---

## Sources

### Primary (HIGH confidence)

- [sing-box V2Ray API config](https://sing-box.sagernet.org/configuration/experimental/v2ray-api/) — structure, stats.enabled, stats.inbounds, stats.users
- [sing-box experimental/v2rayapi/stats.go](https://github.com/SagerNet/sing-box/blob/master/experimental/v2rayapi/stats.go) — stat name format: `inbound>>>{tag}>>>traffic>>>uplink`, `user>>>{name}>>>traffic>>>uplink`
- [v2fly/v2ray-core command.proto](https://github.com/v2fly/v2ray-core/blob/master/app/stats/command/command.proto) — GetStats, QueryStats, Stat types
- [ARCHITECTURE.md](.planning/research/ARCHITECTURE.md) — Traffic flow, StatsClient, ConfigGenerator v2ray_api

### Secondary (MEDIUM confidence)

- PITFALLS.md 5.1, 5.2 — Core API mismatch, stats accuracy
- Phase 5 subscription — upload/download placeholder; Phase 6 supplies real values

### Tertiary (LOW confidence)

- WebSearch: 3x-ui uses Xray (not sing-box); no direct sing-box panel reference

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — proto + grpc from official sources; cron from STACK.md
- Architecture: HIGH — ARCHITECTURE.md + sing-box source code
- Pitfalls: HIGH — PITFALLS.md + stats.go logic

**Research date:** 2026-02-12
**Valid until:** 30 days (sing-box API stable)
