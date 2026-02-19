# Phase 10: Sidebar Pages & Config Fix - Research

**Researched:** 2026-02-19
**Domain:** Frontend page creation, dashboard refactoring, Go config path consistency
**Confidence:** HIGH

## Summary

Phase 10 is a UX/architecture improvement phase with two distinct streams: (1) enabling three disabled sidebar pages (Subscriptions, Traffic, Core), refactoring Dashboard into a stats overview, and moving logout to the sidebar; (2) fixing config path inconsistencies in the Go backend where `ApplyConfig()` uses the env-only `NewProcessManager()` instead of the config-aware variant, and where `ConfigPathFromEnv()` defaults to `./config.json` instead of `sing-box.json`.

The frontend work is primarily new React page components using existing data from `GET /api/users` and `GET /api/inbounds` plus a new lightweight stats summary endpoint. The core management code (status, restart, update, rollback, version list) moves from `Dashboard.tsx` to a new `Core.tsx` page. The backend config path fix is surgical — `ApplyConfig()` needs to accept a `*ProcessManager` parameter, `ConfigPathFromEnv()` should be removed (dead code), and `NewProcessManager()` env-only constructor should be deprecated or removed.

**Primary recommendation:** Build new pages from existing API data and components; fix `ApplyConfig()` to accept `*ProcessManager` parameter; add `GET /api/stats/summary` for dashboard; use shadcn Tabs for traffic page.

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions

**订阅页面:**
- 页面定位由 Claude 决定（用户订阅列表或概览均可）
- 用户页的 UserSubscriptionModal 保留作为快捷入口，订阅页提供完整视图
- 列表信息精简：用户名、订阅链接、状态（活跃/过期/超限）
- 只读操作：查看订阅信息、复制链接、查看 QR 码（不含重置 token 等修改操作）

**流量页面:**
- Tab 切换布局："按入站"和"按用户"两个 Tab，各自一个表格
- 顶部需要流量概览卡片（总上行/下行、活跃用户数等汇总数据）
- 入站页/用户页原有的流量列保留不变，流量页提供聚合视图
- 暂不需要图表（折线图/柱状图），表格展示即可，后续阶段再加

**核心页面:**
- 完整迁移 Dashboard 中的核心管理功能（状态、版本、重启、更新、回滚、版本列表）
- 增强内容：配置文件查看、运行日志等（具体范围由 Claude 在研究阶段确定可行性）

**仪表盘重构:**
- 移除现有的入站数/用户数占位卡片（硬编码 0）
- 改为统计概览页：展示入站数、用户数、总流量等实际数据（调 API 获取）
- 核心管理功能全部迁出到独立核心页面
- "退出登录"按钮从仪表盘右上角移到侧边栏底部（全局可见）

**配置路径修复:**
- 全面清理所有配置路径相关代码，确保一致性
- 修复 `ApplyConfig()` 中 `NewProcessManager()` 不读取 panel config 的问题
- 修复 `NewProcessManager()` 和 `ConfigPathFromEnv()` 默认路径 `./config.json` 与实际 sing-box 配置路径 `sing-box.json` 不一致
- 统一消除遗留的 env-only ProcessManager 用法
- Docker 配置（Dockerfile/docker-compose）路径也需检查和修正
- 启动时验证配置路径有效性（文件存在、目录可写等），无效时报警

### Claude's Discretion

- 订阅页面的具体定位和布局设计
- 配置路径修复的具体实现方式（传递 config 参数 vs 删除遗留函数 vs 两者结合）
- 核心页面增强功能的具体范围（配置查看、日志等需研究可行性）
- 仪表盘统计卡片的具体数据和布局
- 各页面的 loading/error/empty 状态处理

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope

</user_constraints>

## Standard Stack

### Core (already in project)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React | ^19.2.0 | UI framework | Already the project framework |
| react-router-dom | ^7.13.0 | Client-side routing | Already used for all routes |
| @tanstack/react-query | ^5.90.20 | Data fetching & caching | Already used by Dashboard, Users, etc. |
| shadcn/ui | ^3.8.4 | Component library | Already used for all UI components |
| radix-ui | ^1.4.3 | Headless primitives | Already installed, includes @radix-ui/react-tabs |
| sonner | ^2.0.7 | Toast notifications | Already used throughout |
| lucide-react | ^0.563.0 | Icons | Already used for sidebar icons |
| qrcode.react | ^4.2.0 | QR code generation | Already used by UserSubscriptionCard |
| tailwindcss | ^4.1.18 | Styling | Already configured |
| go-chi/chi/v5 | (Go) | HTTP router | Already used for all API routes |
| gorm + glebarez/sqlite | (Go) | Database | Already used for all DB operations |

