# Phase 1: Foundation - Research

**Researched:** 2026-02-11
**Domain:** Admin panel foundation — auth, sing-box integration, dark theme UI
**Confidence:** HIGH

## Summary

Phase 1 delivers the foundational control-plane: admin authentication (Setup + Login), sing-box process integration (version detection, config apply, restart), and a dark-theme UI shell (shadcn/ui + react-bits). The stack is already defined by project research: Chi router, GORM + SQLite, SCS sessions, React + Vite + shadcn/ui. sing-box has no management API—the panel controls it via JSON config file and process lifecycle. Version detection uses `sing-box version`, config validation uses `sing-box check -c path`, and config changes require a full restart (no hot-reload).

**Primary recommendation:** Use Chi (not Gin), SCS for session auth with SQLite store, ThemeProvider with `defaultTheme="dark"` for Vercel/Linear-style dark UI, Sonner for toasts (position `top-right` per CONTEXT), shadcn Sidebar for fixed left nav. Run `sing-box check` before every config apply; use temp file + atomic rename for config writes.

---

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions

#### 登录与首次设置流程
- 首次部署使用独立 Setup 页面，包含设置用户名、密码、确认密码
- 登录后 Session 有效期 7 天，支持「记住我」选项
- 登录页采用居中极简风格：纯色背景 + 居中登录卡片（Vercel Dashboard 风格）
- 登录失败显示简单提示「用户名或密码错误」，不做重试次数限制

#### 面板整体布局
- 固定左侧边栏导航（Linear/Vercel 风格）
- 侧边栏预留所有将来菜单项（入站、用户、订阅、证书、流量、核心），未实现的置灰/禁用
- 仪表盘（首页）显示 sing-box 状态卡片 + 预留统计概览卡片（入站数、用户数等，Phase 1 显示 0）
- Phase 1 仅考虑桌面端，不做移动端响应式适配

#### sing-box 状态与操作反馈
- sing-box 运行状态仅在仪表盘卡片中展示（运行/停止状态、版本号、运行时长）
- 配置应用成功/失败使用 Toast 通知（右上角弹出，几秒后自动消失）
- 重启 sing-box 时按钮变为 Loading 状态，完成后恢复
- sing-box check 验证失败时弹出 Modal 显示原始错误输出（代码块格式）

#### 视觉风格与微交互
- 暗色主题使用深灰系配色（类似 Linear/GitHub Dark，#1a1a2e/#111827 色域），非纯黑
- 强调色使用紫色系（类似 Linear 紫色强调），用于按钮、链接、活动状态
- react-bits 微交互程度适中：页面切换、列表项出现、卡片 hover、Modal 弹出等均有过渡动画
- 无边框设计：用背景色差和阴影区分层次（Vercel 风格），不使用显式边框线

### Claude's Discretion
- 具体的色值定义与 CSS 变量命名
- 登录页 Logo/品牌展示方式
- Toast 通知的精确位置和持续时间
- Loading 骨架屏的具体样式
- 侧边栏图标选择
- Setup 页面的具体表单校验规则

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope

</user_constraints>

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| **Chi** | v5.x | HTTP router | sing-box uses Chi; lighter than Gin; stdlib-compatible (STACK.md) |
| **GORM** | v1.31+ | ORM | Industry standard; 3x-ui pattern; migrations, soft deletes |
| **alexedwards/scs** | v2 | Session | Router-agnostic; SQLite store; supports IdleTimeout, Lifetime |
| **golang.org/x/crypto/bcrypt** | latest | Password hash | OWASP-recommended; GenerateFromPassword, CompareHashAndPassword |
| **shadcn/ui** | latest | Component library | Copy-paste; dark theme; new-york style |
| **Sonner** | — | Toast | shadcn-recommended; `toast()`, `position`, `duration` |
| **react-bits** | — | Micro-interactions | 110+ animated components; shadcn/jsrepo compatible |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **Vite** | 6.x | Build tool | React SPA; fast HMR |
| **Tailwind CSS** | v4 | Styling | shadcn native |
| **TanStack Query** | v5 | Server state | API fetch, caching, loading |
| **React Router** | v7 | Routing | SPA routes; protected routes |
| **React Hook Form** | v7 | Forms | Setup, Login; Zod validation |
| **gorm.io/driver/sqlite** | v1.6+ | SQLite | Single-node DB |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Chi | Gin | Gin heavier; sing-box uses Chi; align with ecosystem |
| SCS | gorilla/sessions | SCS lighter; 3x-ui uses gorilla; SCS has SQLite store |
| Sonner | shadcn Toast | shadcn deprecated toast; Sonner is recommended |

