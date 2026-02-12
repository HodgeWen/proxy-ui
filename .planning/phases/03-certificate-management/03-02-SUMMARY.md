---
phase: 03-certificate-management
plan: 02
subsystem: api
tags: [chi, react, certificates, cert-crud, rest-api]

# Dependency graph
requires:
  - phase: 03-01
    provides: Certificate model, DB CRUD, ConfigGenerator resolveCertInTLS
provides:
  - GET/POST/PUT/DELETE /api/certs REST API
  - Certificates page with table and add/edit form modal
  - InboundsReferencingCert blocks cert delete when in use
affects: [03-03, inbound-form-modal]

# Tech tracking
tech-stack:
  added: []
  patterns: [certs API follows inbounds pattern; RequireAuth on all cert routes]

key-files:
  created: [internal/api/certs.go, web/src/pages/Certificates.tsx, web/src/components/certificates/CertificateTable.tsx, web/src/components/certificates/CertificateFormModal.tsx]
  modified: [internal/db/certificate.go, internal/api/routes.go, web/src/components/layout/Sidebar.tsx, web/src/routes.tsx]

key-decisions:
  - "Delete cert returns 400 with message listing inbound tags when cert in use"
  - "No config apply on cert mutate (certs used only when inbound config applied)"

patterns-established:
  - "InboundsReferencingCert: iterate inbounds, unmarshal config_json, check tls.certificate_id for float64/int"

# Metrics
duration: ~5 min
completed: 2026-02-12
---

# Phase 03 Plan 02: Certificate CRUD API and Certificates UI Summary

**Certificate REST API with list/create/update/delete; Certificates page with table and form modal; delete blocked when cert referenced by inbound.**

## Performance

- **Duration:** ~5 min
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments

- InboundsReferencingCert in db/certificate.go: lists inbounds whose config_json tls.certificate_id matches certID
- Cert CRUD API: ListCertificatesHandler, GetCertificateHandler, CreateCertificateHandler, UpdateCertificateHandler, DeleteCertificateHandler
- Delete returns 400 "证书正在被以下入站使用: tag1, tag2" when cert in use
- CertificateTable with columns 名称, 证书路径, 私钥路径, 创建时间, 操作 (edit inline, delete in DropdownMenu)
- CertificateFormModal with react-hook-form, zod, name/fullchain_path/privkey_path
- Certificates page with useQuery fetch, add/edit/delete handlers, window.confirm for delete
- Sidebar certificates nav enabled; /certificates route added

## Task Commits

Each task was committed atomically:

1. **Task 1: Certificate CRUD API and block delete when in use** - `f46f523` (feat)
2. **Task 2: Certificates page with table and form modal** - `a2cd67c` (feat)

## Files Created/Modified

- `internal/db/certificate.go` - Add InboundsReferencingCert
- `internal/api/certs.go` - Cert CRUD handlers
- `internal/api/routes.go` - Add /api/certs routes under RequireAuth
- `web/src/components/certificates/CertificateTable.tsx` - Table with edit/delete
- `web/src/components/certificates/CertificateFormModal.tsx` - Add/edit form
- `web/src/pages/Certificates.tsx` - Page with useQuery and handlers
- `web/src/components/layout/Sidebar.tsx` - Enable certificates nav
- `web/src/routes.tsx` - Add certificates route

## Decisions Made

- Delete blocked when cert in use; 400 response with comma-separated inbound tags
- No config apply on cert mutate (certs are only used when inbound config is applied)
- certificate_id in config_json parsed as float64 (JSON default); InboundsReferencingCert handles float64, int, int64

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Cert CRUD API and Certificates page complete
- Ready for 03-03 (InboundFormModal cert selector)

## Self-Check: PASSED

- internal/api/certs.go: FOUND
- web/src/pages/Certificates.tsx: FOUND
- web/src/components/certificates/CertificateTable.tsx: FOUND
- web/src/components/certificates/CertificateFormModal.tsx: FOUND
- Commits f46f523, a2cd67c: FOUND

---
*Phase: 03-certificate-management*
*Completed: 2026-02-12*