### Supporting (to add)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| shadcn Tabs component | N/A (generate) | Tab switching for Traffic page | `npx shadcn@latest add tabs` — uses @radix-ui/react-tabs already in lockfile |

### Alternatives Considered

None — this phase uses only existing stack. No new libraries needed.

## Architecture Patterns

### Current Project Structure (frontend)
```
web/src/
├── pages/              # Page components (Dashboard, Inbounds, Users, Certificates)
├── components/
│   ├── layout/         # AppLayout, Sidebar
│   ├── ui/             # shadcn components
│   ├── inbounds/       # InboundTable, InboundFormModal
│   ├── users/          # UserTable, UserFormModal, UserSubscriptionModal, etc.
│   └── certificates/   # CertificateTable, CertificateFormModal
├── lib/                # format.ts, utils.ts
├── routes.tsx          # createBrowserRouter config
└── main.tsx            # App entry
```

### New files to create (frontend)
```
web/src/
├── pages/
│   ├── Subscriptions.tsx     # NEW: subscription overview page
│   ├── Traffic.tsx           # NEW: traffic aggregation page
│   └── Core.tsx              # NEW: core management page (migrated from Dashboard)
```

### Current Project Structure (backend)
```
internal/
├── api/
│   ├── routes.go       # Chi router setup
│   ├── core.go         # Status, Restart, Config, Versions, Update, Rollback handlers
│   ├── users.go        # CRUD + batch + subscription
│   ├── inbounds.go     # CRUD
│   └── ...
├── core/
│   ├── config.go       # ApplyConfig — THE BUG IS HERE
│   ├── process.go      # ProcessManager (3 constructors)
│   ├── updater.go      # CoreUpdater + ConfigPathFromEnv
│   ├── generator.go    # ConfigGenerator
│   └── ...
├── config/
│   └── panel.go        # LoadConfig, Config struct
└── db/
    ├── user.go         # User model + queries
    ├── inbound.go      # Inbound model + queries
    └── ...
```

### Pattern 1: Page Component with useQuery
**What:** All pages follow the same pattern: `useQuery` for data fetching, table/card for display, mutation for actions.
**When to use:** Every new page.
**Example (from existing Users.tsx):**
```typescript
const { data, isLoading } = useQuery({
  queryKey: ["users"],
  queryFn: async () => {
    const res = await fetch("/api/users", { credentials: "include" })
    if (!res.ok) throw new Error("获取失败")
    return res.json()
  },
})
```

### Pattern 2: Config Path Resolution in Go
**What:** Config paths resolved once in `LoadConfig()`, threaded through via `*config.Config` to handlers, then to `NewProcessManagerFromConfig(cfg)`.
**When to use:** Any handler that needs sing-box binary or config path.
**Correct pattern (from api/core.go):**
```go
pm := core.NewProcessManagerFromConfig(cfg) // cfg is *config.Config from Routes()
```
**Bug pattern (from core/config.go):**
```go
pm := NewProcessManager() // WRONG: reads from env, defaults to ./config.json
```

### Pattern 3: Sidebar Navigation Item
**What:** Each sidebar item is an entry in the `navItems` array with `to`, `label`, `icon`, and optional `disabled`.
**Current:** Subscriptions, Traffic, Core are `disabled: true`. Remove `disabled` to enable.

### Anti-Patterns to Avoid
- **Creating new ProcessManager constructors:** The project already has 3 (`NewProcessManager`, `NewProcessManagerFromConfig`, `NewProcessManagerWithBinary`). Reduce to 2 (remove the env-only one).
- **Client-side computation of stats in multiple places:** Create a single `/api/stats/summary` endpoint rather than having each page re-fetch and sum.
- **Duplicating core management UI:** Move it cleanly to Core.tsx; Dashboard should NOT retain any core management buttons.

## Codebase Investigation Results

### Config Path Bug: Full Analysis

