---
phase: 04-user-management
plan: 03
subsystem: ui
tags: react, users, table, form, checkbox, popover, react-hook-form, zod, tanstack-query

# Dependency graph
requires:
  - phase: 04-user-management
    plan: 02
    provides: User REST API (GET/POST/PUT/DELETE /api/users)
  - phase: 02-inbound-management
    provides: Inbound API, InboundFormModal pattern
provides:
  - Users page with table, add/edit/delete
  - UserTable with row/header checkbox for batch select (Plan 04 wires actions)
  - UserFormModal with inbound multi-select, UUID/password copy when editing
  - Traffic/expire display per CONTEXT format
affects: 04-04 batch action bar

# Tech tracking
tech-stack:
  added: shadcn checkbox, popover, badge
  patterns: InboundFormModal/CertificateFormModal (Dialog + react-hook-form + zod)

key-files:
  created: web/src/pages/Users.tsx, web/src/components/users/UserTable.tsx, web/src/components/users/UserFormModal.tsx
  modified: web/src/components/layout/Sidebar.tsx, web/src/routes.tsx, web/src/components/ui/checkbox.tsx, web/src/components/ui/popover.tsx, web/src/components/ui/badge.tsx

key-decisions:
  - "Traffic limit form: user enters GB, converted to bytes for API"
  - "Expire_at: date input YYYY-MM-DD, send ISO midnight for API"

patterns-established:
  - "UserTable: header checkbox 全选当前页, row checkbox controlled by selectedIds"
  - "UserFormModal: Popover + Checkbox for inbound multi-select; UUID/password read-only with copy button when editing"

# Metrics
duration: ~8min
completed: 2026-02-12
---

# Phase 4 Plan 3: Users Page UI Summary

**Users page with table, UserFormModal (inbound multi-select), row/header checkbox for batch select**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-02-12
- **Completed:** 2026-02-12
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments

- Users page: useQuery ["users"] fetch /api/users, UserTable, Add button
- UserTable: columns 名称, 备注, 状态, 流量, 到期, 节点, 操作
- Status Badge: 启用/禁用/过期/超限 per CONTEXT
- Traffic format: "X / Y GB (N%)"; "无限制" when limit=0
- Expire format: YYYY-MM-DD; "永不过期" when null
- Header checkbox: 全选当前页; row checkbox for batch select (Plan 04 wires actions)
- UserFormModal: name, remark, inbound_ids (Popover+Checkbox), traffic_limit (GB), expire_at
- Form: UUID and Password read-only with copy button when editing
- Create: POST /api/users; Update: PUT /api/users/:id
- Sidebar: users nav enabled; routes: /users -> Users

## Task Commits

Each task was committed atomically:

1. **Task 1: Users page and UserTable** - `6f8727c` (feat)
2. **Task 2: UserFormModal** - `b3ea0a2` (feat)

## Self-Check: PASSED

- web/src/pages/Users.tsx: FOUND
- web/src/components/users/UserTable.tsx: FOUND
- web/src/components/users/UserFormModal.tsx: FOUND
- web npm run build: PASSED
- Commits 6f8727c, b3ea0a2: FOUND

## Files Created/Modified

- `web/src/pages/Users.tsx` - Users page with useQuery, Add button, edit/delete handlers
- `web/src/components/users/UserTable.tsx` - User table with checkbox, status badge, traffic/expire display
- `web/src/components/users/UserFormModal.tsx` - Create/edit form with inbound multi-select, UUID/password copy
- `web/src/components/layout/Sidebar.tsx` - Enable users nav
- `web/src/routes.tsx` - Add /users route
- `web/src/components/ui/checkbox.tsx` - shadcn checkbox
- `web/src/components/ui/popover.tsx` - shadcn popover
- `web/src/components/ui/badge.tsx` - shadcn badge

## Decisions Made

- Traffic limit: user enters GB in form; convert to bytes for API (0 = unlimited)
- Expire_at: date input returns YYYY-MM-DD; send ISO midnight (T23:59:59.999Z) for API

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- User CRUD from UI complete
- Batch checkbox ready for Plan 04
- ApplyConfig + Restart triggered on create/edit/delete via backend

---
*Phase: 04-user-management*
*Completed: 2026-02-12*
