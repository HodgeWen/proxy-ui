# Project Research Summary

**Project:** s-ui (sing-box proxy management panel)
**Domain:** Admin panel enhancement — UI polish & core management optimization
**Researched:** 2026-02-19
**Confidence:** HIGH

## Executive Summary

s-ui v1.1 is an enhancement milestone for an existing, functional proxy management panel. The codebase already contains every npm dependency needed — react-bits (v1.0.5) is installed but completely unused, tw-animate-css (v1.4.0) is imported, and the consolidated radix-ui (v1.4.3) includes the Progress primitive. The Go backend requires zero new modules; SSE uses stdlib `http.Flusher`. This is a "activate and wire up" milestone, not a "research and adopt" one. The risk of scope creep into heavy animation territory (route transitions, particle effects, 3D cards) must be actively resisted — this is a server management tool used repeatedly by power users.

The recommended approach splits work into two independent tracks that converge at the end: (1) UI animation polish using react-bits + tw-animate-css applied across existing pages, and (2) core management improvements adding explicit Start/Stop controls and SSE-based update progress. The animation track is low-risk and can proceed page-by-page. The core management track carries the milestone's only critical risks: concurrent update corruption (needs mutex), SSE connection leaks (needs useEffect cleanup), and reverse proxy buffering (needs `X-Accel-Buffering: no` header). All three have straightforward mitigations documented in PITFALLS.md.

The key architectural decision is keeping SSE strictly for the update progress flow while retaining TanStack Query's 5-second polling for status. SSE adds complexity — connection lifecycle, proxy compatibility, cleanup — and that complexity is only justified where sub-second granularity matters (download progress). Status changes are infrequent enough that polling is the correct pattern.

## Key Findings

### Recommended Stack

No new npm packages or Go modules. The entire milestone uses already-installed dependencies that need activation, plus one new shadcn/ui component wrapper (Progress).

**Core technologies:**
- **react-bits** (v1.0.5, installed): Animation primitives — CountUp, AnimatedContent, SpotlightCard, Stepper — all via shadcn CLI install of copy-paste components
- **tw-animate-css** (v1.4.0, imported): `animate-in`/`animate-out` utility classes for card entrance animations and staggered timing
- **radix-ui Progress** (v1.4.3, installed): Accessible progress bar for download tracking — just needs `bunx shadcn@latest add progress`
- **Native EventSource API**: Browser-native SSE client for update progress — no library needed, works with cookie auth
- **Go stdlib http.Flusher**: Server-sent events for real-time progress during core update — zero new Go dependencies

See [STACK.md](STACK.md) for version verification, installation commands, and rejected alternatives.

### Expected Features

**Must have (table stakes):**
- Animated number transitions on dashboard stats (CountUp)
- Card entrance animations across all pages (AnimatedContent + tw-animate-css)
- Download progress bar during core update (SSE + Progress)
- Explicit start/stop controls for sing-box process
- Animated status indicator (pulse when running, clear state distinction)
- Error detail display for failed update/rollback operations

**Should have (differentiators):**
- Multi-step Stepper visualization for update process (download → extract → verify → restart)
- SpotlightCard hover effect on Core status card
- Staggered card entrance timing on Dashboard

**Defer to future milestone:**
- Update to specific version (requires version selection UI, API parameter change)
- Auto-start on panel boot (operational concern, needs config option)
- Real-time log streaming (significant backend infrastructure)

See [FEATURES.md](FEATURES.md) for dependency map, anti-features list, and MVP rationale.

### Architecture Approach

The architecture adds three elements to the existing stack: an **Animation Layer** (react-bits + tw-animate-css wrapping existing shadcn/ui components), a **native EventSource** client for SSE update progress, and **three new backend endpoints** (Start, Stop, UpdateSSE). TanStack Query remains the server state manager for all non-streaming concerns.

**Major components:**
1. **Animation Layer** — Presentational wrappers around existing shadcn/ui components; no API communication, no state management
2. **ProcessManager extensions** — `Start()` and `Stop()` methods added to existing Go struct; `Start()` starts without killing first, `Stop()` stops without restarting
3. **UpdateSSEHandler** — New SSE endpoint at `GET /api/core/update/stream` with mutex protection, step-based progress events, and context cancellation for the download phase
4. **useUpdateStream hook** — React hook managing EventSource lifecycle, progress state, and TanStack Query cache invalidation on completion