**Bug 1: `ApplyConfig()` uses `NewProcessManager()`**
- File: `internal/core/config.go` line 22
- `ApplyConfig(configPath, configJSON)` calls `NewProcessManager()` which reads `SINGBOX_CONFIG_PATH` env or defaults to `./config.json`
- The `configPath` parameter is already the correct path, but the ProcessManager is created without knowledge of the binary path
- The PM is only used for `pm.Check(tmpPath)` — it needs the correct binary path to find sing-box
- **Fix:** Change signature to `ApplyConfig(configPath string, configJSON []byte, pm *ProcessManager) error` — callers already have a PM from `NewProcessManagerFromConfig(cfg)`

**Bug 2: `NewProcessManager()` default `./config.json`**
- File: `internal/core/process.go` line 24
- Falls back to `./config.json` when `SINGBOX_CONFIG_PATH` is not set
- But `LoadConfig()` in `panel.go` defaults to `filepath.Join(cfg.DataDir, "sing-box.json")` = `./data/sing-box.json`
- These are different paths! The env-only constructor is inconsistent with the panel config
- **Fix:** Remove `NewProcessManager()` entirely. All callers should use `NewProcessManagerFromConfig(cfg)` or `NewProcessManagerWithBinary(path, bin)`

**Bug 3: `ConfigPathFromEnv()` default `./config.json`**
- File: `internal/core/updater.go` line 332-338
- Same wrong default as `NewProcessManager()`
- **Not currently called** by any Go code (dead code from Phase 1/7 evolution)
- **Fix:** Delete this function entirely

**Current usage audit:**
| Constructor | Used By | Status |
|-------------|---------|--------|
| `NewProcessManager()` | `ApplyConfig()` only | **BUG — remove** |
| `NewProcessManagerFromConfig(cfg)` | All API handlers (core, users, inbounds, certs) | Correct |
| `NewProcessManagerWithBinary(path, bin)` | `CoreUpdater.Update()`, `CoreUpdater.Rollback()` | Correct |
| `ConfigPathFromEnv()` | Nothing (dead code) | **Remove** |

### Docker Path Analysis

**Dockerfile (line 40):**
```
ENV CONFIG_PATH=/data/config.json DATA_DIR=/data SINGBOX_BINARY_PATH=/usr/local/bin/sing-box
```
- `CONFIG_PATH=/data/config.json` → panel config file
- `DATA_DIR=/data` → data directory
- `SINGBOX_BINARY_PATH=/usr/local/bin/sing-box` → sing-box binary
- `LoadConfig()` resolves `SingboxConfigPath` = `filepath.Join("/data", "sing-box.json")` = `/data/sing-box.json` ✓

**docker-compose.yml:**
```yaml
environment:
  CONFIG_PATH: /data/config.json
  DATA_DIR: /data
```
- Missing `SINGBOX_BINARY_PATH` — relies on Dockerfile ENV inheriting. This works because docker-compose only overrides specified vars.
- **Recommendation:** Add `SINGBOX_BINARY_PATH: /usr/local/bin/sing-box` to docker-compose.yml for explicitness, or document that it's inherited from Dockerfile.

**docker-entrypoint.sh:** Simple `mkdir -p /data && exec /s-ui` — no path issues.

### Existing API Endpoints for New Pages

**Subscription page data — already available:**
- `GET /api/users` returns: `name`, `subscription_url`, `traffic_used`, `traffic_limit`, `expire_at`, `enabled`, `traffic_uplink`, `traffic_downlink`
- Status derivation: same logic as `getStatusBadge()` in `UserTable.tsx` — disabled/expired/over-limit/active
- No new backend endpoint needed for basic subscription list

**Traffic page data — already available:**
- `GET /api/inbounds` returns: `tag`, `protocol`, `traffic_uplink`, `traffic_downlink`
- `GET /api/users` returns: `name`, `traffic_uplink`, `traffic_downlink`, `traffic_used`
- Summary stats (total uplink/downlink, active user count) can be computed client-side from these lists
- **Recommendation:** Add `GET /api/stats/summary` to avoid double-fetching and for cleaner separation

**Core page data — already available:**
- `GET /api/core/status` → running state, version
- `GET /api/core/versions` → GitHub releases list
- `POST /api/core/restart`, `POST /api/core/update`, `POST /api/core/rollback`
- **New needed:** `GET /api/core/config-file` to read the current sing-box config JSON for the config viewer

