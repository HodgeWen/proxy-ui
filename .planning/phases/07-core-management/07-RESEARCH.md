# Phase 7: sing-box Core Management - Research

**Researched:** 2026-02-12
**Domain:** sing-box binary update, GitHub Releases API, rollback, dashboard integration
**Confidence:** HIGH

## Summary

Phase 7 adds one-click sing-box core update and rollback from the panel. The backend fetches releases from GitHub API (`/repos/SagerNet/sing-box/releases`), detects OS/ARCH via `runtime.GOOS`/`runtime.GOARCH`, downloads the matching `sing-box-{version}-{os}-{arch}.tar.gz` asset, extracts the binary, replaces it atomically (with backup for rollback), and restarts sing-box. The update flow must stop sing-box before replacing the binary and start it after; use temp file + atomic rename (like ApplyConfig) for the binary. The panel currently invokes `sing-box` via PATH; for managed updates, introduce a configurable binary path (e.g. `SINGBOX_BINARY_PATH` env or data dir) so the panel owns the binary location.

**Primary recommendation:** Extend core layer with `CoreUpdater`: fetch releases, map GOOS/GOARCH to sing-box asset names, download→extract→backup→atomic replace→restart. Use stop→replace→start flow; auto-rollback on post-update check failure. Keep at least the previous binary for manual rollback.

---

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions

- 从 sing-box 官方 GitHub Releases API 获取可用版本列表
- 一键更新到最新版本（无需从列表中手动选择特定版本）
- 展示所有版本（包括预发布版），但清晰标记 stable / pre-release
- 服务端自动检测 OS/ARCH，下载对应平台的二进制文件

### 版本管理界面

- 嵌入仪表盘页面，作为 Dashboard 的一个板块展示
- 展示当前安装版本 + 最新可用版本
- 点击更新按钮后弹出确认对话框，确认后执行
- Sidebar 或仪表盘上显示新版本可用的徽标提示

### Claude's Discretion

- 回滚机制：保留几个旧版本、回滚触发方式、自动 vs 手动回滚
- 更新安全：更新过程中 sing-box 服务的处理流程（停止→替换→启动）
- 进度展示：步骤进度 vs spinner
- 失败处理：自动回滚 + 通知 vs 仅通知
- 更新后验证：是否检查新版本启动成功和配置有效

### Deferred Ideas (OUT OF SCOPE)

- 自动更新调度
- 配置迁移

</user_constraints>

---

## Standard Stack

### Core (existing)

| Library         | Version | Purpose                          | Why Standard                          |
| --------------- | ------- | -------------------------------- | ------------------------------------- |
| Go stdlib       | —       | `runtime.GOOS`, `runtime.GOARCH` | Platform detection at compile/runtime |
| `archive/tar`   | stdlib  | Extract tar.gz                   | No external deps; standard for tar.gz |
| `compress/gzip` | stdlib  | Decompress gzip                  | tar.gz extraction                     |
| `net/http`      | stdlib  | HTTP client                      | GitHub API, binary download           |
| `encoding/json` | stdlib  | JSON parsing                     | GitHub API response                   |

### Supporting

| Library        | Version | Purpose       | When to Use                       |
| -------------- | ------- | ------------- | --------------------------------- |
| Chi            | v5.x    | HTTP router   | API routes (existing)             |
| TanStack Query | v5      | Server state  | Status, versions, update mutation |
| shadcn Dialog  | —       | Confirm modal | Update confirmation               |
| Sonner         | —       | Toast         | Success/error feedback            |

### Alternatives Considered

| Instead of         | Could Use                     | Tradeoff                                     |
| ------------------ | ----------------------------- | -------------------------------------------- |
| stdlib archive/tar | `helm.sh/helm/v3/pkg/archive` | stdlib sufficient; no extra deps             |
| GitHub API         | GitHub GraphQL                | REST simpler; no auth needed for public repo |

**No new npm/go packages required** — stdlib covers download, decompress, extract.

---

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── core/
│   ├── process.go       # existing ProcessManager
│   ├── config.go        # existing ApplyConfig
│   └── updater.go       # NEW: CoreUpdater (fetch, download, extract, replace, rollback)
├── api/
│   └── core.go          # extend: add /versions, /update, /rollback handlers
web/src/
├── pages/
│   └── Dashboard.tsx    # extend: add CoreVersionCard block
└── components/
    └── core/            # optional: CoreVersionCard, UpdateConfirmDialog
