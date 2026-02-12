# Phase 3: Certificate Management - Research

**Researched:** 2026-02-12
**Domain:** sing-box TLS certificate management, Certificate entity, inbound TLS association
**Confidence:** HIGH

## Summary

Phase 3 adds a dedicated Certificate entity and UI so admins can define fullchain + privkey paths once, then associate them with inbound TLS. Phase 2 already supports inline certificate paths in config_json; Phase 3 adds a reusable Certificate model and cert selection in the inbound form. sing-box expects `certificate_path` and `key_path` in the TLS config; ConfigGenerator must resolve cert references (by ID) to paths before emitting sing-box JSON. Manual path mode only (no ACME; CRT-10/11 deferred to v2).

**Primary recommendation:** Add Certificate model (fullchain_path, privkey_path, optional name/domain); Cert CRUD API and page; InboundFormModal TLS section: add "选择证书" dropdown to pick from certificates; ConfigGenerator resolves certificate_id to paths when building tls.

---

## User Constraints

No CONTEXT.md exists for this phase. No locked decisions from /gsd:discuss-phase.

---

## Standard Stack

### Core (from Phase 1–2)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| GORM | v1.31+ | ORM | Phase 1; Certificate model |
| Chi | v5.x | HTTP router | Phase 1; /api/certs |
| shadcn/ui | latest | Components | Phase 1; Modal, Table, Select |
| TanStack Query | v5 | Server state | Phase 1; cert list |
| react-hook-form | v7 | Form state | Phase 2; InboundFormModal |
| zod | v4 | Validation | Phase 2; cert path validation |

### Phase 3 Additions
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| — | — | Extend existing | No new deps; reuse InboundFormModal, Table, Select |

**Installation:** None required for Phase 3.

---

## Architecture Patterns

### Recommended Project Structure (extend Phase 2)

```
internal/
├── api/
│   ├── auth.go
│   ├── core.go
│   ├── inbounds.go
│   ├── certs.go        # NEW: list, create, update, delete
│   └── routes.go       # add /api/certs routes
├── core/
│   ├── config.go
│   ├── generator.go    # MODIFY: resolve certificate_id to paths
│   └── process.go
├── db/
│   ├── admin.go
│   ├── db.go           # add Certificate to AutoMigrate
│   ├── inbound.go
│   └── certificate.go  # NEW: Certificate model, CRUD
└── session/

web/src/
├── components/
│   ├── layout/
│   ├── inbounds/       # MODIFY: InboundFormModal add cert selector
│   ├── certificates/   # NEW: CertificateTable, CertificateFormModal
│   └── ui/
├── pages/
│   ├── Dashboard.tsx
│   ├── Inbounds.tsx
│   └── Certificates.tsx # NEW
└── ...
```

### Pattern 1: Certificate Model

**What:** Certificate stores fullchain_path and privkey_path (and optional name/domain). Used by ConfigGenerator.

**Fields:**
| Field | Type | Purpose |
|-------|------|---------|
| id | uint | PK |
| name | string | Display label (e.g. "example.com") |
| fullchain_path | string | Path to cert chain (PEM) |
| privkey_path | string | Path to private key (PEM) |
| created_at | time.Time | |

