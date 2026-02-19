# Technology Stack: UI Polish & Core Management Optimization

**Project:** s-ui (sing-box proxy management panel)
**Researched:** 2026-02-19
**Milestone:** UI polish + core management optimization (subsequent milestone)

## TL;DR — Zero New npm Dependencies Required

The existing stack already contains everything needed. **react-bits** (v1.0.5) is installed but completely unused — it provides all animation primitives. **tw-animate-css** (v1.4.0) is imported and handles shadcn/ui enter/exit animations. **radix-ui** (v1.4.3, consolidated package) includes `Progress` — just add the shadcn wrapper component. The Go backend needs no new modules; SSE uses stdlib `http.Flusher`.

---

## Currently Installed (Verified from package.json)

| Package | Version | Status | Relevance to This Milestone |
|---------|---------|--------|---------------------------|
| `react-bits` | ^1.0.5 | **Installed, ZERO imports** | Primary animation library — needs activation |
| `tw-animate-css` | ^1.4.0 | Imported in `index.css` | Enter/exit animations for shadcn components |
| `radix-ui` | ^1.4.3 | Active (Dialog, Select, etc.) | Includes Progress primitive — needs wrapper |
| `@tanstack/react-query` | ^5.90.20 | Active | Status polling, mutation state — extend for SSE |
| `tailwindcss` | ^4.1.18 | Active | Native transitions, custom `@keyframes` |
| `sonner` | ^2.0.7 | Active | Toast notifications — already handles errors |
| `lucide-react` | ^0.563.0 | Active | Icons — has all needed process state icons |

## Recommended Stack Additions

### New shadcn/ui Components to Add (0 new npm packages)

The consolidated `radix-ui` v1.4.3 already includes the Progress primitive. Only the shadcn wrapper component file needs to be added.

| Component | Install Command | Purpose | Why |
|-----------|----------------|---------|-----|
| Progress | `bunx shadcn@latest add progress` | Download progress bar during core update | Radix `Progress` is already in `radix-ui` v1.4.3; this just adds the styled `web/src/components/ui/progress.tsx` wrapper |

**Verification:** The project uses `import { Dialog as DialogPrimitive } from "radix-ui"` pattern (see `dialog.tsx` line 3). Progress follows the same pattern: `import { Progress as ProgressPrimitive } from "radix-ui"`.

### react-bits Components to Use (already installed v1.0.5)

Install individual components via shadcn CLI. Each is tree-shakeable, copy-paste, TS+Tailwind variant.

| Component | Category | Use Case in s-ui | Install |
|-----------|----------|-----------------|---------|
| **CountUp** | TextAnimations | Dashboard stat cards (inbound count, user count, traffic totals) — animated number counting on load/change | `bunx shadcn@latest add @react-bits/CountUp-TS-TW` |
| **AnimatedContent** | Animations | Wrap page sections for entrance animations on mount — cards, tables, forms | `bunx shadcn@latest add @react-bits/AnimatedContent-TS-TW` |
| **FadeContent** | Animations | Lighter alternative to AnimatedContent for simple fade-in transitions | `bunx shadcn@latest add @react-bits/FadeContent-TS-TW` |
| **SpotlightCard** | Components | Status card on Core page — hover spotlight effect on the main sing-box status card | `bunx shadcn@latest add @react-bits/SpotlightCard-TS-TW` |
| **ShinyText** | TextAnimations | Subtle shimmer on "Running" status text or version badge | `bunx shadcn@latest add @react-bits/ShinyText-TS-TW` |
| **Stepper** | Components | Multi-step visualization for core update progress (download → extract → verify → restart) | `bunx shadcn@latest add @react-bits/Stepper-TS-TW` |
| **Counter** | Components | Alternative to CountUp for simpler animated counters with more control | `bunx shadcn@latest add @react-bits/Counter-TS-TW` |

**Recommended subset for this milestone:** CountUp, AnimatedContent, SpotlightCard, Stepper. The others are nice-to-have.

### tw-animate-css Classes to Leverage (already imported)

Already in `index.css` line 2: `@import "tw-animate-css";`

| Class Pattern | Use Case |
|---------------|----------|
| `animate-in fade-in` | Card entrance on page load |
| `animate-in fade-in slide-in-from-bottom-4` | Staggered card entrance (combine with delay utilities) |
| `animate-in zoom-in-95` | Dialog/modal open animation |
| `animate-out fade-out zoom-out-95` | Dialog/modal close animation |
| `duration-300 delay-150` | Timing control for staggered animations |

