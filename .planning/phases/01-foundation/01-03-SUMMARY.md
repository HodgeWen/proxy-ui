---
phase: 01-foundation
plan: 03
subsystem: ui
tags: [shadcn, react, tailwind, sonner, tanstack-query, dark-theme]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: sing-box core API (status, restart), auth
provides:
  - Dark theme UI with ThemeProvider, shadcn components
  - Fixed sidebar with full nav (仪表盘 active, 6 disabled placeholders)
  - Dashboard with sing-box status card, restart button, stats placeholder
  - Toast (Sonner) and Dialog for feedback
affects: [02-inbound, 03-cert, 04-user, 05-subscription, 06-traffic, 07-core]

# Tech tracking
tech-stack:
  added: [shadcn/ui, sonner, react-bits, @ alias]
  patterns: [ThemeProvider dark default, SidebarProvider + SidebarInset, TanStack Query for API]

key-files:
  created: [web/src/components/theme-provider.tsx, web/src/components/layout/Sidebar.tsx, web/src/components/layout/AppLayout.tsx]
  modified: [web/src/index.css, web/src/main.tsx, web/src/routes.tsx, web/src/pages/Dashboard.tsx]

key-decisions:
  - "Custom ThemeProvider (no next-themes) with defaultTheme=dark, storageKey=s-ui-theme"
  - "Vite proxy /api -> localhost:8080 for dev"
  - "Config check error Modal opens when error is long/multi-line (heuristic)"

patterns-established:
  - "Dark theme: oklch colors #1a1a2e/#111827, purple accent, borderless cards"
  - "Dashboard: useQuery status + useMutation restart, toast/Modal feedback"

# Metrics
duration: 5min
completed: 2026-02-11
---

# Phase 01 Plan 03: UI Foundation Summary

**暗色主题管理面板：shadcn/ui + ThemeProvider + 固定侧边栏 + 仪表盘（sing-box 状态、重启、Toast/Modal 反馈）**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-02-11T08:50:41Z
- **Completed:** 2026-02-11T08:55:20Z
- **Tasks:** 3
- **Files modified:** 20+

## Accomplishments

- ThemeProvider with defaultTheme="dark", storageKey="s-ui-theme"
- shadcn init (new-york) + sidebar, dialog, card, skeleton, button, sonner
- Fixed left Sidebar: 仪表盘(active), 入站/用户/订阅/证书/流量/核心 (disabled)
- AppLayout: Sidebar + main content via Outlet
- Dashboard: sing-box status card (running/stopped, version), restart button with loading
- Stats placeholder: 入站数=0, 用户数=0
- Toast (Sonner) top-right, duration 4s; Dialog for check error output
- Vite proxy /api for dev

## Task Commits

Each task was committed atomically:

1. **Task 1: UI foundation (ThemeProvider, shadcn, Sidebar)** - `3688c6d` (feat)
2. **Task 2: Dashboard (sing-box status + restart)** - `fe8434b` (feat)
3. **Task 3: Feedback polish (Toast, Modal, Loading)** - `66f941b` (chore)

## Files Created/Modified

- `web/src/components/theme-provider.tsx` - Custom ThemeProvider, dark default
- `web/src/components/layout/Sidebar.tsx` - Fixed nav with 7 items
- `web/src/components/layout/AppLayout.tsx` - Sidebar + main content
- `web/src/index.css` - Dark theme oklch variables (#1a1a2e, purple accent)
- `web/src/main.tsx` - ThemeProvider, Toaster
- `web/src/routes.tsx` - AppLayout with Dashboard child
- `web/src/pages/Dashboard.tsx` - Status card, restart, stats, Toast/Modal
- `web/vite.config.ts` - @ alias, /api proxy
- `web/components.json` - shadcn config
- `web/src/components/ui/*` - shadcn components

## Decisions Made

- Custom ThemeProvider instead of next-themes (simpler, no extra dep)
- Error Modal opens when restart/config error is long or multi-line (heuristic for check output)
- Phase 1 desktop only; sidebar collapsible="none" (always visible)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- shadcn init required @ alias in tsconfig.json; added baseUrl/paths and vite resolve.alias
- next-themes added by shadcn sonner; replaced with custom useTheme, removed next-themes

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Dashboard ready for inbound/user management features
- Sidebar placeholders ready to wire to future routes
- Toast/Modal pattern established for config operations

## Self-Check: PASSED

- ThemeProvider, Sidebar, AppLayout, Dashboard exist
- Commits 3688c6d, fe8434b, 66f941b verified

---
*Phase: 01-foundation*
*Completed: 2026-02-11*
