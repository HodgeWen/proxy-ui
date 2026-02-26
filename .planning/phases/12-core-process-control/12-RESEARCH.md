# Phase 12: Core Process Control - Research

**Researched:** 2026-02-26
**Domain:** sing-box 进程生命周期控制（状态建模、启动失败恢复、前后端动作语义一致性）
**Confidence:** HIGH

<user_constraints>
## User Constraints

### Locked Decisions
- No CONTEXT.md for this phase; no pre-locked implementation choices from discuss-phase.
- v1.1 决策：状态检测继续使用 TanStack Query 轮询（5s），不改为 SSE。
- v1.1 决策：不新增 npm/Go 依赖。
- 本阶段必须覆盖：CORE-01, CORE-02, CORE-03, CORE-04。

### Claude's Discretion
- 状态 API 的响应结构（字段命名、是否携带可用动作）。
- 启动失败后的错误上下文保存策略（内存态 vs 持久化）。
- “查看日志”的最小可用实现（非实时，按需读取）。

### Deferred Ideas (OUT OF SCOPE)
- 实时日志流（Requirements 已明确 out-of-scope）。
- 引入新基础设施或第三方依赖来做进程管理/日志流。
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CORE-01 | 拆分现有重启为独立 start/stop/restart，并按状态显示 | 建议新增独立 API 与后端状态机；前端以状态映射动作矩阵渲染，避免硬编码按钮。 |
| CORE-02 | 核心二进制不存在时不自动启动，UI 引导安装 | `not_installed` 由后端统一判定（`os.Stat`），启动入口返回语义化错误，前端显示安装 CTA。 |
| CORE-03 | 启动失败时显示语义正确动作（重试启动/查看日志） | 建议记录最近一次启动失败上下文（错误文本、时间、阶段），状态进入 `error`，动作改为 `retry_start + view_logs`。 |
| CORE-04 | 状态 API 区分 not_installed/stopped/running/error | 建议将 `running:boolean` 升级为 `state enum`，并保持向后兼容字段过渡（短期可保留 running）。 |
</phase_requirements>

## Summary

当前代码具备基础进程能力，但状态语义仍是 `running:boolean`，且核心操作仍以 `restart` 为中心：`/api/core/status` 只返回 running/version/path，`/api/core/restart` 是唯一控制入口，前端 `Core` 页面也只渲染“重启”按钮。这与本阶段的四态状态机和上下文动作要求存在直接差距。

关键风险在于“假成功”和“无诊断”：`ProcessManager.Restart()` 在二进制不存在时会直接返回 nil（no-op），并且 `cmd.Start()` 成功不代表进程健康运行；此外当前将 `Stdout/Stderr` 置空，按 Go 官方文档会连接到 `os.DevNull`，导致失败后几乎没有可读线索。这会直接破坏 CORE-02/03 的用户感知与动作语义。

本阶段应建立一个轻量的后端生命周期状态层（无新依赖），把状态判定和动作策略收敛到一个“单一真相源”，前端仅按状态渲染按钮和提示。日志侧按约束采用“非实时”方案：利用 sing-box 原生 `log.output` 写文件，提供按需查看而非流式推送。

**Primary recommendation:** 先在后端落地四态状态机与 start/stop/restart 独立 API，再让前端完全按状态驱动动作渲染；错误态必须携带最近失败上下文与“重试/查看日志”动作。

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `os/exec` | Go 1.25.6 | 启停 sing-box 进程 | 当前已使用，满足 start/stop/restart 能力，无需引入 supervisor 依赖 |
| Go stdlib `os`/`path/filepath` | Go 1.25.6 | 二进制存在性、路径管理 | `not_installed` 状态判定核心依据 |
| `github.com/go-chi/chi/v5` | v5.2.5 | API 路由 | 现有 `/api/core/*` 路由已在此栈中 |
| `@tanstack/react-query` | ^5.90.20 | 状态轮询与 mutation 后失效刷新 | 已有 5s 轮询与 invalidate 模式，符合项目决策 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sonner` | ^2.0.7 | 状态变更反馈 | start/stop/restart 成功与失败提示 |
| shadcn/ui (`Button`, `Badge`, `Dialog`) | in-repo | 状态动作 UI | 4 态下按钮矩阵、错误详情弹窗 |
| sing-box `log.output` 配置 | 官方能力 | 非实时日志落盘 | `error` 态“查看日志”操作的最小实现 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 后端 `state enum` | 前端自行用 `running + version + 错误文案`拼状态 | 容易状态漂移，按钮语义不一致，维护成本高 |
| TanStack Query 轮询 | SSE 状态流 | 与当前里程碑决策冲突（SSE 仅用于更新进度） |
| 引入进程管理依赖 | 继续 stdlib + 现有命令 | 本阶段规模不需要新依赖，且存在“不可新增依赖”约束 |

**Installation:**
```bash
# No new dependencies for Phase 12
```

## Architecture Patterns

### Recommended Project Structure
```text
internal/
├── core/
│   ├── process.go          # 现有进程操作（建议补充 Start/Stop、错误语义）
│   ├── lifecycle_state.go  # 新增：四态判定、最近失败上下文（内存态）
│   └── config.go           # 保持现有配置写入校验
├── api/
│   ├── core.go             # 扩展 status/start/stop/restart/logs handlers
│   └── routes.go           # 注册新路由
web/src/
├── pages/
│   └── Core.tsx            # 按 state 渲染动作矩阵
└── components/
    └── core/               # 可选：StateBadge / ActionGroup / ErrorPanel
