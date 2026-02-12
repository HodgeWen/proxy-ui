# Phase 4: User Management - Research

**Researched:** 2026-02-12
**Domain:** Proxy user CRUD, User-Inbound assignment, ConfigGenerator user filtering, bulk operations
**Confidence:** HIGH

## Summary

Phase 4 introduces a User model (proxy users, not admin) with CRUD, node assignment via UserInbound many-to-many, traffic limits and expiry, UUID/password auto-generation, search/filter, and bulk operations. The ConfigGenerator must change: instead of reading users from inbound config_json, it queries User + UserInbound, filters by enabled/traffic/expiry, and builds sing-box users arrays per inbound. VLESS users use `uuid`; Hysteria2 users use `password`. Same user on multiple VLESS inbounds shares one UUID. Traffic stats writing is Phase 6; Phase 4 adds `traffic_used` column (default 0) and filtering logic. Frontend follows InboundFormModal/CertificateFormModal patterns: Dialog + react-hook-form + zod; add checkbox for batch select, Popover+Checkbox for multi-select inbound assignment.

**Primary recommendation:** Add User and UserInbound models; ConfigGenerator derives users from DB and filters disabled/over-limit/expired; use existing patterns (Inbound API, ApplyConfig rollback) for user CRUD; add Users page with table, UserFormModal, batch action bar.

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions

#### 用户表单与节点分配
- 创建/编辑用户在同一弹框表单中完成，包含用户基本信息和节点分配
- 节点分配使用多选下拉框（适合节点数不多的场景）
- UUID（VLESS）和密码（Hysteria2）由系统自动生成，用户详情中只读展示 + 一键复制
- 同一用户分配到多个 VLESS 节点时，所有节点共享同一个 UUID

#### 流量与到期展示
- 流量用量纯文字展示（如「2.3 / 10 GB (23%)」），紧凑省空间
- 到期时间使用绝对日期展示（如「2026-03-15」）

#### 流量与到期处理
- 用户流量达到上限时，系统自动禁用该用户（从 sing-box 配置中移除）
- 用户到期时，系统自动禁用该用户（从 sing-box 配置中移除）
- ConfigGenerator 需过滤掉被禁用/超限/过期的用户

#### 批量操作
- 每行左侧 Checkbox 选中用户，勾选后顶部出现批量操作栏
- 表头 Checkbox 全选当前页
- 批量操作包括：删除、启用/禁用、流量重置（来自 USR-09）
- 危险操作（如批量删除）使用简单确认弹框（「确定删除 5 个用户？」）
- 操作完成后 Toast 提示 + 列表自动刷新

### Claude's Discretion
- 用户列表的具体列选择和信息密度
- 状态标签的颜色和样式设计
- 搜索筛选的具体范围和交互方式
- 表单字段的具体排列和分组
- 空状态的展示设计

### Deferred Ideas (OUT OF SCOPE)
- None — 讨论保持在阶段范围内

</user_constraints>

---

## Standard Stack

### Core (from Phase 1–3)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Chi | v5.x | HTTP router | Phase 1; existing |
| GORM | v1.31+ | ORM | Phase 1; User, UserInbound |
| SCS | v2 | Session | Phase 1 auth |
| shadcn/ui | latest | Components | Phase 1–3; Dialog, Table, Select |
| TanStack Query | v5 | Server state | Phase 1; list, invalidation |
| Sonner | — | Toast | Phase 1 success/error |
| react-hook-form | v7 | Form state | Phase 2; UserFormModal |
| zod | v4 | Validation | Phase 2; form validation |

### Phase 4 Additions
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/google/uuid | v1.6+ | UUID generation | VLESS user uuid; already in go.mod |
| shadcn checkbox | — | Row select, multi-select | Batch select; inbound assignment |
| shadcn popover | — | Multi-select dropdown | Inbound multi-select (节点数不多) |
| shadcn alert-dialog | — | Confirmation | Batch delete (optional; window.confirm acceptable) |
| shadcn badge | — | Status labels | 启用/禁用/过期/超限 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Popover + Checkbox list | shadcn-multi-select-component (cmdk) | Simpler for "节点数不多"; cmdk adds search |
| window.confirm | AlertDialog | Phase 2 uses window.confirm; AlertDialog more consistent |

**Installation:**
```bash
cd web && bunx shadcn@latest add checkbox popover badge
# alert-dialog optional; go.mod already has google/uuid
```

---

## Architecture Patterns

### Recommended Project Structure (extend Phase 1–3)

```
internal/
├── api/
│   ├── users.go       # NEW: list, get, create, update, delete, batch
│   └── routes.go      # add /api/users routes
├── core/
│   └── generator.go   # MODIFY: users from User+UserInbound, filter logic
├── db/
│   ├── db.go         # add User, UserInbound to AutoMigrate
│   └── user.go       # NEW: User model, UserInbound, CRUD, ListWithFilter
└── ...

web/src/
├── components/
│   ├── users/        # NEW: UserTable, UserFormModal, BatchActionBar
│   └── ui/
├── pages/
│   └── Users.tsx     # NEW: replace disabled /users placeholder
└── routes.tsx        # add /users route
```

