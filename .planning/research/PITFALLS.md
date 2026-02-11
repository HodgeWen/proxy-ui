# Pitfalls Research: s-ui

**Domain:** sing-box proxy management panel  
**Researched:** 2026-02-11  
**Mode:** Pitfalls dimension — common mistakes, critical errors, security concerns

---

## Critical Pitfalls

### 1. Security

#### 1.1 Panel Authentication Bypass / Weak Auth

**What goes wrong:** Panel admin interface is accessible without proper authentication or uses weak/default credentials. Attackers gain full control over proxy infrastructure.

**Why it happens:** Default passwords not changed on first run; no forced password change; session/token handling bugs; no rate limiting on login.

**Consequences:** Full takeover of proxy server; user credentials leaked; subscription links stolen; malicious config injection.

**Prevention:**
- Require password change on first login (no default credentials)
- Use bcrypt/argon2 for password hashing; salt per user
- Implement session timeout and secure cookie flags (HttpOnly, Secure, SameSite)
- Add login rate limiting (e.g., 5 attempts per 15 min)
- Consider 2FA for admin (3x-ui attempted but had implementation issues)

**Detection:** Security audit; test default credentials; check if session survives logout/re-login.

**Phase:** Setup / System & Operations

---

#### 1.2 HTTPS Drop to HTTP on Panel Restart

**What goes wrong:** After panel restart or config change, users are redirected from HTTPS to unencrypted HTTP. Authentication tokens and panel paths can be exposed over the wire.

**Why it happens:** Panel stores or derives scheme from wrong source (e.g., request port instead of `X-Forwarded-Proto`); reverse proxy setup with non-standard ports breaks scheme detection.

**Consequences:** Credentials and tokens sent in plaintext; man-in-the-middle attacks; session hijacking.

**Evidence:** 3x-ui Issue #2857 — panel redirects to HTTP after restart when using reverse proxy with non-standard ports (e.g., `https://example.com:25560`). Project has not prioritized fix across versions.

**Prevention:**
- Always respect `X-Forwarded-Proto` and `X-Forwarded-Host` when behind reverse proxy
- Generate all redirect URLs and links using configured base URL (env var or config), not request headers
- Test restart flow behind nginx/Caddy with non-standard ports before release

**Detection:** Deploy behind reverse proxy, restart panel, check if redirect stays HTTPS.

**Phase:** Deployment / System & Operations

---

#### 1.3 sing-box Core Authentication Bypass (CVE-2023-43644)

**What goes wrong:** SOCKS5 inbounds with user authentication can be bypassed with crafted requests. Attackers gain proxy access without valid credentials.

**Why it happens:** sing-box core bug (fixed in 1.4.5 / 1.5.0-rc.4+). Panel does not control this; it is a core dependency.

**Consequences:** Unauthorized proxy usage; traffic exfiltration; abuse.

**Prevention:**
- **Pin sing-box version:** Use sing-box ≥ 1.4.5 or ≥ 1.5.0-rc.5
- Document minimum required sing-box version in install guide
- Automatically check sing-box version on startup and warn if vulnerable
- Do not expose SOCKS5 inbounds to untrusted networks if unable to update

**Detection:** Check `sing-box version` output; compare against CVE advisory.

**Phase:** sing-box Integration / Inbound Management

---

#### 1.4 Listen Address Over-Exposure

**What goes wrong:** Panel or sing-box inbounds listen on `0.0.0.0`, exposing services on all interfaces. Panel or proxy becomes reachable from unintended networks.

**Why it happens:** Default config uses `0.0.0.0` for convenience; no security guidance in UI.

**Consequences:** Panel accessible from internet when meant for localhost; proxy inbounds reachable from LAN when only WAN intended.

**Prevention:**
- Panel: Default to `127.0.0.1` for panel listen; document reverse proxy setup for remote access
- Inbounds: Provide `listen` field with sensible default (`127.0.0.1` for local-only, `0.0.0.0` only when explicitly chosen)
- Use `bind_interface` (sing-box 1.12+) to restrict to specific interface when needed

**Detection:** Check listen address in config; verify firewall rules.

**Phase:** Inbound Management / Deployment

---

#### 1.5 Subscription Token Exposure

**What goes wrong:** Subscription URLs contain long-lived tokens that, if leaked (logs, Referer, browser history, shared screenshots), give full access to user's proxy config.

**Why it happens:** Tokens in URL; no rotation; logged or cached.

**Consequences:** Anyone with the link can import the subscription; hard to revoke without changing user identity.