These compose with shadcn/ui's existing component classes.

### Tailwind v4 Native Capabilities (no additions needed)

| Capability | CSS | Use Case |
|-----------|-----|----------|
| Transitions | `transition-all duration-200` | Button hover, card hover elevation |
| Custom keyframes | `@theme { --animate-pulse-slow: pulse-slow 3s infinite }` | Status indicator pulsing |
| Color transitions | `transition-colors duration-150` | Status dot color changes (running → stopped) |
| Transform | `hover:scale-[1.02] transition-transform` | Subtle card hover lift |

### Backend: SSE Progress Tracking (0 new Go modules)

Go's stdlib provides everything needed for SSE:

| Go Stdlib Feature | Purpose |
|------------------|---------|
| `http.Flusher` / `http.ResponseController` | Flush SSE events to client immediately |
| `io.TeeReader` or custom `io.Reader` wrapper | Track download bytes for progress calculation |
| `context.Context.Done()` | Detect client disconnect |
| `fmt.Fprintf(w, "data: %s\n\n", json)` | SSE message format |

**New endpoint:** `GET /api/core/update/stream` — SSE stream emitting progress events during core update.

**Event format:**
```
event: progress
data: {"step":"download","percent":45,"message":"Downloading sing-box v1.11.0..."}

event: progress
data: {"step":"extract","percent":100,"message":"Extracting binary..."}

event: progress
data: {"step":"verify","percent":100,"message":"Verifying configuration..."}

event: complete
data: {"success":true,"version":"1.11.0"}

event: error
data: {"message":"Asset not found for linux-amd64","step":"download"}
```

**Frontend consumption:** Native `EventSource` API (no library needed):
```typescript
const es = new EventSource("/api/core/update/stream");
es.addEventListener("progress", (e) => {
  const data = JSON.parse(e.data);
  setProgress(data);
});
es.addEventListener("complete", (e) => {
  es.close();
  queryClient.invalidateQueries({ queryKey: ["core", "status"] });
});
```

---

## What NOT to Add

| Library | Why NOT |
|---------|---------|
| **motion** (framer-motion successor, v12.34.0) | 50KB+ bundle. react-bits already provides animation primitives. motion excels at layout animations and spring physics, but this is an admin panel — not a marketing site. The animation needs are entrance/exit transitions and micro-interactions, which react-bits + tw-animate-css handle completely. |
| **react-spring** | Same category as motion. Unnecessary given react-bits is already installed. |
| **@react-spring/web** | See above. |
| **lottie-react** | No use case in a proxy management panel. Lottie is for complex vector animations. |
| **gsap** | Overkill. GSAP is for timeline-based animation sequences. The needs here are simple entrance/exit and state transitions. |
| **auto-animate** | Provides automatic animations for list additions/removals. Could be useful but react-bits AnimatedContent + tw-animate-css covers the same needs without another dependency. |
| **react-transition-group** | Legacy library. tw-animate-css is the modern replacement. |
| **ws / socket.io-client** | WebSocket is bidirectional — overkill for unidirectional progress updates. SSE via native EventSource is simpler, automatically reconnects, and works through proxies. |
| **@microsoft/fetch-event-source** | Polyfill for EventSource with POST support. Not needed — the update endpoint can use GET with SSE since authentication is cookie-based (credentials: include). |

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Recommended |
|----------|-------------|-------------|----------------|
| Animation | react-bits (installed) | motion v12 | Already installed, zero new bytes, TS+TW variants, copy-paste components designed for exactly these use cases |
| Enter/exit | tw-animate-css (installed) | CSS @starting-style | tw-animate-css is already imported and provides composable utility classes; @starting-style has limited browser support |
| Progress bar | shadcn/ui Progress (add component) | Custom div with width transition | Radix primitive provides accessibility (ARIA progressbar role), data attributes for state, consistent with existing component library |
| Progress tracking | SSE (native EventSource) | WebSocket | Unidirectional (server→client only), auto-reconnect, works through HTTP proxies, simpler Go implementation, no npm dependency |
| Progress tracking | SSE | Polling with TanStack Query | SSE provides real-time updates without 1-5s polling delay; download progress needs sub-second granularity |
| Status polling | TanStack Query refetchInterval (current) | SSE for status too | Current 5s polling is fine for status — status changes are infrequent. Only download progress needs SSE granularity. |
| Number animation | react-bits CountUp | react-countup (separate package) | react-bits is already a dependency; CountUp is one of its built-in text animation components |
| Step visualization | react-bits Stepper | Custom stepper | react-bits Stepper is designed for this, with animation built in. No reason to hand-roll. |

