# 去除前端样式噪点 — 回归 HeroUI 3.0 原生风格

> 状态: 已执行

## 目标

清理整个前端项目中偏离 HeroUI 3.0 设计语言的自定义样式覆盖，使 UI 组件呈现原汁原味的 HeroUI 风格。同时确保所有输入性质的组件（Input、Select、Checkbox、Radio、Switch、Textarea、DatePicker 等）100% 使用 HeroUI 组件，杜绝原生 HTML 表单元素和手动样式拼接。

## 问题审计清单

### P1 — 自定义 CSS 变量体系与 HeroUI 冲突

- `index.css` 中定义了一整套自定义 CSS 变量（`--surface`、`--border`、`--field-*`、`--overlay-shadow` 等），这些变量 **不属于 HeroUI 的设计 token**。
- 全局 `*:focus-visible` 样式覆盖了 HeroUI 自带的 focus ring。
- 全局 `button, input, select, textarea { font: inherit }` 可能干扰 HeroUI 内部样式。
- `App.css` 是 Vite 脚手架残留，包含 `.logo`、`.card`、`.read-the-docs` 等无用样式。

### P2 — 大量 Tailwind 硬编码颜色引用自定义变量

几乎所有组件中充斥着 `text-[color:var(--muted)]`、`bg-[color:var(--surface)]`、`border-[color:var(--border)]`、`shadow-[var(--surface-shadow)]` 等硬编码样式，而不是使用 HeroUI 的内置 semantic color slots（如 `text-foreground-500`、`bg-content1`、`border-divider` 等）。

涉及文件（几乎全部）：

- 所有 pages: `Login.tsx`, `Setup.tsx`, `Dashboard.tsx`, `Inbounds.tsx`, `Users.tsx`, `Certificates.tsx`, `Subscriptions.tsx`, `Traffic.tsx`, `Core.tsx`
- 所有 components: `AppLayout.tsx`, `Sidebar.tsx`, `PageHero.tsx`, `FieldLabel.tsx`, `InboundTable.tsx`, `InboundFormModal.tsx`, `UserTable.tsx`, `UserFormModal.tsx`, `UserSubscriptionCard.tsx`, `UserSubscriptionModal.tsx`, `BatchActionBar.tsx`, `CertificateTable.tsx`, `CertificateFormModal.tsx`

### P3 — 输入组件样式覆盖

- `Input` 组件到处带 `variant="secondary"` 和 `className="h-12"` 等自定义高度。
- `Card` 组件到处带 `className="border border-[color:var(--border)] bg-[color:var(--surface)]..."` 覆盖了 HeroUI Card 自带样式。
- `Table` 组件到处带 `className="rounded-xl border border-[color:var(--border)] bg-[color:var(--surface)] shadow-[var(--surface-shadow)]"`。
- `Chip` 组件到处带复杂的手动颜色类用于状态指示（destructive / secondary / default），而 HeroUI Chip 自带 `color` prop。
- `Button` 的 `className` 覆盖（`shadow-[var(--surface-shadow)]`、`h-12`、手动色彩）。
- `Modal.Container` 的 `className` 覆盖。

### P4 — 导航/侧边栏使用原生 HTML 链接而非 HeroUI 组件

- `Sidebar.tsx` 中导航项使用 `<Link>` (react-router-dom) 手动拼接 active/hover 样式，而不是使用 HeroUI 的 `ListBox` 或专门的导航组件。

### P5 — 错误提示使用手动 `<div>` 拼接而非 HeroUI Alert

- `Login.tsx`, `Setup.tsx` 的错误提示使用手动 `<div>` + 硬编码样式。
- `InboundFormModal.tsx`, `CertificateFormModal.tsx` 的 `checkError` 使用手动 `<div>`。
- `Core.tsx` 的 `not_installed` / `error` 状态使用手动 amber/red 样式块。

### P6 — 状态 Chip 的手动颜色拼接

