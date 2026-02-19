---
phase: 01-foundation
plan: 01
subsystem: auth
tags: [chi, gorm, sqlite, scs, bcrypt, react, vite, tailwind, react-router]

# Dependency graph
requires: []
provides:
  - Admin model with bcrypt password hash
  - SCS session with SQLite store, 7-day lifetime
  - Setup/Login/Logout API and UI
  - RequireSetupMiddleware: no admin → /setup; logged in → redirect /setup,/login to /
affects: [02-inbound, 03-cert, 04-user]

# Tech tracking
tech-stack:
  added: [ Chi, GORM, SQLite, SCS, bcrypt, Vite, React, TanStack Query, React Router v7, Tailwind v4 ]
  patterns: [ go:embed for SPA, SPA fallback via ServeContent, RequireSetup flow ]

key-files:
  created: [ internal/db/admin.go, internal/session/session.go, internal/api/auth.go, web/src/pages/Setup.tsx, web/src/pages/Login.tsx, web/src/pages/Dashboard.tsx, web/src/routes.tsx, web/embed.go ]
  modified: [ cmd/server/main.go, internal/api/routes.go, internal/db/db.go, web/src/App.tsx, web/vite.config.ts ]

key-decisions:
  - "SCS sqlite3store as separate module (github.com/alexedwards/scs/sqlite3store)"
  - "Sessions table created in session.Init; gormstore not used (sqlite3store works with same DB)"
  - "SPA fallback: serve index.html via ServeContent to avoid FileServer 301 redirect on /index.html"

patterns-established:
  - "Pattern: RequireSetupMiddleware runs before routes; no admin → /setup; logged in + /setup|/login → /"
  - "Pattern: credentials: 'include' for fetch; GET /api/me for auth check in React Router loader"

# Metrics
duration: ~70min
completed: 2026-02-11
---

# Phase 1 Plan 1: Foundation Summary

**Go + React scaffold with Setup/Login auth flow: Admin model, SCS session, bcrypt, Vercel-style dark UI**

## Performance

- **Duration:** ~70 min
- **Started:** 2026-02-11T08:34:07Z
- **Completed:** 2026-02-11T08:46:00Z
- **Tasks:** 3
- **Files modified:** 22

## Accomplishments

- Go server with Chi router, GORM + SQLite, go:embed for React build
- Admin model (username, password_hash); bcrypt hashing
- SCS session with SQLite store, 7-day lifetime, sessions table
- POST /api/setup, /api/login, /api/logout; GET /api/me for auth check
- RequireSetupMiddleware: first deploy → /setup; logged in → redirect /setup,/login to /
- Setup page: username, password, confirm; validation; POST /api/setup
- Login page: Vercel-style centered card, remember me; POST /api/login
- Dashboard placeholder with logout; React Router v7 protected routes

## Task Commits

1. **Task 1: Project scaffold (Go + React)** - `acf5ee8` (feat)
2. **Task 2: Auth backend (Admin, Session, API)** - `a07ee1e` (feat)
3. **Task 3: Auth UI (Setup + Login pages)** - `a296dc8` (feat)

## Files Created/Modified

- `cmd/server/main.go` - Chi server, embed, session middleware
- `internal/api/routes.go` - API routes, spaHandler with SPA fallback
- `internal/api/auth.go` - Setup, Login, Logout, Me handlers; RequireSetupMiddleware
- `internal/db/db.go` - GORM + SQLite init, AutoMigrate Admin
- `internal/db/admin.go` - Admin model, HasAdmin, CreateAdmin, GetAdminByUsername
- `internal/session/session.go` - SCS manager, SQLite store, sessions table creation
- `web/embed.go` - go:embed dist/*
- `web/src/pages/Setup.tsx` - Setup form, validation, POST /api/setup
- `web/src/pages/Login.tsx` - Login form (Vercel style), remember me, POST /api/login
- `web/src/pages/Dashboard.tsx` - Placeholder, logout
- `web/src/routes.tsx` - React Router, protected route loader
- `web/src/App.tsx` - RouterProvider
- `web/vite.config.ts` - Tailwind v4, base: /
- `go.mod`, `go.sum` - deps

## Decisions Made

- **SCS sqlite3store:** v2 module doesn't include stores; use `github.com/alexedwards/scs/sqlite3store` (separate)
- **Sessions table:** Created in session.NewManager via raw SQL; sqlite3store expects it
- **Cookie.Secure:** false for local dev; set true when behind HTTPS
- **SPA fallback:** http.FileServer redirects /index.html → ./ (301); use ServeContent+bytes.NewReader for index.html fallback

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] SCS sqlite3store package path**
- **Found during:** Task 2 (Auth backend)
- **Issue:** Plan said `go get github.com/alexedwards/scs/sqlite3store` but v2 module doesn't contain it
- **Fix:** Use `github.com/alexedwards/scs/sqlite3store` as separate module (v0.0.0-20251002162104-209de6e426de)
- **Files modified:** go.mod, go.sum
- **Committed in:** a07ee1e

**2. [Rule 3 - Blocking] Sessions table missing**
- **Found during:** Task 2 (verification)
- **Issue:** sqlite3store expects `sessions` table; "no such table: sessions"
- **Fix:** Add CREATE TABLE IF NOT EXISTS in session.NewManager before creating store
- **Files modified:** internal/session/session.go
- **Committed in:** a07ee1e

**3. [Rule 1 - Bug] SPA 301 redirect loop**
- **Found during:** Task 3 (verification)
- **Issue:** http.FileServer redirects requests for /index.html to ./ (301); caused redirect loop on /setup
- **Fix:** Serve index.html via ServeContent + bytes.NewReader; skip FileServer for path "index.html"
- **Files modified:** internal/api/routes.go
- **Committed in:** a296dc8

---

**Total deviations:** 3 auto-fixed (1 blocking x2, 1 bug)
**Impact on plan:** All necessary for correctness. No scope creep.

## Issues Encountered

- SCS v2 module structure: stores are in separate packages; RESEARCH suggested sqlite3store as subpackage
- FileServer /index.html redirect: Go net/http behavior; documented in serveFile

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Auth foundation complete; ready for 01-02 (sing-box integration)
- Build order: `cd web && bun run build` then `go build ./cmd/server` (embed requires dist)

## Self-Check: PASSED

- All created files exist
- Commits acf5ee8, a07ee1e, a296dc8 present in git log

---
*Phase: 01-foundation*
*Completed: 2026-02-11*
