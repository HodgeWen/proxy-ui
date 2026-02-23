# Phase 11: UI Animation Foundation - Research

**Researched:** 2026-02-23
**Domain:** React animation components, CSS animation utilities, hover interaction patterns
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **数字动画风格 (CountUp)**: 动画时长 0.8-1s, Ease-out 缓动, 所有统计数字同时开始, 动画中保持千分位/小数点格式化
- **卡片入场效果 (Card Entrance)**: Fade + 微缩放 (95% → 100%), 交错出现 50-100ms, 单卡 200-300ms, 仅首次渲染触发
- **Spotlight 光效**: Dashboard 所有主要卡片, 跟随主题 primary 色, 微妙低调, 暗色模式下更亮更明显
- **全局 hover 一致性**: 卡片边框微光, 按钮色彩加深, 表格行背景高亮, 过渡 120-150ms

### Claude's Discretion
- CountUp 的具体起始延迟（页面加载后何时开始）
- 卡片入场的具体 stagger 间隔（50-100ms 范围内选择）
- Spotlight 光晕半径和具体 opacity 值
- 边框微光的具体颜色值和渐变方式
- 动画在 reduced-motion 偏好下的降级策略

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| UIANM-01 | 仪表盘统计数字使用 CountUp 动画展示 | Custom `useCountUp` hook with requestAnimationFrame + ease-out math (no external deps needed); see Code Examples section |
| UIANM-02 | 所有页面卡片使用入场动效 | tw-animate-css `animate-in fade-in zoom-in-95` classes + custom `useAnimateOnMount` hook with IntersectionObserver for trigger + staggered delays |
| UIANM-03 | 核心状态卡片使用 SpotlightCard 聚光悬停效果 | Copy SpotlightCard source from reactbits.dev (zero external deps, pure React refs + state); customize spotlightColor for theme |
| UIANM-04 | 全局统一组件悬停交互效果 | Tailwind CSS `transition-colors duration-150` utilities on Card/Button/TableRow base components |
</phase_requirements>

## Summary

This phase adds animation polish to the s-ui admin panel using zero new npm dependencies. The critical discovery is that the npm package `react-bits@1.0.5` (installed in package.json) is a **completely different library** from React Bits at reactbits.dev — it provides cross-platform React primitives by Doug Miller, NOT animation components. The animation components (CountUp, AnimatedContent, SpotlightCard) from reactbits.dev are copy-paste components with their own peer dependencies: CountUp needs `motion@^12`, AnimatedContent needs `gsap@^3`, and only SpotlightCard has zero external dependencies.

Given the "no new npm dependencies" constraint, the implementation strategy is: (1) **SpotlightCard** — copy directly from reactbits.dev (no deps), (2) **CountUp** — build a lightweight custom `useCountUp` hook using `requestAnimationFrame` with ease-out timing (~40 lines), (3) **Card entrance animations** — use the already-imported `tw-animate-css` utility classes (`animate-in fade-in zoom-in-95 duration-300`) triggered by a custom `useAnimateOnMount` hook with IntersectionObserver, and (4) **Hover consistency** — update the shared shadcn Card/Button/TableRow base components with uniform Tailwind transition utilities.

**Primary recommendation:** Copy SpotlightCard as-is from reactbits.dev, build custom CountUp hook with requestAnimationFrame, use tw-animate-css for all card entrance animations, and standardize hover transitions in shadcn base components.

## Standard Stack

### Core (already installed, no changes)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| tw-animate-css | ^1.4.0 | CSS animation utility classes | Already imported in index.css; provides `animate-in`, `fade-in`, `zoom-in-*`, `slide-in-from-*`, `duration-*`, `delay-*` |
| tailwindcss | ^4.1.18 | Utility-first CSS framework | Provides `transition-*`, `duration-*`, `hover:*`, `motion-reduce:*` variants |
| react | ^19.2.0 | UI framework | useRef, useEffect, useState, useCallback for custom hooks |