See [ARCHITECTURE.md](ARCHITECTURE.md) for data flow diagrams, code patterns, and anti-patterns.

### Critical Pitfalls

1. **Concurrent update corruption** — No mutex on current `UpdateHandler`; two simultaneous updates corrupt the binary. Fix: `sync.Mutex` with `TryLock()`, return HTTP 409 if locked.
2. **SSE connection leaks** — EventSource stays open if user navigates away mid-update. Fix: close in `useEffect` cleanup, store reference in `useRef`.
3. **Reverse proxy buffering** — Nginx/Caddy buffer SSE events by default, defeating real-time progress. Fix: set `X-Accel-Buffering: no` header, document `proxy_buffering off` for Nginx.
4. **Progress bar stuck at 0%** — GitHub API fetch takes 1-3s before download starts; bar sits at 0%. Fix: use step model with indeterminate state for non-download steps.
5. **react-bits dark mode colors** — Components may use hardcoded colors instead of CSS variables. Fix: review each generated component file after CLI install, replace with project's CSS variable classes.

See [PITFALLS.md](PITFALLS.md) for all 11 pitfalls with phase-specific warnings.

## Implications for Roadmap

Based on research, the milestone splits into 4 phases with clear dependency ordering:

### Phase 1: UI Animation Foundation
**Rationale:** Zero backend dependencies, low risk, immediately visible results. Establishes the animation patterns that later phases (like update progress UI) build on. Gets react-bits activated and validated before using it in more complex contexts.
**Delivers:** Polished dashboard with animated stats, entrance animations on all pages, hover effects, status indicator animation.
**Features addressed:** CountUp on stats, AnimatedContent on pages, staggered card entrance, status indicator pulse, consistent hover effects.
**Avoids:** Pitfall 4 (style conflicts) — catch early by testing dark mode for each component; Pitfall 5 (jank) — set reasonable durations, prefer GPU-composited transforms; Pitfall 9 (re-render animation) — only animate on mount.
**Stack:** react-bits (CountUp, AnimatedContent, SpotlightCard), tw-animate-css classes, Tailwind v4 transitions.

### Phase 2: Core Process Control
**Rationale:** Adds Start/Stop before the SSE update flow because the update flow's final step is a restart — the ProcessManager needs clean start/stop separation first. Also delivers user-facing value (explicit controls) independently of the more complex SSE work.
**Delivers:** Separate Start and Stop buttons on Core page, improved status detection, clear process state transitions in the UI.
**Features addressed:** Explicit start/stop controls, status indicator with state transitions.
**Avoids:** Pitfall 8 (pgrep false negatives) — keep existing approach, sufficient for controlled environment.
**Stack:** Go ProcessManager extensions (Start, Stop), new API endpoints, TanStack Query mutations.

### Phase 3: Update Progress System
**Rationale:** Most complex phase — SSE endpoint, progress tracking, multi-step visualization, error handling. Depends on Phase 2 (restart capability) and Phase 1 (animation components for Stepper/Progress UI). Contains the milestone's three critical pitfalls, all addressable with documented mitigations.
**Delivers:** Real-time update progress with step visualization, download percentage, error display, and graceful completion.
**Features addressed:** Download progress bar, Stepper visualization, error detail display.
**Avoids:** Pitfall 1 (concurrent updates) — mutex with TryLock; Pitfall 2 (SSE leaks) — useEffect cleanup; Pitfall 3 (disconnect) — context cancellation for download phase; Pitfall 6 (proxy buffering) — X-Accel-Buffering header; Pitfall 11 (stuck at 0%) — step model with indeterminate states.
**Stack:** Go SSE handler (stdlib), EventSource API, shadcn/ui Progress, react-bits Stepper, sonner for error toasts.

