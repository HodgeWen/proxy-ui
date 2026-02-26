# Phase 14: UI Polish & Consistency - Research

**Researched:** 2026-02-26
**Domain:** 前端 UI 一致性收敛（spacing/color tokens）与核心状态指示器动效
**Confidence:** HIGH

<user_constraints>
## User Constraints (no CONTEXT.md; from roadmap/requirements/STATE + user prompt)

### Locked Decisions
- 本阶段目标固定：全站视觉一致性 + 核心状态指示器动效（Phase 14: UI Polish & Consistency）。
- 必须覆盖需求：`UICON-01`, `UICON-02`。
- v1.1 约束：**不新增 npm/Go 依赖**（除非绝对必要）。
- Phase 11 已建立动效基线（统一 hover/entrance），Phase 14 应复用并收敛，不重做动画体系。
- Phase 12 已集中核心状态契约在 `web/src/lib/core-status.ts`，Phase 14 动效应基于该契约扩展，不改状态语义。
- Phase 13 决策保持：SSE 仅用于更新进度；核心状态仍由 TanStack Query 5s 轮询。

### Claude's Discretion
- 间距规范如何落地（公共 class 常量 vs 逐页修正）。
- 色彩收敛范围（先修高风险硬编码，再做增量全量审计）。
- 核心状态动效节奏参数（pulse/blink 周期、是否对 reduced-motion 降级）。

### Deferred Ideas (OUT OF SCOPE)
- 引入新的动画库、设计系统库或主题库。
- 改动核心状态来源/轮询策略（不改为状态 SSE）。
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| UICON-01 | 全局间距和色彩一致性审查与修正（页面间距统一、色彩变量规范、暗色主题一致性） | 已定位到高风险不一致热点（认证页硬编码色值、QR 白底、局部 gap 不统一）；建议“先热点、后扫尾”的最小安全收敛方案。 |
| UICON-02 | 核心状态指示器动效（运行中绿色脉冲、已停止灰色、未安装黄色提示、异常红色闪烁） | 现有状态源已集中在 `core-status.ts`，可在不改业务逻辑前提下增加状态到动效 class 的映射，并在 `index.css` 增加 blink keyframes。 |
</phase_requirements>

## Summary

当前仓库已经具备较好的统一基础：主业务页大多采用 `p-6 space-y-6`，shadcn token（`bg-background`, `text-foreground`, `border-border`）也已在核心组件广泛使用。真正影响 Phase 14 成功率的不是“大规模重构”，而是少量“绕开 token 的硬编码点”与“状态动效缺口”。

`UICON-01` 的最小风险路径是先修复视觉破坏性最高的热点：认证页硬编码深色 hex/gray、QR 区块 `bg-white`、以及个别页面栅格间距偏差（如 `Traffic` 统计卡 `gap-4` vs `Dashboard` 的 `gap-6`）。这些修复都可在现有组件体系内完成，不需要新增依赖，也不会触碰后端接口。

`UICON-02` 的最小风险路径是保持状态来源不变（`core-status.ts` + 5s 轮询），仅为指示点增加动画 class：running=绿色 pulse，stopped=灰色静态，not_installed=黄色静态，error=红色 blink；并加 `motion-reduce:animate-none` 保障可访问性。  

**Primary recommendation:** 采用“热点收敛 + 状态指示点动效映射”双轨实现，避免全量重构页面结构，确保最小改动满足 UICON-01/UICON-02。

## Current Inconsistency Hotspots (Repo Audit)