### Supporting (copy-paste, not npm)
| Library | Source | Purpose | When to Use |
|---------|--------|---------|-------------|
| SpotlightCard | reactbits.dev | Mouse-tracking spotlight glow | Dashboard/Core status cards |

### Why NOT use reactbits.dev CountUp/AnimatedContent
| Component | Requires | Status | Alternative |
|-----------|----------|--------|-------------|
| CountUp (reactbits.dev) | `motion@^12.23.12` | NOT installed, violates constraint | Custom `useCountUp` hook with rAF |
| AnimatedContent (reactbits.dev) | `gsap@^3.13.0` | NOT installed, violates constraint | tw-animate-css classes + IntersectionObserver |
| SpotlightCard (reactbits.dev) | None | Safe to copy | Direct copy-paste |

**Installation:** No installation needed. All dependencies already present. SpotlightCard source is copied into `web/src/components/ui/`.

## Architecture Patterns

### Recommended Component Structure
```
web/src/
├── components/
│   ├── ui/
│   │   ├── spotlight-card.tsx    # Copied from reactbits.dev, adapted for theme
│   │   ├── card.tsx              # Updated with hover transition classes
│   │   ├── button.tsx            # Updated with transition timing
│   │   └── table.tsx             # Updated with row hover timing
│   └── ...
├── hooks/
│   ├── use-count-up.ts           # Custom CountUp hook (rAF + ease-out)
│   └── use-animate-on-mount.ts   # IntersectionObserver trigger for entrance animations
└── pages/
    ├── Dashboard.tsx             # CountUp + SpotlightCard + entrance
    ├── Core.tsx                  # SpotlightCard on status card + entrance
    ├── Traffic.tsx               # CountUp on stat numbers + entrance
    └── ...                       # All pages get card entrance animations
```

### Pattern 1: Custom useCountUp Hook
**What:** Lightweight number animation using requestAnimationFrame with ease-out easing
**When to use:** Dashboard/Traffic stat numbers that animate from 0 to target value
**Key design:**
- Uses `requestAnimationFrame` for smooth 60fps animation
- Ease-out cubic: `1 - Math.pow(1 - t, 3)` where t = elapsed/duration
- Returns formatted string (supports separator, decimals) via `Intl.NumberFormat`
- Triggered by IntersectionObserver (`useInView` pattern) + `startWhen` prop
- Duration configurable (user decision: 0.8-1s)

```typescript
// web/src/hooks/use-count-up.ts
import { useEffect, useRef, useState, useCallback } from 'react';

interface UseCountUpOptions {
  from?: number;
  to: number;
  duration?: number;        // seconds, default 1
  delay?: number;           // seconds, default 0
  startWhen?: boolean;      // external trigger
  separator?: string;       // thousands separator
  decimals?: number;        // fixed decimal places
}

function easeOutCubic(t: number): number {
  return 1 - Math.pow(1 - t, 3);
}

export function useCountUp({
  from = 0,
  to,
  duration = 1,
  delay = 0,
  startWhen = true,
  separator = ',',
  decimals,
}: UseCountUpOptions) {
  const [value, setValue] = useState(from);
  const rafRef = useRef<number>(0);
  const startTimeRef = useRef<number>(0);
  const hasAnimated = useRef(false);

  const maxDecimals = decimals ?? Math.max(
    (from.toString().split('.')[1] || '').length,
    (to.toString().split('.')[1] || '').length,
  );

  const formatValue = useCallback((num: number) => {
    return new Intl.NumberFormat('en-US', {
      useGrouping: !!separator,
      minimumFractionDigits: maxDecimals,
      maximumFractionDigits: maxDecimals,
    }).format(num).replace(/,/g, separator);
  }, [separator, maxDecimals]);

  useEffect(() => {
    if (!startWhen || hasAnimated.current) return;
    hasAnimated.current = true;

    const delayMs = delay * 1000;
    const durationMs = duration * 1000;
    const range = to - from;

    const timeout = setTimeout(() => {
      const animate = (timestamp: number) => {
        if (!startTimeRef.current) startTimeRef.current = timestamp;
        const elapsed = timestamp - startTimeRef.current;
        const progress = Math.min(elapsed / durationMs, 1);
        const easedProgress = easeOutCubic(progress);

        setValue(from + range * easedProgress);

        if (progress < 1) {
          rafRef.current = requestAnimationFrame(animate);
        } else {
          setValue(to);
        }
      };
      rafRef.current = requestAnimationFrame(animate);
    }, delayMs);

    return () => {
      clearTimeout(timeout);
      cancelAnimationFrame(rafRef.current);
    };
  }, [startWhen, from, to, duration, delay]);

  return { value, formatted: formatValue(value) };
}
```