**Installation (bun per PROJECT.md):**

```bash
# Backend
go get github.com/go-chi/chi/v5
go get github.com/alexedwards/scs/v2
go get github.com/alexedwards/scs/sqlite3store
go get golang.org/x/crypto/bcrypt
go get gorm.io/gorm gorm.io/driver/sqlite

# Frontend
bun create vite@latest web -- --template react-ts
cd web && bun add @tanstack/react-query @tanstack/react-query-devtools
bunx shadcn@latest init
bunx shadcn@latest add sonner sidebar dialog card skeleton button input label
bun add react-bits react-hook-form @hookform/resolvers zod
```

---

## Architecture Patterns

### Recommended Project Structure

```
s-ui/
├── cmd/server/           # main.go, embed static
├── internal/
│   ├── api/             # Chi routes, handlers
│   │   ├── auth.go      # login, logout, setup
│   │   └── core.go      # sing-box status, restart
│   ├── core/            # sing-box integration
│   │   ├── process.go   # version, check, run, restart
│   │   └── config.go    # config write (Phase 2)
│   ├── db/              # GORM models, migrations
│   │   └── admin.go     # Admin model (username, password_hash)
│   └── session/         # SCS setup
├── web/                 # React SPA
│   ├── src/
│   │   ├── components/  # ui/, layout/
│   │   ├── pages/       # Login, Setup, Dashboard
│   │   ├── lib/         # api client
│   │   └── routes/      # protected routes
│   └── ...
└── embed.go             # go:embed web/dist
```

### Pattern 1: First-Time Setup Flow

**What:** Redirect to `/setup` when no admin exists; Setup page creates admin; then redirect to login.

**When to use:** First deployment; no default credentials.

**Implementation:**
- DB: `admins` table (id, username, password_hash, created_at)
- Middleware: if no admin in DB and path != /setup → redirect to /setup
- Setup API: POST /api/setup → validate → bcrypt hash → insert admin → create session → redirect to /
- Frontend: Setup page form (username, password, confirm) → call /api/setup

### Pattern 2: Session Auth with SCS

**What:** Cookie-based session; 7-day lifetime when "remember me"; session-only when not.

**Example (Go):**

```go
// Source: pkg.go.dev/github.com/alexedwards/scs/v2
sessionManager := scs.New()
sessionManager.Store = sqlite3store.New(db)
sessionManager.Lifetime = 7 * 24 * time.Hour  // when remember me
sessionManager.Cookie.Persist = false          // session cookie when !remember
sessionManager.Cookie.HttpOnly = true
sessionManager.Cookie.Secure = true            // production
sessionManager.Cookie.SameSite = http.SameSiteLaxMode
r.Use(sessionManager.LoadAndSave)
```

**Remember me:** Store `remember` in form; when true, set `Cookie.Persist = true` and `Lifetime = 7*24*time.Hour`; when false, `Cookie.Persist = false` and `IdleTimeout = 24*time.Hour` (or session-only).

### Pattern 3: sing-box Process Management

**What:** Version via `sing-box version`; config validation via `sing-box check -c path`; restart = stop + start.

**Commands:**
- Version: `sing-box version` or `sing-box version -n` (no color, for parsing)
- Check: `sing-box check -c /path/to/config.json`
- Run: `sing-box run -c /path/to/config.json` (long-running)

**ProcessManager responsibilities:**
- `Version() (string, error)` — exec, capture stdout
- `Check(configPath string) (output string, err error)` — exec, capture stdout+stderr
- `IsRunning() bool` — check process existence (PID file or pgrep)
- `Restart(configPath string) error` — stop → start

**Config write:** Write to temp file → `sing-box check` → if OK, atomic rename to config.json → Restart.

### Pattern 4: Dark Theme with ThemeProvider

**What:** shadcn ThemeProvider with `defaultTheme="dark"`; no mode toggle in Phase 1 (locked dark).

**Example:**

```tsx
// Source: ui.shadcn.com/docs/dark-mode/vite
import { ThemeProvider } from "@/components/theme-provider"

function App() {
  return (
    <ThemeProvider defaultTheme="dark" storageKey="s-ui-theme">
      {children}
    </ThemeProvider>
  )
}
```

### Pattern 5: Fixed Sidebar with Placeholder Items