```

### Pattern 1: Backend Single Source of Truth（四态判定集中化）
**What:** 在后端统一输出 `state`，前端只消费，不自行推断。
**When to use:** 所有核心状态展示与动作可用性判断。
**Example:**
```go
// Source: internal/api/core.go + internal/core/process.go (recommended extension)
type CoreState string

const (
    CoreNotInstalled CoreState = "not_installed"
    CoreStopped      CoreState = "stopped"
    CoreRunning      CoreState = "running"
    CoreError        CoreState = "error"
)

// 优先级：not_installed > running > error > stopped
func ResolveState(pm *ProcessManager, lastStartErr string) CoreState {
    if !pm.Available() {
        return CoreNotInstalled
    }
    if pm.IsRunning() {
        return CoreRunning
    }
    if lastStartErr != "" {
        return CoreError
    }
    return CoreStopped
}
```

### Pattern 2: Command API Idempotency + Semantic Errors
**What:** `start/stop/restart` 分离，重复调用返回语义正确结果（不是泛化 500）。
**When to use:** CORE-01/02/03 的所有控制入口。
**Example:**
```go
// 推荐错误码语义：
// - not_installed: 409/400 + code=CORE_NOT_INSTALLED
// - already_running: 409 + code=CORE_ALREADY_RUNNING
// - already_stopped: 409 + code=CORE_ALREADY_STOPPED
// - start_failed: 500 + code=CORE_START_FAILED (含简要 detail)
```

### Pattern 3: UI Action Matrix by State
**What:** 前端动作由状态映射得到，不写分散 if/else。
**When to use:** Core 页面主操作区。
**Example:**
```ts
const ACTIONS_BY_STATE = {
  not_installed: ["install"],
  stopped: ["start"],
  running: ["stop", "restart"],
  error: ["retry_start", "view_logs"],
} as const
```

### Pattern 4: Non-stream Log Visibility
**What:** 使用 sing-box `log.output` 写文件，按需读取最近日志片段。
**When to use:** `error` 态下“查看日志”。
**Example:**
```json
{
  "log": {
    "level": "info",
    "output": "data/sing-box.log",
    "timestamp": true
  }
}
```

### Anti-Patterns to Avoid
- **前端推断状态:** 只拿 `running:boolean` 在前端拼 `error`，会与后端实际失败原因脱节。
- **继续复用“单 restart 按钮”:** 无法满足 CORE-01 与 CORE-03 的语义动作要求。
- **吞掉进程失败信息:** 启动失败只 toast，不保留最近错误上下文，用户无法恢复。
- **把“查看日志”实现成实时流:** 超出当前范围并违反已定义 out-of-scope。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 状态轮询调度 | 自写 `setInterval + cleanup` 全局轮询器 | TanStack Query `refetchInterval` | 现有已稳定使用，且项目已决策保留 |
| 动作可用性逻辑 | 分散在多个组件的 if/else | 单一动作矩阵（state -> actions） | 可测试、可审计，避免按钮语义漂移 |
| 日志采集基础设施 | 新增实时日志推送系统 | sing-box `log.output` + 按需读取接口 | 满足“查看日志”且不引入新依赖 |
| 启动失败推断 | 仅依赖 `cmd.Start` 返回值 | `cmd.Start` + 短窗口健康复核 + 最近失败记录 | `Start` 成功不等于服务可用 |

**Key insight:** 本阶段的复杂度不在“启停命令本身”，而在“状态语义与错误恢复语义的一致性”。必须由后端统一建模，前端仅消费。

## Common Pitfalls

### Pitfall 1: Missing Binary Returns Success (False Positive)
**What goes wrong:** 二进制缺失时调用 restart，API 仍返回成功，用户看到误导状态。
**Why it happens:** 当前 `Restart()` 在 `!Available()` 时直接返回 nil（no-op）。
**How to avoid:** 为控制接口提供明确错误码 `CORE_NOT_INSTALLED`，并将状态置为 `not_installed`。
**Warning signs:** 日志出现 “sing-box not found ... skipping restart”，但 UI 仍显示“重启成功”。

### Pitfall 2: Start Success != Running Success
**What goes wrong:** `cmd.Start()` 成功但进程立即退出，UI 短暂显示成功后又回落。
**Why it happens:** `Start` 只表示子进程已创建，不保证持续运行。
**How to avoid:** 启动后做短窗口复核（例如 300-1000ms 检查 `IsRunning`），失败则写入最近错误并置 `error`。
**Warning signs:** 启动 API 200，但下一次轮询立即变成 stopped/error。

### Pitfall 3: No Logs to Show in Error State
**What goes wrong:** 用户点击“查看日志”，却没有任何可读信息。
**Why it happens:** 当前 `cmd.Stdout/cmd.Stderr = nil`，默认输出到 `os.DevNull`。
**How to avoid:** 配置 sing-box `log.output` 文件，或至少保存最近一次启动错误文本供查看。
**Warning signs:** error 态频繁出现，但诊断面板为空。

### Pitfall 4: Cross-Module Restart Side Effects
**What goes wrong:** 入站/用户/证书接口在配置变更后仍“best-effort restart”，与新状态语义冲突。
**Why it happens:** 多处接口直接调用 `pm.Restart()` 且忽略错误。
**How to avoid:** 明确“配置应用成功”与“核心重启成功”为两个结果层级，必要时返回 warning 字段。
**Warning signs:** CRUD 成功但核心实际未运行，状态与提示不一致。

## Code Examples

Verified patterns from official sources:

### React Query Polling + Mutation Invalidation
```tsx
// Source: Context7 /tanstack/query (useQuery + invalidations-from-mutations)
const { data } = useQuery({
  queryKey: ['core', 'status'],
  queryFn: fetchStatus,
  refetchInterval: 5000,
})