### Pattern 2: CSS-first Card Entrance with tw-animate-css
**What:** Fade + scale entrance using tw-animate-css utilities, triggered on mount
**When to use:** All cards across all pages on first render
**Key design:**
- Classes: `animate-in fade-in zoom-in-95 duration-300 fill-mode-both`
- Stagger via `delay-*` utility or inline `style={{ animationDelay }}` for dynamic values
- `fill-mode-both` ensures element stays invisible before animation and visible after
- Only plays once on first render (CSS animation plays once by default)
- No JS library needed — pure CSS animation

```tsx
// Pattern for staggered card entrance
function Dashboard() {
  return (
    <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
      {cards.map((card, i) => (
        <div
          key={card.id}
          className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both"
          style={{ animationDelay: `${i * 75}ms` }}
        >
          <SpotlightCard>
            {/* card content */}
          </SpotlightCard>
        </div>
      ))}
    </div>
  );
}
```

### Pattern 3: SpotlightCard Adaptation
**What:** Mouse-tracking radial gradient overlay that follows cursor
**When to use:** Dashboard and Core status cards
**Key design:**
- Copy SpotlightCard.tsx from reactbits.dev — zero external dependencies
- Adapt `spotlightColor` to use CSS variable `var(--primary)` for theme consistency
- Convert rgba format to use oklch from theme (or pass computed value)
- In dark mode: increase opacity for more visible glow (user decision)

### Pattern 4: Consistent Hover Transitions
**What:** Uniform transition timing and effects across all interactive elements
**When to use:** All buttons, cards, and table rows
**Key design:**
- Apply at the base component level (card.tsx, button.tsx, table.tsx)
- Cards: `transition-colors duration-150` + `hover:border-border/80` (border brightening)
- Buttons: already have `transition-all`, ensure timing is `duration-150`
- Table rows: already have `transition-colors`, add explicit `duration-150`

### Anti-Patterns to Avoid
- **Animating layout properties:** Never animate `width`, `height`, `padding`, `margin` — causes layout shifts. Only animate `opacity`, `transform`, `filter`, `box-shadow`, `border-color`.
- **Re-triggering entrance animations on data refresh:** User explicitly decided "仅首次渲染触发, 数据刷新不重播". Use a ref flag (`hasAnimated`) or CSS `animation-fill-mode: both` which plays once.
- **Adding motion/gsap as "just a small dep":** The constraint is explicit — no new npm dependencies. Use native APIs.
- **Using JS for transitions that CSS can handle:** Button/card/table hover effects should be pure CSS transitions, not JS-driven.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Fade/scale entrance animations | Custom JS animation loop | tw-animate-css `animate-in fade-in zoom-in-95` | CSS animations are GPU-accelerated, declarative, no JS overhead |
| Spotlight glow effect | Custom radial gradient + mouse tracking | SpotlightCard from reactbits.dev | Handles edge cases (mouse leave, focus, blur, SSR) |
| Animation delay stagger | Custom setTimeout chains | CSS `animation-delay` via inline style | Simpler, no state management, GPU-scheduled |
| Hover transitions | JS onMouseEnter/onMouseLeave | Tailwind `transition-colors duration-150 hover:*` | Pure CSS, zero JS, better performance |
| Reduced-motion handling | Custom media query listener | Tailwind `motion-reduce:*` variant | Framework-native, tree-shaken |