### Phase 4: Polish & Consistency Pass
**Rationale:** Final pass after all functional work is complete. Ensures consistent spacing, colors, animation timing across all pages. Catches dark mode issues that slipped through Phase 1. Low-risk finishing work.
**Delivers:** Visually consistent panel across all pages, verified dark mode, refined animation timing, documentation for proxy configuration.
**Features addressed:** Remaining spacing/color inconsistencies, dark mode verification, animation timing refinement.
**Avoids:** Pitfall 10 (dark mode colors) — systematic review of all react-bits components in dark mode.

### Phase Ordering Rationale

- **Phase 1 → 2:** Independent tracks, but Phase 1 first because it validates react-bits integration patterns used in Phase 3's Stepper UI
- **Phase 2 → 3:** Phase 3's update flow ends with a restart — needs Phase 2's clean Start/Stop separation
- **Phase 3 → 4:** Polish pass must come after all components are in place to assess consistency holistically
- **Phases 1 and 2 could technically run in parallel** since they touch different parts of the stack (frontend animation vs. backend process control), but sequential ordering reduces context-switching and ensures Phase 1's patterns inform Phase 3's UI work

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3 (Update Progress):** SSE implementation details, Go `http.ResponseController` vs `http.Flusher`, EventSource error handling edge cases, proxy configuration documentation. The critical pitfalls cluster here — worth a focused research pass before implementation.

Phases with standard patterns (skip research-phase):
- **Phase 1 (UI Animation):** react-bits components have clear documentation, install via CLI, copy-paste with adjustment. Well-trodden path.
- **Phase 2 (Process Control):** Straightforward Go process management. `exec.Command` + signal handling is well-documented stdlib territory.
- **Phase 4 (Polish):** No research needed — this is a review-and-refine pass.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All packages verified in package.json/bun.lock; versions confirmed; react-bits GitHub active (35.9K stars, pushed 2026-02-14) |
| Features | HIGH | Based on direct codebase analysis of existing pages and components; feature scope is well-bounded |
| Architecture | HIGH | Patterns verified against existing code structure; SSE is stdlib Go; EventSource is browser-native |
| Pitfalls | HIGH | Critical pitfalls (mutex, SSE cleanup, proxy buffering) are well-documented problems with known solutions |

**Overall confidence:** HIGH

### Gaps to Address

- **react-bits component quality in practice:** While react-bits is well-maintained and popular, the specific components (CountUp, SpotlightCard, Stepper) haven't been tested in this codebase. Phase 1 serves as validation — if components don't meet quality bar after CSS variable adjustment, fall back to hand-rolled equivalents using tw-animate-css.
- **SSE through s-ui's specific reverse proxy setup:** The PITFALLS.md documents the general proxy buffering issue, but the specific proxy configuration users run (Nginx, Caddy, or direct) should be validated. The `X-Accel-Buffering: no` header is the primary mitigation.
- **EventSource browser support on mobile admin access:** EventSource is supported in all modern browsers, but users accessing the panel from older mobile browsers should be considered. Fallback to polling if EventSource is unavailable (graceful degradation).

## Sources

### Primary (HIGH confidence)
- react-bits GitHub (35.9K stars): https://github.com/DavidHDev/react-bits — component catalog, TS-TW variants
- tw-animate-css (5.8M weekly downloads): https://github.com/Wombosvideo/tw-animate-css — utility class reference
- shadcn/ui Progress: https://ui.shadcn.com/docs/components/progress — Radix primitive wrapper
- Radix UI consolidated package: https://www.npmjs.com/package/radix-ui — v1.4.3 includes Progress
- Go SSE stdlib: https://freecodecamp.org/news/how-to-implement-server-sent-events-in-go — verified implementation pattern
- MDN EventSource API: https://developer.mozilla.org/en-US/docs/Web/API/EventSource — browser-native SSE client
- motion v12.34.0: https://www.npmjs.com/package/motion — verified for "not recommended" rationale

### Secondary (MEDIUM confidence)
- TanStack Query + SSE integration: https://ollioddi.dev/blog/tanstack-sse-guide — community pattern, not official TanStack guidance
- Nginx SSE proxy configuration: Nginx `proxy_buffering` documentation — standard but depends on user's setup

### Tertiary (LOW confidence)
- react-bits dark mode behavior: Inferred from component variant structure — needs validation during Phase 1

---
*Research completed: 2026-02-19*
*Ready for roadmap: yes*
