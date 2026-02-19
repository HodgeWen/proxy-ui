---
phase: 02-inbound-management
plan: 04
subsystem: ui
tags: [react, shadcn, table, dropdown-menu, useQuery, inbounds]

# Dependency graph
requires:
  - phase: 02-03
    provides: GET /api/inbounds REST API
provides:
  - Inbounds list page with table layout
  - Edit button inline, Delete in DropdownMenu per CONTEXT
  - Sidebar 入站 link navigates to /inbounds
affects: [02-05]

# Tech tracking
tech-stack:
  added: [shadcn table, shadcn dropdown-menu]
  patterns: [useQuery for API fetch, InboundTable with onEdit/onDelete callbacks]

key-files:
  created: [web/src/pages/Inbounds.tsx, web/src/components/inbounds/InboundTable.tsx, web/src/components/ui/table.tsx, web/src/components/ui/dropdown-menu.tsx]
  modified: [web/src/routes.tsx, web/src/components/layout/Sidebar.tsx]

key-decisions:
  - "Edit inline (Pencil icon), Delete in dropdown per CONTEXT"
  - "onEdit/onDelete no-op for now; Plan 05 wires to form modal and delete flow"

patterns-established:
  - "InboundTable: onEdit(id), onDelete(id) callbacks for parent control"

# Metrics
duration: 1min
completed: 2026-02-11
---

# Phase 02 Plan 04: Inbound List Page Summary

**Inbound list page with table layout, Edit inline, Delete in dropdown menu, and sidebar 入站 enabled**

## Performance

- **Duration:** ~1 min
- **Started:** 2026-02-11T10:09:44Z
- **Completed:** 2026-02-11T10:10:51Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Inbounds page with useQuery fetching GET /api/inbounds
- InboundTable with columns: 标签, 协议, 端口, TLS, 传输, 监听地址, 用户数, 创建时间, 操作
- Edit button (Pencil icon) inline per row
- DropdownMenu with 删除 item (Trash2 icon)
- Sidebar 入站 enabled — navigates to /inbounds
- /inbounds route added under AppLayout

## Task Commits

Each task was committed atomically:

1. **Task 1: Inbounds page, route, and shadcn table/dropdown-menu** - `4f52d0b` (feat)
2. **Task 2: Table actions and sidebar** - `2fb1a70` (feat)

**Plan metadata:** (pending docs commit)

## Files Created/Modified
- `web/src/pages/Inbounds.tsx` - Inbound list page with useQuery, loading/error states
- `web/src/components/inbounds/InboundTable.tsx` - Table with Edit, DropdownMenu
- `web/src/components/ui/table.tsx` - shadcn table component
- `web/src/components/ui/dropdown-menu.tsx` - shadcn dropdown-menu component
- `web/src/routes.tsx` - /inbounds route
- `web/src/components/layout/Sidebar.tsx` - 入站 nav enabled (removed disabled)

## Decisions Made
- Edit inline, Delete in dropdown per CONTEXT
- onEdit/onDelete passed as no-ops for Plan 05 to wire

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Inbound list page ready; Plan 05 will wire onEdit to form modal, onDelete to delete flow

## Self-Check: PASSED

---
*Phase: 02-inbound-management*
*Completed: 2026-02-11*