**Key insight:** The entire card entrance animation system can be built with zero JavaScript animation code — tw-animate-css provides all the CSS primitives, and CSS `animation-delay` handles staggering. Only CountUp requires a JS animation loop because it needs to interpolate numeric values.

## Common Pitfalls

### Pitfall 1: Layout Shift from Card Entrance Animations
**What goes wrong:** Cards animate from `opacity: 0` + `scale(0.95)` but their space isn't reserved, causing content below to jump.
**Why it happens:** The element is hidden/scaled before animation starts but still affects layout.
**How to avoid:** Use `fill-mode-both` which applies the `from` state before animation starts. The element is transparent but occupies its full space. The scale from 95% to 100% is subtle enough (5% change) to not cause noticeable layout shift.
**Warning signs:** Content jumping when page loads, elements appearing to push each other.

### Pitfall 2: CountUp Flashing Wrong Value
**What goes wrong:** Number briefly shows the final value before animation starts (FOUC-like).
**Why it happens:** React renders the component with data before the animation hook takes effect.
**How to avoid:** Initialize displayed value to `from` (0) in the hook's initial state, and only update via the animation loop. The hook returns `formatted` which starts at "0" and animates to the target.
**Warning signs:** Numbers flash to full value then reset to 0 and animate up.

### Pitfall 3: SpotlightCard Color Mismatch in Dark Mode
**What goes wrong:** Spotlight color doesn't match theme or looks wrong in dark mode.
**Why it happens:** Hard-coded rgba values don't adapt to theme. The default `rgba(255, 255, 255, 0.25)` is visible on dark but invisible on light backgrounds.
**How to avoid:** Compute spotlight color from the CSS `--primary` variable. Use `getComputedStyle` to read the oklch value and convert, or simply pass different rgba values per mode. The user decided: light mode subtle, dark mode brighter.
**Warning signs:** Spotlight invisible in light mode, too harsh in dark mode.

### Pitfall 4: Entrance Animations Re-playing on Route Change
**What goes wrong:** Cards re-animate every time user navigates away and back.
**Why it happens:** React re-mounts page components on route change, re-triggering CSS animations.
**How to avoid:** This is actually acceptable behavior per user decision ("仅首次渲染触发"). CSS animations play on mount and won't replay on data refresh (TanStack Query updates). Route changes DO re-mount and re-animate, which is the desired UX — you want entrance animations when navigating to a page.
**Warning signs:** None — this is expected behavior.

### Pitfall 5: Animation Timing Mismatch Between CountUp and Card Entrance
**What goes wrong:** Numbers finish counting before cards finish fading in, or vice versa.
**Why it happens:** CountUp duration (0.8-1s) and card entrance duration (200-300ms) run on different timelines.
**How to avoid:** Card entrance completes quickly (300ms), then CountUp runs (0.8-1s). Start CountUp after a brief delay matching the card entrance duration so the number animation begins after the card is fully visible.
**Warning signs:** Numbers animating inside a still-transparent card.

### Pitfall 6: react-bits npm Package Confusion
**What goes wrong:** Developer tries to import CountUp from `react-bits` npm package.
**Why it happens:** The npm package `react-bits@1.0.5` (by Doug Miller) is a completely different library from React Bits (reactbits.dev, by David Huynh). The npm package provides cross-platform React primitives (Animated, Dimensions, StyleSheet), NOT animation components.
**How to avoid:** Do NOT import from `react-bits`. Build custom CountUp hook. Copy SpotlightCard as a local component file.
**Warning signs:** TypeScript errors about missing exports, "cannot find module" for CountUp/SpotlightCard.

## Code Examples

