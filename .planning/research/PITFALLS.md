# Domain Pitfalls: UI Polish & Core Management Optimization

**Domain:** Admin panel enhancement (proxy management)
**Researched:** 2026-02-19

## Critical Pitfalls

Mistakes that cause rewrites or major issues.

### Pitfall 1: Concurrent Core Updates Corrupt Binary

**What goes wrong:** Two users (or two tabs) trigger core update simultaneously. Both download, both try to replace the binary. One overwrites the other's backup, or the binary is written to while being extracted by the other.
**Why it happens:** Current `UpdateHandler` has no concurrency protection — no mutex, no "update in progress" state.
**Consequences:** Corrupted sing-box binary, service down, backup also corrupted (unrecoverable without manual intervention).
**Prevention:** Add `sync.Mutex` around the update flow. Return HTTP 409 Conflict if update is already in progress. The SSE handler should `TryLock()` and fail fast.
**Detection:** Multiple "update" requests in access logs at overlapping timestamps.

### Pitfall 2: SSE Connection Leaks on Client Navigation

**What goes wrong:** User starts update, sees progress streaming, navigates away from Core page. EventSource stays open. If they return and start another update, now there are two connections.
**Why it happens:** EventSource isn't cleaned up on React component unmount.
**Consequences:** Memory leak, server holds stale connections, potential double-update if not mutex-protected.
**Prevention:** Close EventSource in `useEffect` cleanup. Store reference in `useRef`. The backend mutex prevents actual double-updates, but the frontend should still clean up.
**Detection:** Network tab shows multiple open SSE connections.

### Pitfall 3: Update Runs to Completion Even If Client Disconnects

**What goes wrong:** User starts update, closes browser. The Go handler continues downloading, extracting, and replacing the binary. If the download fails halfway, there's no one to see the error. The binary may be in a half-replaced state.
**Why it happens:** The current `Update()` function is synchronous and doesn't check `r.Context().Done()`.
**Consequences:** Stale SSE connection on server, wasted bandwidth, potential partial update state.
**Prevention:** Check `r.Context().Done()` at each step boundary in the update flow. The download phase should use `r.Context()` as the request context. However, once extraction starts, it's safer to complete than to abort mid-replacement. Design the cancellation boundary: download is cancellable, replacement is not.
**Detection:** Server logs showing completed updates with no client reading the response.

## Moderate Pitfalls

### Pitfall 4: react-bits Components Conflicting with shadcn/ui Styles

**What goes wrong:** react-bits components bring their own Tailwind classes that override or conflict with shadcn/ui's design tokens.
**Prevention:** Use the TS-TW (TypeScript + Tailwind) variant of react-bits components. Review the generated component file after CLI installation — react-bits components are copy-paste, so you can adjust classes. Test dark mode specifically — react-bits may use hardcoded colors instead of CSS variables.

### Pitfall 5: Animation Jank on Low-End Devices

**What goes wrong:** CountUp, AnimatedContent, and card entrance animations cause visible frame drops on low-power devices (common for users who self-host on Raspberry Pi or cheap VPS accessed via mobile).
**Prevention:** Use `will-change: transform` sparingly. Prefer `transform` and `opacity` animations (GPU-composited) over `height`, `width`, or `top` (trigger layout reflow). react-bits components generally use GPU-friendly transforms. Set reasonable `duration` (200-500ms, not 1000ms+). Consider `prefers-reduced-motion` media query.

### Pitfall 6: SSE and Reverse Proxies (Nginx/Caddy Buffering)

**What goes wrong:** s-ui is typically deployed behind Nginx or Caddy. These reverse proxies buffer responses by default. SSE events aren't delivered until the buffer fills or the connection closes — defeating the purpose of real-time progress.
**Prevention:** The SSE handler should set `X-Accel-Buffering: no` (Nginx-specific) and `Cache-Control: no-cache, no-transform`. Document the Nginx config needed: `proxy_buffering off;` for the SSE endpoint. Caddy handles streaming correctly by default.