### Pattern 1: User Model and UserInbound

**What:** User (proxy client) with many Inbounds via UserInbound junction.

**When to use:** User assignment to inbounds; same user on multiple VLESS inbounds shares UUID.

**User model:**
```go
type User struct {
    ID           uint      `gorm:"primaryKey"`
    Name         string    `gorm:"size:100;not null"`   // username for sing-box
    Remark       string    `gorm:"size:255"`            // optional note
    UUID         string    `gorm:"size:36;uniqueIndex"` // VLESS; auto-generated
    Password     string    `gorm:"size:255"`            // Hysteria2; auto-generated
    TrafficLimit int64     `gorm:"default:0"`           // bytes; 0 = unlimited
    TrafficUsed  int64     `gorm:"default:0"`           // bytes; Phase 6 writes
    ExpireAt     *time.Time                             // nil = no expiry
    Enabled      bool      `gorm:"default:true"`
    CreatedAt    time.Time `gorm:"autoCreateTime"`
    UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

type UserInbound struct {
    UserID    uint `gorm:"primaryKey"`
    InboundID uint `gorm:"primaryKey"`
}
```

**GORM many-to-many:**
```go
type User struct {
    // ...
    Inbounds []Inbound `gorm:"many2many:user_inbounds;"`
}
```

### Pattern 2: ConfigGenerator User Filtering

**What:** ConfigGenerator builds users array per inbound from User+UserInbound; includes only valid users.

**Filter logic:** User is valid if:
- `Enabled == true`
- `TrafficLimit == 0 OR TrafficUsed < TrafficLimit`
- `ExpireAt == nil OR ExpireAt.After(time.Now())`