### CountUp Usage in Dashboard
```tsx
// Source: Custom implementation pattern
import { useCountUp } from '@/hooks/use-count-up';

function StatCard({ value, label, icon: Icon }: StatCardProps) {
  const { formatted } = useCountUp({
    to: value,
    duration: 1,
    separator: ',',
  });

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">{label}</CardTitle>
        <Icon className="size-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">{formatted}</div>
      </CardContent>
    </Card>
  );
}
```

### SpotlightCard Source (from reactbits.dev — zero deps)
```tsx
// Source: https://reactbits.dev/r/SpotlightCard-TS-TW
// This is the FULL component — no external dependencies
import React, { useRef, useState } from 'react';

interface SpotlightCardProps extends React.PropsWithChildren {
  className?: string;
  spotlightColor?: string;
}

const SpotlightCard: React.FC<SpotlightCardProps> = ({
  children,
  className = '',
  spotlightColor = 'rgba(255, 255, 255, 0.25)'
}) => {
  const divRef = useRef<HTMLDivElement>(null);
  const [isFocused, setIsFocused] = useState(false);
  const [position, setPosition] = useState({ x: 0, y: 0 });
  const [opacity, setOpacity] = useState(0);

  const handleMouseMove: React.MouseEventHandler<HTMLDivElement> = e => {
    if (!divRef.current || isFocused) return;
    const rect = divRef.current.getBoundingClientRect();
    setPosition({ x: e.clientX - rect.left, y: e.clientY - rect.top });
  };

  const handleFocus = () => { setIsFocused(true); setOpacity(0.6); };
  const handleBlur = () => { setIsFocused(false); setOpacity(0); };
  const handleMouseEnter = () => { setOpacity(0.6); };
  const handleMouseLeave = () => { setOpacity(0); };

  return (
    <div
      ref={divRef}
      onMouseMove={handleMouseMove}
      onFocus={handleFocus}
      onBlur={handleBlur}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
      className={`relative overflow-hidden rounded-xl ${className}`}
    >
      <div
        className="pointer-events-none absolute inset-0 transition-opacity duration-300"
        style={{
          opacity,
          background: `radial-gradient(600px circle at ${position.x}px ${position.y}px, ${spotlightColor}, transparent 40%)`,
        }}
      />
      {children}
    </div>
  );
};
```

### Card Entrance with tw-animate-css (Staggered)
```tsx
// Source: tw-animate-css docs (https://github.com/wombosvideo/tw-animate-css)
// Pattern: animate-in + fade-in + zoom-in-95 + stagger delay
<div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
  {stats.map((stat, index) => (
    <div
      key={stat.key}
      className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both"
      style={{ animationDelay: `${index * 75}ms` }}
    >
      <SpotlightCard spotlightColor="rgba(139, 92, 246, 0.15)">
        <Card>
          {/* ... card content ... */}
        </Card>
      </SpotlightCard>
    </div>
  ))}
</div>
```

### tw-animate-css Key Classes Reference
```css
/* Base: required for enter animations */
.animate-in { animation: enter 150ms ease; }

/* Opacity: fade from 0% */
.fade-in { --tw-enter-opacity: 0; }

/* Scale: zoom from 95% (subtle micro-scale) */
.zoom-in-95 { --tw-enter-scale: 0.95; }

/* Duration override (use Tailwind duration utilities) */
.duration-300 { --tw-duration: 300ms; }

/* Fill mode: keep end state after animation */
.fill-mode-both { animation-fill-mode: both; }

/* Delay: stagger cards (use inline style for dynamic values) */
.delay-75 { animation-delay: 75ms; }
.delay-150 { animation-delay: 150ms; }
```

### Hover Transition Updates for Base Components
```tsx
// Card: add border transition
// Current: "bg-card text-card-foreground flex flex-col gap-6 rounded-xl border py-6 shadow-sm"
// Updated: add "transition-colors duration-150 hover:border-ring/50"

// TableRow: ensure timing
// Current: "hover:bg-muted/50 data-[state=selected]:bg-muted border-b transition-colors"
// Updated: add "duration-150" for explicit 150ms

// Button: already has transition-all, just verify timing
// Current: "transition-all" (uses Tailwind default 150ms — matches user decision)
```

