# Phase 13: Core Update Progress - Research

**Researched:** 2026-02-26
**Domain:** 核心更新进度流（SSE）与并发触发保护（Mutex TryLock）
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
### 进度信息呈现
- 主文案偏好：简洁样式，使用“更新中 xx%”风格
- 进度条外显示百分比数字（固定可见）
- 不需要额外阶段提示（如准备中/下载中/完成），避免信息冗余
- 信息密度偏低，只保留关键进度信息

### 页面切换与返回
- 页面刷新后应尽量恢复到当前进度，保证连续感
- 若更新已完成，返回页面时直接回归普通状态，不额外展示成功提示
- 多标签页场景下，非发起标签页的更新操作偏好“静默禁用按钮”

### Claude's Discretion
- 用户离开“核心管理”页后再返回时，是否展示连续百分比细节由 Claude 决定（可在“连续感”与“界面简洁”之间权衡）
- 当按钮已被禁用但后端仍返回 409 时，是否补充轻量提示由 Claude 决定

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| UPDT-01 | SSE 实时推送核心更新下载进度，前端使用 Progress 组件展示进度条 | 建议拆分“触发更新”和“订阅进度”：`POST /api/core/update` 仅触发，`GET /api/core/update/stream` 持续推送；后端发送初始快照支持刷新恢复，前端以 EventSource + Progress 渲染“更新中 xx%”。 |
| UPDT-02 | 并发更新保护（sync.Mutex + TryLock），防止多次点击重复触发更新，返回 HTTP 409 | 在更新入口加全局 `sync.Mutex` + `TryLock`，抢锁失败直接 409；多标签页通过共享更新状态将按钮静默禁用，后端仍保底返回 409。 |
</phase_requirements>

## Summary

当前实现中，`/api/core/update` 是同步阻塞调用 `CoreUpdater.Update()`，前端仅显示按钮 loading 文案“更新中...”，没有进度事件通道，也没有并发更新锁。因此 Phase 13 的核心不是“新能力从 0 到 1”，而是把现有更新流程改造成“可观察 + 可互斥”的后台任务模型。

官方资料与现有代码共同指向同一套实现路径：后端继续使用 Go stdlib 实现 SSE（`text/event-stream` + flush），并在更新入口使用 `sync.Mutex.TryLock` 做并发拒绝（409）；前端继续保留 TanStack Query 的 5s 状态轮询（项目既有决策），同时新增 EventSource 消费更新进度并在组件卸载时 `close()`，避免连接泄漏。该路径满足“无新依赖”约束，并与当前项目架构兼容。

刷新连续感与多标签静默禁用建议通过“服务端保存更新快照 + SSE 首包立即回放”实现：用户刷新后可快速恢复最新百分比；非发起标签页可根据共享更新状态禁用按钮，不需要高信息密度 UI。

**Primary recommendation:** 采用“`POST` 触发更新 + `GET` SSE 订阅进度 + 全局 TryLock 防重入 + 进度快照回放”的组合方案，保持 UI 极简，仅显示固定可见百分比与进度条。

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `net/http` (`Flusher` / `ResponseController`) | Go 1.25.6 | SSE 流写出与实时 flush | 官方原生能力，零额外依赖，已满足本阶段需求 |
| Go stdlib `sync.Mutex` + `TryLock` | Go 1.25.6 | 并发更新互斥与 409 快速失败 | Requirement 明确要求，语义直达“已在更新中” |
| `github.com/go-chi/chi/v5` | v5.2.5 | 扩展 `/api/core/update/stream` 路由 | 当前 API 路由框架，增量接入成本最低 |
| Browser `EventSource` API | 标准 Web API | 前端订阅 SSE 进度 | 浏览器原生，自动重连，适合单向服务端推送 |
| `@tanstack/react-query` | ^5.90.20 | 保持核心状态轮询与 mutation 失效刷新 | 与既有“状态仍轮询、仅进度用 SSE”决策一致 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `radix-ui` (包含 `@radix-ui/react-progress`) | ^1.4.3 / 1.1.7 | Progress 基础组件 | 核心页更新中展示实时百分比条 |
| shadcn/ui 组件体系 | in-repo | 统一按钮/卡片/提示样式 | 保持“极简、低信息密度”视觉一致性 |
| `sonner` | ^2.0.7 | 轻量错误反馈 | 可选用于 409 边界提示（按 discretion） |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| SSE 进度流 | 纯轮询进度接口 | 无法满足“实时下载百分比”体验，刷新颗粒度太粗 |
| `TryLock` 互斥 | 仅前端禁用按钮 | 多标签/刷新竞态无法兜底，后端仍可能重复触发 |
| 阻塞式 `POST /update` | 异步触发 + SSE | 阻塞式无法稳定承载实时流与页面刷新恢复 |

**Installation:**
```bash
# No new npm or Go dependencies for Phase 13
```

## Architecture Patterns

