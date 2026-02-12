# Phase 9: Certificate Config Sync — Verification Checklist

**Target:** Verify Cert update → Generate → Apply → Restart闭环，满足 v1.0-MILESTONE-AUDIT 集成缺口闭环。

**Audit gap closed:** "Certificate update does not trigger config regeneration — inbounds referencing updated cert keep old paths until another mutation"

---

## Verification Items

| ID | Scope | Description | Status |
|----|-------|--------------|--------|
| 09-01-BE | Backend | UpdateCertificateHandler 更新证书后自动 Generate → ApplyConfig → Restart | ☐ |
| 09-01-RB | Backend | ApplyConfig 失败时证书回滚，API 返回 400 `{"error": "..."}` | ☐ |
| 09-02-FE | Frontend | ApplyConfig 失败时 Modal 内显示 checkError（sing-box check 输出） | ☐ |
| 09-03-LINK | Linkage | 证书路径更新后，引用该证书的入站立即生效，无需编辑入站或用户 | ☐ |

---

## Executable Steps

### Prerequisites

1. **Create certificate A**
   - POST `/api/certs` with valid `fullchain_path` and `privkey_path` (e.g. `/etc/ssl/certs/example.pem`, `/etc/ssl/private/example.key`)
   - Record returned `id` as `CERT_A_ID`

2. **Create VLESS inbound referencing cert A**
   - POST `/api/inbounds` with TLS enabled, `certificate_id: CERT_A_ID` (or via config_json)
   - Record inbound `id` for later verification

### Step 1: Update certificate paths

- **Action:** `PUT /api/certs/:id` with updated `fullchain_path` and `privkey_path` (e.g. new valid paths)
- **Expected:** 200 OK, JSON body with updated certificate
- **Verifies:** 09-01-BE (handler accepts update)

### Step 2: Verify generated config

- **Action:** Read sing-box config file (from `SINGBOX_CONFIG_PATH`) or `GET /api/core/config` (if available)
- **Check:** For the inbound from prerequisites, the generated JSON has `tls.certificate_path` and `tls.key_path` equal to the **new** paths (not the old ones)
- **Expected:** Inbound TLS block reflects the updated certificate paths
- **Verifies:** 09-01-BE (Generate runs), 09-03-LINK (paths in config updated)

### Step 3: Verify sing-box restart

- **Action:** Check sing-box process or `GET /api/core/status`
- **Check:** Process exists and is running; restart timestamp/uptime suggests recent restart
- **Expected:** sing-box is running with the new config
- **Verifies:** 09-01-BE (Restart executed)

### Step 4: Verify rollback on invalid path

- **Action:** `PUT /api/certs/:id` with invalid path (e.g. `fullchain_path: "/nonexistent/cert.pem"`)
- **Expected:** 400 Bad Request, JSON body `{"error": "..."}` with sing-box check output
- **Action:** `GET /api/certs/:id`
- **Expected:** Certificate still has **previous** (valid) paths — rollback successful
- **Verifies:** 09-01-RB (rollback, 400 JSON)

### Step 5: Verify frontend error display (optional, UI)

- **Action:** In CertificateFormModal, submit with invalid path (e.g. `/nonexistent/cert.pem`)
- **Expected:** Modal shows red error block with sing-box check output (checkError), no toast-only failure
- **Verifies:** 09-02-FE (checkError display)

---

## Success Criteria

- [ ] All steps above pass
- [ ] Audit gap "Cert update → Generate → Apply" is marked closed in v1.0-MILESTONE-AUDIT or equivalent tracking

---

## Notes

- **Step 2:** If `GET /api/core/config` is not implemented, read the config file directly from `SINGBOX_CONFIG_PATH` env.
- **Step 3:** `pgrep sing-box` or equivalent can confirm process; `/api/core/status` may return running state.
- **Step 4:** Use a clearly invalid path (e.g. `/nonexistent/cert.pem`) to force ApplyConfig failure.

---

*Phase: 09-certificate-config-sync*  
*Closes: v1.0-MILESTONE-AUDIT integration gap — Cert update → Generate → Apply*
