# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-19)

**Core value:** 提供一个界面美观、体验流畅、部署简单的 sing-box 管理面板
**Current focus:** v1.1 UI与核心优化 — Phase 13 (Core Update Progress)

## Current Position

Phase: 13 of 14 (Core Update Progress)
Plan: 1 of ? (next)
Status: Phase 12 complete, ready for Phase 13 planning/execution
Last activity: 2026-02-26 — Completed 12-02 (Core State-Driven Frontend UI & Logs Flow)

Progress: [█████░░░░░] 50%

## Performance Metrics

**Velocity:**
- Total plans completed: 34 (v1.0)
- v1.1 plans completed: 4
- Average duration: 3min for v1.1

**By Phase (v1.1):**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 11. UI Animation Foundation | 2/2 | 4min | 2min |
| 12. Core Process Control | 2/2 | 9min | 4.5min |
| 13. Core Update Progress | 0/? | - | - |
| 14. UI Polish & Consistency | 0/? | - | - |
| Phase 12-core-process-control P01 | 5 min | 3 tasks | 6 files |
| Phase 12-core-process-control P02 | 4 min | 3 tasks | 2 files |

## Accumulated Context

### Decisions

All decisions logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- v1.1: SSE only for update progress; status polling stays with TanStack Query 5s interval
- v1.1: No new npm/Go dependencies — activate already-installed react-bits, use stdlib SSE
- 11-02: Card hover uses border-ring/50 for theme-aware border glow
- 11-02: Entrance animation pattern: animate-in fade-in zoom-in-95 duration-300 with 75ms stagger
- 11-01: Used Intl.NumberFormat for per-frame CountUp formatting, refs for SpotlightCard mouse tracking
- [Phase 12-core-process-control]: State priority fixed to not_installed > running > error > stopped to avoid misleading control hints.
- [Phase 12-core-process-control]: Process control methods return semantic codes instead of silent success for missing binary or invalid lifecycle transitions.
- [Phase 12-core-process-control]: Status API keeps legacy running field while introducing state/actions/lastError for compatibility-first migration.
- [Phase 12-core-process-control]: Centralized frontend core state contract in web/src/lib/core-status.ts for consistent four-state rendering.
- [Phase 12-core-process-control]: Lifecycle controls are rendered via state matrix intersected with backend actions to prevent invalid actions in error state.
- [Phase 12-core-process-control]: Error-state diagnostics use on-demand /api/core/logs dialog with explicit loading, empty, and error feedback.

### Pending Todos

None.

### Roadmap Evolution

- v1.0 MVP shipped with 10 phases (34 plans)
- v1.1 roadmap created with 4 phases (11-14), 12 requirements mapped

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-02-26
Stopped at: Completed 12-02-PLAN.md
Resume file: None
Next: Begin Phase 13 with 13-01-PLAN.md
