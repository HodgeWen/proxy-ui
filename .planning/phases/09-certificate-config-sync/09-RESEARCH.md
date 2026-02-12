# Phase 9: Certificate Config Sync (Gap Closure) - Research

**Researched:** 2026-02-12
**Domain:** Certificate update → config regeneration flow, rollback pattern
**Confidence:** HIGH

## Summary

Phase 9 closes the integration gap identified in v1.0-MILESTONE-AUDIT.md: `UpdateCertificateHandler` updates the certificate in DB but does not call `Generate → ApplyConfig → Restart`. As a result, inbounds referencing the updated cert keep old paths until another mutation (e.g. inbound edit) triggers the flow. The fix is to align `UpdateCertificateHandler` with the established Inbound/User mutation pattern: persist → Generate → ApplyConfig; on ApplyConfig failure, rollback DB and return 400 with error; on success, Restart (best-effort).

**Primary recommendation:** Add `panelCfg *config.Config` to `UpdateCertificateHandler`, implement fetch-before-save + Generate → ApplyConfig → Restart flow with rollback on failure, mirroring `UpdateInboundHandler` exactly.

---

## Standard Stack

### Core (unchanged from Phase 1–3)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Chi | v5.x | HTTP router | Phase 1 |
| GORM | v1.31+ | ORM | Phase 1 |
| internal/core.ConfigGenerator | — | Full sing-box JSON from DB | Phase 2; resolves cert paths via resolveCertInTLS |
| internal/core.ApplyConfig | — | Write tmp → sing-box check → atomic rename | Phase 1 |
| internal/core.NewProcessManagerFromConfig | — | Restart sing-box | Phase 1 |

### No New Dependencies
Phase 9 only wires existing components; no new packages required.

---

## Architecture Patterns

### Pattern 1: Mutate → Generate → ApplyConfig → Restart (from Phase 2/4)

**What:** After any DB mutation that affects generated config, call `gen.Generate()` → `core.ApplyConfig(path, cfg)`. On ApplyConfig failure, rollback the DB change and return 400 with error body `{"error": err.Error()}`. On success, call `pm.Restart(path)` (best-effort).

**When to use:** Inbound Create/Update/Delete, User Create/Update/Delete/Batch, and—after Phase 9—Certificate Update.

**Where implemented (verified):**
- `internal/api/inbounds.go`: CreateInboundHandler, UpdateInboundHandler, DeleteInboundHandler
- `internal/api/users.go`: CreateUserHandler, UpdateUserHandler, DeleteUserHandler, BatchUsersHandler

**Example (UpdateInboundHandler, lines 260–279):**
```go
path := configPath(panelCfg)
gen := &core.ConfigGenerator{}
cfg, err := gen.Generate()
if err != nil {
    db.UpdateInbound(old)
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
}
if err := core.ApplyConfig(path, cfg); err != nil {
    db.UpdateInbound(old)
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusBadRequest)
    json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
    return
}
pm := core.NewProcessManagerFromConfig(panelCfg)
if err := pm.Restart(path); err != nil {
    // Config applied; restart failure is best-effort
}
```

### Pattern 2: Fetch-Before-Save for Rollback

**What:** Before mutating, fetch the current entity. On ApplyConfig failure, call `db.UpdateX(old)` to revert.

**When to use:** Update handlers where rollback is required.

**Example (UpdateInboundHandler):**
```go
old, err := db.GetInboundByID(id)
// ... update ...
if err := core.ApplyConfig(path, cfg); err != nil {
    db.UpdateInbound(old)
    // return 400
}
```

---

## Current Gap (Audit)

| Location | Current Behavior | Expected Behavior |
|----------|------------------|-------------------|
| `UpdateCertificateHandler` | `db.UpdateCertificate(c)` → return 200 | persist → Generate → ApplyConfig → Restart; rollback on failure |
| `UpdateCertificateHandler` signature | `(sm *scs.SessionManager)` | `(sm *scs.SessionManager, panelCfg *config.Config)` |
| routes.go | `UpdateCertificateHandler(sm)` | `UpdateCertificateHandler(sm, cfg)` |

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Config regeneration | Custom logic | `core.ConfigGenerator.Generate()` | Reads DB (inbounds, certs, users); resolveCertInTLS maps cert_id → paths |
| Config apply + validation | Custom write | `core.ApplyConfig(path, cfg)` | Writes tmp, runs sing-box check, atomic rename; error includes check output |
| Restart | Custom exec | `core.NewProcessManagerFromConfig(cfg).Restart(path)` | Existing ProcessManager handles start/stop |

