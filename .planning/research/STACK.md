# Stack Research: s-ui

**Project:** sing-box proxy management panel  
**Researched:** 2026-02-11  
**Mode:** Ecosystem  
**Downstream:** Roadmap creation

---

## Recommended Stack

### Backend (Go)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **Go** | 1.24+ | Runtime | sing-box 1.13.0 requires Go 1.24 minimum ([changelog](https://sing-box.sagernet.org/changelog/)). Same language as sing-box enables single-binary deployment and easier integration. |
| **Chi** | v5.x | HTTP router | sing-box uses Chi internally; light, stdlib-compatible, no magic. Gin (3x-ui) is heavier; Chi aligns with sing-box ecosystem. |
| **GORM** | v1.31+ | ORM | Industry standard for Go. 3x-ui uses GORM + SQLite. Supports migrations, soft deletes, hooks. |
| **embed** | stdlib | Static assets | Bundle React build into single binary. No CGO, no extra deploy step. |
| **caddyserver/certmagic** | v0.25+ | ACME TLS | Same library sing-box uses. Automatic cert issuance/renewal for panel HTTPS. |

**Rationale:** Go 1.24 is required by sing-box 1.13.0 (Go 1.23 dropped). Chi is used by sing-box itself; Gin dominates proxy panels (3x-ui) but Chi fits better for a lean API. GORM + SQLite matches 3x-ui and supports single-binary + file-based DB.

### Frontend (React)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **React** | 18.x or 19.x | UI framework | Stable baseline. React 19 supported by shadcn (Feb 2025); 18.x safer for compatibility. |
| **Vite** | 6.x | Build tool | CRA deprecated; Vite is default for new React. Fast HMR, ESM dev, Rollup prod. shadcn supports Vite. |
| **TypeScript** | 5.x | Type safety | Required for shadcn components; API types shared with Go. |
| **shadcn/ui** | latest | Component library | Copy-paste components, full control. Dark theme built-in. Use `new-york` style (default deprecated). |
| **Tailwind CSS** | v4 | Styling | shadcn native. Tailwind v4 + `@theme` supported (Feb 2025). Prefer v4 if using React 19 shadcn canary. |
| **TanStack Query** | v5 | Server state | Caching, invalidation, loading/error states. Replaces manual fetch + useState. |
| **react-bits** | — | Micro-interactions | 110+ animated components, Tailwind compatible. Matches PROJECT.md requirement for "react-bits 微交互". |
| **Sonner** | — | Toast | shadcn-recommended toast (replaces deprecated toast component). |
| **React Hook Form** | v7 | Forms | shadcn Forms integration. Zod for validation. |
| **React Router** | v7 | Routing | SPA routing. Default with Vite + React. |

