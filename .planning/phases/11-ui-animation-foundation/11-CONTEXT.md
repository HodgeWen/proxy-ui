# Phase 11: UI Animation Foundation - Context

**Gathered:** 2026-02-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Activate react-bits animations (CountUp, AnimatedContent, SpotlightCard) and tw-animate-css across all pages. Deliver polished visual feedback: number roll-ups on dashboard, card entrance animations, spotlight hover glow, and consistent hover transitions. No new npm dependencies — use already-installed packages only.

</domain>

<decisions>
## Implementation Decisions

### 数字动画风格 (CountUp)
- 动画时长：快速（0.8-1s），简短利落
- 缓动曲线：Ease-out（开始快、接近终点减速）
- 触发节奏：所有统计数字同时开始滚动
- 格式化：动画过程中保持格式化显示（千分位、小数点始终可见）

### 卡片入场效果 (Card Entrance)
- 动画类型：Fade + 微缩放（从 95% 放大到 100%）
- 出现节奏：交错出现，每张卡片错开 50-100ms
- 单卡时长：短（200-300ms），快速且微妙
- 触发时机：仅首次渲染触发，数据刷新不重播

### Spotlight 光效交互
- 应用范围：Dashboard 所有主要卡片（不仅限 core 状态卡）
- 光晕颜色：跟随主题 primary 色
- 光晕强度：微妙低调，仔细看才注意到
- 暗色模式：暗色下更亮更明显，利用暗背景突出效果

### 全局 hover 一致性
- 卡片 hover：边框微光（边框颜色变亮，不移动位置）
- 按钮 hover：色彩加深（背景色加深）
- 表格行 hover：背景高亮（行背景变浅色）
- 过渡时间：快速（120-150ms），立即响应几乎无延迟

### Claude's Discretion
- CountUp 的具体起始延迟（页面加载后何时开始）
- 卡片入场的具体 stagger 间隔（50-100ms 范围内选择）
- Spotlight 光晕半径和具体 opacity 值
- 边框微光的具体颜色值和渐变方式
- 动画在 reduced-motion 偏好下的降级策略

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 11-ui-animation-foundation*
*Context gathered: 2026-02-23*
