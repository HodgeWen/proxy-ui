# Phase 2: Inbound Management - Research

**Researched:** 2026-02-11
**Domain:** sing-box VLESS/Hysteria2 inbound configuration, ConfigGenerator, CRUD API
**Confidence:** HIGH

## Summary

Phase 2 adds full inbound management for VLESS and Hysteria2. The panel persists inbounds to SQLite (GORM), generates sing-box JSON via ConfigGenerator, and applies via the existing `ApplyConfig` + `Restart` flow. VLESS supports TLS (certificate path), Reality (VLESS-only), and V2Ray transports (TCP, WebSocket, gRPC, HTTP). Hysteria2 uses QUIC, requires TLS, and has distinct fields (up/down_mbps, obfs, masquerade). Reality is VLESS-only; Hysteria2 TLS options are limited to standard TLS (certificate_path + server_name). The existing Dialog, Tooltip, and API patterns from Phase 1 extend directly; add shadcn select, label, dropdown-menu, and table for the inbound form and list.

**Primary recommendation:** Use DB-first config flow: Inbound CRUD → ConfigGenerator builds full config from DB → ApplyConfig → Restart. Add Inbound model and ConfigGenerator; keep `ApplyConfig` and `ProcessManager` unchanged. Extend Dialog for wide Modal form; add select, label, dropdown-menu, table, radio-group for form and list UI.

---

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions
- 统一表单 — 先选协议类型（VLESS / Hysteria2），表单动态切换对应字段
- 分段显示 — 表单按「基本设置 / TLS / 传输」分块，每块有标题
- 所有字段始终可见，不折叠高级选项
- 使用宽 Modal 弹框，给表单充足空间，减少滚动
- 表格布局，每行一个入站；丰富信息列
- 操作：常用操作（编辑）内联，更多操作（删除等）放下拉菜单
- TLS 下拉：无 TLS / TLS / Reality（VLESS）；Hysteria2 仅 TLS（无 Reality）
- 传输协议单选：TCP / WebSocket / gRPC / HTTP/2
- 智能默认值 + info 图标悬浮提示
- 表单验证失焦触发；sing-box check 失败时 Modal 内顶部显示错误，不关闭弹框

### Claude's Discretion
- 表格列的具体宽度与排列顺序
- 表单字段的具体排列顺序
- info tooltip 的具体文案措辞
- 空状态（无入站时）的展示方式
- 删除确认的交互方式

### Deferred Ideas (OUT OF SCOPE)
- 证书管理（Phase 3）
- 用户分配到入站（Phase 4）
- 流量统计（Phase 6）

</user_constraints>

---

## Standard Stack

### Core (from Phase 1)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Chi | v5.x | HTTP router | Phase 1; sing-box ecosystem |
| GORM | v1.31+ | ORM | Phase 1; Inbound model |
| SCS | v2 | Session | Phase 1 auth |
| shadcn/ui | latest | Components | Phase 1; Dialog, Tooltip present |
| TanStack Query | v5 | Server state | Phase 1; list, invalidation |
| Sonner | — | Toast | Phase 1 success/error |

### Phase 2 Additions
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| react-hook-form | v7 | Form state | Inbound add/edit form |
| zod | v4 | Validation | Port, UUID, flow, TLS fields |
| @hookform/resolvers | latest | Zod integration | Form validation |
| shadcn select | — | TLS/transport dropdown | TLS type, transport type |
| shadcn label | — | Form labels | Field labeling |
| shadcn dropdown-menu | — | Row actions | Edit/Delete menu |
| shadcn table | — | Inbound list | Table layout per CONTEXT |
| shadcn radio-group | — | Transport radio | TCP/WS/gRPC/HTTP2 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| react-hook-form | useState | RHF handles nested/conditional fields, validation on blur |
| Zod | Yup | Zod v4 in stack; shadcn form examples use Zod |

**Installation:**
```bash
cd web && bun add react-hook-form @hookform/resolvers zod
bunx shadcn@latest add select label dropdown-menu table radio-group
```

---

## Architecture Patterns

### Recommended Project Structure (extend Phase 1)

```
internal/
├── api/
│   ├── auth.go
│   ├── core.go
│   ├── inbounds.go    # NEW: list, create, update, delete
│   └── routes.go      # add /api/inbounds routes
├── core/
│   ├── config.go      # existing ApplyConfig
│   ├── generator.go   # NEW: ConfigGenerator from DB
│   └── process.go
├── db/
│   ├── admin.go
│   ├── db.go          # add Inbound to AutoMigrate
│   └── inbound.go     # NEW: Inbound model, CRUD
└── session/

web/src/
├── components/
│   ├── layout/
│   ├── inbounds/      # NEW: InboundTable, InboundForm, InboundFormModal
│   └── ui/
├── pages/
│   ├── Dashboard.tsx
│   └── Inbounds.tsx   # NEW
├── lib/
│   └── api.ts         # NEW: inbound API client
└── ...
```

