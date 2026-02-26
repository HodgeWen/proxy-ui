---
phase: 12-core-process-control
verified: 2026-02-26T10:06:00Z
status: passed
score: 10/10 must-haves verified
human_verification:
  - test: "四态 UI 与动作矩阵端到端切换"
    expected: "running=Stop+Restart，stopped=Start，not_installed=安装引导，error=Retry Start+View Logs"
    why_human: "需要真实核心状态流转和页面可见行为验证，静态代码无法覆盖全部交互路径"
  - test: "缺失二进制时控制链路"
    expected: "status 为 not_installed，Start/Restart 返回语义化错误，UI 明确提示下载安装"
    why_human: "涉及真实运行环境文件路径与运维安装流程提示有效性"
  - test: "启动失败后的诊断可用性"
    expected: "进入 error 状态并可查看日志，日志为空/缺失/读取失败均有可理解反馈"
    why_human: "需要人工判断错误信息可理解性与诊断流程体验"
---

# Phase 12: Core Process Control Verification Report

**Phase Goal:** Users have clear, accurate control over sing-box core lifecycle across all states
**Verified:** 2026-02-26T10:06:00Z
**Status:** passed
**Re-verification:** Yes - approved after human verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | GET `/api/core/status` 返回 `state` 且仅四态之一 | ✓ VERIFIED | `internal/core/lifecycle_state.go` 定义四态，`internal/api/core.go` 输出 `state`，`internal/api/core_test.go` 覆盖状态断言 |
| 2 | 二进制缺失时状态为 `not_installed`，`start/restart` 返回语义化错误 | ✓ VERIFIED | `internal/core/process.go` 在 `Start` 检查 `Available()` 并返回 `CORE_NOT_INSTALLED`；`internal/api/core_test.go` 覆盖 start/restart 缺失场景 |
| 3 | `running` 可执行 stop/restart，`stopped` 可执行 start | ✓ VERIFIED | `internal/core/lifecycle_state.go` 的 `ActionMatrix` + `internal/core/process.go` 的 `Start/Stop/Restart` 语义 |
| 4 | 启动失败后进入 `error` 并保留最近失败上下文 | ✓ VERIFIED | `internal/core/process.go` `setLastFailure(...)`；`internal/core/lifecycle_state.go` 在 error 状态回传 `LastError`；`internal/api/core_test.go` 覆盖 |
| 5 | `error` 状态动作为 `retry_start` 与 `view_logs`，不误导为 restart | ✓ VERIFIED | `internal/core/lifecycle_state.go` error 动作矩阵；`web/src/lib/core-status.ts` 与 `web/src/pages/Core.tsx` 动作渲染 |
| 6 | Core 页面按四态渲染不同文案与引导 | ✓ VERIFIED | `web/src/pages/Core.tsx` 分支渲染 `running/stopped/not_installed/error` 卡片内容 |
| 7 | running 显示 Stop + Restart；stopped 显示 Start | ✓ VERIFIED | `web/src/lib/core-status.ts` `ACTIONS_BY_STATE` + `web/src/pages/Core.tsx` `renderLifecycleAction` |
| 8 | not_installed 显示安装提示与明确下载/安装引导 | ✓ VERIFIED | `web/src/pages/Core.tsx` not_installed 分支显示路径提示与下载链接 |
| 9 | error 显示 Retry Start + View Logs，且不显示误导性 Restart | ✓ VERIFIED | `web/src/lib/core-status.ts` error 动作为 `retry_start/view_logs`，`web/src/pages/Core.tsx` 仅渲染对应动作 |
| 10 | 状态查询保持 TanStack Query 5s 轮询，动作后可刷新状态并反馈结果 | ✓ VERIFIED | `web/src/pages/Core.tsx` `refetchInterval: 5000`；动作 mutation `invalidateQueries(["core","status"])` + toast |