### Core Page Enhancement Feasibility

**Config file viewer (FEASIBLE):**
- sing-box config is a JSON file at `cfg.SingboxConfigPath`
- Backend endpoint: `GET /api/core/config-file` → read file, return as JSON
- Frontend: read-only JSON viewer (use `<pre>` with syntax highlighting or just formatted JSON)
- Risk: File may not exist yet (before first inbound creation) — handle gracefully

**Log viewer (COMPLEX, RECOMMEND DEFERRING):**
- sing-box is started via `cmd.Start()` with `cmd.Stdout = nil, cmd.Stderr = nil` — output goes to /dev/null
- To capture logs, would need to redirect stdout/stderr to a file, then serve that file
- Alternative: sing-box supports `log.output` config option to write to a file
- This requires changing the process management model significantly
- **Recommendation:** Defer log viewer to a future phase. For Phase 10, add the config viewer only.

### Stats Summary Endpoint Design

New endpoint: `GET /api/stats/summary`

```go
type StatsSummary struct {
    InboundCount   int   `json:"inbound_count"`
    UserCount      int   `json:"user_count"`
    ActiveUserCount int  `json:"active_user_count"`
    TotalUplink    int64 `json:"total_uplink"`
    TotalDownlink  int64 `json:"total_downlink"`
}
```

Implementation: Simple COUNT and SUM queries on users and inbounds tables. No new DB models needed.

### Sidebar Logout Button

Currently in `Dashboard.tsx` line 163-166:
```tsx
const handleLogout = async () => {
    await fetch("/api/logout", { method: "POST", credentials: "include" })
    navigate("/login", { replace: true })
}
```

Move to `Sidebar.tsx` bottom section. The `SidebarFooter` component from shadcn/ui sidebar provides the correct slot. Need `useNavigate()` from react-router-dom in the Sidebar component.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tab switching | Custom tab state management | shadcn Tabs (`npx shadcn@latest add tabs`) | Handles a11y, keyboard nav, ARIA roles |
| QR code display | Canvas-based QR rendering | `qrcode.react` (already installed) | Mature, tested, handles edge cases |
| Data fetching/caching | Custom fetch + state | `@tanstack/react-query` (already installed) | Dedup, cache invalidation, refetch |
| JSON syntax highlighting | Custom tokenizer | `<pre>` with `JSON.stringify(data, null, 2)` | Simple, sufficient for read-only config view |
| Status badge logic | Per-page status derivation | Shared utility function extracted from UserTable | Single source of truth for active/expired/over-limit |

**Key insight:** This phase creates new pages from existing data. Almost all logic and UI patterns already exist in the codebase — extract and compose, don't rebuild.

## Common Pitfalls

### Pitfall 1: Forgetting to Update `ApplyConfig` Callers
**What goes wrong:** Fix `ApplyConfig()` signature but miss a caller, causing compilation errors or runtime panics.
**Why it happens:** `ApplyConfig()` is called in 7 places across 4 files (core.go, users.go, inbounds.go, certs.go).
**How to avoid:** Grep for `core.ApplyConfig` before and after the change. Go compiler will catch missing argument, but verify with `go build`.
**Warning signs:** Any `core.ApplyConfig(path, cfg)` call without a PM argument after the refactor.

### Pitfall 2: Route Registration Order
**What goes wrong:** New routes conflict or shadowed by catch-all.
**Why it happens:** The SPA handler `r.NotFound(spaHandler(staticFS))` catches all unmatched routes, and the `checkAuth` loader in the browser router must cover new paths.
**How to avoid:** Add routes inside the existing `children` array of the authenticated layout route.
**Warning signs:** New page shows login screen or blank page.

### Pitfall 3: Duplicated Dashboard Core State
**What goes wrong:** Dashboard still has core management code, causing stale queries or conflicting mutations.
**Why it happens:** Incomplete migration — copying code to Core.tsx but not removing from Dashboard.tsx.
**How to avoid:** Extract core management into Core.tsx, then strip Dashboard.tsx clean.
**Warning signs:** Two places calling `POST /api/core/restart` with different queryKey invalidation.