- `Subscriptions.tsx`, `Traffic.tsx`, `UserTable.tsx`, `Core.tsx` 中 `renderStatusChip` 函数通过手动 className 拼接颜色，而不是使用 HeroUI Chip 的 `color` prop（`color="success"`, `color="danger"`, `color="default"`）。

## 内容

按步骤逐一实施，每步独立可验证。

### 步骤 1：清理全局 CSS 噪点

1. **删除 `App.css`** — 纯 Vite 脚手架残留，无任何业务代码引用。
2. **精简 `index.css`**：
   - 移除不属于 HeroUI 设计系统的自定义 CSS 变量（`--surface`、`--surface-foreground`、`--surface-secondary`、`--surface-tertiary`、`--overlay`、`--overlay-foreground`、`--muted`、`--default`、`--default-foreground`、`--accent`、`--accent-foreground`、`--success*`、`--warning*`、`--danger*`、`--field-*`、`--border`、`--separator`、`--focus`、`--surface-shadow`、`--overlay-shadow`、`--field-shadow` 等）。
   - 移除 `.dark` 下同样的额外变量。
   - 移除 `@layer base` 中的 `*:focus-visible` 覆盖（让 HeroUI 自带 focus ring 接管）。
   - 移除 `button, input, select, textarea { font: inherit }` 覆盖。
   - 保留 `body` 基本设置，但使用 HeroUI 的 `bg-background text-foreground` 等。
   - 保留 `@import "tailwindcss"` 和 `@import "@heroui/styles"`、`@custom-variant dark`。
   - 保留字体栈和 `text-rendering` 等非侵入性全局样式。
   - 保留 `@keyframes status-blink`（业务动画）。

**完成标准**：`index.css` 仅包含 Tailwind/HeroUI 导入、字体栈、body 基本样式、业务动画，不含任何自定义 CSS 变量。

### 步骤 2：替换全局颜色引用 — 建立映射表

将 `var(--xxx)` 的 Tailwind 硬编码类全部替换为 HeroUI semantic token 类。映射关系：

| 旧（自定义变量）                          | 新（HeroUI 语义）                  |
| ----------------------------------------- | ---------------------------------- |
| `text-[color:var(--foreground)]`          | `text-foreground`                  |
| `text-[color:var(--muted)]`               | `text-foreground-500`              |
| `bg-[color:var(--background)]`            | `bg-background`                    |
| `bg-[color:var(--surface)]`               | `bg-content1`                      |
| `bg-[color:var(--surface)]/84` 等带透明度 | `bg-content1`（去掉透明度 hack）   |
| `bg-[color:var(--surface-secondary)]`     | `bg-content2`                      |
| `bg-[color:var(--surface-tertiary)]`      | `bg-content3`                      |
| `border-[color:var(--border)]`            | `border-divider`                   |
| `text-[color:var(--accent)]`              | `text-primary`                     |
| `bg-[color:var(--accent)]/12` 等          | `bg-primary/10` 或 `bg-primary-50` |
| `text-[color:var(--danger)]`              | `text-danger`                      |
| `bg-[color:var(--danger)]/10`             | `bg-danger-50` 或 `bg-danger/10`   |
| `text-[color:var(--success)]`             | `text-success`                     |
| `bg-[color:var(--success)]/12`            | `bg-success-50` 或 `bg-success/10` |
| `shadow-[var(--surface-shadow)]`          | `shadow-medium`（HeroUI 内置）     |
| `shadow-[var(--overlay-shadow)]`          | `shadow-large`                     |
| `text-red-xxx / text-amber-xxx`           | `text-danger` / `text-warning`     |
| `border-red-500 / border-amber-500`       | `border-danger` / `border-warning` |

逐文件替换，涉及全部 pages 和 components。

**完成标准**：grep `var(--` 在 `web/src/` 下返回 0 结果（除 `index.css` 保留的 body 相关）。

