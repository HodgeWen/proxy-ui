# Architecture Patterns: UI Polish & Core Management Optimization

**Domain:** Admin panel enhancement (proxy management)
**Researched:** 2026-02-19

## Current Architecture (Relevant Parts)

```
┌─────────────────────────────────────────────────┐
│  Frontend (React + Vite)                         │
│                                                  │
│  Pages: Dashboard, Core, Users, Inbounds, ...   │
│  UI: shadcn/ui + Tailwind v4                    │
│  State: TanStack Query (server state)           │
│  Animations: tw-animate-css (imported, minimal)  │
│  react-bits: installed, unused                   │
│                                                  │
│  ┌──────────────┐  ┌──────────────┐             │
│  │ useQuery     │  │ useMutation  │             │
│  │ polling 5s   │  │ one-shot     │             │
│  └──────┬───────┘  └──────┬───────┘             │
│         │                  │                     │
└─────────┼──────────────────┼─────────────────────┘
          │ GET              │ POST
          ▼                  ▼
┌─────────────────────────────────────────────────┐
│  Backend (Go + Chi)                              │
│                                                  │
│  /api/core/status      → StatusHandler          │
│  /api/core/restart     → RestartHandler         │
│  /api/core/update      → UpdateHandler          │
│  /api/core/rollback    → RollbackHandler        │
│  /api/core/versions    → VersionsHandler        │
│  /api/core/config-file → ConfigFileHandler      │
│                                                  │
│  ┌──────────────────┐  ┌────────────────────┐   │
│  │ ProcessManager   │  │ CoreUpdater        │   │
│  │ - IsRunning()    │  │ - ListReleases()   │   │
│  │ - Restart()      │  │ - Update()         │   │
│  │ - Check()        │  │ - Rollback()       │   │
│  │ - Version()      │  │ - downloadFile()   │   │
│  └──────────────────┘  └────────────────────┘   │
└─────────────────────────────────────────────────┘
```

## Target Architecture (After This Milestone)

```
┌──────────────────────────────────────────────────────┐
│  Frontend (React + Vite)                              │
│                                                       │
│  ┌─────────────────────────────────────────────────┐ │
│  │ Animation Layer                                  │ │
│  │ react-bits: CountUp, AnimatedContent,           │ │
│  │            SpotlightCard, Stepper               │ │
│  │ tw-animate-css: animate-in/out on cards         │ │
│  │ Tailwind v4: transitions, hover states          │ │
│  └─────────────────────────────────────────────────┘ │
│                                                       │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ useQuery     │  │ useMutation  │  │ EventSource│ │
│  │ polling 5s   │  │ start/stop   │  │ SSE stream │ │
│  └──────┬───────┘  └──────┬───────┘  └──────┬─────┘ │
│         │                  │                  │       │
└─────────┼──────────────────┼──────────────────┼───────┘
          │ GET              │ POST             │ GET (SSE)
          ▼                  ▼                  ▼
┌──────────────────────────────────────────────────────┐
│  Backend (Go + Chi)                                   │
│                                                       │
│  /api/core/status          → StatusHandler           │
│  /api/core/start           → StartHandler    (NEW)  │
│  /api/core/stop            → StopHandler     (NEW)  │
│  /api/core/restart         → RestartHandler          │
│  /api/core/update/stream   → UpdateSSEHandler(NEW)  │
│  /api/core/rollback        → RollbackHandler         │
│  /api/core/versions        → VersionsHandler         │
│  /api/core/config-file     → ConfigFileHandler       │
│                                                       │
│  ┌──────────────────┐  ┌────────────────────────┐   │
│  │ ProcessManager   │  │ CoreUpdater            │   │
│  │ + Start()  (NEW) │  │ + UpdateWithProgress() │   │
│  │ + Stop()   (NEW) │  │   (callback-based)     │   │
│  │ - IsRunning()    │  │ - Update() (existing)  │   │
│  │ - Restart()      │  │ - Rollback()           │   │
│  └──────────────────┘  └────────────────────────┘   │
└──────────────────────────────────────────────────────┘
```

### Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| Animation Layer (react-bits + tw-animate-css) | Visual polish — entrance animations, number counters, card effects | Wraps existing shadcn/ui components; no API communication |
| TanStack Query (existing) | Server state management — status polling, mutation state | GET /api/core/status (5s interval), POST /api/core/start\|stop\|restart |
| EventSource (new, native API) | Real-time progress during update | GET /api/core/update/stream (SSE) |
| ProcessManager (Go) | sing-box process lifecycle | OS exec (pgrep, pkill, sing-box binary) |
| CoreUpdater (Go) | Binary download, extract, replace, verify | GitHub Releases API, filesystem, ProcessManager |

### Data Flow: Update with Progress

```
User clicks "Update"
        │
        ▼
Frontend: new EventSource("/api/core/update/stream")
        │
        ▼
Backend: UpdateSSEHandler
  │
  ├─ Set SSE headers (Content-Type: text/event-stream)
  ├─ Acquire update mutex (prevent concurrent updates)
  │
  ├─ Step 1: Fetch releases from GitHub
  │   └─ Send: event:progress data:{"step":"fetch","percent":100}
  │
  ├─ Step 2: Download binary (with progress reader)
  │   ├─ Send: event:progress data:{"step":"download","percent":0}
  │   ├─ ... (every ~2% or 500ms)
  │   └─ Send: event:progress data:{"step":"download","percent":100}
  │
  ├─ Step 3: Extract
  │   └─ Send: event:progress data:{"step":"extract","percent":100}
  │
  ├─ Step 4: Verify (sing-box check)
  │   └─ Send: event:progress data:{"step":"verify","percent":100}
  │
  ├─ Step 5: Restart
  │   └─ Send: event:progress data:{"step":"restart","percent":100}
  │
  └─ Send: event:complete data:{"success":true,"version":"1.11.0"}
        │
        ▼
Frontend: es.close()
  └─ queryClient.invalidateQueries(["core", "status"])
```