### Pitfall 4: Missing ProcessManager in ApplyConfig Check
**What goes wrong:** `ApplyConfig()` can't find sing-box binary because ProcessManager has wrong binary path.
**Why it happens:** The current bug — `NewProcessManager()` doesn't read panel config's `SingboxBinaryPath`.
**How to avoid:** Pass the `*ProcessManager` that callers already construct from config.
**Warning signs:** `sing-box check` fails with "not found" even though sing-box is installed at a custom path.

### Pitfall 5: Empty Config File on Core Page
**What goes wrong:** Config viewer shows error or crashes when no config file exists yet.
**Why it happens:** Config file is only created on first inbound save. Fresh installs have no file.
**How to avoid:** Backend returns 404 gracefully; frontend shows "暂无配置文件" empty state.
**Warning signs:** Core page breaks on fresh installation before any inbound is created.

## Code Examples

### New Route Registration (routes.tsx)
```typescript
import { Subscriptions } from './pages/Subscriptions'
import { Traffic } from './pages/Traffic'
import { Core } from './pages/Core'

// Inside children array of authenticated layout route:
{ path: "subscriptions", element: <Subscriptions /> },
{ path: "traffic", element: <Traffic /> },
{ path: "core", element: <Core /> },
```

### Enable Sidebar Items (Sidebar.tsx)
```typescript
// Remove disabled: true from these entries:
{ to: "/subscriptions", label: "订阅", icon: Link2 },
{ to: "/traffic", label: "流量", icon: Gauge },
{ to: "/core", label: "核心", icon: Box },
```

### Sidebar Footer with Logout (Sidebar.tsx)
```typescript
import { SidebarFooter } from "@/components/ui/sidebar"
import { LogOut } from "lucide-react"
import { useNavigate } from "react-router-dom"

// Inside Sidebar component, after SidebarContent:
<SidebarFooter className="p-4">
  <Button variant="ghost" size="sm" className="w-full justify-start" onClick={handleLogout}>
    <LogOut className="size-4" />
    <span>退出登录</span>
  </Button>
</SidebarFooter>
```

### Fix ApplyConfig (internal/core/config.go)
```go
// Before (BUG):
func ApplyConfig(configPath string, configJSON []byte) error {
    pm := NewProcessManager() // WRONG: env-only, defaults to ./config.json
    _, err := pm.Check(tmpPath)

// After (FIX):
func ApplyConfig(configPath string, configJSON []byte, pm *ProcessManager) error {
    _, err := pm.Check(tmpPath)
```

### Caller Update Pattern (api handlers)
```go
// Before:
if err := core.ApplyConfig(path, cfg); err != nil {

// After:
pm := core.NewProcessManagerFromConfig(panelCfg)
if err := core.ApplyConfig(path, cfg, pm); err != nil {

// Note: pm is often already created later for Restart. 
// Create once, use for both ApplyConfig and Restart.
```

### Remove Dead Code (internal/core/updater.go)
```go
// DELETE ConfigPathFromEnv() entirely — not called by any code
```

### Remove NewProcessManager() (internal/core/process.go)
```go
// DELETE NewProcessManager() — only caller was ApplyConfig, now takes *ProcessManager
```

### Stats Summary Endpoint (internal/api/stats.go)
```go
func StatsSummaryHandler(sm *scs.SessionManager) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var inboundCount, userCount, activeUserCount int64
        var totalUplink, totalDownlink int64
        
        db.DB.Model(&db.Inbound{}).Count(&inboundCount)
        db.DB.Model(&db.User{}).Count(&userCount)
        db.DB.Model(&db.User{}).Where("enabled = ?", true).Count(&activeUserCount)
        
        var uplinkResult, downlinkResult struct{ Total int64 }
        db.DB.Model(&db.Inbound{}).Select("COALESCE(SUM(traffic_uplink), 0) as total").Scan(&uplinkResult)
        db.DB.Model(&db.Inbound{}).Select("COALESCE(SUM(traffic_downlink), 0) as total").Scan(&downlinkResult)
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]any{
            "inbound_count":    inboundCount,
            "user_count":       userCount,
            "active_user_count": activeUserCount,
            "total_uplink":     uplinkResult.Total,
            "total_downlink":   downlinkResult.Total,
        })
    }
}
```