```

### Pattern 1: Stop → Replace → Start

**What:** Stop sing-box, replace binary atomically, start sing-box. Avoids running process holding open file descriptor on replaced binary.

**When to use:** Any binary update for a service the panel controls.

**Example:**

```go
// Pseudo-flow (internal/core/updater.go)
func (u *CoreUpdater) Update(version string) error {
    pm := NewProcessManager()
    // 1. Stop
    exec.Command("pkill", "-x", "sing-box").Run()
    time.Sleep(100 * time.Millisecond) // allow process exit
    // 2. Backup current
    if err := u.backupCurrent(); err != nil { return err }
    // 3. Download + extract to temp
    tmpPath, err := u.downloadAndExtract(version)
    if err != nil { u.restoreBackup(); return err }
    // 4. Atomic replace
    if err := os.Rename(tmpPath, u.binaryPath); err != nil {
        u.restoreBackup(); return err
    }
    os.Chmod(u.binaryPath, 0755)
    // 5. Start
    return pm.Restart(pm.ConfigPath())
}
```

### Pattern 2: GitHub Releases API

**What:** `GET https://api.github.com/repos/SagerNet/sing-box/releases` returns releases with `tag_name`, `prerelease`, `assets[]`. Each asset has `name` and `browser_download_url`.

**Example response shape:**

```json
{
  "tag_name": "v1.12.21",
  "prerelease": false,
  "assets": [
    {
      "name": "sing-box-1.12.21-linux-amd64.tar.gz",
      "browser_download_url": "https://github.com/..."
    }
  ]
}
```

**Asset naming:** `sing-box-{version}-{os}-{arch}.tar.gz` where version strips `v` prefix. Map `runtime.GOOS`/`runtime.GOARCH` to sing-box convention: `linux`/`amd64` → `linux-amd64`; `linux`/`arm64` → `linux-arm64`; `darwin`/`arm64` → `darwin-arm64`; `darwin`/`amd64` → `darwin-amd64`; `windows`/`amd64` → `windows-amd64`. Fallback for `386` → `386`, `arm` → `armv7` (check asset list for exact names).

### Pattern 3: Atomic Binary Replace

**What:** Write new binary to temp file, then `os.Rename(tmpPath, targetPath)`. On POSIX this is atomic. ProcessManager must use explicit binary path (not PATH lookup) when running sing-box.

**When to use:** Any file replacement where partial write would corrupt target.

### Anti-Patterns to Avoid

- **Replacing binary while process running:** Old process may hold fd; behavior undefined. Always stop first.
- **No backup before replace:** Rollback impossible. Always copy current binary to backup dir before overwrite.
- **Relying on PATH for managed binary:** Panel cannot update binary in arbitrary PATH locations (e.g. /usr/bin). Use dedicated dir + explicit path.

---

## Don't Hand-Roll

| Problem            | Don't Build          | Use Instead                                | Why                                                                |
| ------------------ | -------------------- | ------------------------------------------ | ------------------------------------------------------------------ |
| Tar.gz extraction  | Shell `tar -xzf`     | `archive/tar` + `compress/gzip`            | Cross-platform; no shell dependency                                |
| GitHub API polling | Custom cache         | TanStack Query cache + refetch             | Already in stack; handles dedup                                    |
| Version comparison | Custom semver parser | Simple string compare or `version` package | "Latest" = first in API; pre-release flagged by `prerelease` field |

**Key insight:** GitHub API returns releases in reverse chronological order. "Latest" = first non-draft release (or first including prerelease if UI shows both). No complex semver needed for one-click "update to latest".

---

## Common Pitfalls

### Pitfall 1: Binary Path Ownership

**What goes wrong:** Panel updates a binary in `/usr/local/bin` but runs as non-root; permission denied. Or binary is in PATH but panel doesn't know which path—can't replace.

**Why it happens:** ProcessManager uses `exec.LookPath("sing-box")`; found path may be read-only or system-managed.

**How to avoid:** Introduce `SINGBOX_BINARY_PATH` (or default `./bin/sing-box` / `$DATA_DIR/sing-box`). Panel installs and updates only in that path. ProcessManager uses it when set.

**Warning signs:** Update fails with "permission denied"; `which sing-box` returns system path.

### Pitfall 2: Archive Structure Mismatch

**What goes wrong:** Extracted tar.gz contains `sing-box-1.12.21-linux-amd64/sing-box` but code expects `sing-box` at root. Extraction fails or wrong file used.

**Why it happens:** sing-box releases use `sing-box-{version}-{os}-{arch}/` folder with `sing-box` binary inside. Verified: `tar tf sing-box-1.12.21-linux-amd64.tar.gz` shows `sing-box-1.12.21-linux-amd64/sing-box` and `.../LICENSE`.

**How to avoid:** Extract to temp dir; copy `{extractDir}/sing-box-{version}-{os}-{arch}/sing-box` to target, or use `filepath.Join(extractDir, fmt.Sprintf("sing-box-%s-%s-%s", ver, goos, goarch), "sing-box")`.

**Warning signs:** Update "succeeds" but `sing-box version` fails or runs old binary.

### Pitfall 3: Update Without Stopping

**What goes wrong:** Replacing binary while sing-box runs. On Linux, executable can be replaced (unlink + rename) but running process keeps using old inode; restart may spawn new binary. Risk: if replace fails mid-way, running process may get truncated/corrupt file.

**Why it happens:** Skipping stop step for "faster" update.