const mutation = useMutation({
  mutationFn: startCore,
  onSuccess: async () => {
    await queryClient.invalidateQueries({ queryKey: ['core', 'status'] })
  },
})
```

### Go Process Lifecycle Resource Handling
```go
// Source: https://pkg.go.dev/os/exec#Cmd.Start
// "After a successful call to Start the Cmd.Wait method must be called
// in order to release associated system resources."
//
// Source: https://pkg.go.dev/os#Process.Release
// "Release only needs to be called if Process.Wait is not."
```

### sing-box File Log Output
```json
// Source: https://sing-box.sagernet.org/configuration/log/
{
  "log": {
    "level": "info",
    "output": "data/sing-box.log",
    "timestamp": true
  }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `running:boolean` 单态判断 | `state enum` 四态判断 | Phase 12 | UI 可表达 not_installed/error 语义 |
| 单一 Restart 操作 | start/stop/restart 分离 + 状态动作矩阵 | Phase 12 | 用户操作更可预期、可恢复 |
| 错误仅即时 toast | 错误上下文进入状态 API（error detail） | Phase 12 | 支持 Retry/View Logs |
| 无日志可见性 | 非实时日志文件按需查看 | Phase 12 | 不加依赖下可诊断启动失败 |

**Deprecated/outdated:**
- 仅依赖 `/api/core/status` 的 `running` 字段进行 UI 渲染（不足以表达 CORE-04）。
- 在 `error` 或 `not_installed` 状态下继续暴露“重启”作为主操作（语义错误）。

## Open Questions

1. **“查看日志”的最小接口边界**
   - What we know: 实时日志流 out-of-scope；sing-box 支持 `log.output` 文件。
   - What's unclear: 是否需要新增 `/api/core/logs`，以及返回最近 N 行还是整文件。
   - Recommendation: 先做“最近 N 行”只读接口（分页可选），避免一次性返回大文件。

2. **启动失败判定窗口**
   - What we know: `cmd.Start()` 不代表健康运行。
   - What's unclear: 用多少毫秒窗口判定“立即失败”最稳妥。
   - Recommendation: 先用 500ms 复核 `IsRunning`，并在验证阶段根据真实环境调优。

3. **配置变更接口与核心状态关系**
   - What we know: 多个 CRUD 接口当前“重启失败忽略”。
   - What's unclear: Phase 12 是否同时改造这些接口的反馈语义。
   - Recommendation: 至少补充 warning 字段并记录失败原因；完整一致性可在后续 plan 拆子任务。

## Sources

### Primary (HIGH confidence)
- Context7 `/tanstack/query` — `useQuery.refetchInterval`、mutation 后 `invalidateQueries` 模式
- Go official docs: `https://pkg.go.dev/os/exec#Cmd.Start`（Start/Wait 资源释放语义）
- Go official docs: `https://pkg.go.dev/os#Process.Release`（Wait vs Release 关系）
- sing-box official docs: `https://sing-box.sagernet.org/configuration/log/`（`log.output` 文件输出）
- Repository code: `internal/core/process.go`, `internal/api/core.go`, `internal/api/routes.go`, `web/src/pages/Core.tsx`, `cmd/server/main.go`

### Secondary (MEDIUM confidence)
- GitHub REST releases docs: `https://docs.github.com/en/rest/releases/releases#list-releases`（用于理解现有 core 管理上下文）
- Historical phase research: `.planning/milestones/v1.0-phases/07-core-management/07-RESEARCH.md`

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 全部来自仓库现状与官方文档，可直接执行。
- Architecture: HIGH - 与当前代码组织和项目决策一致，无新增依赖冲突。
- Pitfalls: HIGH - 关键坑点都能在现有代码或官方文档中直接验证。

**Research date:** 2026-02-26
**Valid until:** 2026-03-28 (30 days)