| Path | Hotspot | Impact | Priority |
|------|---------|--------|----------|
| `web/src/pages/Login.tsx` | 大量硬编码 `bg-[#111827]` / `text-gray-*` / `border-gray-600` | 绕过主题变量，light/dark 一致性不可控 | P0 |
| `web/src/pages/Setup.tsx` | 与 Login 同类硬编码色值和原生控件样式 | 与主站 token 体系割裂，后续维护成本高 | P0 |
| `web/src/pages/Subscriptions.tsx` | QR 容器 `bg-white` + 非 token 边框 | 暗色下视觉突兀（白块） | P1 |
| `web/src/components/users/UserSubscriptionCard.tsx` | QR 区域同样 `bg-white` | 与上条一致，组件级不一致 | P1 |
| `web/src/pages/Traffic.tsx` | 顶部统计卡使用 `grid gap-4`（与 Dashboard `gap-6` 不一致） | 页面密度感不统一 | P1 |
| `web/src/lib/core-status.ts` + `web/src/pages/Core.tsx` | 状态点仅颜色无动效 | 未满足 UICON-02 | P0 |
| `web/src/App.css` | Vite 初始样式残留（且当前未被引用） | 主题规范噪音源，易误导后续开发 | P2 |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React | `^19.2.0` | 页面与组件渲染 | 当前主框架，动效 class 映射改动成本最低 |
| Tailwind CSS | `^4.1.18` | 间距/色彩/动效 utility | 已有完整 token 与 dark variant 机制 |
| tw-animate-css | `^1.4.0` | 既有入场动画体系 | 已在项目引入，Phase 11 已使用 |
| @tanstack/react-query | `^5.90.20` | 核心状态轮询 | 与既有 5s 轮询决策一致 |
| shadcn/ui 组件（in-repo） | in-repo | token 化组件基线 | 一致性收敛应继续依赖它而非重建 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `lucide-react` | `^0.563.0` | 状态/动作图标 | 不涉及方案变更 |
| `sonner` | `^2.0.7` | 交互提示 | 错误态交互保持现状 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 局部热点修复 | 全量页面重构为统一“PageShell”组件 | 变更面过大，回归风险高，不符合“最小安全” |
| CSS 动效 class | 引入 motion/GSAP/Framer | 违反“无新依赖”与阶段目标 |
| 状态点动效映射 | 重做核心状态展示组件层次 | 价值低、风险高，且不必要 |

**Installation:**
```bash
# No new dependencies for Phase 14
```

## Smallest-Safe Implementation Strategy

### Strategy A: Hotspot-first UI consistency (UICON-01)
**What:** 仅修高风险不一致点，不做大规模页面结构改造。  
**When to use:** 现有页面已基本统一，但有少量硬编码破口。  
**How (repo-practical):**
1. 认证页（`Login`/`Setup`）将硬编码色值替换为 token utility（`bg-background`, `bg-card`, `text-foreground`, `border-input`, `text-muted-foreground`）。
2. QR 容器白底改为 token 背景（避免 dark 模式白块）。
3. 统一页面级 spacing 差异点（优先 `Traffic` 卡片区 gap）。
4. 清理未使用的 `App.css`（或明确标注弃用），减少规范噪音。

### Strategy B: Status-indicator animation by state map (UICON-02)
**What:** 在 `core-status.ts` 扩展“状态 -> 指示点动画 class”映射，不改状态数据流。  
**When to use:** 状态契约已稳定（Phase 12），只需补齐动效表现。  
**How (repo-practical):**
1. `running`: 绿色 + `animate-pulse`。
2. `stopped`: 灰色静态。
3. `not_installed`: 黄色静态（必要时保留轻提示文案）。
4. `error`: 红色 + 自定义 blink keyframes（`index.css`），并加 `motion-reduce:animate-none`。

## Architecture Patterns

### Recommended Project Structure (delta-oriented)
```text
web/src/
├── index.css                         # 主题变量 + status blink keyframes（增量）
├── lib/
│   └── core-status.ts                # 扩展状态元数据：indicator 动效 class
├── pages/
│   ├── Core.tsx                      # 使用状态元数据渲染指示点（不改轮询）
│   ├── Login.tsx                     # 去硬编码色值，转 token
│   ├── Setup.tsx                     # 去硬编码色值，转 token
│   ├── Traffic.tsx                   # spacing 对齐
│   └── Subscriptions.tsx             # QR 区域 token 化
└── components/users/
    └── UserSubscriptionCard.tsx      # QR 区域 token 化
```

### Pattern 1: Token-first color usage
**What:** 颜色优先使用语义 token utility，而不是 hex/gray 硬编码。  
**When to use:** 登录/初始化页与局部强调块（可保留语义色如 red/amber/emerald 作为状态色）。  
**Example:**
```tsx
// Source: shadcn theming convention + current repo token setup
<div className="min-h-screen bg-background text-foreground">
  <div className="bg-card border border-border rounded-xl">
    <input className="bg-background border-input text-foreground placeholder:text-muted-foreground" />
  </div>
</div>
```

### Pattern 2: Status animation via metadata (single source)
**What:** 状态颜色与动效集中在 `core-status.ts`，页面只消费 class。  
**When to use:** 任何核心状态视觉渲染（点、badge、辅助文案色）。  
**Example:**
```ts
// Source: repo pattern in web/src/lib/core-status.ts
type CoreStateMeta = {
  label: string
  description: string
  indicatorClassName: string
}
```

### Pattern 3: Reduced-motion safe animation
**What:** 对 pulse/blink 添加 `motion-reduce:animate-none`。  
**When to use:** 所有持续动画（尤其无限循环）。  
**Example:**
```tsx
<span className="size-2.5 rounded-full bg-red-500 animate-[status-blink_1s_steps(2,end)_infinite] motion-reduce:animate-none" />
```

