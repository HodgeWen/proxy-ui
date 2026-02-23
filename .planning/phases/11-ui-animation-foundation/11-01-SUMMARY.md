---
phase: 11-ui-animation-foundation
plan: 01
subsystem: ui
tags: [react, animation, rAF, countup, spotlight, tw-animate-css]

requires: []
provides:
  - "useCountUp hook with rAF animation loop, ease-out cubic, Intl.NumberFormat formatting"
  - "SpotlightCard component with cursor-tracking radial gradient overlay"
  - "Dashboard and Traffic pages with animated stat numbers and entrance animations"
affects: [14-ui-polish-consistency]

tech-stack:
  added: []
  patterns: [rAF-based number animation hook, ref-based mouse tracking to avoid re-renders, tw-animate-css entrance animations with stagger]

key-files:
  created:
    - web/src/hooks/use-count-up.ts
    - web/src/components/ui/spotlight-card.tsx
  modified:
    - web/src/pages/Dashboard.tsx
    - web/src/pages/Traffic.tsx

key-decisions:
  - "Used Intl.NumberFormat for per-frame number formatting during animation"
  - "SpotlightCard uses refs instead of state for mouse tracking to avoid re-renders"
  - "Dark mode detection via document.documentElement.classList for spotlight color"

patterns-established:
  - "useCountUp pattern: startWhen gate + re-animate on value change for TanStack Query refetch"
  - "Staggered entrance: animate-in fade-in zoom-in-95 with 75ms delay per card"
  - "SpotlightCard wrapping pattern: SpotlightCard > Card for hover glow effect"

requirements-completed: [UIANM-01, UIANM-03]

duration: 2min
completed: 2026-02-23
---

# Phase 11 Plan 01: Animation Primitives & Page Integration Summary

**useCountUp hook with rAF ease-out cubic animation and SpotlightCard cursor-tracking glow, integrated into Dashboard and Traffic pages with staggered entrance animations**

## Performance

- **Duration:** 2 min
- **Started:** 2026-02-23T03:54:29Z
- **Completed:** 2026-02-23T03:56:31Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Created useCountUp hook with requestAnimationFrame loop, ease-out cubic easing, per-frame Intl.NumberFormat, prefers-reduced-motion support, and re-animation on value change
- Created SpotlightCard component with ref-based mouse tracking (no re-renders), dark/light mode aware opacity, 300ms fade transition
- Dashboard: all 3 stat cards wrapped in SpotlightCard, numbers animate with CountUp (including formatBytes for traffic), staggered entrance at 75ms intervals
- Traffic: all stat numbers animate with CountUp, staggered card entrance, Tabs section entrance at 225ms delay

## Task Commits

Each task was committed atomically:

1. **Task 1: Create useCountUp hook and SpotlightCard component** - `15bfec3` (feat)
2. **Task 2: Integrate animations into Dashboard and Traffic pages** - `fb85edf` (feat)

## Files Created/Modified
- `web/src/hooks/use-count-up.ts` - Custom hook: rAF animation with ease-out cubic, formatted output, reduced-motion skip
- `web/src/components/ui/spotlight-card.tsx` - Wrapper component: cursor-following radial gradient overlay, dark mode aware
- `web/src/pages/Dashboard.tsx` - CountUp on all stats, SpotlightCard on all 3 cards, staggered fade+scale entrance
- `web/src/pages/Traffic.tsx` - CountUp on all stats (including sub-text values), staggered card + Tabs entrance

## Decisions Made
- Used `Intl.NumberFormat` for per-frame formatting during animation to ensure comma separators are visible throughout
- SpotlightCard tracks mouse via refs and direct DOM style mutation instead of React state to avoid re-renders on every mousemove
- Dark mode detected via `document.documentElement.classList.contains('dark')` on mouseenter rather than a React context — simpler and works with the existing theme toggle
- Traffic bytes animate the raw number through useCountUp, then pass `value` through `formatBytes()` on each render for real-time formatted animation

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- useCountUp and SpotlightCard are reusable primitives available for Plan 02 and Phase 14
- Entrance animation pattern (tw-animate-css classes with stagger delay) established for consistency across remaining pages

## Self-Check: PASSED

- All 4 files verified on disk
- Commits `15bfec3` and `fb85edf` confirmed in git log

---
*Phase: 11-ui-animation-foundation*
*Completed: 2026-02-23*