### Reduced Motion Accessibility
```tsx
// Tailwind CSS v4 provides motion-reduce: variant natively
<div className="animate-in fade-in zoom-in-95 duration-300 motion-reduce:animate-none">
  {/* Content shows instantly without animation for prefers-reduced-motion users */}
</div>

// For CountUp hook: respect prefers-reduced-motion
const prefersReducedMotion =
  typeof window !== 'undefined' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

// If reduced motion preferred, skip to final value immediately
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| tailwindcss-animate (JS plugin) | tw-animate-css (pure CSS) | Tailwind CSS v4 (2024) | No JS plugin needed, CSS-first architecture |
| framer-motion | motion (npm package) | Rebranded 2024 | Same API, new package name `motion` |
| GSAP ScrollTrigger for entrance | IntersectionObserver + CSS | Native browser API | No library dependency needed for simple entrance triggers |
| Custom animation hooks | react-bits copy-paste | 2024-2025 | Pre-built but requires peer deps; custom hooks work for simple cases |

**Deprecated/outdated:**
- `tailwindcss-animate`: JS plugin, incompatible with Tailwind CSS v4 CSS-first approach. Replaced by `tw-animate-css`.
- `framer-motion` package name: Still works but new name is `motion` from `motion/react`.

## Open Questions

1. **SpotlightCard spotlightColor for oklch theme**
   - What we know: Theme uses oklch color space. SpotlightCard uses rgba for radial gradient.
   - What's unclear: CSS `radial-gradient` with oklch works in modern browsers, but spotlightColor prop accepts string.
   - Recommendation: Use rgba approximation of the primary color. For the dark theme purple (`oklch(0.55 0.25 290)`), use approximately `rgba(139, 92, 246, 0.15)` for light mode and `rgba(139, 92, 246, 0.25)` for dark mode. Can compute dynamically with `getComputedStyle` if needed.

2. **CountUp with formatBytes (Traffic page)**
   - What we know: Traffic page shows bytes formatted as "↑ 1.5 GB". CountUp animates numbers.
   - What's unclear: Should CountUp animate the raw byte count and format each frame, or animate the converted number?
   - Recommendation: Animate the raw byte count and reformat each frame using the existing `formatBytes` utility. This gives smooth animation while keeping human-readable format. May need a custom formatter callback in the hook.

## Sources

### Primary (HIGH confidence)
- Context7 `/davidhdev/react-bits` — CountUp, AnimatedContent, SpotlightCard component APIs and source code
- Context7 `/websites/reactbits_dev` — AnimatedContent props, installation instructions, dependency requirements
- Context7 `/wombosvideo/tw-animate-css` — Animation utility classes, CSS variable system, usage patterns
- reactbits.dev registry endpoints — Actual component source code and dependency manifests:
  - `https://reactbits.dev/r/CountUp-TS-TW` — Confirms `motion@^12.23.12` dependency
  - `https://reactbits.dev/r/AnimatedContent-TS-TW` — Confirms `gsap@^3.13.0` dependency
  - `https://reactbits.dev/r/SpotlightCard-TS-TW` — Confirms zero dependencies

### Secondary (MEDIUM confidence)
- npm `react-bits@1.0.5` package.json — Verified this is a DIFFERENT library (by Doug Miller, cross-platform primitives)
- tw-animate-css GitHub README — Full utility class reference and compatibility notes

### Tertiary (LOW confidence)
- None — all findings verified with primary sources

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — verified installed versions in package.json, confirmed tw-animate-css import in index.css
- Architecture: HIGH — examined all 7 page components, all UI base components, understood full scope
- Pitfalls: HIGH — react-bits package confusion confirmed by inspecting actual package.json and typings
- SpotlightCard color theming: MEDIUM — oklch-to-rgba conversion approach works but may need runtime tuning

**Research date:** 2026-02-23
**Valid until:** 2026-03-23 (stable — no fast-moving dependencies)
