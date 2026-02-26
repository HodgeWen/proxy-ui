---
phase: 13-core-update-progress
status: passed
requirements: [UPDT-01, UPDT-02]
verified_at: 2026-02-26
verifier: gsd-verifier
---

# Phase 13 Verification

## 验证输入

已读取并分析以下规划与交付文档：

- `.planning/ROADMAP.md`
- `.planning/REQUIREMENTS.md`
- `.planning/phases/13-core-update-progress/13-01-PLAN.md`
- `.planning/phases/13-core-update-progress/13-02-PLAN.md`
- `.planning/phases/13-core-update-progress/13-01-SUMMARY.md`
- `.planning/phases/13-core-update-progress/13-02-SUMMARY.md`

## 目标反推式验证（Goal-backward）

目标：用户在 core 更新时看到 SSE 实时下载百分比；重复更新触发受保护（409）；SSE 头与连接清理正确。

### Must-have 覆盖与证据

1) **重复触发稳定返回 409，且不启动第二个下载任务**  
**结论：已覆盖（代码 + 自动化测试）**

- 后端入口使用 `TryLock` 防重入：`internal/core/update_progress_state.go` 的 `Begin()`。
- 更新接口在锁失败时返回冲突：`internal/api/core.go` 的 `UpdateHandler()`（`http.StatusConflict` + `CORE_UPDATE_CONFLICT`）。
- 自动化测试验证“第二次触发返回 409 且 runner 仅执行一次”：`internal/api/core_test.go` 的 `TestUpdateHandlerReturnsConflictWhenUpdateAlreadyInProgress`。

2) **SSE 客户端持续收到真实下载百分比；刷新后先拿快照再增量**  
**结论：已覆盖（代码 + 自动化测试 + 构建验证）**

- 下载百分比基于真实字节/`Content-Length` 回调：`internal/core/updater.go` 的 `downloadFileWithProgress()` 与 `progressWriter`。
- 对已知长度验证“连续、单调、含中间百分比、最终 100”：`internal/core/updater_progress_test.go` 的 `TestUpdateDownloadProgressWithContentLength`。
- 对未知长度验证“可解释降级，完成时 100”：`internal/core/updater_progress_test.go` 的 `TestUpdateDownloadProgressWithoutContentLengthFallsBackToCompletion`。
- SSE 订阅首包即快照：`internal/core/update_progress_state.go` 的 `Subscribe()` 会立即 `sendLatest`。
- 前端流状态合并与显示：`web/src/hooks/use-core-update-stream.ts`（`EventSource` + `onmessage` merge）与 `web/src/pages/Core.tsx`（`更新中 xx%` + `Progress` 组件）。

3) **SSE 必要头正确，连接断开/离开页面可清理，无泄漏**  
**结论：部分自动化覆盖，仍需人验确认运行态无泄漏**

- SSE 头设置：`internal/api/core.go` 的 `UpdateStreamHandler()` 设置
  - `Content-Type: text/event-stream`
  - `Cache-Control: no-cache, no-transform`
  - `X-Accel-Buffering: no`
- 服务端清理：`UpdateStreamHandler()` 监听 `r.Context().Done()` 并 `defer state.Unsubscribe(subID)`。
- 前端清理：`web/src/hooks/use-core-update-stream.ts` 在 `useEffect` cleanup 中 `source.close()`。
- 自动化测试覆盖头部与 context cancel 退出：`internal/api/core_test.go` 的 `TestUpdateStreamHandlerSendsSnapshotWithRequiredHeaders`。
- 但“真实浏览器导航离开后的连接释放”缺少端到端自动化证明，需人工 UAT 关闭最后不确定性。

## Requirement IDs 验证

- **UPDT-01（SSE 实时推送 + 前端 Progress 展示）**：**代码满足，自动化部分满足，需人工确认最终用户感知路径。**
- **UPDT-02（并发更新保护 TryLock + 409）**：**代码满足，自动化测试明确通过。**

## 已执行检查

- `go test ./internal/core ./internal/api -count=1` ✅ 通过
- `npm --prefix web run build` ✅ 通过

## 人工验证结论

请按以下步骤执行一次 UAT：

1. 在核心页点击“更新”，观察文案持续为 `更新中 xx%`，并且进度条随时间推进。  
2. 更新进行中刷新页面，确认能快速恢复到当前百分比（不是从 0 长时间停留）。  
3. 打开两个标签页并发触发更新，确认只有一个更新真正启动；另一侧表现为禁用或低噪音处理（无持续错误风暴），后端语义为 409。  
4. 在 DevTools Network 观察 `/api/core/update/stream`，离开核心页或关闭标签后连接应结束，不应长期残留。  

人工验证结果：`approved`（用户确认 4 项 UAT 均通过）。

## 残余风险

- 当前缺少前端 E2E 自动化（多标签并发、路由离开清理）作为长期回归保障。
- 前端构建存在 chunk size 警告，但不影响 Phase 13 功能目标达成判断。
