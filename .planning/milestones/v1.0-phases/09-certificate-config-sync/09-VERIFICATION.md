---
phase: 09-certificate-config-sync
verified: "2026-02-12T00:00:00Z"
status: passed
score: 4/4 must-haves verified
---

# Phase 9: Certificate Config Sync Verification Report

**Phase Goal:** 修复证书更新后配置未联动问题，确保更新证书路径后立即重生成配置并应用到 sing-box

**Verified:** 2026-02-12
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Success Criteria (from ROADMAP.md)

1. UpdateCertificateHandler 在更新证书后自动执行 Generate -> ApplyConfig -> Restart
2. 若配置应用失败，证书更新会回滚，API 返回明确错误信息
3. 关联该证书的入站在证书更新后立即生效，无需额外入站/用户变更触发

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | UpdateCertificateHandler 在更新证书后自动执行 Generate -> ApplyConfig -> Restart | ✓ VERIFIED | certs.go:146–165: `path := configPath(panelCfg)`, `gen.Generate()`, `core.ApplyConfig(path, cfg)`, `pm.Restart(path)` after db.UpdateCertificate(c) |
| 2 | ApplyConfig 失败时证书回滚，API 返回 400 `{"error": "..."}` | ✓ VERIFIED | certs.go:155–161: `db.UpdateCertificate(old)` on ApplyConfig err, `w.WriteHeader(http.StatusBadRequest)`, `json.Encode(map[string]string{"error": err.Error()})` |
| 3 | 关联该证书的入站在证书更新后立即生效，无需额外入站/用户变更触发 | ✓ VERIFIED | Flow: DB updated first → Generate reads DB (resolveCertInTLS calls db.GetCertificateByID → current paths) → ApplyConfig → Restart. generator.go:241–247 |
| 4 | 前端 400 错误时 Modal 显示 checkError（sing-box check 输出） | ✓ VERIFIED | CertificateFormModal.tsx:82 checkError state, 125–128 `res.status === 400 && data.error` → setCheckError, 148–152 display block |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/api/certs.go` | UpdateCertificateHandler with Generate/ApplyConfig/Restart chain | ✓ VERIFIED | Lines 113–167: panelCfg param, full chain, rollback on Generate/ApplyConfig failure |
| `internal/api/routes.go` | cfg passed to UpdateCertificateHandler | ✓ VERIFIED | Line 50: `UpdateCertificateHandler(sm, cfg)` |
| `web/src/components/certificates/CertificateFormModal.tsx` | checkError for 400 | ✓ VERIFIED | checkError state, 400 parse, styled error block |
| `09-VERIFICATION.md` | Verification checklist | ✓ VERIFIED | Exists with executable steps |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| internal/api/certs.go | internal/core | ConfigGenerator.Generate | ✓ WIRED | Line 148: `gen := &core.ConfigGenerator{}`; 149: `cfg, err := gen.Generate()` |
| internal/api/certs.go | internal/core | ApplyConfig | ✓ WIRED | Line 155: `core.ApplyConfig(path, cfg)` |
| internal/api/certs.go | internal/core | NewProcessManagerFromConfig | ✓ WIRED | Line 162: `pm := core.NewProcessManagerFromConfig(panelCfg)`; 163: `pm.Restart(path)` |
| CertificateFormModal.tsx | /api/certs | fetch PUT, 400 parse | ✓ WIRED | 114–128: fetch, `res.status === 400 && data.error` → setCheckError |

### Requirements Coverage

| Requirement | Status | Blocking Issue |
|-------------|--------|----------------|
| CRT-01, CRT-02 (cert paths in TLS reflected) | ✓ SATISFIED | Phase 9 closes audit gap: cert update → Generate → Apply → Restart |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | None | — | — |

### Human Verification Required

None. Automated checks cover all must-haves. Optional manual confirmation:

1. **Invalid path rollback:** PUT `/api/certs/:id` with `/nonexistent/cert.pem` → 400, GET shows old paths.
2. **UI error display:** Submit invalid path in CertificateFormModal → red error block with check output.

---

_Verified: 2026-02-12_
_Verifier: Claude (gsd-verifier)_