### Anti-Patterns to Avoid
- **全量重写所有页面布局：** 变更面过大，Phase 14 目标不需要。
- **在组件内 scattered 硬编码状态色：** 会导致同状态多种视觉版本。
- **用 JS `setInterval` 手写闪烁：** CSS 动画足够且更稳定。
- **修改核心状态来源协议：** 会与 Phase 12/13 已稳定约束冲突。

## Plan Decomposition Candidates

### Plan 14-01 (UICON-01-A): Global spacing/color hotspot audit & fixes
**Scope:** `Login`, `Setup`, `Subscriptions`, `UserSubscriptionCard`, `Traffic`。  
**Deliverables:**
- 移除高风险硬编码色值，改为 token utility。
- 统一页面关键间距（`p-6 space-y-6` 维持、局部 gap 对齐）。
- 确认 `App.css` 处理策略（删除或明确不再使用）。  
**Wave hint:** Wave 1（可先独立完成，风险最低）。

### Plan 14-02 (UICON-02): Core status indicator animations
**Scope:** `core-status.ts`, `Core.tsx`, `index.css`。  
**Deliverables:**
- 4 态指示点颜色+动效达成：running pulse / stopped gray / not_installed yellow / error blink。
- 动效可访问性降级（reduced-motion）。
- 保持 Query 5s 轮询与 SSE 更新进度逻辑不变。  
**Dependencies:** 依赖 Phase 12 状态契约（已完成）；可与 14-01 并行开发、串行合并。  
**Wave hint:** Wave 2。

### Plan 14-03 (cross-cutting): Visual regression verification & consistency checklist
**Scope:** 全页面核对与回归验证。  
**Deliverables:**
- 路由级视觉核对清单（含 light/dark 与核心 4 态）。
- 自动化命令回归（lint/build）。
- 问题闭环记录（避免 Phase 14 收尾遗漏）。  
**Dependencies:** 14-01、14-02 完成后执行。  
**Wave hint:** Wave 3（收尾）。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 状态点闪烁 | JS 定时器控制 class 切换 | Tailwind/CSS keyframes | 代码更少、性能和可维护性更好 |
| 主题一致性 | 再造主题上下文或新 token 系统 | 现有 `index.css` + shadcn token 约定 | 当前体系已稳定，增量修复即可 |
| 页面统一 | 一次性重写 Page 框架 | 热点优先的增量统一 | 更安全、回归面更可控 |

**Key insight:** Phase 14 是“收敛破口”而非“重建体系”；复用现有 token 与状态契约即可满足目标。

## Common Pitfalls

### Pitfall 1: 修了主页面，漏掉认证页硬编码
**What goes wrong:** 登录/初始化页仍与主页面风格割裂。  
**Why it happens:** 这些页面不在 AppLayout 下，容易被审查漏掉。  
**How to avoid:** 将认证页纳入同一视觉审计清单。  
**Warning signs:** dark/light 切换后认证页颜色不随 token 变化。

### Pitfall 2: 只给 running 加动画，忽略 error 的“闪烁”语义
**What goes wrong:** UICON-02 被部分满足。  
**Why it happens:** 默认 `animate-pulse` 好用，但不等价 blink。  
**How to avoid:** 明确为 `error` 定义独立 blink keyframes。  
**Warning signs:** error 状态视觉与 running 区分不明显。

### Pitfall 3: 动效影响可访问性
**What goes wrong:** 用户开启减少动态后仍持续闪烁。  
**Why it happens:** 忽略 `motion-reduce`。  
**How to avoid:** 所有循环动效附加 `motion-reduce:animate-none`。  
**Warning signs:** 系统 reduced-motion 开启后 UI 仍持续动画。

### Pitfall 4: 视觉修复与状态逻辑耦合过深
**What goes wrong:** UI polish 引入业务回归（按钮可用性或状态判断变化）。  
**Why it happens:** 在同次改动中混入状态逻辑变更。  
**How to avoid:** 限定 Phase 14 只改“视觉层映射”，不改状态计算和 API。  
**Warning signs:** core action 行为与 Phase 12 验证结果不一致。

## Code Examples

Verified patterns from official sources + current repo conventions:

### Core status dot with state animation mapping
```tsx
// Source: repo contract web/src/lib/core-status.ts + web/src/pages/Core.tsx
<span
  className={`inline-block size-2.5 rounded-full ${stateMeta?.indicatorClassName ?? "bg-muted-foreground"}`}
/>
```

### Error blink keyframes (Tailwind custom animation value)
```css
/* Source: Tailwind animation docs (custom animate-[]/theme vars) */
@keyframes status-blink {
  0%, 49% { opacity: 1; }
  50%, 100% { opacity: 0.35; }
}
```