---

## Integration Points

### react-bits + shadcn/ui Compatibility

react-bits components are designed to work alongside shadcn/ui:
- Both install via `shadcn` CLI
- Both use Tailwind for styling
- react-bits components are headless/presentational — they wrap content, not replace shadcn primitives
- Example: `<SpotlightCard>` wrapping a shadcn `<Card>`, `<CountUp>` inside `<CardContent>`

### tw-animate-css + shadcn/ui

Already integrated. shadcn/ui v3+ uses tw-animate-css for Tailwind v4 compatibility. The `animate-in`/`animate-out` classes are used internally by Dialog, Sheet, and other overlay components. Adding these classes to Cards and page sections extends the same animation language.

### SSE + TanStack Query Coexistence

The SSE stream is independent of TanStack Query's cache. Pattern:
1. User clicks "Update" → POST `/api/core/update/stream` (or GET with action param)
2. EventSource receives progress events → local React state (`useState`)
3. On `complete` event → `queryClient.invalidateQueries(["core", "status"])` refreshes status
4. On `error` event → display error via sonner toast + error state

TanStack Query continues to manage status polling (`refetchInterval: 5000`) independently.

### Process Manager Enhancements (Go, no new deps)

Current `ProcessManager` has: `IsRunning()`, `Restart()`, `Check()`, `Version()`, `Available()`.

Add for this milestone:
- `Start(configPath string) error` — start without killing first (currently only `Restart` exists)
- `Stop() error` — stop without starting (currently buried in `Restart` as side effect)
- `Uptime() (time.Duration, error)` — optional: parse `/proc` or track start time for status display

---

## Installation Summary

### Frontend (run from `web/` directory)

```bash
# Add shadcn/ui Progress component (radix-ui already installed)
bunx shadcn@latest add progress

# Add react-bits components (react-bits already installed)
# Recommended core set:
bunx shadcn@latest add @react-bits/CountUp-TS-TW
bunx shadcn@latest add @react-bits/AnimatedContent-TS-TW
bunx shadcn@latest add @react-bits/SpotlightCard-TS-TW
bunx shadcn@latest add @react-bits/Stepper-TS-TW

# Optional (add if needed during implementation):
bunx shadcn@latest add @react-bits/FadeContent-TS-TW
bunx shadcn@latest add @react-bits/ShinyText-TS-TW
bunx shadcn@latest add @react-bits/Counter-TS-TW
```

**No `bun add` / `npm install` commands needed.** All npm packages are already in `package.json`.

### Backend (Go)

```bash
# No new dependencies. All changes use stdlib:
# - net/http (Flusher for SSE)
# - io (TeeReader for progress)
# - encoding/json (event serialization)
# - context (client disconnect detection)
```

---

## Version Verification

| Package | Declared | Lock | Source |
|---------|----------|------|--------|
| react-bits | ^1.0.5 | 1.0.5 | bun.lock, verified installed |
| tw-animate-css | ^1.4.0 | 1.4.0 | bun.lock, imported in index.css |
| radix-ui | ^1.4.3 | verified in package.json | Consolidated package includes Progress |
| @tanstack/react-query | ^5.90.20 | verified | Supports SSE integration patterns |
| tailwindcss | ^4.1.18 | verified | v4 CSS-first architecture with @theme blocks |

---

## Sources

- react-bits GitHub: https://github.com/DavidHDev/react-bits (35.9K stars, 110+ components, last push 2026-02-14) — HIGH confidence
- react-bits component catalog: verified via GitHub `src/ts-tailwind/` directory listing — HIGH confidence
- tw-animate-css: https://github.com/Wombosvideo/tw-animate-css (v1.4.0, 5.8M weekly downloads) — HIGH confidence
- shadcn/ui Progress: https://ui.shadcn.com/docs/components/progress, built on Radix `@radix-ui/react-progress` — HIGH confidence
- Radix UI consolidated package: https://www.npmjs.com/package/radix-ui (v1.4.3 includes Progress) — HIGH confidence
- Go SSE: https://freecodecamp.org/news/how-to-implement-server-sent-events-in-go, verified stdlib approach — HIGH confidence
- SSE + TanStack Query: https://ollioddi.dev/blog/tanstack-sse-guide — MEDIUM confidence (community pattern, not official TanStack guidance)
- motion v12.34.0: https://www.npmjs.com/package/motion — verified current version for "not recommended" rationale — HIGH confidence
- Native EventSource API: MDN Web Docs, browser-native — HIGH confidence
