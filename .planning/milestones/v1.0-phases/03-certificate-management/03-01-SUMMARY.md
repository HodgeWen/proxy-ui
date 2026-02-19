---
phase: 03-certificate-management
plan: 01
subsystem: database
tags: [gorm, certificate, sing-box, tls, config-generator]

# Dependency graph
requires:
  - phase: 02-inbound-management
    provides: Inbound model, ConfigGenerator, vlessToSingBox, hysteria2ToSingBox
provides:
  - Certificate model and CRUD (ListCertificates, GetCertificateByID, CreateCertificate, UpdateCertificate, DeleteCertificate)
  - ConfigGenerator resolve certificate_id to certificate_path and key_path in VLESS and Hysteria2 TLS
affects: [03-02, 03-03, inbound-form-modal]

# Tech tracking
tech-stack:
  added: []
  patterns: [Certificate CRUD follows Inbound pattern; resolveCertInTLS modifies tls map in-place]

key-files:
  created: [internal/db/certificate.go]
  modified: [internal/db/db.go, internal/core/generator.go]

key-decisions:
  - "certificate_id in config_json takes precedence; inline paths used when absent (backward compat)"
  - "toUint helper handles float64, int, int64, json.Number for JSON config parsing"

patterns-established:
  - "resolveCertInTLS: in-place tls map modification; delete certificate_id before emitting (sing-box unaware)"

# Metrics
duration: ~2 min
completed: 2026-02-12
---

# Phase 03 Plan 01: Certificate Model and ConfigGenerator Resolution Summary

**Certificate model with CRUD; ConfigGenerator resolves certificate_id to fullchain_path and key_path when emitting sing-box TLS config for VLESS and Hysteria2.**

## Performance

- **Duration:** ~2 min
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Certificate model with ID, Name, FullchainPath, PrivkeyPath; certificates table via AutoMigrate
- ListCertificates, GetCertificateByID, CreateCertificate, UpdateCertificate, DeleteCertificate
- ConfigGenerator resolveCertInTLS: when tls.certificate_id present, fetches cert from DB, injects certificate_path and key_path, removes certificate_id before emitting
- Backward compat: inline certificate_path/key_path in config_json used when certificate_id absent or lookup fails

## Task Commits

Each task was committed atomically:

1. **Task 1: Certificate model and CRUD** - `fac48b0` (feat)
2. **Task 2: ConfigGenerator resolve certificate_id** - `5f062a6` (feat)

## Files Created/Modified

- `internal/db/certificate.go` - Certificate struct, CRUD
- `internal/db/db.go` - Add Certificate to AutoMigrate
- `internal/core/generator.go` - toUint, resolveCertInTLS; applied in vlessToSingBox and hysteria2ToSingBox

## Decisions Made

- certificate_id stored as number in config_json; toUint supports float64, int, int64, json.Number for JSON unmarshaling
- resolveCertInTLS modifies tls map in-place; always deletes certificate_id before emitting (sing-box does not know this field)
- Backward compat preserved: existing inline paths used when certificate_id absent or lookup fails

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Certificate model and CRUD operational
- ConfigGenerator resolves certificate_id to paths; backward compat with inline paths preserved
- Ready for 03-02 (Cert REST API) and 03-03 (InboundFormModal cert selector)

## Self-Check: PASSED

- internal/db/certificate.go: FOUND
- 03-01-SUMMARY.md: FOUND
- Commits fac48b0, 5f062a6: FOUND

---
*Phase: 03-certificate-management*
*Completed: 2026-02-12*