```tsx
<span className="bg-red-500 animate-[status-blink_1s_steps(2,end)_infinite] motion-reduce:animate-none" />
```

### Token-based QR container
```tsx
<div className="rounded-lg border border-border bg-card p-4">
  <QRCodeSVG value={url} size={200} />
</div>
```

## Verification Strategy

### Automated
- `cd web && npm run lint`
- `cd web && npm run build`
- 若改动触及共享组件样式，补充一次全仓快速回归：`go test ./...`（确认无意外波及）

### Manual (UICON-01)
- 路由巡检：`/`, `/inbounds`, `/users`, `/subscriptions`, `/certificates`, `/traffic`, `/core`, `/login`, `/setup`。
- 检查点：页面外边距、标题与首块间距、卡片/表格区块间距、边框/背景是否出现“突兀灰阶”。
- light/dark 双模式核对（可通过 `localStorage['s-ui-theme']` 切换后刷新）。

### Manual (UICON-02)
- `running`: 启动核心后，状态点应为绿色脉冲。
- `stopped`: 停止核心后，状态点应为灰色静态。
- `not_installed`: 核心缺失时，状态点应为黄色静态且文案提示安装。
- `error`: 制造启动失败后，状态点应为红色闪烁。
- reduced-motion 场景：动画应正确降级为静态。

## Risk Mitigation

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| 认证页改造引发样式回归 | Medium | Medium | 先做 class 映射替换，不改交互逻辑；逐页手测 |
| 状态动效误伤逻辑层 | Low | High | 严格限制在 `className` 映射，不改状态计算/请求逻辑 |
| 暗色主题出现新对比度问题 | Medium | Medium | 对比审查重点放在 `bg-white` 与硬编码 gray 移除点 |
| Phase 14 收尾遗漏页面 | Medium | Medium | 用固定路由清单 + checklist 验收 |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 页面局部硬编码颜色 | token-first (`bg-background`/`text-foreground`) | Phase 14 | light/dark 一致性提升，后续维护简化 |
| 状态点静态纯色 | 状态驱动的色彩+动效映射 | Phase 14 | 核心状态辨识度显著提升 |
| 视觉审查靠主观浏览 | 路由清单 + 核对点 + 命令回归 | Phase 14 | 可重复、可交付、可复盘 |

**Deprecated/outdated:**
- 在新改动中继续引入 hex/gray 硬编码作为主视觉颜色来源。
- 在状态指示器中写散落的颜色 class（不集中在 `core-status.ts`）。

## Open Questions

1. **认证页是否应完全切到 shadcn 组件**
   - What we know: 当前可仅通过 token class 修复主要不一致。
   - What's unclear: 是否本阶段就做组件级统一（Input/Button/Card 全替换）。
   - Recommendation: Phase 14 先做最小 class 修复；组件重构放后续。

2. **error blink 频率参数**
   - What we know: 需要红色闪烁且可立即识别。
   - What's unclear: 1s 与 1.2s 在当前 UI 上的感知最优值。
   - Recommendation: 默认 1s（steps），验收时按视觉反馈微调。

## Sources

### Primary (HIGH confidence)
- Repository code audit:
  - `web/src/index.css`
  - `web/src/lib/core-status.ts`
  - `web/src/pages/Core.tsx`
  - `web/src/pages/Login.tsx`
  - `web/src/pages/Setup.tsx`
  - `web/src/pages/Traffic.tsx`
  - `web/src/pages/Subscriptions.tsx`
  - `web/src/components/users/UserSubscriptionCard.tsx`
  - `web/src/components/ui/card.tsx`
  - `web/src/components/ui/table.tsx`
  - `web/src/components/ui/button.tsx`
  - `.planning/ROADMAP.md`
  - `.planning/REQUIREMENTS.md`
  - `.planning/STATE.md`

### Secondary (MEDIUM confidence)
- Tailwind animation docs: `https://tailwindcss.com/docs/animation`（`animate-pulse`、自定义 `animate-[]`、`motion-reduce`）
- Tailwind dark mode docs: `https://tailwindcss.com/docs/dark-mode`（自定义 dark variant 与 class 驱动）
- shadcn theming docs: `https://ui.shadcn.com/docs/theming`（CSS 变量约定与 token 用法）

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 依赖版本与约束来自仓库实际 `web/package.json` 与既有 phase 决策。
- Architecture: HIGH - 基于现有文件结构和状态契约，方案为增量式、低入侵。
- Pitfalls: HIGH - 风险点均已在当前代码中可直接定位并可验证。

**Research date:** 2026-02-26
**Valid until:** 2026-03-28 (30 days)
