# Feature Landscape: UI Polish & Core Management Optimization

**Domain:** Admin panel enhancement (proxy management)
**Researched:** 2026-02-19

## Table Stakes

Features users expect from a polished management panel. Missing = feels unfinished.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Animated number transitions on dashboard stats | Every modern dashboard animates stat changes — static numbers feel broken | Low | react-bits CountUp, already installed |
| Page/card entrance animations | Content appearing without animation feels like a loading glitch | Low | react-bits AnimatedContent + tw-animate-css |
| Download progress during core update | Currently shows spinner with no indication of progress or time remaining — feels frozen | Medium | SSE endpoint + Progress component |
| Explicit start/stop controls for sing-box | Current UI only has "Restart" — users need to stop without restarting (maintenance) | Low | Backend: `ProcessManager.Stop()`, Frontend: button |
| Status indicator with clear visual states | Current status dot is a plain colored circle — needs pulse animation for "running", clear distinction | Low | Tailwind `animate-pulse` + color transition |
| Error detail display for failed operations | Update/rollback errors show toast only — long error messages need detail view | Low | Already partially implemented (error modal exists for restart) — extend to update/rollback |

## Differentiators

Features that elevate beyond functional. Not expected, but valued.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Multi-step update visualization (Stepper) | Shows download → extract → verify → restart as discrete steps, not just a percentage | Medium | react-bits Stepper component |
| Card hover spotlight effect on status card | Makes the primary status card feel interactive and premium | Low | react-bits SpotlightCard wrapping shadcn Card |
| Animated version badge when update available | "New version" badge with subtle shimmer draws attention without being annoying | Low | react-bits ShinyText on Badge content |
| Staggered card entrance on Dashboard | Cards appear one after another instead of all at once — feels intentional | Low | tw-animate-css with incremental delay classes |
| Auto-start on panel boot | sing-box starts automatically when s-ui starts if config exists | Low | Backend: check on startup, call `ProcessManager.Start()` |
| Update to specific version | Currently updates to "latest" only — allow choosing from version list | Medium | Modify `Update()` to accept version parameter |

## Anti-Features

Features to explicitly NOT build in this milestone.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Heavy page transition animations (route transitions) | Admin panels are used repeatedly — route transitions add latency and annoy power users | Keep instant route switches; animate content within pages only |
| Background particle effects or animated backgrounds | This is a server management tool, not a landing page — react-bits has gorgeous backgrounds but they're inappropriate here | Use subtle card/text animations only |
| Cursor trail effects (BlobCursor, SplashCursor, etc.) | react-bits has these but they're distracting in a functional admin UI | Skip entirely |
| 3D card effects (TiltedCard, DecayCard) | Looks impressive in demos but makes an admin panel feel gimmicky | Use SpotlightCard for subtle hover effect only |
| Sound effects on interactions | Never appropriate for a server management panel | Skip entirely |
| WebSocket for status updates | Bidirectional communication is unnecessary — status only flows server→client | Use SSE for progress, keep polling for status |
| Real-time log streaming | Out of scope for this milestone — would require significant backend infrastructure | Defer to future milestone |

## Feature Dependencies

```
Progress component ──→ SSE endpoint ──→ Update progress visualization
                                    └──→ Stepper visualization

Start/Stop controls ──→ ProcessManager.Start() + ProcessManager.Stop()
                    └──→ Status indicator animation (needs to show transitions)

CountUp animation ──→ (none — independent, apply to existing Dashboard)

Card animations ──→ (none — independent, apply to existing pages)

Staggered entrance ──→ AnimatedContent component (react-bits)
```

## MVP Recommendation

**Must have (this milestone):**
1. Dashboard CountUp animations on stat cards
2. Card entrance animations (AnimatedContent) on all pages
3. Download progress bar during core update (SSE + Progress)
4. Explicit start/stop buttons on Core page
5. Animated status indicator (pulse when running)
6. Consistent card hover effects across all pages

**Should have:**
7. Multi-step Stepper visualization for update process
8. Staggered card entrance timing
9. SpotlightCard on status card

**Defer:**
- Update to specific version (requires API change, version selection UI)
- Auto-start on boot (operational concern, needs config option)

## Sources

- Codebase analysis: `web/src/pages/Dashboard.tsx`, `web/src/pages/Core.tsx`, `internal/core/updater.go`, `internal/core/process.go`
- react-bits component catalog: https://github.com/DavidHDev/react-bits/tree/main/src/ts-tailwind
- shadcn/ui components: https://ui.shadcn.com/docs/components