**sing-box expects:** `certificate_path` (fullchain path) and `key_path` (privkey path). sing-box auto-reloads when files change (per [TLS docs](https://sing-box.sagernet.org/configuration/shared/tls/)).

### Pattern 2: Certificate Association in Inbound

**What:** Inbound config_json can reference a certificate by ID or use inline paths.

**Option A — cert_id in config_json:**
```json
{
  "tls": {
    "enabled": true,
    "server_name": "example.com",
    "certificate_id": 1
  }
}
```
ConfigGenerator: when `certificate_id` present, fetch `cert` from DB, inject `certificate_path: cert.FullchainPath`, `key_path: cert.PrivkeyPath`.

**Option B — inline paths (backward compat):**
```json
{
  "tls": {
    "enabled": true,
    "server_name": "example.com",
    "certificate_path": "/etc/certs/fullchain.pem",
    "key_path": "/etc/certs/privkey.pem"
  }
}
```
ConfigGenerator: when paths present, use as-is (Phase 2 behavior).

**Recommendation:** Support both. `certificate_id` takes precedence; if absent, use inline paths. This allows backward compat with Phase 2 inbounds.

### Pattern 3: InboundFormModal TLS UX

**What:** When TLS type selected (VLESS TLS or Hysteria2), show either:
- **选择证书:** Select dropdown with certificate list (name or id)
- **手动输入:** Inline path inputs (current Phase 2 fields)

Mutually exclusive: selecting a cert disables/clears inline paths; clearing cert selection re-enables inline inputs.

### Pattern 4: ConfigGenerator Flow

```go
// When building tls for inbound:
if certID, ok := tls["certificate_id"]; ok && certID != nil {
    cert, err := db.GetCertificateByID(certID)
    if err == nil {
        tls["certificate_path"] = cert.FullchainPath
        tls["key_path"] = cert.PrivkeyPath
        delete(tls, "certificate_id") // sing-box doesn't know this
    }
}
// Otherwise use existing certificate_path/key_path from config_json
```

### Anti-Patterns to Avoid

- **Cert path validation on server:** Don't read files on disk to validate; panel may not have filesystem access. Paths are strings; sing-box check validates at apply time.
- **Reality for Hysteria2:** Reality is VLESS-only; Hysteria2 TLS = cert path only (unchanged from Phase 2).
- **ACME in Phase 3:** CRT-10/11 deferred to v2; manual path only.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cert path validation | File read | sing-box check | Panel may not have access; check validates at apply |
| Cert selection UI | Custom dropdown | shadcn Select | Same as Phase 2 TLS/transport selects |
| Cert CRUD | Custom logic | GORM + InboundService pattern | Consistent with Phase 2 inbounds |

---

## Common Pitfalls

### Pitfall 1: Deleting Certificate in Use

**What goes wrong:** Admin deletes cert; inbounds still reference it; ConfigGenerator fails or emits empty paths.

**Why it happens:** No referential check before delete.

**How to avoid:** Option (a) Block delete if cert is in use by any inbound; return error. Option (b) Allow delete; ConfigGenerator returns error when cert missing; admin must update inbounds first. Option (a) simpler for UX.

**Warning signs:** 500 or check failure after cert delete.

### Pitfall 2: cert_id vs certificate_path Conflict

**What goes wrong:** config_json has both certificate_id and certificate_path; precedence unclear.

**How to avoid:** Form and API enforce: only one of certificate_id or inline paths. When cert selected, clear inline paths; when paths entered, clear cert selection.

### Pitfall 3: Path Format (Windows vs Linux)

**What goes wrong:** Admin runs panel on Windows; paths are C:\certs\fullchain.pem; sing-box runs on Linux; paths invalid.

**How to avoid:** Document that paths are for sing-box process (which runs on the server). Panel typically runs on same host; if not, document deployment topology.

### Pitfall 4: Hysteria2 Without TLS

**What goes wrong:** Hysteria2 inbound with no cert; sing-box check fails.

**Why it happens:** Hysteria2 requires TLS (Phase 2 RESEARCH).

**How to avoid:** Hysteria2 form always shows TLS section; require at least cert selection or inline paths when TLS enabled.

---

## Code Examples

### Certificate Model (Go)

```go
// internal/db/certificate.go
type Certificate struct {
    ID            uint      `gorm:"primaryKey"`
    Name          string    `gorm:"not null"` // display label
    FullchainPath string    `gorm:"column:fullchain_path;not null"`
    PrivkeyPath   string    `gorm:"column:privkey_path;not null"`
    CreatedAt     time.Time `gorm:"autoCreateTime"`
    UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

func (Certificate) TableName() string { return "certificates" }
```

### sing-box TLS Config (certificate_path + key_path)

```json
{
  "enabled": true,
  "server_name": "example.com",
  "certificate_path": "/etc/certs/fullchain.pem",
  "key_path": "/etc/certs/privkey.pem"
}
```

Source: [sing-box TLS](https://sing-box.sagernet.org/configuration/shared/tls/)

### Cert API Response (List)

```json
{
  "data": [
    {
      "id": 1,
      "name": "example.com",
      "fullchain_path": "/etc/certs/fullchain.pem",
      "privkey_path": "/etc/certs/privkey.pem",
      "created_at": "2026-02-12T12:00:00Z"
    }
  ]
}
```

### InboundFormModal TLS Section (cert selector)

```tsx
{/* When tls_type === "tls" (VLESS) or protocol === "hysteria2" */}
<div>
  <FieldLabel label="证书" tooltip="选择已保存的证书或手动输入路径" />
  <Select
    value={certId ? String(certId) : "manual"}
    onValueChange={(v) => {
      if (v === "manual") {
        form.setValue("certificate_id", null)
      } else {
        form.setValue("certificate_id", Number(v))
        form.setValue("tls_certificate_path", "")
        form.setValue("tls_key_path", "")
      }
    }}
  >
    <SelectItem value="manual">手动输入</SelectItem>
    {certificates?.map((c) => (
      <SelectItem key={c.id} value={String(c.id)}>{c.name}</SelectItem>
    ))}
  </Select>
</div>
{certId === null && (
  <>
    <Input {...form.register("tls_certificate_path")} placeholder="/etc/certs/fullchain.pem" />
    <Input {...form.register("tls_key_path")} placeholder="/etc/certs/privkey.pem" />
  </>
)}
```

### ConfigGenerator Resolve cert_id

```go
// In vlessToSingBox / hysteria2ToSingBox, when tls is present:
if tls, ok := cfg["tls"].(map[string]any); ok && tls != nil {
    if certID, ok := tls["certificate_id"]; ok {
        if id, ok := toUint(certID); ok {
            cert, err := db.GetCertificateByID(id)
            if err == nil {
                tls["certificate_path"] = cert.FullchainPath
                tls["key_path"] = cert.PrivkeyPath
            }
        }
        delete(tls, "certificate_id")
    }
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Inline paths only | Cert entity + select | Phase 3 | Reusable certs; less duplication |
| 3x-ui CLI cert mgmt | Web UI cert CRUD | Phase 3 | Consistent with panel UX |

**Deprecated/outdated:**
- ACME in Phase 3: Deferred to v2 (CRT-10, CRT-11).

---

## Open Questions

1. **Cert delete when in use**
   - What we know: Inbounds can reference cert by ID
   - What's unclear: Whether to block delete or allow (and fail at apply)
   - Recommendation: Block delete if any inbound references cert; return error with list of affected inbounds

2. **certificate_id in config_json format**
   - What we know: config_json is JSON; we need to store cert reference
   - What's unclear: Use `certificate_id` (numeric) or `certificate_id` (string) for consistency with JSON
   - Recommendation: Store as number in JSON; ConfigGenerator converts to uint for lookup

3. **Certificate name uniqueness**
   - What we know: Name is display label only
   - What's unclear: Whether to enforce unique names
   - Recommendation: Allow duplicate names; id is the unique identifier

---

## Sources

### Primary (HIGH confidence)
- [sing-box TLS](https://sing-box.sagernet.org/configuration/shared/tls/) — certificate_path, key_path, auto-reload
- [sing-box VLESS inbound](https://sing-box.sagernet.org/configuration/inbound/vless/) — tls structure
- [sing-box Hysteria2 inbound](https://sing-box.sagernet.org/configuration/inbound/hysteria2/) — tls required
- internal/core/generator.go — current vlessToSingBox, hysteria2ToSingBox
- internal/db/inbound.go — Inbound model, ConfigJSON
- web/src/components/inbounds/InboundFormModal.tsx — TLS fields, buildConfigJson

### Secondary (MEDIUM confidence)
- .planning/research/ARCHITECTURE.md — certificates table, /api/certs
- .planning/phases/02-inbound-management/02-RESEARCH.md — TLS options, Reality, Hysteria2

### Tertiary (LOW confidence)
- WebSearch 3x-ui certificate management — CLI-based; different UX pattern

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — Phase 1–2 stack; no new deps
- Architecture: HIGH — ARCHITECTURE.md + existing patterns
- Pitfalls: HIGH — derived from config flow and cert-inbound relationship

**Research date:** 2026-02-12
**Valid until:** 30 days (stable sing-box config format)