### Recommended Project Structure
```text
internal/
├── core/
│   ├── updater.go                  # 扩展：下载过程进度回调（真实下载百分比）
│   └── update_progress_state.go    # 新增：全局更新状态、TryLock、订阅广播
├── api/
│   ├── core.go                     # 扩展：POST /update 异步触发；GET /update/stream SSE
│   └── routes.go                   # 注册 /api/core/update/stream
web/src/
├── components/ui/progress.tsx      # 新增：Progress 包装组件（shadcn 风格）
├── hooks/use-core-update-stream.ts # 新增：EventSource 生命周期与 cleanup
└── pages/Core.tsx                  # 扩展：极简进度条 + 百分比 + 静默禁用更新按钮
```

### Pattern 1: Trigger/Stream Split（触发与流分离）
**What:** `POST /api/core/update` 负责开始任务，`GET /api/core/update/stream` 只负责推送状态。
**When to use:** 需要支持刷新后恢复、以及多个页面/标签共享同一进度源。
**Example:**
```go
// Source: repo pattern + MDN SSE/EventSource docs
// POST returns quickly (202/409); SSE endpoint keeps streaming progress events.
```

### Pattern 2: Global Update Coordinator with TryLock
**What:** 以单例协调器维护 `inProgress`、`percent`、`error`、`updatedAt` 和订阅者集合；更新入口 `TryLock` 失败即 409。
**When to use:** 所有触发更新请求（包括多标签重复点击）。
**Example:**
```go
// Source: https://pkg.go.dev/sync#Mutex.TryLock
if !updateMu.TryLock() {
    writeCoreError(w, http.StatusConflict, "CORE_UPDATE_CONFLICT", "core update already in progress", "")
    return
}
defer updateMu.Unlock()
```

### Pattern 3: SSE Streaming with Flush + Request Context Cleanup
**What:** SSE handler设置 `Content-Type: text/event-stream`、`X-Accel-Buffering: no` 并在每次写后 flush；`r.Context().Done()` 时退出并退订。
**When to use:** 任何实时进度推送连接。
**Example:**
```go
// Source: https://pkg.go.dev/net/http#Flusher + https://pkg.go.dev/net/http#Request.Context
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache, no-transform")
w.Header().Set("X-Accel-Buffering", "no")

flusher, ok := w.(http.Flusher)
if !ok {
    http.Error(w, "stream unsupported", http.StatusInternalServerError)
    return
}

for {
    select {
    case <-r.Context().Done():
        return // navigation/connection close => cleanup
    case msg := <-subscriber:
        _, _ = w.Write([]byte("data: " + msg + "\n\n"))
        flusher.Flush()
    }
}
```

### Pattern 4: Real Download Percent via Instrumented Copy
**What:** 在 `downloadFile` 中基于 `Content-Length` 与已写字节计算百分比并上报。
**When to use:** 满足 UPDT-01 的“实际下载百分比”要求。
**Example:**
```go
// Source: existing internal/core/updater.go downloadFile + phase requirement
type progressWriter struct {
    total int64
    wrote int64
    onPct func(int)
}
func (pw *progressWriter) Write(p []byte) (int, error) {
    n := len(p)
    pw.wrote += int64(n)
    if pw.total > 0 && pw.onPct != nil {
        pw.onPct(int(pw.wrote * 100 / pw.total))
    }
    return n, nil
}
```

### Anti-Patterns to Avoid
- **SSE 连接和更新任务绑定同一请求上下文:** 用户刷新会中断更新，破坏“连续感”。
- **仅靠前端按钮禁用防并发:** 无法防止跨标签或直接 API 调用导致的重复更新。
- **在 UI 展示多阶段复杂文案:** 与“极简、低密度”锁定决策冲突。
- **EventSource 未在 `useEffect` cleanup 关闭:** 导致导航后连接泄漏与重复监听。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SSE 客户端 | 第三方 SSE 客户端封装层 | 原生 `EventSource` | 已满足需求，减少依赖与维护面 |
| 并发控制 | 自定义锁协议/数据库锁 | `sync.Mutex` + `TryLock` | Requirement 明确，语义简单可测 |
| 进度 UI | 自写复杂动画条 | 现有 Progress 基础组件 | 保持统一风格，满足极简需求 |
| 状态同步 | 跨标签自定义事件总线 | 服务端快照 + SSE 首包回放 | 刷新/多标签统一由后端真相源驱动 |

**Key insight:** 该阶段的关键不是“做一个炫的更新流程”，而是把“真实下载进度 + 防重入语义”稳定地落在后端单一真相源上，前端只做极简可视化。

## Common Pitfalls

### Pitfall 1: Proxy Buffering Breaks Real-Time SSE
**What goes wrong:** 事件被代理缓冲，前端长时间不更新，最后一次性跳变。
**Why it happens:** 代理默认开启响应缓冲。
**How to avoid:** SSE 响应设置 `X-Accel-Buffering: no`；必要时文档中补充 Nginx `proxy_buffering off`。
**Warning signs:** Network 面板中 SSE 请求长期无增量事件，进度突然跃迁到高值。