**VLESS user format (sing-box):**
```json
{ "name": "user1", "uuid": "bf000d23-0752-40b4-affe-68f7707a9661", "flow": "xtls-rprx-vision" }
```
Source: [sing-box VLESS inbound](https://sing-box.sagernet.org/configuration/inbound/vless/)

**Hysteria2 user format (sing-box):**
```json
{ "name": "user1", "password": "goofy_ahh_password" }
```
Source: [sing-box Hysteria2 inbound](https://sing-box.sagernet.org/configuration/inbound/hysteria2/)

### Pattern 3: UUID and Password Generation

**UUID (VLESS):** `uuid.NewString()` from `github.com/google/uuid` — RFC 4122, already in go.mod.

**Password (Hysteria2):** `uuid.NewString()` sufficient; or `crypto/rand` + base64. Keep simple: `uuid.NewString()`.

### Pattern 4: Inbound Multi-Select (节点分配)

**What:** Popover with checkbox list of inbounds. CONTEXT: "节点数不多的场景".

**Implementation:** Popover + list of Checkbox; fetch inbounds via GET /api/inbounds; form field `inbound_ids: number[]`.

### Pattern 5: Batch Operations

**What:** POST /api/users/batch with `{ "action": "delete"|"enable"|"disable"|"reset_traffic", "ids": [1,2,3] }`.

**Flow:** Validate ids exist; apply action; invalidate ConfigGenerator; ApplyConfig; Restart; return 200.

### Anti-Patterns to Avoid

- **Storing users in Inbound config_json:** Phase 4 moves users to User model; ConfigGenerator builds from DB.
- **Per-inbound UUID:** Same user on multiple VLESS inbounds shares one UUID (CONTEXT).
- **Custom UUID generation:** Use `github.com/google/uuid`; do not hand-roll.
- **Omitting traffic_used:** Phase 6 writes it; Phase 4 needs column + filter logic (traffic_used defaults 0).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| UUID generation | Manual hex/random | `github.com/google/uuid` | RFC 4122; crypto/rand; edge cases |
| Many-to-many | Manual join table CRUD | GORM `many2many` | Auto migration; Preload; Association |
| Multi-select UI | Custom dropdown | Popover + Checkbox list | shadcn primitives; "节点数不多" |
| Confirmation dialog | Custom modal | window.confirm or AlertDialog | Phase 2 uses window.confirm |

**Key insight:** UUID and many-to-many have subtle edge cases; use established libraries.

---

## Common Pitfalls

### Pitfall 1: ConfigGenerator Still Reading users from config_json

**What goes wrong:** ConfigGenerator keeps using inbound ConfigJSON users; User model ignored.

**Why it happens:** Phase 2 reads users from config_json (currently empty); easy to forget to switch.

**How to avoid:** ConfigGenerator must NOT read users from config_json for inbounds; always query User+UserInbound, filter, build users array.

**Warning signs:** Inbound config_json contains users array; ConfigGenerator uses `cfg["users"]` for inbounds.

### Pitfall 2: Different UUID per VLESS Inbound

**What goes wrong:** User assigned to multiple VLESS inbounds gets different UUID per inbound.

**Why it happens:** Generating UUID per UserInbound instead of per User.

**How to avoid:** UUID lives on User model; one UUID per user; all VLESS inbounds for that user use same UUID.

### Pitfall 3: User Valid Logic Inconsistency

**What goes wrong:** ConfigGenerator includes disabled/expired/over-limit users; or excludes valid users.

**Why it happens:** Off-by-one (traffic_used >= vs >); timezone for ExpireAt; Enabled default.

**How to avoid:** Central filter: `enabled && (traffic_limit == 0 || traffic_used < traffic_limit) && (expire_at == nil || expire_at.After(now))`. Use UTC for expiry comparison.

### Pitfall 4: Batch Delete Without ApplyConfig

**What goes wrong:** Batch delete succeeds in DB but sing-box config not updated; users still in config.

**Why it happens:** Forgetting to call ConfigGenerator.Generate + ApplyConfig after batch mutate.

**How to avoid:** All user mutations (create, update, delete, batch) must trigger ConfigGenerator + ApplyConfig + Restart; rollback DB on ApplyConfig failure (per Phase 2).

### Pitfall 5: Search Filter Resets on Action

**What goes wrong:** User applies filter (e.g. "过期用户"); performs batch action; list refreshes and filter resets.

**How to avoid:** Preserve search/filter query params in TanStack Query key; invalidate same key after mutation. PITFALLS.md documents 3x-ui filter reset issue.

---

## Code Examples

### UUID Generation (Go)
```go
import "github.com/google/uuid"

func generateUUID() string {
    return uuid.NewString() // e.g. "bf000d23-0752-40b4-affe-68f7707a9661"
}
```
Source: [pkg.go.dev/github.com/google/uuid](https://pkg.go.dev/github.com/google/uuid)

### GORM Many-to-Many Preload
```go
var users []User
db.Preload("Inbounds").Find(&users)
```

### sing-box VLESS Inbound Users
```json
"users": [
  { "name": "user1", "uuid": "bf000d23-0752-40b4-affe-68f7707a9661", "flow": "xtls-rprx-vision" }
]
```

### sing-box Hysteria2 Inbound Users
```json
"users": [
  { "name": "user1", "password": "a1b2c3d4-e5f6-7890-abcd-ef1234567890" }
]
```

### Traffic Display Format
```
"2.3 / 10 GB (23%)"  // traffic_used / traffic_limit; percentage when limit > 0
```

### Expiry Display Format
```
"2026-03-15"  // YYYY-MM-DD from expire_at
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Users in config_json | User model + UserInbound | Phase 4 | ConfigGenerator reads from DB |
| No user filtering | Filter by enabled/traffic/expiry | Phase 4 | Auto-disable on limit/expiry |

**Deprecated/outdated:**
- Storing users array in Inbound ConfigJSON: Phase 4 replaces with User model.

---

## Open Questions

1. **Traffic stats source (Phase 6)**
   - What we know: Phase 6 adds StatsClient (V2Ray API); writes traffic_used.
   - What's unclear: Exact API mapping; tag → user name mapping.
   - Recommendation: Phase 4 add traffic_used column (default 0); ConfigGenerator filter uses it. Phase 6 implements write path.

2. **Copy to clipboard (UUID/password)**
   - What we know: CONTEXT requires "一键复制".
   - What's unclear: Browser clipboard API; fallback for HTTP.
   - Recommendation: navigator.clipboard.writeText(); fallback: select + document.execCommand('copy'). Requires HTTPS or localhost.

---

## Sources

### Primary (HIGH confidence)
- [sing-box VLESS inbound](https://sing-box.sagernet.org/configuration/inbound/vless/) — users, uuid, flow
- [sing-box Hysteria2 inbound](https://sing-box.sagernet.org/configuration/inbound/hysteria2/) — users, password
- [pkg.go.dev/github.com/google/uuid](https://pkg.go.dev/github.com/google/uuid) — NewString, New
- [GORM Many-to-Many](https://gorm.io/docs/many_to_many.html) — many2many, Preload
- Project: internal/core/generator.go, internal/db/inbound.go, internal/api/inbounds.go, web InboundFormModal, CertificateFormModal

### Secondary (MEDIUM confidence)
- [shadcn Checkbox](https://ui.shadcn.com/docs/components/checkbox) — Table checkbox example
- [shadcn Popover](https://ui.shadcn.com/docs/components/popover) — Multi-select pattern
- PITFALLS.md — filter reset, user assignment

### Tertiary (LOW confidence)
- WebSearch: shadcn multi-select — community components; Popover+Checkbox sufficient for "节点数不多"

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — existing stack + uuid, checkbox, popover verified
- Architecture: HIGH — Phase 2/3 patterns; GORM many-to-many; sing-box docs
- Pitfalls: HIGH — ConfigGenerator change, UUID sharing, filter reset from PITFALLS

**Research date:** 2026-02-12
**Valid until:** ~30 days (stable domain)
