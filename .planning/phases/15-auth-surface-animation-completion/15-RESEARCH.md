# Phase 15: Auth Surface Animation Completion - Research

**Researched:** 2026-02-26  
**Domain:** React auth surfaces animation/interaction consistency (Login/Setup)  
**Confidence:** HIGH

<user_constraints>
## User Constraints (from phase input, no CONTEXT.md)

### Locked Decisions
- No new npm/go dependencies for v1.1; prefer existing installed stack.
- Existing entrance animation baseline: `animate-in fade-in zoom-in-95 duration-300` with `75ms` stagger.
- Existing hover baseline: `border-ring/50` and consistent transition classes across UI.
- This phase must close audit gaps for `UIANM-02` and `UIANM-04` specifically on Login/Setup.

### Claude's Discretion
- How to apply animation wrappers on Login/Setup while preserving current auth behavior.
- Whether to refactor Login/Setup to shared shadcn primitives (`Card`/`Button`/`Input`/`Checkbox`) in this phase.
- Verification strategy depth for parity check across `login/setup/subscriptions/traffic/core`.

### Deferred Ideas (OUT OF SCOPE)
- New animation libraries or route transition animation systems.
- Backend auth API behavior changes (`/api/login`, `/api/setup` contract changes).
- Large auth UX redesign beyond consistency closure.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| UIANM-02 | all pages card entrance animations with AnimatedContent + tw-animate-css | Reuse existing page wrapper pattern already used by `Subscriptions/Traffic/Core`: `animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none` with 0/75ms stagger on logical blocks. |
| UIANM-04 | global unified hover interactions across button/card/table row | Eliminate raw auth action controls and route them to shared primitives (`Button`, `Card`, optional `Input/Checkbox`) so hover transitions come from centralized component definitions (`button.tsx`, `card.tsx`, `table.tsx`) instead of page-local class drift. |
</phase_requirements>

## Summary

Phase 15 is a **consistency completion** phase, not a new animation feature phase. Main pages already implement the established Phase 11 baseline: entrance animation via `tw-animate-css` utility classes and interaction consistency via shared UI primitives (`Card`, `Button`, `TableRow`). The current gap is isolated to `Login.tsx` and `Setup.tsx`, which still use raw `div/button/input` styling and therefore bypass part of the shared hover/transition system.

For planning, the safest implementation is to **port auth surfaces onto existing primitives and class conventions**, not to introduce a new animation runtime. `tw-animate-css` is already installed/imported and provides the exact utilities in use. Tailwind’s `motion-reduce` variant is already the project’s accessibility pattern and should be retained on auth entrance wrappers.

`REQUIREMENTS.md` wording mentions “AnimatedContent”, but current codebase baseline uses utility-driven wrappers rather than a dedicated `AnimatedContent` component symbol. This is consistent with the no-new-dependency decision and with actual shipped patterns in Phase 11/14 pages.

**Primary recommendation:** Implement Login/Setup with the exact same entrance wrapper + stagger + reduced-motion classes as existing pages, and route auth action controls through shared `Button`/`Card` primitives to inherit global hover behavior automatically.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React | `19.2.0` | page rendering/state | Existing app runtime; no migration risk. |
| React Router DOM | `7.13.0` | `/login` `/setup` routing | Auth pages are direct routes outside `AppLayout`; behavior already stable. |
| Tailwind CSS | `4.1.18` | utility styling/variants | All hover and motion-reduce semantics already standardized here. |
| tw-animate-css | `1.4.0` | enter/exit animation utilities | Already imported in `index.css`; current entrance baseline depends on it. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| class-variance-authority | `0.7.1` | centralized variant classes (`Button`) | Use for auth CTA consistency instead of local per-page button classes. |
| shadcn UI local primitives | repo-local | shared `Card/Button/Input/Checkbox/TableRow` behavior | Use for global hover/focus consistency and reduced style drift. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Existing utility wrapper pattern | reactbits.dev `AnimatedContent` runtime component | Official docs show dependency on `gsap`; conflicts with no-new-deps and unnecessary for current scope. |
| Shared `Button/Card` primitives | Keep raw auth element classes | Faster short-term edits, but fails `UIANM-04` parity objective and reintroduces drift risk. |

**Installation:**
```bash
# No new install required for Phase 15
# Existing stack already contains tailwindcss + tw-animate-css + shadcn primitives
```

## Architecture Patterns