## Patterns to Follow

### Pattern 1: react-bits Wrapping shadcn Components

react-bits components are presentational wrappers. They don't replace shadcn primitives.

```typescript
// CORRECT: react-bits wraps shadcn
import { CountUp } from "@/components/ui/count-up"
import { Card, CardContent } from "@/components/ui/card"

<Card>
  <CardContent>
    <CountUp
      from={0}
      to={stats.user_count}
      duration={0.8}
      className="text-2xl font-bold"
    />
  </CardContent>
</Card>

// CORRECT: AnimatedContent wraps page sections
import { AnimatedContent } from "@/components/ui/animated-content"

<AnimatedContent>
  <div className="grid gap-6 md:grid-cols-3">
    {/* shadcn Cards here */}
  </div>
</AnimatedContent>
```

### Pattern 2: SSE with Cleanup in React

```typescript
function useUpdateStream() {
  const [progress, setProgress] = useState<UpdateProgress | null>(null);
  const queryClient = useQueryClient();

  const startUpdate = useCallback(() => {
    const es = new EventSource("/api/core/update/stream", {
      withCredentials: true
    });

    es.addEventListener("progress", (e) => {
      setProgress(JSON.parse(e.data));
    });

    es.addEventListener("complete", (e) => {
      const result = JSON.parse(e.data);
      es.close();
      setProgress(null);
      queryClient.invalidateQueries({ queryKey: ["core", "status"] });
      if (result.success) toast.success("更新成功");
    });

    es.addEventListener("error", (e) => {
      // EventSource fires "error" on connection issues AND custom error events
      if (es.readyState === EventSource.CLOSED) {
        setProgress(null);
      }
    });

    return () => es.close(); // cleanup function
  }, [queryClient]);

  return { progress, startUpdate };
}
```

### Pattern 3: Go SSE Handler with Mutex

```go
var updateMu sync.Mutex

func UpdateSSEHandler(cfg *config.Config) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !updateMu.TryLock() {
            http.Error(w, "update already in progress", http.StatusConflict)
            return
        }
        defer updateMu.Unlock()

        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")

        rc := http.NewResponseController(w)
        send := func(event, data string) {
            fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
            rc.Flush()
        }
        // ... update steps with send() calls
    }
}
```

### Pattern 4: tw-animate-css with Staggered Delays

```tsx
<div className="grid gap-6 md:grid-cols-3">
  <Card className="animate-in fade-in slide-in-from-bottom-4 duration-500">
    {/* Card 1 */}
  </Card>
  <Card className="animate-in fade-in slide-in-from-bottom-4 duration-500 delay-100">
    {/* Card 2 — 100ms later */}
  </Card>
  <Card className="animate-in fade-in slide-in-from-bottom-4 duration-500 delay-200">
    {/* Card 3 — 200ms later */}
  </Card>
</div>
```

## Anti-Patterns to Avoid

### Anti-Pattern 1: Animating Everything

**What:** Adding entrance animations to every element including form inputs, labels, table rows.
**Why bad:** Makes the interface feel slow and frustrating for repeat users. Admin panels are task-oriented — users visit the same pages dozens of times daily.
**Instead:** Animate page-level containers (Cards, sections) on first paint. Don't animate individual form fields or table cells.

### Anti-Pattern 2: SSE for Status Polling

**What:** Replacing the 5-second TanStack Query polling with a persistent SSE connection for status.
**Why bad:** SSE connections consume server resources. Status changes (running/stopped) are infrequent — polling every 5s is perfectly adequate. SSE should only be used for the update progress flow where sub-second granularity matters.
**Instead:** Keep `refetchInterval: 5000` for status. Use SSE only for the update stream.

### Anti-Pattern 3: Mixing Animation Libraries

**What:** Using motion for some animations, react-bits for others, and CSS animations for yet more.
**Why bad:** Inconsistent timing, easing, and behavior. Three mental models for the same concept.
**Instead:** Use react-bits for component-level animations, tw-animate-css for utility-class animations, and Tailwind native for transitions. Clear hierarchy: component → class → inline.

### Anti-Pattern 4: Progress Polling Instead of SSE

**What:** Creating a `/api/core/update/status` endpoint and polling it every 500ms.
**Why bad:** 500ms polling means up to 500ms latency on progress updates. Creates unnecessary request overhead. Requires server-side state management to track progress between requests.
**Instead:** SSE pushes progress events as they happen. Zero latency, single HTTP connection.

## Sources

- Existing codebase: `internal/api/core.go`, `internal/core/updater.go`, `internal/core/process.go`
- Go SSE stdlib: https://freecodecamp.org/news/how-to-implement-server-sent-events-in-go
- EventSource MDN: https://developer.mozilla.org/en-US/docs/Web/API/EventSource
- TanStack Query + SSE: https://ollioddi.dev/blog/tanstack-sse-guide
