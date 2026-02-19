# Roadmap: s-ui

## Milestones

- ✅ **v1.0 MVP** — Phases 1-10 (shipped 2026-02-19) → [archive](milestones/v1.0-ROADMAP.md)
- 🚧 **v1.1 UI与核心优化** — Phases 11-14 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-10) — SHIPPED 2026-02-19</summary>

- [x] Phase 1: Foundation (3/3 plans) — completed 2026-02-11
- [x] Phase 2: Inbound Management (6/6 plans) — completed 2026-02-11
- [x] Phase 3: Certificate Management (3/3 plans) — completed 2026-02-12
- [x] Phase 4: User Management (4/4 plans) — completed 2026-02-12
- [x] Phase 5: Subscription System (3/3 plans) — completed 2026-02-12
- [x] Phase 6: Traffic Statistics (3/3 plans) — completed 2026-02-12
- [x] Phase 7: sing-box Core Management (2/2 plans) — completed 2026-02-12
- [x] Phase 8: Deployment & Production (4/4 plans) — completed 2026-02-12
- [x] Phase 9: Certificate Config Sync (3/3 plans) — completed 2026-02-12
- [x] Phase 10: Sidebar Pages & Config Fix (3/3 plans) — completed 2026-02-19

</details>

### 🚧 v1.1 UI与核心优化 (In Progress)

**Milestone Goal:** 全局润色 UI 提升视觉体验，优化核心管理的启动、状态检测和更新逻辑

- [ ] **Phase 11: UI Animation Foundation** - Activate react-bits animations across all pages for polished visual feedback
- [ ] **Phase 12: Core Process Control** - Explicit start/stop/restart with 4-state status API and context-aware UI
- [ ] **Phase 13: Core Update Progress** - SSE real-time download progress with concurrent update protection
- [ ] **Phase 14: UI Polish & Consistency** - Global spacing/color audit and animated status indicators

## Phase Details

### Phase 11: UI Animation Foundation
**Goal**: Pages feel alive with smooth animations and interactive hover feedback
**Depends on**: Phase 10
**Requirements**: UIANM-01, UIANM-02, UIANM-03, UIANM-04
**Success Criteria** (what must be TRUE):
  1. Dashboard statistics animate from 0 to their actual values on page load (CountUp)
  2. Cards across all pages fade/slide in on first render with no layout shift (AnimatedContent + tw-animate-css)
  3. Hovering over the core status card produces a spotlight glow that follows the cursor (SpotlightCard)
  4. Buttons, cards, and table rows share consistent hover transitions with uniform timing and visual feedback
**Plans**: TBD

### Phase 12: Core Process Control
**Goal**: Users have clear, accurate control over sing-box core lifecycle across all states
**Depends on**: Phase 11
**Requirements**: CORE-01, CORE-02, CORE-03, CORE-04
**Success Criteria** (what must be TRUE):
  1. Status API returns four distinct states: not_installed / stopped / running / error — frontend renders different UI for each
  2. When core is running, user sees Stop and Restart; when stopped, user sees Start; when not installed, user sees a download/install prompt
  3. If core binary is missing at panel startup, core does not auto-start and the UI guides user to install
  4. After a failed start, the UI shows contextually correct actions (Retry Start, View Logs) instead of misleading "Restart"
**Plans**: TBD

### Phase 13: Core Update Progress
**Goal**: Users see real-time download progress during core updates with protection against duplicate triggers
**Depends on**: Phase 12
**Requirements**: UPDT-01, UPDT-02
**Success Criteria** (what must be TRUE):
  1. During core update, a progress bar fills in real-time reflecting actual download percentage via SSE
  2. Clicking Update while an update is already in progress returns HTTP 409 and does not start a duplicate download
  3. SSE stream includes X-Accel-Buffering header and cleans up on navigation (no connection leaks)
**Plans**: TBD

### Phase 14: UI Polish & Consistency
**Goal**: Every page has consistent visual treatment and dynamic status indicators match core state
**Depends on**: Phase 11, 12, 13
**Requirements**: UICON-01, UICON-02
**Success Criteria** (what must be TRUE):
  1. Spacing (padding, margins, gaps) is uniform across all pages — no page looks cramped or loose relative to others
  2. Color variables are consistent across light and dark themes with no mismatched grays, borders, or backgrounds
  3. Core status indicator is animated: running = green pulse, stopped = gray, not installed = yellow, error = red blink — each state immediately distinguishable
**Plans**: TBD

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 3/3 | ✓ Complete | 2026-02-11 |
| 2. Inbound Management | v1.0 | 6/6 | ✓ Complete | 2026-02-11 |
| 3. Certificate Management | v1.0 | 3/3 | ✓ Complete | 2026-02-12 |
| 4. User Management | v1.0 | 4/4 | ✓ Complete | 2026-02-12 |
| 5. Subscription System | v1.0 | 3/3 | ✓ Complete | 2026-02-12 |
| 6. Traffic Statistics | v1.0 | 3/3 | ✓ Complete | 2026-02-12 |
| 7. sing-box Core Management | v1.0 | 2/2 | ✓ Complete | 2026-02-12 |
| 8. Deployment & Production | v1.0 | 4/4 | ✓ Complete | 2026-02-12 |
| 9. Certificate Config Sync | v1.0 | 3/3 | ✓ Complete | 2026-02-12 |
| 10. Sidebar Pages & Config Fix | v1.0 | 3/3 | ✓ Complete | 2026-02-19 |
| 11. UI Animation Foundation | v1.1 | 0/? | Not started | - |
| 12. Core Process Control | v1.1 | 0/? | Not started | - |
| 13. Core Update Progress | v1.1 | 0/? | Not started | - |
| 14. UI Polish & Consistency | v1.1 | 0/? | Not started | - |