**Rationale:** Vite is the recommended React build tool (CRA deprecated). shadcn/ui v2025 supports React 19 + Tailwind v4; use `npx shadcn@canary init` for v4. react-bits ([DavidHDev/react-bits](https://github.com/DavidHDev/react-bits), ~35k stars) provides Tailwind + shadcn-compatible micro-interactions.

### Database

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **SQLite** | — | Primary store | Single file, no server, portable. 3x-ui and most panels use SQLite. GORM driver: `gorm.io/driver/sqlite`. |

**Optional:** PostgreSQL for production multi-instance deployments. SQLite is sufficient for single-node panel deployment.

### sing-box Integration

| Approach | Purpose | Why |
|----------|---------|-----|
| **Config file** | Primary control | sing-box is config-file driven. Panel writes JSON config, runs `sing-box check`, then restarts sing-box. |
| **Clash API** (experimental) | Dynamic selector | Optional; for outbound switching. Require `with_clash_api` build tag. |
| **V2Ray API** (experimental) | — | Optional; less relevant for VLESS + Hysteria2. |

**Key:** No gRPC API like Xray. sing-box uses JSON config + CLI. Panel must: read config → modify → validate → write → restart sing-box.

### Deployment

| Method | Stack | Purpose |
|--------|-------|---------|
| **Single binary** | `embed` + `go build` | React build embedded in Go binary. One binary for API + static assets. |
| **Docker Compose** | `Dockerfile` multi-stage | Build frontend + Go backend; serve binary. |
| **Bash script** | `install.sh` | Download binary, systemd service, config dir. |

### Key Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| [go-chi/chi](https://github.com/go-chi/chi) | v5 | HTTP router | API routes |
| [alexedwards/scs](https://github.com/alexedwards/scs) | v2 | Session | Router-agnostic; store-agnostic (SQLite, Redis). 3x-ui uses gorilla/sessions; SCS is lighter. |
| [caddyserver/certmagic](https://github.com/caddyserver/certmagic) | v0.25+ | ACME TLS | Panel HTTPS |
| [gorm.io/gorm](https://gorm.io) | v1.31+ | ORM | DB |
| [gorm.io/driver/sqlite](https://gorm.io/driver/sqlite) | v1.6+ | SQLite | Single-node DB |
| [robfig/cron](https://github.com/robfig/cron/v3) | v3 | Cron | Cert renewal, stats aggregation |
| [gorilla/websocket](https://github.com/gorilla/websocket) | v1.5+ | WebSocket | Real-time stats (optional) |

---

## What NOT to Use

| Avoid | Why |
|-------|-----|
| **Create React App** | Deprecated by React team; prefer Vite. |
| **Gin** | Heavier than Chi; sing-box uses Chi. Align with sing-box ecosystem. |
| **Fiber** | Fasthttp-based; not stdlib. Different mental model; unnecessary for API panel. |
| **PostgreSQL as default** | Overkill for single-node; SQLite is standard for panels. |
| **Next.js** | Panel is SPA + API; no SSR needed. Adds complexity. |
| **Redux** | TanStack Query + Zustand (or React state) sufficient. |
| **Direct sing-box API** | No built-in management API; config file is the contract. |

---

## Installation

```bash
# Backend
go mod init github.com/your-org/s-ui
go get github.com/go-chi/chi/v5
go get github.com/caddyserver/certmagic
go get gorm.io/gorm gorm.io/driver/sqlite

# Frontend
pnpm create vite@latest web -- --template react-ts
cd web && pnpm add @tanstack/react-query @tanstack/react-query-devtools
pnpm dlx shadcn@latest init   # or shadcn@canary for Tailwind v4 + React 19
pnpm add react-bits sonner react-hook-form @hookform/resolvers zod
```

---

## Confidence Levels

| Area | Level | Reason |
|------|-------|--------|
| Go 1.24+ | HIGH | sing-box changelog and go.mod explicitly require Go 1.24+ |
| Chi | HIGH | sing-box uses Chi; official. |
| Vite + React | HIGH | CRA deprecated; Vite is default for React. |
| shadcn + Tailwind | HIGH | Official docs; Feb 2025 changelog for v4 + React 19. |
| SQLite | HIGH | 3x-ui and similar panels use SQLite. |
| react-bits | MEDIUM | Popular library; verify Tailwind compatibility with shadcn v4. |
| certmagic | HIGH | Used by sing-box; Caddy ecosystem. |

---

## Sources & References

| Source | Type | Confidence |
|--------|------|------------|
| [sing-box changelog](https://sing-box.sagernet.org/changelog/) | Official | HIGH |
| [sing-box go.mod](https://github.com/SagerNet/sing-box/blob/dev-next/go.mod) | Official | HIGH |
| [sing-box build from source](https://sing-box.sagernet.org/installation/build-from-source/) | Official | HIGH |
| [sing-box experimental (Clash/V2Ray API)](https://sing-box.sagernet.org/configuration/experimental) | Official | HIGH |
| [3x-ui go.mod](https://github.com/MHSanaei/3x-ui/blob/main/go.mod) | Ecosystem | HIGH |
| [shadcn/ui installation](https://ui.shadcn.com/docs/installation) | Official | HIGH |
| [shadcn changelog 2025-02 Tailwind v4](https://ui.shadcn.com/docs/changelog/2025-02-tailwind-v4) | Official | HIGH |
| [DavidHDev/react-bits](https://github.com/DavidHDev/react-bits) | Library | MEDIUM |
| [caddyserver/certmagic](https://pkg.go.dev/github.com/caddyserver/certmagic) | Library | HIGH |
| [Go embed + React SPA](https://www.smartinary.com/blog/how-to-embed-a-react-app-in-a-go-binary/) | Pattern | MEDIUM |