---

## Common Pitfalls

### Pitfall 1: Forgetting panelCfg

**What goes wrong:** Handler cannot call `configPath(panelCfg)` or `NewProcessManagerFromConfig(panelCfg)`.

**Why it happens:** `UpdateCertificateHandler` and `CreateCertificateHandler` were added without `panelCfg` because Phase 3 plan stated "No config apply on cert mutate".

**How to avoid:** Add `panelCfg *config.Config` to `UpdateCertificateHandler`; update routes.go to pass `cfg`.

### Pitfall 2: Skipping Rollback on ApplyConfig Failure

**What goes wrong:** Certificate is updated in DB but config is invalid; sing-box keeps old config or fails; DB and runtime diverge.

**Why it happens:** Not following the Inbound/User pattern.

**How to avoid:** Fetch old cert before update; on `ApplyConfig` error, call `db.UpdateCertificate(old)` and return 400.

### Pitfall 3: Inconsistent Error Response Shape

**What goes wrong:** Frontend expects `{"error": "..."}` for ApplyConfig failure but receives plain text.

**Why it happens:** Using `http.Error` instead of JSON body for 400.

**How to avoid:** Match Inbound/User handlers: `w.Header().Set("Content-Type", "application/json")`, `w.WriteHeader(http.StatusBadRequest)`, `json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})`.

---

## Code Examples

### UpdateCertificateHandler Flow (Target)

```go
// UpdateCertificateHandler handles PUT /api/certs/:id.
func UpdateCertificateHandler(sm *scs.SessionManager, panelCfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ... parse id, validate request ...

		old, err := db.GetCertificateByID(id)
		if err != nil { /* 404 */ }

		c := &db.Certificate{ID: id, Name: ..., FullchainPath: ..., PrivkeyPath: ...}
		if err := db.UpdateCertificate(c); err != nil { /* 500 */ }

		path := configPath(panelCfg)
		gen := &core.ConfigGenerator{}
		cfg, err := gen.Generate()
		if err != nil {
			db.UpdateCertificate(old)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := core.ApplyConfig(path, cfg); err != nil {
			db.UpdateCertificate(old)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		pm := core.NewProcessManagerFromConfig(panelCfg)
		if err := pm.Restart(path); err != nil {
			// best-effort
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(certFromDB(c))
	}
}
```

### routes.go Change

```go
r.Put("/{id}", UpdateCertificateHandler(sm, cfg))
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Cert update without config apply | Cert update → Generate → Apply | Phase 9 | CRT-02: cert paths in TLS immediately reflected |

**Deprecated/outdated:**
- Phase 3 plan's "No config apply on cert mutate": superseded by audit gap; UpdateCertificate must trigger apply.

---

## Open Questions

None. The fix is a direct application of the established Inbound/User pattern.

---

## Sources

### Primary (HIGH confidence)
- `internal/api/inbounds.go` — UpdateInboundHandler flow (lines 211–282)
- `internal/api/certs.go` — UpdateCertificateHandler current impl (lines 111–147)
- `internal/api/routes.go` — handler registration (line 51)
- `internal/core/generator.go` — resolveCertInTLS, Generate (lines 229–248, 14–39)
- `.planning/v1.0-MILESTONE-AUDIT.md` — integration gap (lines 13–14, 176, 201)

### Secondary (MEDIUM confidence)
- `.planning/phases/02-inbound-management/02-RESEARCH.md` — DB-first config flow
- `.planning/phases/04-user-management/04-02-PLAN.md` — rollback pattern

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — existing codebase verified
- Architecture: HIGH — pattern fully established in Inbound/User handlers
- Pitfalls: HIGH — direct code inspection

**Research date:** 2026-02-12
**Valid until:** 30 days (stable pattern)