**How to avoid:** Always stop (pkill or equivalent), replace, then start. Same pattern as Restart.

### Pitfall 4: No Rollback on Check Failure

**What goes wrong:** New binary fails `sing-box check` or crashes on start. Users stuck with broken install.

**Why it happens:** Not restoring backup when post-update validation fails.

**How to avoid:** After replace, run `sing-box check -c config` and optionally verify process starts. On failure, restore backup and restart. Return error to frontend.

---

## Code Examples

### GitHub Releases Fetch (Go)

```go
// GET https://api.github.com/repos/SagerNet/sing-box/releases
type GitHubRelease struct {
    TagName    string `json:"tag_name"`
    Prerelease bool   `json:"prerelease"`
    Assets     []struct {
        Name               string `json:"name"`
        BrowserDownloadURL string `json:"browser_download_url"`
    } `json:"assets"`
}

resp, err := http.Get("https://api.github.com/repos/SagerNet/sing-box/releases")
if err != nil { return nil, err }
defer resp.Body.Close()
var releases []GitHubRelease
if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil { return nil, err }
```

### Asset Name for Platform

```go
func assetName(version, goos, goarch string) string {
    // version = "1.12.21" (no v prefix)
    osArch := goos + "-" + goarch
    if goarch == "amd64" && goos == "windows" {
        osArch = "windows-amd64"
    }
    return fmt.Sprintf("sing-box-%s-%s.tar.gz", version, osArch)
}
```

### Tar.gz Extract Binary (Go)

Archive structure: `sing-box-{version}-{os}-{arch}/sing-box`.

```go
import "archive/tar"
import "compress/gzip"

func extractBinary(tarGzPath, destPath string) error {
    f, err := os.Open(tarGzPath)
    if err != nil { return err }
    defer f.Close()
    gr, err := gzip.NewReader(f)
    if err != nil { return err }
    defer gr.Close()
    tr := tar.NewReader(gr)
    for {
        h, err := tr.Next()
        if err == io.EOF { break }
        if err != nil { return err }
        if h.Typeflag == tar.TypeReg && strings.HasSuffix(h.Name, "/sing-box") {
            out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
            if err != nil { return err }
            defer out.Close()
            _, err = io.Copy(out, tr)
            return err
        }
    }
    return fmt.Errorf("sing-box binary not found in archive")
}
```

---

## Recommendations for Claude's Discretion

| Area            | Recommendation                                                      | Rationale                                                                                         |
| --------------- | ------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| **Rollback**    | Keep 1 previous version; manual rollback only                       | Simple; matches "rollback to previous" in success criteria. Auto-rollback only on update failure. |
| **Update flow** | Stop → Backup → Download → Extract → Replace → Start → Verify       | Matches ApplyConfig reliability; verify = `sing-box check` + optional process alive check.        |
| **Progress**    | Step progress (Downloading → Extracting → Replacing → Restarting)   | More informative than spinner; update can take 10–30s.                                            |
| **Failure**     | Auto-restore backup + return error to frontend                      | Reliability-first; user notified via toast + optional modal with details.                         |
| **Post-update** | Run `sing-box check` on config; restart; optional `Version()` check | Low cost; catches config incompatibility early.                                                   |

---

## Open Questions

1. **Supported platforms**
   - What we know: Linux amd64/arm64 assets verified. darwin, windows assets exist.
   - What's unclear: Whether s-ui deploys only on Linux.
   - Recommendation: Implement Linux first; map GOOS/GOARCH for others when needed.

---

## State of the Art

| Old Approach              | Current Approach           | When Changed | Impact                         |
| ------------------------- | -------------------------- | ------------ | ------------------------------ |
| Manual download + replace | Panel-managed update       | Phase 7      | One-click; backup for rollback |
| sing-box from PATH        | Explicit binary path (env) | Phase 7      | Panel owns binary location     |

**Deprecated/outdated:** None for this phase.

---

## Sources

### Primary (HIGH confidence)

- [GitHub Releases API](https://docs.github.com/en/rest/releases/releases) — list releases, assets
- [SagerNet/sing-box releases](https://github.com/SagerNet/sing-box/releases) — asset naming verified (v1.12.21)
- Go `runtime` package — GOOS, GOARCH
- Go `archive/tar`, `compress/gzip` — extraction

### Secondary (MEDIUM confidence)

- WebSearch: sing-box Linux binary naming — `sing-box-{version}-linux-{arch}.tar.gz`
- ApplyConfig pattern (internal/core/config.go) — temp file + atomic rename

### Tertiary (LOW confidence)

- None — archive structure verified via `tar tf` on v1.12.21-linux-amd64.tar.gz

---

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — stdlib only; GitHub API well-documented
- Architecture: HIGH — consistent with existing ProcessManager, ApplyConfig
- Pitfalls: MEDIUM — binary path and archive structure need runtime verification

**Research date:** 2026-02-12
**Valid until:** 30 days (stable API and release format)
