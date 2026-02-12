---
phase: 03-certificate-management
plan: 03
subsystem: web
tags: [inbound-form-modal, certificates, tls, cert-selector]

# Dependency graph
requires:
  - phase: 03-01
    provides: Certificate model, ConfigGenerator resolveCertInTLS
  - phase: 03-02
    provides: GET /api/certs, Certificates page
provides:
  - Cert selector in InboundFormModal TLS section for VLESS and Hysteria2
  - Mutual exclusivity: 选择证书 vs 手动输入
affects: [inbound-form-modal, config_json]

# Tech tracking
tech-stack:
  added: []
  patterns: [useQuery for cert list; buildConfigJson/parseConfigFromJson handle certificate_id]

key-files:
  created: []
  modified: [web/src/components/inbounds/InboundFormModal.tsx]

key-decisions:
  - "certificate_id stored in config_json.tls when cert selected; ConfigGenerator resolves to paths"
  - "Select value: certificate_id when set, else 'manual'; paths hidden when cert selected"

# Metrics
duration: ~3 min
completed: 2026-02-12
---

# Phase 03 Plan 03: InboundFormModal TLS Cert Selector Summary

**Certificate selector in InboundFormModal TLS section; admin can choose 选择证书 (dropdown) or 手动输入 (inline paths); certificate_id stored in config_json when cert selected.**

## Performance

- **Duration:** ~3 min
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Schema: certificate_id and tls_cert_mode added to vlessSchema and hysteria2Schema
- useQuery fetches cert list from GET /api/certs when TLS enabled (VLESS tls_type=tls or Hysteria2)
- TLS section: "证书" FieldLabel with Select (手动输入 | cert options); when cert selected hide inline path inputs
- buildConfigJson: when certificate_id present and > 0, set config.tls.certificate_id; else set certificate_path/key_path
- parseConfigFromJson: if tls.certificate_id exists set certificate_id and tls_cert_mode="cert"; else manual and paths
- Form reset: handles certificate_id and tls_cert_mode; clears paths when cert selected
- Mutual exclusivity: switching cert clears paths; switching manual clears certificate_id

## Task Commits

Each task was committed atomically:

1. **Task 1: InboundFormModal TLS cert selector** - `f4126e1` (feat)

## Files Created/Modified

- `web/src/components/inbounds/InboundFormModal.tsx` - Cert selector, buildConfigJson, parseConfigFromJson, form reset

## Decisions Made

- certificate_id stored in config_json.tls; ConfigGenerator (from Plan 01) resolves to paths before sing-box
- certificate_id parsed as number or string from JSON for robustness

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - uses existing /api/certs endpoint.

## Next Phase Readiness

- InboundFormModal fully supports cert selection
- CRT-02 complete: admin can associate certificate with inbound TLS options

## Self-Check: PASSED

- web/src/components/inbounds/InboundFormModal.tsx: FOUND
- Commit f4126e1: FOUND
