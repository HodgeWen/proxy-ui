---
phase: 07-core-management
verified: "2026-02-12T00:00:00Z"
status: passed
score: 2/2 must-haves verified
---

# Phase 7: sing-box Core Management Verification Report

**Phase Goal:** 管理员可在面板中更新或回滚 sing-box 核心版本

**Verified:** 2026-02-12

**Status:** passed

**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth                                                                 | Status     | Evidence                                                                 |
| --- | --------------------------------------------------------------------- | ---------- | ------------------------------------------------------------------------ |
| 1   | 管理员可在面板中将 sing-box 核心更新到最新版本                         | ✓ VERIFIED | Dashboard 有「更新」按钮；确认弹框；POST /api/core/update；updateMutation 调用 updateCore；成功时 invalidate 并 toast.success；失败时 toast.error；后端 Update() 完整流程：stop→backup→download→extract→atomic replace→verify→start |
| 2   | 管理员可将 sing-box 核心回滚到上一个已安装版本                        | ✓ VERIFIED | Dashboard 有「回滚」按钮；确认弹框；POST /api/core/rollback；rollbackMutation 调用 rollbackCore；成功时 invalidate 并 toast.success；失败时 toast.error；后端 Rollback() 存在：swap backup→current，restart |

**Score:** 2/2 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/core/updater.go` | CoreUpdater: fetch, download, extract, backup, atomic replace, restart | ✓ VERIFIED | 335 lines; ListReleases, Update, Rollback, assetForPlatform, restoreBackup; 完整实现 |
| `internal/core/process.go` | Explicit binary path support for managed updates | ✓ VERIFIED | binaryPath field; NewProcessManagerWithBinary; Available/Version/Check/Restart use binaryPath |
| `internal/api/core.go` | GET /versions, POST /update, POST /rollback | ✓ VERIFIED | VersionsHandler, UpdateHandler, RollbackHandler; 均调用 CoreUpdater |
| `web/src/pages/Dashboard.tsx` | Core version card with update/rollback, version list, confirm dialog, new version badge | ✓ VERIFIED | 当前版本/最新版本; 更新/回滚按钮; 确认弹框; 版本列表 modal; 「有新版本」badge |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| `web/src/pages/Dashboard.tsx` | `/api/core/versions` | fetchVersions → useQuery | ✓ WIRED | `fetch("/api/core/versions")`; queryKey ["core","versions"] |
| `web/src/pages/Dashboard.tsx` | `/api/core/update` | updateMutation → updateCore | ✓ WIRED | `fetch("/api/core/update", {method:"POST"})`; 成功时 invalidate 并 toast |
| `web/src/pages/Dashboard.tsx` | `/api/core/rollback` | rollbackMutation → rollbackCore | ✓ WIRED | `fetch("/api/core/rollback", {method:"POST"})`; 成功时 invalidate 并 toast |
| `internal/api/core.go` | `internal/core/updater.go` | CoreUpdater.ListReleases, Update, Rollback | ✓ WIRED | NewCoreUpdater(binaryPath()); u.ListReleases(); u.Update(); u.Rollback() |
| `internal/core/updater.go` | `internal/core/process.go` | ProcessManager with binary path | ✓ WIRED | NewProcessManagerWithBinary(ConfigPathFromEnv(), u.binaryPath); pm.Check; pm.Restart |

### Requirements Coverage

| Requirement | Status | Blocking Issue |
| ----------- | ------ | -------------- |
| COR-03 管理员可以在面板中将 sing-box 核心更新到最新版本 | ✓ SATISFIED | — |
| COR-04 管理员可以将 sing-box 核心回滚到上一个已安装版本 | ✓ SATISFIED | — |

### Anti-Patterns Found

None. No TODO/FIXME/placeholder/return null stubs in key files.

### Human Verification Required

Optional end-to-end test (code paths complete; automated checks cannot run real sing-box + GitHub):

1. **Update flow (real binary)** — 点击「更新」→ 确认 → 应完成下载、替换、重启；当前版本应更新
2. **Rollback flow (real backup)** — 在更新后点击「回滚」→ 确认 → 应恢复上一版本

### Gaps Summary

None. Phase goal achieved.

---

_Verified: 2026-02-12_
_Verifier: Claude (gsd-verifier)_
