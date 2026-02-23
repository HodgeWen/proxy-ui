---
phase: 11-ui-animation-foundation
plan: 02
subsystem: ui
tags: [tailwindcss, tw-animate-css, card, entrance-animation, hover-transition]

requires:
  - phase: 11-ui-animation-foundation
    provides: tw-animate-css installed and configured (plan 01 dependency chain)
provides:
  - Card base hover border glow transition (affects all Card instances app-wide)
  - Entrance animations on Core, Inbounds, Users, Certificates, Subscriptions pages
affects: [14-ui-polish-consistency]

tech-stack:
  added: []
  patterns: [tw-animate-css entrance classes with staggered delays, motion-reduce accessibility]

key-files:
  created: []
  modified:
    - web/src/components/ui/card.tsx
    - web/src/pages/Core.tsx
    - web/src/pages/Inbounds.tsx
    - web/src/pages/Certificates.tsx
    - web/src/pages/Users.tsx
    - web/src/pages/Subscriptions.tsx

key-decisions:
  - "Card hover uses border-ring/50 for subtle theme-aware border glow"
  - "Entrance animations use animate-in fade-in zoom-in-95 with 75ms stagger between sections"
  - "fill-mode-both prevents flash of invisible content before animation starts"
  - "BatchActionBar excluded from entrance animation to preserve its conditional show/hide behavior"

patterns-established:
  - "Entrance animation: animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none"
  - "Stagger pattern: 75ms increments between logical page sections"
  - "Accessibility: motion-reduce:animate-none on all entrance animations"

requirements-completed: [UIANM-02, UIANM-04]

duration: 2min
completed: 2026-02-23
---

# Phase 11 Plan 02: Card Hover & Page Entrance Animations Summary

**Card hover border glow transition on base component + staggered fade+scale entrance animations across all 5 non-Dashboard pages using tw-animate-css**

## Performance

- **Duration:** 2 min
- **Started:** 2026-02-23T03:54:28Z
- **Completed:** 2026-02-23T03:56:19Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Card base component now has `transition-colors duration-150 hover:border-ring/50` providing consistent hover feedback across the entire app
- Core page cards fade+scale in with staggered timing (0ms, 75ms)
- Inbounds, Certificates, Users, Subscriptions pages all have staggered entrance animations on their logical content sections
- All entrance animations respect `prefers-reduced-motion` via `motion-reduce:animate-none`

## Task Commits

Each task was committed atomically:

1. **Task 1: Card hover transition + Core/Inbounds/Certificates entrance** - `aba9268` (feat)
2. **Task 2: Users and Subscriptions entrance animations** - `0f82341` (feat)

## Files Created/Modified
- `web/src/components/ui/card.tsx` - Added hover border glow transition to Card base
- `web/src/pages/Core.tsx` - Wrapped both cards with entrance animation divs
- `web/src/pages/Inbounds.tsx` - Header and table sections with staggered entrance
- `web/src/pages/Certificates.tsx` - Header and table sections with staggered entrance
- `web/src/pages/Users.tsx` - Heading, search bar, and table with 3-step stagger (0/75/150ms)
- `web/src/pages/Subscriptions.tsx` - Heading and table with staggered entrance

## Decisions Made
- Used `border-ring/50` (theme ring color at 50% opacity) for Card hover — adapts to both light and dark themes
- Used `fill-mode-both` to prevent elements from being invisible before animation starts, eliminating layout shift
- Users page gets 3 stagger steps (0/75/150ms) due to having 3 distinct content sections; other pages use 2 steps
- BatchActionBar on Users page excluded from entrance animation since it has conditional visibility logic

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All 5 non-Dashboard/Traffic pages now have consistent entrance animations
- Card hover transition applies globally to every Card instance in the app
- Dashboard already has its own hover handling (transition-shadow) from v1.0; Plan 01 handles Dashboard entrance separately
- Ready for Phase 14 (UI Polish & Consistency) refinement pass

## Self-Check: PASSED

- All 6 modified files exist on disk
- Commit `aba9268` found in git log (Task 1)
- Commit `0f82341` found in git log (Task 2)
- Build succeeds with zero errors

---
*Phase: 11-ui-animation-foundation*
*Completed: 2026-02-23*