### Pitfall 2: EventSource Connection Leak on Route Change
**What goes wrong:** 离开页面后连接未关闭，返回页面出现重复事件消费。
**Why it happens:** `useEffect` 未返回 cleanup 或依赖数组不稳定。
**How to avoid:** `useEffect` 内创建 EventSource，cleanup 中 `es.close()`；确保 effect 触发条件可控。
**Warning signs:** 同一标签页出现多个并发 SSE 连接，百分比抖动/重复 toast。

### Pitfall 3: 409 Semantics Not Aligned with UI
**What goes wrong:** 按钮看似可点击，但请求频繁 409，用户感知不一致。
**Why it happens:** 前端本地状态与后端真实更新状态不同步（多标签/刷新场景）。
**How to avoid:** 服务端维护 `inProgress` 快照；前端用该快照静默禁用按钮，409 作为兜底。
**Warning signs:** 非发起标签页出现“可点击但总报错”。

### Pitfall 4: Percent Gets Stuck at 0/100
**What goes wrong:** 只在开始/结束发事件，看不到“实时下载”。
**Why it happens:** 下载过程中未按字节上报增量，或 `Content-Length` 未处理。
**How to avoid:** `io.Copy` 过程持续上报；`Content-Length` 不可用时采用可解释降级策略（如保持上次值并附状态标记）。
**Warning signs:** 大文件下载期间进度长时间静止。

## Code Examples

Verified patterns from official sources:

### EventSource Lifecycle Cleanup
```tsx
// Source: React useEffect docs + MDN EventSource close
useEffect(() => {
  const es = new EventSource("/api/core/update/stream")
  es.onmessage = (e) => {
    // setProgress(JSON.parse(e.data))
  }
  es.onerror = () => {
    // optional lightweight handling
  }
  return () => {
    es.close()
  }
}, [])
```

### Mutation Success + Query Invalidation
```tsx
// Source: Context7 /tanstack/query (invalidations from mutations)
const queryClient = useQueryClient()
const mutation = useMutation({
  mutationFn: triggerUpdate,
  onSuccess: async () => {
    await queryClient.invalidateQueries({ queryKey: ["core", "status"] })
  },
})
```

### SSE Headers + Flush
```go
// Source: MDN SSE guide + Go net/http Flusher docs
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache, no-transform")
w.Header().Set("X-Accel-Buffering", "no")
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 同步 `POST /api/core/update` + 按钮 loading 文案 | 异步触发 + SSE 连续百分比 | Phase 13 | 用户可见真实下载进度，刷新后可恢复 |
| 仅前端 pending 防重复 | 后端 `TryLock` + 409 冲突语义 | Phase 13 | 并发触发被硬性阻断，多标签安全 |
| 单标签局部状态 | 服务端进度快照 + 多客户端订阅 | Phase 13 | 非发起标签页可静默禁用，状态一致 |

**Deprecated/outdated:**
- 用“更新中...”静态文案代替实际百分比（无法满足 UPDT-01）。
- 无锁更新入口（无法满足 UPDT-02）。

## Open Questions

1. **刷新回到核心页时的百分比细节密度**
   - What we know: 用户强调“连续感优先”，同时偏好低信息密度。
   - What's unclear: 是否在返回后展示最近一次百分比历史痕迹（如短暂过渡）。
   - Recommendation: 默认仅展示当前百分比与进度条，不展示历史轨迹。

2. **按钮已禁用但后端仍返回 409 的反馈强度**
   - What we know: 该点被标为 Claude's Discretion。
   - What's unclear: 是否需要提示以避免“点击无反应”误解。
   - Recommendation: 默认静默；仅在同会话连续触发时给一次轻量 toast（非阻断）。

## Sources

### Primary (HIGH confidence)
- Context7 `/mdn/content` — EventSource 创建、自动重连、`close()`、SSE 事件格式与示例头部
- Context7 `/tanstack/query` — `useMutation` 后 `invalidateQueries`、`useQuery` `refetchInterval`
- Context7 `/reactjs/react.dev` — `useEffect` cleanup 生命周期（依赖变化前 + unmount）
- Go official docs: `https://pkg.go.dev/net/http#Flusher`
- Go official docs: `https://pkg.go.dev/net/http#Request.Context`
- Go official docs: `https://pkg.go.dev/sync#Mutex.TryLock`
- MDN HTTP 409: `https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/409`
- Repository code: `internal/api/core.go`, `internal/api/routes.go`, `internal/core/updater.go`, `internal/core/lifecycle_state.go`, `web/src/pages/Core.tsx`, `.planning/REQUIREMENTS.md`, `web/bun.lock`

### Secondary (MEDIUM confidence)
- Nginx proxy docs: `https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_buffering`（`proxy_buffering` 与 `X-Accel-Buffering` 行为）

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 主要来自官方文档与仓库锁文件/依赖声明，约束明确。
- Architecture: HIGH - 与现有路由、更新流程、前端状态模型可直接拼接，改造边界清晰。
- Pitfalls: HIGH - 关键风险（缓冲、泄漏、并发）均有官方依据与仓库可验证切入点。

**Research date:** 2026-02-26
**Valid until:** 2026-03-28 (30 days)