**Score:** 10/10 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `internal/core/lifecycle_state.go` | 四态判定、失败上下文、动作矩阵 | ✓ VERIFIED | 文件存在，定义 `CoreState/ResolveCoreState/ActionMatrix`，并被 API 使用 |
| `internal/core/process.go` | 语义化 Start/Stop/Restart | ✓ VERIFIED | 文件存在，`ProcessError` 错误码完善，缺失二进制不再假成功 |
| `internal/api/core.go` | status/start/stop/restart/logs handlers | ✓ VERIFIED | 文件存在，状态输出与结构化错误响应完整 |
| `internal/api/routes.go` | 注册 `/start` `/stop` `/restart` `/logs` | ✓ VERIFIED | 文件存在，路由全部注册在 `/api/core` 下 |
| `internal/core/lifecycle_state_test.go` | 四态与动作矩阵测试 | ✓ VERIFIED | 文件存在，覆盖优先级与动作矩阵、失败快照拷贝 |
| `internal/api/core_test.go` | 控制接口与错误语义测试 | ✓ VERIFIED | 文件存在，覆盖 not_installed、error 动作、logs 场景 |
| `web/src/lib/core-status.ts` | 前端状态常量/动作矩阵/状态元信息 | ✓ VERIFIED | 文件存在，导出 `CORE_STATES/ACTIONS_BY_STATE/getStateMeta` 并被页面消费 |
| `web/src/pages/Core.tsx` | 四态驱动控制 UI 与日志查看 | ✓ VERIFIED | 文件存在，接入 `/api/core/status/start/stop/restart/logs`，动作与文案按状态渲染 |

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| `internal/api/core.go` | `internal/core/lifecycle_state.go` | `StatusHandler` 调用状态解析 | WIRED | 存在 `core.ResolveCoreState(pm)` |
| `internal/api/core.go` | `internal/core/process.go` | Start/Stop/Restart handlers 调用进程控制 | WIRED | 存在 `pm.Start(...)` / `pm.Stop()` / `pm.Restart(...)` |
| `internal/api/routes.go` | `internal/api/core.go` | 注册 `/api/core/start|stop|restart|logs` | WIRED | 路由显式注册四个端点 |
| `internal/api/core.go` | `data/sing-box.log` | LogsHandler 读取日志文件 | WIRED | `filepath.Join(cfg.DataDir, "sing-box.log")` + `os.ReadFile(logPath)` |
| `web/src/pages/Core.tsx` | `/api/core/status` | `useQuery` 轮询状态 | WIRED | `queryKey: ["core","status"]` + `refetchInterval: 5000` + `fetch("/api/core/status")` |
| `web/src/pages/Core.tsx` | `/api/core/start|/stop|/restart` | `useMutation` 生命周期动作 | WIRED | `CORE_ACTION_ENDPOINTS` 指向三个端点并由 mutation 调用 |
| `web/src/pages/Core.tsx` | `/api/core/logs` | error 态按需读取日志 | WIRED | `fetchCoreLogs` 请求 `/api/core/logs?lines=...`，`view_logs` 触发 mutation |
| `web/src/pages/Core.tsx` | `web/src/lib/core-status.ts` | 状态动作映射驱动渲染 | WIRED | 导入并使用 `ACTIONS_BY_STATE/getStateMeta/normalizeCoreState` |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| CORE-01 | `12-01`, `12-02` | 独立 start/stop/restart，按状态显示可用操作 | ✓ SATISFIED | 后端新增三个控制端点+语义错误；前端按状态矩阵渲染动作按钮 |
| CORE-02 | `12-01`, `12-02` | 二进制不存在不自动启动，UI 引导安装 | ✓ SATISFIED | `ProcessErrorNotInstalled` + status `not_installed`；前端安装提示与下载 CTA |
| CORE-03 | `12-01`, `12-02` | 启动失败显示正确恢复动作（重试/日志） | ✓ SATISFIED | 后端 error 动作矩阵 `retry_start/view_logs`；前端 error 分支渲染对应动作 |
| CORE-04 | `12-01`, `12-02` | 状态 API 区分四态并驱动差异化 UI | ✓ SATISFIED | 状态 API 输出四态，Core 页面四态分支渲染 |

Requirement ID accounting check:
- PLAN frontmatter 覆盖：`CORE-01, CORE-02, CORE-03, CORE-04`
- `.planning/REQUIREMENTS.md` 对应 Phase 12 覆盖：`CORE-01, CORE-02, CORE-03, CORE-04`
- Orphaned requirements: none

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| `web/src/pages/Core.tsx` | 382 | `return null`（switch default 分支） | ℹ️ Info | 属于未知 action 的防御性返回，不构成占位实现或目标阻断 |

### Human Verification

### 1. 四态真实状态切换验证

**Test:** 在真实环境触发 running/stopped/not_installed/error 四种状态并观察 Core 页面
**Expected:** 每个状态显示对应文案与动作集合，且按钮可执行
**Why human:** 需要真实进程与 UI 交互闭环，静态检查无法完全覆盖

### 2. 缺失核心安装引导可用性

**Test:** 删除/重命名核心二进制后刷新页面并尝试 Start/Restart
**Expected:** 状态为 not_installed，动作含 install，引导链接与路径提示清晰
**Why human:** 需要人工确认提示可理解、引导可执行

### 3. 启动失败恢复路径体验

**Test:** 构造启动失败（坏配置或假二进制）后点击 Retry Start 与 View Logs
**Expected:** 保持 error 语义，日志弹窗区分 loading/empty/error，并给出明确反馈
**Why human:** 错误可读性与恢复体验需人工评估

### Gaps Summary

未发现阻断 Phase 12 目标的代码缺口；must_haves 与 requirement IDs 均已在代码中具备实现与连线。  
人工验证已完成并获得用户 `approved` 确认，Phase 12 判定为 `passed`。

---

_Verified: 2026-02-26T10:06:00Z_  
_Verifier: Claude (gsd-verifier)_