### Config File Viewer Endpoint (internal/api/core.go)
```go
func ConfigFileHandler(sm *scs.SessionManager, cfg *config.Config) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        path := configPath(cfg)
        data, err := os.ReadFile(path)
        if err != nil {
            if os.IsNotExist(err) {
                http.Error(w, "config file not found", http.StatusNotFound)
                return
            }
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        w.Write(data)
    }
}
```

### Startup Config Validation (cmd/server/main.go)
```go
// After LoadConfig(), before starting server:
if cfg.SingboxConfigPath != "" {
    dir := filepath.Dir(cfg.SingboxConfigPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        log.Printf("[warn] cannot create config dir %s: %v", dir, err)
    }
}
pm := core.NewProcessManagerFromConfig(cfg)
if !pm.Available() {
    log.Printf("[warn] sing-box binary not found at configured path")
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `NewProcessManager()` env-only | `NewProcessManagerFromConfig(cfg)` | Phase 8 | Most callers already updated; `ApplyConfig()` is the remaining outlier |
| Hardcoded 0 stats cards | Real stats from API | Phase 10 (this phase) | Dashboard shows actual data |
| Core management in Dashboard | Dedicated Core page | Phase 10 (this phase) | Cleaner separation of concerns |
| `ConfigPathFromEnv()` | Unused (dead code) | Phase 8 evolved past it | Should be deleted |

**Deprecated/outdated:**
- `NewProcessManager()`: Should be removed entirely — all callers should use config-aware constructors
- `ConfigPathFromEnv()`: Dead code, never called — should be deleted
- Dashboard core management UI: Moving to dedicated Core page

## Open Questions

1. **Log viewer implementation timing**
   - What we know: sing-box currently runs with stdout/stderr to /dev/null. Supporting log viewing requires either: (a) redirecting output to a file + log rotation, or (b) using sing-box's built-in `log.output` config option
   - What's unclear: Whether users need this urgently enough to justify the complexity in this phase
   - Recommendation: **Defer to future phase.** Phase 10 adds config file viewer only. Log viewer requires process management changes that are better scoped separately.

2. **Stats summary endpoint vs client-side computation**
   - What we know: All data is available from existing endpoints. Summary could be computed client-side.
   - What's unclear: Performance impact of fetching full user/inbound lists just for counts on dashboard
   - Recommendation: **Add `GET /api/stats/summary`** — lightweight DB COUNT/SUM queries are more efficient than transferring and computing on full lists. Also cleaner API design.

3. **docker-compose SINGBOX_BINARY_PATH**
   - What we know: Dockerfile ENV sets it to `/usr/local/bin/sing-box`, docker-compose doesn't override it
   - What's unclear: Whether some deployment tools strip Dockerfile ENV when docker-compose environment is specified
   - Recommendation: **Add explicitly** to docker-compose.yml for robustness. Low risk, high clarity.

## Sources

### Primary (HIGH confidence)
- **Codebase analysis** — Direct reading of all relevant source files:
  - `internal/core/config.go` — ApplyConfig bug confirmed (line 22)
  - `internal/core/process.go` — 3 constructors, NewProcessManager env-only defaults to `./config.json`
  - `internal/core/updater.go` — ConfigPathFromEnv dead code confirmed
  - `internal/config/panel.go` — LoadConfig defaults to `sing-box.json` in DataDir
  - `web/src/components/layout/Sidebar.tsx` — 3 items disabled: true
  - `web/src/routes.tsx` — 4 routes in children, need 3 more
  - `web/src/pages/Dashboard.tsx` — core management code to migrate
  - All API handler files — caller audit for ApplyConfig and ProcessManager

### Secondary (MEDIUM confidence)
- **shadcn Tabs** — @radix-ui/react-tabs already in bun.lock as dependency of radix-ui; `npx shadcn@latest add tabs` should work without new installs

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all from existing codebase, no new libraries
- Architecture: HIGH — patterns directly observed from existing code, simple extension
- Config path bug: HIGH — root cause confirmed from source code reading
- Core page enhancements: MEDIUM — config viewer feasible and straightforward; log viewer deferred due to complexity
- Docker paths: HIGH — verified Dockerfile and docker-compose.yml ENV values

**Research date:** 2026-02-19
**Valid until:** 2026-03-19 (stable — patterns unlikely to change)