**What:** shadcn Sidebar; `SidebarMenuButton` with `isActive` for current route; disabled items use `disabled` or `aria-disabled`.

**Example:**

```tsx
// Source: ui.shadcn.com/docs/components/radix/sidebar
<SidebarMenu>
  <SidebarMenuItem>
    <SidebarMenuButton asChild isActive>
      <Link to="/">仪表盘</Link>
    </SidebarMenuButton>
  </SidebarMenuItem>
  <SidebarMenuItem>
    <SidebarMenuButton asChild disabled>
      <Link to="/inbounds">入站</Link>
    </SidebarMenuButton>
  </SidebarMenuItem>
  {/* ... 用户、订阅、证书、流量、核心 — disabled */}
</SidebarMenu>
```

### Anti-Patterns to Avoid

- **Gin:** Project research recommends Chi; sing-box uses Chi.
- **Default credentials:** First deployment must require Setup; no admin/admin.
- **Skip sing-box check:** Always run `sing-box check` before applying config.
- **Direct config overwrite:** Write to temp file, validate, then atomic rename.
- **Light theme default:** CONTEXT locks dark; use `defaultTheme="dark"`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Password hashing | Custom hash | bcrypt (golang.org/x/crypto/bcrypt) | Salt, cost factor, industry standard |
| Session management | Cookie parsing | alexedwards/scs | IdleTimeout, Lifetime, secure cookies, SQLite store |
| Toast notifications | Custom div | Sonner | Positioning, duration, types (success/error) |
| Dark theme | Manual CSS | ThemeProvider + Tailwind dark: | shadcn components expect it |
| Sidebar layout | Flex/Grid from scratch | shadcn Sidebar | Collapsible, theming, accessibility |
| Modal forms | Custom overlay | shadcn Dialog | Focus trap, ESC, scroll lock |

**Key insight:** Auth (bcrypt, session) and sing-box process control have edge cases (replay, race, signal handling). Use established libraries.

---

## Common Pitfalls

### Pitfall 1: Panel Auth Bypass / Weak Auth

**What goes wrong:** Panel accessible without auth; default credentials not changed.

**Why it happens:** No Setup flow; default admin/admin; session bugs.

**How to avoid:** Require Setup on first deploy; no default credentials; bcrypt; HttpOnly, Secure cookies; validate session on every protected route.

**Warning signs:** Login works with empty password; session survives logout.

### Pitfall 2: HTTPS→HTTP Redirect After Restart

**What goes wrong:** After panel restart behind reverse proxy, redirect goes to HTTP.