### Pattern 1: DB-First Config Flow

**What:** Inbound CRUD persists to DB; ConfigGenerator builds full sing-box JSON from DB; ApplyConfig + Restart. No raw config edit.

**When to use:** All inbound create/update/delete.

**Flow:**
1. POST/PUT/DELETE /api/inbounds → InboundService save/update/delete
2. ConfigGenerator.Generate() reads all inbounds from DB
3. Build full config JSON (log, inbounds, outbounds, route)
4. ApplyConfig(tempPath, json) → sing-box check
5. If check fails: return error with check output; rollback DB (for create/update)
6. If check OK: atomic rename; Restart()

**Existing code (config.go):**

```go
// ApplyConfig already does: write temp → check → atomic rename
// Returns error containing check output on failure
func ApplyConfig(configPath string, configJSON []byte) error
```

**New:** ConfigGenerator produces `configJSON` from DB; InboundService calls it before ApplyConfig.

### Pattern 2: sing-box Inbound JSON Structure

**VLESS** (from [sing-box VLESS docs](https://sing-box.sagernet.org/configuration/inbound/vless/)):

```json
{
  "type": "vless",
  "tag": "vless-in-1",
  "listen": "::",
  "listen_port": 443,
  "users": [],
  "tls": {},
  "transport": {}
}
```

- **users:** Phase 2 empty; Phase 4 adds users. Panel manages structure.
- **tls:** `enabled: false` (no TLS), or `{ enabled: true, server_name, certificate_path, key_path }`, or `{ enabled: true, reality: { handshake, private_key, short_id, ... } }` (VLESS only).
- **transport:** omit = TCP; or `{ type: "ws", path }`, `{ type: "grpc", service_name }`, `{ type: "http", host, path }` (HTTP/2 via HTTP transport).

**Hysteria2** (from [sing-box Hysteria2 docs](https://sing-box.sagernet.org/configuration/inbound/hysteria2/)):

```json
{
  "type": "hysteria2",
  "tag": "hy2-in-1",
  "listen": "::",
  "listen_port": 443,
  "up_mbps": 100,
  "down_mbps": 100,
  "obfs": { "type": "salamander", "password": "..." },
  "users": [],
  "tls": { "enabled": true, "server_name": "...", "certificate_path": "...", "key_path": "..." }
}
```

- **tls:** Required for Hysteria2. No Reality.
- **up_mbps, down_mbps:** Optional; conflict with `ignore_client_bandwidth`.
- **obfs:** Optional; `type: "salamander"` + `password`.

### Pattern 3: TLS Options by Protocol

| Protocol | No TLS | TLS (cert) | Reality |
|----------|--------|------------|---------|
| VLESS | ✓ | ✓ | ✓ |
| Hysteria2 | ✗ | ✓ | ✗ |

**Reality fields (VLESS only):** `handshake.server`, `handshake.server_port`, `private_key`, `short_id`, `max_time_diff` (optional). `public_key` is client-side; server uses `private_key`. Generate via `sing-box generate reality-keypair`.

### Pattern 4: V2Ray Transport (VLESS)

From [V2Ray Transport](https://sing-box.sagernet.org/configuration/shared/v2ray-transport/):

| Type | Config Object | Key Fields |
|------|---------------|------------|
| TCP | omit `transport` | — |
| WebSocket | `{ type: "ws", path, headers }` | path (required) |
| gRPC | `{ type: "grpc", service_name }` | service_name (default "TunService") |
| HTTP/2 | `{ type: "http", host, path }` | host, path |

**Note:** gRPC requires `with_grpc` build tag for standard gRPC; default sing-box may use different implementation. Verify with `sing-box check`.

### Pattern 5: Wide Modal Form (CONTEXT)

**What:** Use Dialog with `max-w-3xl` or `max-w-4xl` for inbound form. Sections: 基本设置 | TLS | 传输.

**Example (extend existing dialog.tsx):**

```tsx
<Dialog open={open} onOpenChange={onOpenChange}>
  <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
    <DialogHeader>
      <DialogTitle>添加入站</DialogTitle>
    </DialogHeader>
    {checkError && (
      <div className="rounded-lg bg-destructive/10 text-destructive p-3 text-sm">
        <pre className="whitespace-pre-wrap">{checkError}</pre>
      </div>
    )}
    <form onSubmit={...}>
      <section>
        <h3>基本设置</h3>
        {/* tag, listen, port, protocol type */}
      </section>
      <section>
        <h3>TLS</h3>
        {/* TLS type select; conditional Reality/cert fields */}
      </section>
      <section>
        <h3>传输</h3>
        {/* transport radio; conditional path/service_name */}
      </section>
    </form>
  </DialogContent>
</Dialog>
```

### Anti-Patterns to Avoid

- **Raw config POST for inbound:** Use Inbound CRUD + ConfigGenerator; DB is source of truth.
- **Reality for Hysteria2:** Reality is VLESS-only; Hysteria2 TLS = cert path only.
- **Skipping sing-box check:** Always run check before atomic rename; present error in Modal per CONTEXT.
- **Config from file:** ConfigGenerator reads from DB, not from config.json.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| UUID generation | Manual | `github.com/google/uuid` | RFC 4122; VLESS users.uuid |
| Form validation | Custom | Zod + react-hook-form | Blur validation, nested schema |
| Reality keypair | Custom | `sing-box generate reality-keypair` | CLI generates; store private_key, short_id |
| Table + dropdown | Custom | shadcn Table + DropdownMenu | Accessibility, styling |
| Select/radio | Custom | shadcn Select, RadioGroup | Controlled, labels |

**Key insight:** sing-box config schema is strict; mismatched fields cause `sing-box check` failure. Use official docs as schema source; validate before submit.

---

## Common Pitfalls

### Pitfall 1: Hysteria2 Without TLS

**What goes wrong:** Hysteria2 inbound with `tls.enabled: false`; sing-box check fails.

**Why it happens:** Hysteria2 requires TLS.

**How to avoid:** For Hysteria2, always require TLS; no "无 TLS" option in form.

**Warning signs:** `sing-box check` error about TLS.

### Pitfall 2: Reality on Hysteria2

**What goes wrong:** User selects Reality for Hysteria2; config invalid.

**Why it happens:** Reality is VLESS-only.

**How to avoid:** TLS dropdown for Hysteria2 shows only "TLS" (cert-based); no Reality option.

**Warning signs:** Config has `reality` under Hysteria2 inbound.

### Pitfall 3: Port Conflict

**What goes wrong:** Two inbounds on same port; sing-box fails to bind.

**Why it happens:** No uniqueness check before apply.

**How to avoid:** Validate port uniqueness in InboundService; return clear error. Optionally check against existing inbounds in DB.

**Warning signs:** `bind: address already in use` in check output.

### Pitfall 4: Deprecated sing-box Fields

**What goes wrong:** Generated config has deprecated fields; future upgrade breaks.

**Why it happens:** Using old schema.

**How to avoid:** Follow current [sing-box config](https://sing-box.sagernet.org/configuration/); avoid deprecated inbound fields (sniff, sniff_override_destination per Listen docs).

**Warning signs:** Deprecation warnings in `sing-box check`.

### Pitfall 5: ApplyConfig Without Restart

**What goes wrong:** Config file updated but sing-box still runs old config.

**Why it happens:** sing-box has no hot-reload.

**How to avoid:** After ApplyConfig success, always call ProcessManager.Restart().

**Warning signs:** UI shows "应用成功" but behavior unchanged.

---

## Code Examples

### VLESS Inbound (TLS + WebSocket)

```json
{
  "type": "vless",
  "tag": "vless-in-1",
  "listen": "::",
  "listen_port": 443,
  "users": [],
  "tls": {
    "enabled": true,
    "server_name": "example.com",
    "certificate_path": "/etc/certs/fullchain.pem",
    "key_path": "/etc/certs/privkey.pem"
  },
  "transport": {
    "type": "ws",
    "path": "/vless"
  }
}
```

### VLESS Inbound (Reality + TCP)

```json
{
  "type": "vless",
  "tag": "vless-in-2",
  "listen": "::",
  "listen_port": 443,
  "users": [],
  "tls": {
    "enabled": true,
    "reality": {
      "enabled": true,
      "handshake": {
        "server": "google.com",
        "server_port": 443
      },
      "private_key": "...",
      "short_id": ["0123456789abcdef"]
    }
  }
}
```

### Hysteria2 Inbound (TLS + Obfs)

```json
{
  "type": "hysteria2",
  "tag": "hy2-in-1",
  "listen": "::",
  "listen_port": 443,
  "up_mbps": 100,
  "down_mbps": 100,
  "obfs": {
    "type": "salamander",
    "password": "obfs_secret"
  },
  "users": [],
  "tls": {
    "enabled": true,
    "server_name": "example.com",
    "certificate_path": "/etc/certs/fullchain.pem",
    "key_path": "/etc/certs/privkey.pem"
  }
}
```

### Inbound API Response (List)

```json
{
  "data": [
    {
      "id": 1,
      "tag": "vless-in-1",
      "protocol": "vless",
      "listen": "::",
      "listen_port": 443,
      "tls_type": "reality",
      "transport_type": "ws",
      "user_count": 0,
      "created_at": "2026-02-11T12:00:00Z"
    }
  ]
}
```

### ConfigGenerator Skeleton (Go)

```go
// Generate builds full sing-box config from DB
func (g *ConfigGenerator) Generate() ([]byte, error) {
  var inbounds []db.Inbound
  db.DB.Find(&inbounds)
  var raw []map[string]any
  for _, ib := range inbounds {
    raw = append(raw, g.inboundToSingBox(ib))
  }
  cfg := map[string]any{
    "log": map[string]any{"level": "info"},
    "inbounds": raw,
    "outbounds": []map[string]any{
      {"type": "direct", "tag": "direct"},
      {"type": "block", "tag": "block"},
    },
    "route": map[string]any{"rules": []any{}},
  }
  return json.MarshalIndent(cfg, "", "  ")
}
```

### Info Tooltip (shadcn)

```tsx
import { Info } from "lucide-react"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"

<TooltipProvider>
  <Tooltip>
    <TooltipTrigger asChild>
      <Info className="size-4 text-muted-foreground cursor-help" />
    </TooltipTrigger>
    <TooltipContent>
      <p>字段说明 + 典型值 + 何时需要修改</p>
    </TooltipContent>
  </Tooltip>
</TooltipProvider>
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Raw config JSON POST | DB + ConfigGenerator | Phase 2 | Inbound CRUD; DB source of truth |
| Xray config format | sing-box JSON | Different project | sing-box schema; no xray fields |
| sniff on inbound | rule sniff | sing-box 1.11+ | Deprecated inbound sniff; use route rules |

**Deprecated/outdated:**
- Inbound `sniff`, `sniff_override_destination`: Moved to route rules (sing-box 1.11+)
- V2Ray core TCP transport: sing-box has no plain TCP transport; omit transport = TCP

---

## Open Questions

1. **Config apply trigger**
   - What we know: Config must be applied after any inbound create/update/delete
   - What's unclear: Whether to auto-apply on each mutation or require explicit "Apply" button
   - Recommendation: Auto-apply on each mutation per ARCHITECTURE; return check error to frontend; do not restart if check fails

2. **Users array in Phase 2**
   - What we know: Phase 2 does not manage users; users array can be empty
   - What's unclear: Whether to omit `users` or send `users: []`
   - Recommendation: send `users: []`; sing-box accepts empty array; Phase 4 will populate

3. **gRPC build tag**
   - What we know: Standard gRPC needs `with_grpc` build tag
   - What's unclear: Default sing-box build includes gRPC or not
   - Recommendation: Document in tooltip; if check fails on gRPC config, show error; user can switch to WebSocket

---

## Sources

### Primary (HIGH confidence)
- [sing-box VLESS inbound](https://sing-box.sagernet.org/configuration/inbound/vless/) — structure, users, tls, transport
- [sing-box Hysteria2 inbound](https://sing-box.sagernet.org/configuration/inbound/hysteria2/) — up/down_mbps, obfs, tls, masquerade
- [sing-box TLS](https://sing-box.sagernet.org/configuration/shared/tls/) — Reality, certificate_path, server_name
- [sing-box V2Ray Transport](https://sing-box.sagernet.org/configuration/shared/v2ray-transport/) — ws, grpc, http
- [sing-box Listen Fields](https://sing-box.sagernet.org/configuration/shared/listen/) — listen, listen_port
- internal/core/config.go, process.go — ApplyConfig, Check, Restart
- internal/api/core.go — ConfigHandler, StatusHandler, RestartHandler
- web/src/pages/Dashboard.tsx — Dialog for check error, TanStack Query, Toast

### Secondary (MEDIUM confidence)
- .planning/research/ARCHITECTURE.md — ConfigGenerator, DB-first flow, API design
- .planning/phases/01-foundation/01-RESEARCH.md — Phase 1 stack, patterns

### Tertiary (LOW confidence)
- WebSearch VLESS WebSocket Reality — community examples; structure verified via official docs

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — Phase 1 + shadcn, official sing-box docs
- Architecture: HIGH — ARCHITECTURE.md + existing core/config/process
- Pitfalls: HIGH — official docs + common config errors

**Research date:** 2026-02-11
**Valid until:** 30 days (stable sing-box config format)