### 步骤 3：Card / Table / Chip 回归 HeroUI 原生

1. **Card 组件**：
   - 移除所有 `className="border border-xxx bg-xxx shadow-xxx backdrop-blur-xxx"` 覆盖。
   - 直接使用 HeroUI Card 无自定义 className（或仅保留布局类如 `w-full`）。
   - Card 的 `shadow` 可通过 `shadow` prop 控制（`shadow="sm"` / `shadow="md"`）。

2. **Table 组件**：
   - 移除所有 `className="rounded-xl border border-xxx bg-xxx shadow-xxx"` 覆盖。
   - HeroUI Table 自带样式。

3. **Chip 状态指示**：
   - 将手动 `className` 颜色拼接替换为 `color` prop：
     - `status === "destructive"` → `<Chip color="danger">`
     - `status === "default"` → `<Chip color="success">`
     - `status === "secondary"` → `<Chip color="default">`
   - 删除 `renderStatusChip` / `stateChipClassName` 等手动拼接函数，用 HeroUI Chip 语义 color。

**完成标准**：Card、Table、Chip 不含 `border-[color:` / `bg-[color:` / `shadow-[var(` 类。

### 步骤 4：Input 等输入组件移除手动覆盖

1. 所有 `<Input variant="secondary" className="h-12">` → `<Input>`（使用 HeroUI 默认尺寸和变体）。
2. 如需统一调高，用 `size="lg"` 而非手动 `h-12`。
3. 所有 `<Input>` 无需带 `variant="secondary"` — 使用 HeroUI 默认 variant。
4. 确认 `CertificateFormModal.tsx` 中 3 个 Input 缺少 `variant` prop — 与其他页面统一。
5. `UserFormModal.tsx` 中的 `<Input type="date">` 检查是否可换为 HeroUI `DatePicker`。
6. `<Button className="h-12 text-base font-semibold shadow-[var(--surface-shadow)]">` → `<Button size="lg">`。

**完成标准**：所有 Input/Button 不含 `h-12`、`shadow-[var(` 等手动样式覆盖。

### 步骤 5：错误提示替换为 HeroUI Alert

1. `Login.tsx` / `Setup.tsx` 的 `{error && <div className="rounded-xl border...">}` 替换为 `<Alert color="danger">{error}</Alert>`。
2. `InboundFormModal.tsx` / `CertificateFormModal.tsx` 的 `checkError` 使用 `<Alert color="danger">`。
3. `Core.tsx` 中 `not_installed` 使用 `<Alert color="warning">`，`error` 使用 `<Alert color="danger">`。

**完成标准**：错误/警告提示全部使用 `<Alert>` 组件，无手动 `<div>` + 颜色拼接。

### 步骤 6：Sidebar 导航样式精简

1. `Sidebar.tsx` 中 `<aside>` 的 `backdrop-blur-xl` / 复杂边框/阴影覆盖精简。
2. 导航项保留 react-router `<Link>` 但颜色使用 HeroUI token（`text-foreground-500`、`text-primary`、`bg-primary/10`）替代自定义变量。
3. `AppLayout.tsx` 的 `<main>` 容器移除 `backdrop-blur-xl` / 自定义阴影。

**完成标准**：Sidebar 和 AppLayout 不含 `var(--` 引用，视觉保持导航结构不变。

### 步骤 7：清理 `<progress>` 为 HeroUI Progress（如可用）

1. `Core.tsx` 中的 `<progress>` 元素使用了大量 vendor-prefix CSS hack。如 HeroUI 提供 Progress 组件则替换；否则用 Tailwind simplify 保留。

### 步骤 8：最终验收

1. `bun run dev` 启动前端，逐页面检查视觉合理性。
2. `grep -r "var(--" web/src/` 确认剩余引用仅限 body 或合理场景。
3. `bun run lint` 确认无新增 lint 错误。
4. 暗色/亮色主题均测试。

## 影响范围

## 历史补丁