### Recommended Project Structure
```text
web/src/pages/
├── Login.tsx         # Auth page, uses shared primitives + entrance wrappers
├── Setup.tsx         # Auth page, same pattern as Login
└── (other pages)     # Existing baseline references (Subscriptions/Traffic/Core)
web/src/components/ui/
├── card.tsx          # hover:border-ring/50 baseline
├── button.tsx        # transition-all + variant hover states baseline
├── input.tsx         # focus ring/token baseline
└── checkbox.tsx      # form control baseline
```

### Pattern 1: Auth Surface Entrance Wrapper Parity
**What:** Wrap auth card sections with the same entrance class bundle used by main pages.  
**When to use:** Login/Setup initial render, plus any top-level auth content block needing first-mount entrance.
**Example:**
```typescript
// Source: existing project baseline (Subscriptions/Traffic/Core patterns)
<div className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none">
  {/* auth content block */}
</div>
<div
  className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none"
  style={{ animationDelay: "75ms" }}
>
  {/* second logical block */}
</div>
```

### Pattern 2: Shared Primitive Inheritance for Hover Consistency
**What:** Use `Card` + `Button` primitives so hover transitions are inherited centrally.  
**When to use:** All auth primary/secondary action controls and card containers.
**Example:**
```typescript
// Source: web/src/components/ui/card.tsx + web/src/components/ui/button.tsx
<Card>{/* ... */}</Card> // includes transition-colors duration-150 hover:border-ring/50
<Button type="submit" disabled={loading}>{loading ? "登录中..." : "登录"}</Button>
```

### Pattern 3: Motion Accessibility on Entrance
**What:** Keep `motion-reduce:animate-none` on all entrance wrappers.  
**When to use:** Every auth entrance animation wrapper.
**Example:**
```typescript
// Source: Tailwind prefers-reduced-motion variants + existing page usage
className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none"
```

### Anti-Patterns to Avoid
- **Raw auth CTA buttons:** page-local `hover:bg-*` duplicates drift from shared `Button` behavior.
- **Mixing custom keyframes for auth entrance:** unnecessary when `tw-animate-css` baseline exists.
- **Dropping `fill-mode-both`:** increases pre-animation state flicker/layout perception risk.
- **Skipping `motion-reduce:*`:** fails established accessibility handling used elsewhere.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Auth card entrance | Custom JS animation hooks for Login/Setup | `tw-animate-css` class bundle baseline | Already standardized, less code, same visual language. |
| Auth submit hover behavior | Per-page handcrafted button transitions | Shared `Button` primitive variants | Single source of truth for hover timing/colors. |
| Card hover feedback | Per-page border/shadow tuning | Shared `Card` primitive hover class | Guarantees parity with main pages. |
| Reduced-motion handling | Manual JS media-query listeners | Tailwind `motion-reduce:*` variant | Existing accessible approach; no runtime complexity. |

**Key insight:** Phase 15 should be implemented as **inheritance of existing primitives**, not new animation engineering.

## Common Pitfalls

### Pitfall 1: Layout shift/flicker on auth entrance
**What goes wrong:** Card appears to pop in inconsistently or flashes before animation starts.  
**Why it happens:** Missing `fill-mode-both` or animating inner content inconsistently.  
**How to avoid:** Apply full baseline class set on stable wrappers and keep card dimensions fixed (`w-full max-w-sm`).  
**Warning signs:** First frame flash, card appears at final state then animates.

### Pitfall 2: Hover parity regression despite “visual similarity”
**What goes wrong:** Login/Setup appears close but does not match global transition behavior.  
**Why it happens:** Raw `<button>` retains page-local classes and bypasses `Button` variants.  
**How to avoid:** Use shared `Button` for action controls; keep variant/size explicit.  
**Warning signs:** Hover duration/contrast feels different from Core/Subscriptions actions.

### Pitfall 3: Reduced-motion ignored on auth pages
**What goes wrong:** Users with reduced-motion still see entrance effects on Login/Setup.  
**Why it happens:** Missing `motion-reduce:animate-none` on new wrappers.  
**How to avoid:** Include reduced-motion variant in every auth entrance class bundle.  
**Warning signs:** DevTools emulated `prefers-reduced-motion: reduce` still shows fades/zoom.

### Pitfall 4: Inconsistent stagger rhythm
**What goes wrong:** Auth surfaces animate with different cadence than main pages.  
**Why it happens:** Ad-hoc delays (`100ms/200ms`) instead of baseline `75ms` step.  
**How to avoid:** Use 0ms then 75ms for two-block auth structure.  
**Warning signs:** Transition pacing looks slower/faster than Subscriptions/Users.

## Code Examples

Verified patterns from current codebase + official docs:

### Auth card entrance baseline
```typescript
// Source: project pages + tw-animate-css docs
const entrance = "animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none"
```

### Auth action control baseline
```typescript
// Source: web/src/components/ui/button.tsx
<Button type="submit" className="w-full" disabled={loading}>
  {loading ? "设置中..." : "完成设置"}
</Button>
```

### Two-block auth stagger pattern
```typescript
// Source: existing 75ms stagger convention in app pages
<div className={entrance}>{/* header/card block */}</div>
<div className={entrance} style={{ animationDelay: "75ms" }}>{/* secondary block */}</div>
```

## Verification Checklist (Planner-ready)

- [ ] Login page has at least one auth card wrapper using `animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none`.
- [ ] Setup page has same entrance class bundle and matches stagger rhythm (`75ms` step where multi-block).
- [ ] Login/Setup primary submit actions use shared `Button` primitive (not raw `<button>`).
- [ ] Auth surface card container uses shared `Card` primitive or exactly equivalent baseline hover transition (`transition-colors duration-150 hover:border-ring/50`) with rationale documented.
- [ ] In browser parity pass, hover behavior across `login/setup/subscriptions/traffic/core` is visually consistent (timing + emphasis).
- [ ] Emulate `prefers-reduced-motion: reduce`; auth entrance animations are disabled while usability remains intact.
- [ ] Route navigation `login -> setup -> login` replays first-mount entrance predictably; local validation/error state updates do not retrigger unrelated animations.
- [ ] `UIANM-02` and `UIANM-04` traceability rows updated to `Complete` only after above checks pass.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Tailwind animation plugin wiring (`tailwindcss-animate`) | Tailwind v4 CSS-first `tw-animate-css` import | 2025+ ecosystem shift | Lower integration cost, utility-first animation parity. |
| Page-local auth control classes | Shared primitive inheritance (`Button/Card`) | Phase 11/15 consistency direction | Prevents drift, keeps hover/transition globally uniform. |

**Deprecated/outdated:**
- Introducing new animation runtime for simple auth entrance in this phase: unnecessary and violates current v1.1 constraint.

## Open Questions

1. **UIANM-02 wording vs implementation language**
   - What we know: Requirement text says “AnimatedContent + tw-animate-css”, but codebase implements utility-wrapper baseline without an `AnimatedContent` symbol.
   - What's unclear: Whether verification requires literal component naming or behavior-equivalent baseline.
   - Recommendation: In PLAN/VERIFICATION, define acceptance by observable behavior + class baseline to avoid semantic ambiguity.

2. **Auth page wrapper granularity**
   - What we know: Login/Setup are single-card layouts; most main pages animate per logical section.
   - What's unclear: Whether to animate only card wrapper or also title/form sub-blocks.
   - Recommendation: Start with card-level wrapper only (lowest risk). Add second wrapper only if needed for parity feel.

## Sources

### Primary (HIGH confidence)
- Local codebase:
  - `web/src/pages/Login.tsx`
  - `web/src/pages/Setup.tsx`
  - `web/src/pages/Subscriptions.tsx`
  - `web/src/pages/Traffic.tsx`
  - `web/src/pages/Core.tsx`
  - `web/src/components/ui/card.tsx`
  - `web/src/components/ui/button.tsx`
  - `web/src/components/ui/table.tsx`
  - `.planning/ROADMAP.md`
  - `.planning/REQUIREMENTS.md`
  - `.planning/v1.1-MILESTONE-AUDIT.md`
- Official docs:
  - https://www.npmjs.com/package/tw-animate-css
  - https://raw.githubusercontent.com/Wombosvideo/tw-animate-css/main/docs/parameters/animation-fill-mode.md
  - https://raw.githubusercontent.com/Wombosvideo/tw-animate-css/main/docs/transforms/scale.md
  - https://tailwindcss.com/docs/animation
  - https://tailwindcss.com/docs/hover-focus-and-other-states

### Secondary (MEDIUM confidence)
- https://reactbits.dev/text-animations/animated-content (AnimatedContent props/dependencies page)

### Tertiary (LOW confidence)
- Web search aggregation results for ecosystem triangulation (used only as pointer, not as primary truth).

## Metadata

**Confidence breakdown:**
- Standard stack: **HIGH** — directly verified via local `package.json` and official docs.
- Architecture: **HIGH** — baseline pattern already exists in multiple shipped pages.
- Pitfalls: **MEDIUM** — partly inferred from implementation behavior and past verification patterns.

**Research date:** 2026-02-26  
**Valid until:** 2026-03-28