### Pitfall 7: EventSource Doesn't Support POST or Custom Headers

**What goes wrong:** Developer tries to send authentication tokens or use POST method with EventSource. EventSource only supports GET with cookies.
**Prevention:** This project uses cookie-based auth (`credentials: "include"`), which works with EventSource since cookies are sent automatically. The update trigger should be a GET request with SSE response (the "action" is implicit — hitting the endpoint starts the update). Alternative: POST to trigger update, return a stream ID, then GET the SSE stream by ID. The simpler approach (GET triggers update) is fine since the mutex prevents accidental re-triggers.

### Pitfall 8: `pgrep -x sing-box` False Negatives

**What goes wrong:** `IsRunning()` uses `pgrep -x sing-box` which does an exact match on process name. If sing-box is started with a full path (e.g., `/opt/s-ui/bin/sing-box`), the process name in `/proc/*/comm` is still "sing-box" — this works. But if the binary is renamed or running under a wrapper, `pgrep -x` fails.
**Prevention:** For the scope of this milestone, `pgrep -x` is sufficient since s-ui controls the binary name. If broader compatibility is needed later, track the PID from `cmd.Start()` and check `/proc/<pid>/status`.

## Minor Pitfalls

### Pitfall 9: tw-animate-css Classes Not Applied on Re-render

**What goes wrong:** Adding `animate-in fade-in` to a Card. It animates on first mount, but when TanStack Query refetches and React re-renders with new data, the card doesn't re-animate because the DOM element was reused (not remounted).
**Prevention:** This is actually desired behavior — you don't want cards re-animating on every 5s poll. Entrance animations should only run on mount. If you need re-animation on data change, use a `key` prop to force remount, or use react-bits CountUp which handles value changes gracefully.

### Pitfall 10: Dark Mode Color Inconsistency with react-bits

**What goes wrong:** react-bits components may use hardcoded colors (e.g., `bg-white`, `text-black`) instead of shadcn's CSS variable system (`bg-card`, `text-foreground`).
**Prevention:** After installing each react-bits component via CLI, review the generated file. Replace any hardcoded colors with the project's CSS variable-based classes. The TS-TW variants are more Tailwind-native but may still need adjustment for dark mode.

### Pitfall 11: Progress Bar Showing 0% for Extended Period

**What goes wrong:** The GitHub API request to list releases takes 1-3 seconds before download even starts. During this time, the progress bar sits at 0% — user thinks it's frozen.
**Prevention:** Use the Stepper/step model rather than a single percentage. Show "Fetching release info..." as the first step with an indeterminate/spinner state. Reserve the percentage bar for the actual download step where `Content-Length` enables accurate progress calculation.

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| UI animations | Pitfall 5 (jank on low-end) + Pitfall 9 (re-render) | Use GPU-friendly transforms; only animate on mount |
| react-bits integration | Pitfall 4 (style conflicts) + Pitfall 10 (dark mode) | Review generated files; test dark mode for each component |
| Core start/stop | Pitfall 8 (pgrep false negatives) | Keep existing pgrep approach; it works for the controlled environment |
| SSE progress | Pitfall 1 (concurrent) + Pitfall 2 (leaks) + Pitfall 6 (proxy buffering) | Mutex on backend, useEffect cleanup on frontend, document proxy config |
| SSE progress | Pitfall 3 (disconnect) + Pitfall 7 (EventSource limitations) | Context cancellation for download; use GET with cookies for auth |
| Progress UX | Pitfall 11 (stuck at 0%) | Use step model, show indeterminate for non-download steps |

## Sources

- Codebase analysis: `internal/core/updater.go` (no mutex), `internal/core/process.go` (pgrep pattern)
- SSE proxy buffering: Nginx `proxy_buffering` documentation
- EventSource limitations: MDN EventSource API documentation
- react-bits dark mode: Observed from component variant structure (TS-TW uses Tailwind classes)