**Why it happens:** Scheme from request port instead of `X-Forwarded-Proto` (3x-ui #2857).

**How to avoid:** Respect `X-Forwarded-Proto`, `X-Forwarded-Host`; generate URLs from config, not request headers; test behind nginx/Caddy.

**Warning signs:** HTTPS login, then redirect to http:// after restart.

### Pitfall 3: sing-box Config Write Corruption

**What goes wrong:** Panel writes config while sing-box reads; partial write; crash.

**Why it happens:** No atomic write; no validation before replace.

**How to avoid:** Write to temp file; `sing-box check -c temp.json`; if OK, `os.Rename(temp, configPath)`; only then restart.

**Warning signs:** sing-box fails to start; config file truncated.

### Pitfall 4: Deprecated sing-box Config Fields

**What goes wrong:** Generated config has deprecated fields; `sing-box check` fails or future upgrade breaks.

**Why it happens:** sing-box frequently deprecates fields; panel uses outdated schema.

**How to avoid:** Run `sing-box check` before every apply; track [sing-box deprecated list](https://sing-box.sagernet.org/deprecated/); test against current stable.

**Warning signs:** `json: unknown field` or deprecation warnings.

### Pitfall 5: Listen Address Over-Exposure

**What goes wrong:** Panel listens on 0.0.0.0; exposed to internet when meant for localhost.

**How to avoid:** Default panel to 127.0.0.1; document reverse proxy for remote access.

---

## Code Examples

### sing-box Version Detection (Go)

```go
cmd := exec.Command("sing-box", "version")
output, err := cmd.Output()
if err != nil {
  return "", err
}
return strings.TrimSpace(string(output)), nil
```

### sing-box Check (Go)

```go
cmd := exec.Command("sing-box", "check", "-c", configPath)
output, err := cmd.CombinedOutput()
if err != nil {
  return "", fmt.Errorf("check failed: %w\n%s", err, output)
}
return string(output), nil
```

### Sonner Toast (React)

```tsx
// Source: sonner.emilkowal.ski/toast
import { toast } from "sonner"

toast.success("配置已应用")
toast.error("配置验证失败")
// Position: top-right per CONTEXT
<Toaster position="top-right" duration={4000} />
```

### shadcn Dialog for Config Error Modal

```tsx
// Source: ui.shadcn.com/docs/components/radix/dialog
<Dialog open={showError} onOpenChange={setShowError}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>配置验证失败</DialogTitle>
      <DialogDescription>
        <pre className="bg-muted p-4 rounded text-sm overflow-auto">
          {errorOutput}
        </pre>
      </DialogDescription>
    </DialogHeader>
  </DialogContent>
</Dialog>
```

### Tailwind Dark Theme Colors (CONTEXT: #1a1a2e / #111827)

```css
/* globals.css - override for Linear/GitHub Dark feel */
@layer base {
  .dark {
    --background: 222 47% 11%;   /* ~#111827 */
    --foreground: 210 40% 98%;
    --primary: 263 70% 58%;      /* purple accent */
    --primary-foreground: 210 40% 98%;
  }
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| CRA | Vite | CRA deprecated | HMR, ESM, Rollup |
| shadcn Toast | Sonner | shadcn deprecated toast | Use Sonner |
| gorilla/sessions | SCS | Lighter, store-agnostic | SCS v2 + SQLite |
| Gin | Chi | sing-box alignment | Chi v5 |

**Deprecated/outdated:**
- shadcn built-in Toast: Replaced by Sonner
- Create React App: Deprecated by React team

---

## Open Questions

1. **sing-box version output format**
   - What we know: `sing-box version` and `sing-box version -n` exist; output is version string
   - What's unclear: Exact format (e.g. "1.12.0" vs "sing-box 1.12.0"); parsing strategy
   - Recommendation: Use `-n` for clean output; regex or strings.TrimSpace; handle "not found" gracefully

2. **"Remember me" session implementation**
   - What we know: SCS has Lifetime, IdleTimeout, Cookie.Persist
   - What's unclear: Per-request override of Lifetime based on login form
   - Recommendation: Two session configs or single config with Cookie.Persist from form; when !remember: Persist=false (session cookie), IdleTimeout=24h; when remember: Persist=true, Lifetime=7d

3. **react-bits + shadcn compatibility**
   - What we know: react-bits has Tailwind variants; shadcn uses Tailwind
   - What's unclear: Tailwind v4 + shadcn new-york compatibility
   - Recommendation: Add react-bits components sparingly; verify in Phase 1; use for page transitions, card hover, modal animations

---

## Sources

### Primary (HIGH confidence)
- [ui.shadcn.com/docs/dark-mode/vite](https://ui.shadcn.com/docs/dark-mode/vite) — ThemeProvider, defaultTheme
- [ui.shadcn.com/docs/components/radix/sidebar](https://ui.shadcn.com/docs/components/radix/sidebar) — Sidebar structure
- [ui.shadcn.com/docs/components/radix/dialog](https://ui.shadcn.com/docs/components/radix/dialog) — Modal
- [ui.shadcn.com/docs/components/radix/sonner](https://ui.shadcn.com/docs/components/radix/sonner) — Sonner
- [sonner.emilkowal.ski/toast](https://sonner.emilkowal.ski/toast) — position, duration (top-right, 4000ms default)
- [pkg.go.dev/github.com/alexedwards/scs/v2](https://pkg.go.dev/github.com/alexedwards/scs/v2) — Lifetime, IdleTimeout, SQLite
- [pkg.go.dev/golang.org/x/crypto/bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt) — password hashing
- [sing-box.sagernet.org/installation/build-from-source](https://sing-box.sagernet.org/installation/build-from-source/) — build tags

### Secondary (MEDIUM confidence)
- [vpnrouter.homes/commands](https://vpnrouter.homes/commands/) — sing-box version -n
- .planning/research/STACK.md, ARCHITECTURE.md, PITFALLS.md — project research
- [DavidHDev/react-bits](https://github.com/DavidHDev/react-bits) — micro-interactions

### Tertiary (LOW confidence)
- Linear dark theme hex values (#1a1a2e, #111827) — CONTEXT reference; exact values need validation

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — project research + official docs
- Architecture: HIGH — ARCHITECTURE.md + sing-box config flow
- Pitfalls: HIGH — PITFALLS.md + 3x-ui issues

**Research date:** 2026-02-11
**Valid until:** 30 days (stable stack)