**Prevention:**
- Use per-user UUID or opaque token; avoid embedding admin-level secrets
- Support token rotation (regenerate subscription link) without changing user identity
- Ensure subscription endpoint is not logged; avoid Referer leakage (use POST or separate domain)
- Consider short-lived tokens if subscription is fetched infrequently (adds complexity)

**Detection:** Audit where subscription URLs appear (logs, analytics, error pages).

**Phase:** Subscription System / User Management

---

### 2. sing-box Integration

#### 2.1 Deprecated / Unknown Field Errors

**What goes wrong:** Panel generates config with deprecated or invalid fields. sing-box fails with `json: unknown field "type"` or similar, or silently ignores deprecated options that will break in future versions.

**Why it happens:** sing-box frequently deprecates fields (legacy inbound fields, GeoIP/Geosite, WireGuard outbound, etc.); panel uses outdated schema.

**Evidence:** sing-box deprecated list includes: legacy `sniff`/`domain_strategy` on inbounds (removal 1.13), GeoIP/Geosite (1.12), TUN address fields merge (1.12), rule item renames. Using `"type": "naive"` in wrong section causes validation errors.

**Consequences:** Config fails `sing-box check`; service won't start; future sing-box upgrade breaks panel.

**Prevention:**
- Run `sing-box check` before every apply/restart
- Track sing-box version compatibility; test against latest stable
- Use official JSON schema or generated types from sing-box schema
- Subscribe to [sing-box deprecated list](https://sing-box.sagernet.org/deprecated/) and migration guides

**Detection:** `sing-box check -c config.json` before deploy; CI against multiple sing-box versions.

**Phase:** Inbound Management / Configuration Management

---

#### 2.2 Protocol-Specific Config Mistakes

**What goes wrong:** VLESS or Hysteria2 inbound misconfigured; clients cannot connect or auth fails.

**VLESS:** Incorrect UUID format; missing `flow`; wrong transport (ws/grpc) params.

**Hysteria2:**
- Port range: docs sometimes show `"2080:3000"` (colon) — **correct is** `"2080-3000"` (dash). Colon causes "bad port range" error.
- Auth: Official Hysteria2 supports `userpass` (`username:password`); sing-box does not. Must put `username:password` as the actual password value.
- `up_mbps`/`down_mbps` conflict with `ignore_client_bandwidth` — set both causes confusion.
- Salamander obfuscation: missing or wrong `obfs.password` causes "salamander: packet too short".
- Users array: every user needs both `name` and `password`.

**Prevention:**
- Implement protocol-specific validation (port range format, UUID v4, Hysteria2 user structure)
- Provide field tooltips explaining sing-box vs official program differences
- Use smart defaults (e.g., BBR vs Hysteria CC based on bandwidth limits)

**Detection:** Test each protocol with real client after config change.

**Phase:** Inbound Management

---

#### 2.3 Config File Write Corruption / Race

**What goes wrong:** Panel writes config.json while sing-box is reading it, or write is interrupted. Result: corrupted config, sing-box crash, or inconsistent state.

**Why it happens:** No atomic write; no lock; concurrent edits (e.g., cert renewal + user edit).

**Prevention:**
- Write to temp file, then `rename` (atomic on POSIX)
- Use file lock (e.g., `flock`) during write
- Serialize config updates (single writer goroutine or mutex)
- After write, run `sing-box check` before signaling restart

**Detection:** Stress test concurrent operations; check for partial writes.

**Phase:** Configuration Management

---

### 3. Configuration Management

#### 3.1 JSON Schema / Version Drift

**What goes wrong:** Panel output does not match current sing-box schema. New fields ignored; old fields rejected.

**Why it happens:** sing-box adds/removes fields per version; panel hardcodes structure.

**Prevention:**
- Validate generated config with `sing-box check` (required)
- Consider generating Go structs from sing-box schema
- Document minimum supported sing-box version

**Phase:** Inbound Management / sing-box Integration

---

#### 3.2 Manual Config Overwrite

**What goes wrong:** User edits config.json by hand; panel overwrites it on next save, losing changes. Or panel cannot parse user-edited config.

**Prevention:**
- Document that panel is the source of truth; manual edits will be overwritten
- Or: implement merge/import of manual config (higher complexity)
- Validate any manual config before overwriting

**Phase:** Configuration Management

---

### 4. Certificate Management

#### 4.1 ACME Rate Limits (Let's Encrypt)

**What goes wrong:** Too many cert requests (new domains, retries, bugs) hit Let's Encrypt rate limits (e.g., 300 orders per 3 hours). Account or domain gets blocked.

**Why it happens:** Retry loops; testing against production; multiple domains in quick succession.

**Prevention:**
- Use staging API (`acme-staging-v02`) for dev/test
- Implement backoff and circuit breaker for ACME failures
- Avoid requesting certs for the same domain repeatedly; cache success
- One ACME account per deployment; do not create new accounts per request

**Detection:** Monitor for 429 responses from ACME; log rate limit headers.

**Phase:** Certificate Management

---

#### 4.2 Proxy / Network Blocking ACME

**What goes wrong:** Panel runs behind HTTP proxy; ACME client (certmagic, acme.sh) does not respect proxy env vars. ACME requests fail.

**Evidence:** Proxmox Mail Gateway reports ACME via GUI fails without `https_proxy`; CLI works when env is set.

**Prevention:**
- Ensure certmagic or chosen ACME client uses `HTTP_PROXY`/`HTTPS_PROXY` when set
- Document proxy requirements for ACME
- If behind strict proxy, consider manual cert upload as fallback

**Phase:** Certificate Management / Deployment

---

#### 4.3 Cert Renewal Failure Silently Ignored

**What goes wrong:** Renewal cron fails (network, permissions, rate limit) but panel does not alert. Certs expire; TLS breaks.

**Prevention:**
- Run renewal well before expiry (e.g., 30 days)
- Log renewal attempts and results
- Optional: in-app warning when cert expires in < 14 days
- Consider webhook/email on renewal failure (defer to v2)

**Phase:** Certificate Management

---

### 5. Traffic Statistics

#### 5.1 Core API Mismatch (gRPC vs xhttp)

**What goes wrong:** sing-box has no built-in gRPC API like Xray. Panels that assume Xray-style gRPC will fail. If sing-box adds alternative API (e.g., xhttp), panel must match.

**Evidence:** 3x-ui Issue #2806 — when Xray uses xhttp instead of gRPC, panel loses connection; gRPC timeouts ("context deadline exceeded") cause Xray core to crash or restart.

**Consequences:** No traffic stats; panel may crash core; panel becomes inaccessible.

**Prevention:**
- sing-box is config-file driven; no gRPC. Traffic stats require: log parsing, custom API, or future sing-box feature.
- If using log parsing: ensure log format is stable; handle rotation.
- If sing-box adds stats API: verify compatibility before depending on it.
- Avoid tight coupling that can crash the core

**Detection:** Verify stats source (logs vs API); test with sing-box restarts.

**Phase:** Traffic & Statistics

---

#### 5.2 Stats Accuracy / Double Counting

**What goes wrong:** Per-user or per-inbound traffic is wrong: double-counted, under-counted, or delayed.

**Why it happens:** Log parsing errors; aggregation logic bugs; connection counting vs byte counting confusion.

**Prevention:**
- Document stats methodology (bytes from logs vs API)
- Use idempotent aggregation (e.g., sum by session ID, not by connection)
- Consider eventual consistency acceptable; avoid real-time guarantee unless supported by core

**Phase:** Traffic & Statistics

---

### 6. Deployment

#### 6.1 Reverse Proxy Scheme/Port Handling

**What goes wrong:** Same as 1.2 — redirect to HTTP after restart when behind reverse proxy.

**Phase:** Deployment

---

#### 6.2 Docker Volume Permissions

**What goes wrong:** Config or data volumes mounted with root ownership; panel runs as non-root and cannot write.

**Prevention:**
- Create dirs with correct ownership in Dockerfile (`chown`)
- Or use init container to fix permissions before main container starts
- Document required volume paths and permissions

**Phase:** Deployment

---

#### 6.3 Docker TUN Mode Failures

**What goes wrong:** sing-box TUN inbound fails in Docker despite `--privileged`, `--cap-add NET_ADMIN`, `--device=/dev/net/tun`. TUN has known limitations in containers.

**Prevention:**
- Document that TUN is not fully supported in Docker; recommend bare metal or VM for TUN
- s-ui v1 focuses on VLESS + Hysteria2 (no TUN required); mention TUN as advanced/unsupported in Docker

**Phase:** Deployment

---

### 7. UX/UI

#### 7.1 Form Validation Too Late

**What goes wrong:** User fills long form, submits; backend rejects (e.g., port conflict, invalid UUID). User loses input; frustration.

**Prevention:**
- Client-side validation for format (UUID, port range, URL)
- Server returns clear, field-level errors
- Preserve form state on validation failure

**Phase:** All form phases (Inbound, User, Cert)

---

#### 7.2 Filter/State Reset on Navigation

**What goes wrong:** User filters large list (e.g., "ended users"); navigates to detail or refresh; filter resets.

**Evidence:** 3x-ui v2.8.7 — user filter resets when viewing ended/depleted users.

**Prevention:**
- Persist filter state in URL query params or sessionStorage
- Restore on mount

**Phase:** User Management / Traffic & Statistics

---

## Common Mistakes from Existing Panels

| Panel   | Mistake                                           | Impact                            |
|---------|----------------------------------------------------|-----------------------------------|
| 3x-ui   | HTTP redirect after restart (reverse proxy)        | Token/credential exposure         |
| 3x-ui   | Traffic stats gRPC mismatch with xhttp             | Core crash, panel inaccessible    |
| 3x-ui   | User filter reset on navigation                    | Poor UX for large user lists      |
| 3x-ui   | 2FA implementation bugs                            | Incomplete auth hardening         |
| Generic | Default admin credentials                          | Unauthorized access               |
| Generic | listen 0.0.0.0 by default                          | Over-exposure                     |
| sing-box | Port range `:` vs `-` (Hysteria2)                 | Config error, "bad port range"    |
| sing-box | Deprecated inbound fields                         | Future upgrade breakage           |

---

## Prevention Strategies Summary

| Pitfall Category          | Top Prevention                                                     |
|---------------------------|--------------------------------------------------------------------|
| Security                  | No default creds; respect X-Forwarded-Proto; pin sing-box version |
| sing-box Integration      | Run `sing-box check` before apply; track deprecated list          |
| Configuration Management  | Atomic write; file lock; validate before restart                 |
| Certificate Management    | Staging for dev; proxy env for ACME; renewal monitoring           |
| Traffic Statistics        | Do not assume Xray-style API; document stats source                |
| Deployment                | Test reverse proxy restart; document Docker limits                 |
| UX/UI                     | Client + server validation; persist filter state                  |

---

## Phase Mapping

| Phase                    | Critical Pitfalls to Address                                         |
|-------------------------|----------------------------------------------------------------------|
| **Setup / Infrastructure** | 1.1 (auth), 1.2 (HTTPS redirect), 1.4 (listen), 6.1, 6.2            |
| **Inbound Management**  | 1.3 (CVE), 2.1 (deprecated), 2.2 (protocol), 3.1, 3.2               |
| **Certificate Management** | 4.1 (rate limit), 4.2 (proxy), 4.3 (renewal)                        |
| **User Management**     | 1.5 (subscription token), 7.1 (validation), 7.2 (filter)            |
| **Subscription System** | 1.5 (token exposure), 7.1                                          |
| **Traffic & Statistics** | 5.1 (API mismatch), 5.2 (accuracy), 7.2                             |
| **Deployment**           | 1.2, 6.1, 6.2, 6.3                                                 |

---

## Sources

| Source | Type | Confidence |
|--------|------|------------|
| [CVE-2023-43644 (sing-box)](https://osv.dev/vulnerability/GHSA-r5hm-mp3j-285g) | Advisory | HIGH |
| [3x-ui #2857 Panel restart HTTP redirect](https://github.com/MHSanaei/3x-ui/issues/2857) | GitHub | HIGH |
| [3x-ui #2806 Traffic stats / gRPC](https://github.com/MHSanaei/3x-ui/issues/2806) | GitHub | HIGH |
| [3x-ui #2786 2FA](https://github.com/MHSanaei/3x-ui/issues/2786) | GitHub | HIGH |
| [sing-box listen fields](https://sing-box.sagernet.org/configuration/shared/listen/) | Official | HIGH |
| [sing-box Hysteria2 inbound](https://sing-box.sagernet.org/configuration/inbound/hysteria2/) | Official | HIGH |
| [sing-box deprecated list](https://sing-box.sagernet.org/deprecated/) | Official | HIGH |
| [sing-box config unknown field](https://github.com/SagerNet/sing-box/issues/620) | GitHub | HIGH |
| [Let's Encrypt rate limits](https://letsencrypt.org/docs/rate-limits) | Official | HIGH |
| [sing-box port range syntax](https://github.com/SagerNet/sing-box/issues/2416) | GitHub | HIGH |
| [Hysteria2 salamander](https://github.com/SagerNet/sing-box/issues/898) | GitHub | HIGH |
| [Docker TUN permission](https://github.com/SagerNet/sing-box/issues/2331) | GitHub | MEDIUM |
| Proxmox ACME proxy (WebSearch) | Community | MEDIUM |
