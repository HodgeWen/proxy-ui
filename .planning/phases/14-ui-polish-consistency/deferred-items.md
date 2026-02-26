# Deferred Items

## 2026-02-26 (Plan 14-02 execution)

- Full `npm --prefix web run lint` is currently blocked by pre-existing, out-of-scope frontend lint errors in files outside this plan's changed scope:
  - `web/src/components/certificates/CertificateFormModal.tsx`
  - `web/src/components/inbounds/InboundFormModal.tsx`
  - `web/src/components/theme-provider.tsx`
  - `web/src/components/ui/badge.tsx`
  - `web/src/components/ui/button.tsx`
  - `web/src/components/ui/select.tsx`
  - `web/src/components/ui/sidebar.tsx`
  - `web/src/components/ui/tabs.tsx`
  - `web/src/components/users/UserFormModal.tsx`
- Plan 14-02 changes were validated via successful `npm --prefix web run build`.
