# HeroUI 迁移运行时错误与样式修复

## 补丁内容

修复 plan-5（前端全面迁移 HeroUI + 样式重构）执行后遗留的 4 个运行时问题：

1. **`/users` 页面 Unexpected Application Error**：`UserTable.tsx` 中的 `Checkbox.Root` 在 HeroUI Table 上下文中缺少 `slot="selection"` 属性，导致 react-aria 抛出 `"A slot prop is required"` 错误，触发 React Router 错误边界。

2. **`/core` 页面整体滚动条导致侧栏溢出**：`AppLayout.tsx` 使用 `min-h-screen` 允许外层容器无限增长，当页面内容（如 Core 页面 ~1363px）超出视口时整页滚动，侧栏随之滑出屏幕。改为 `h-screen` + `overflow-hidden` 固定外框，仅 `<main>` 区域 `overflow-y-auto` 内滚。

3. **圆角过大影响 HeroUI 原生组件样式**：`--radius: 0.9rem` 被 HeroUI 内部样式系统继承，导致 Select 弹出层、Input 等组件的圆角异常偏大。降至 `0.5rem` 并将所有 `calc(var(--radius)*N)` 自定义圆角替换为标准 Tailwind class（`rounded-xl`、`rounded-lg`、`rounded-2xl`），使 HeroUI 组件回归原生设计。

4. **表单 Select 选中后不显示值**：`InboundFormModal.tsx` 中 4 处 `<Select.Trigger />` 缺少 `<Select.Value />` 子组件。HeroUI 3 beta 的 Select 需要显式渲染 `Select.Value` 才会显示选中文本。

**附带修复**：
- 全局统一无效 Button variant（`"bordered"` → `"outline"`、`"light"` → `"ghost"`）、Chip variant（`"bordered"` → `"secondary"`），匹配 HeroUI 3 beta 类型定义。
- 修正无效 Modal.Container size（`"2xl"`/`"3xl"` → `"lg"`）。
- 修正 `RadioGroup.Root` 的 `onValueChange` 为 `onChange`。

## 影响范围

- 修改文件: `web/src/components/users/UserTable.tsx`
- 修改文件: `web/src/components/layout/AppLayout.tsx`
- 修改文件: `web/src/index.css`
- 修改文件: `web/src/components/inbounds/InboundFormModal.tsx`
- 修改文件: `web/src/components/layout/Sidebar.tsx`
- 修改文件: `web/src/components/users/UserSubscriptionCard.tsx`
- 修改文件: `web/src/components/users/BatchActionBar.tsx`
- 修改文件: `web/src/components/users/UserFormModal.tsx`
- 修改文件: `web/src/components/inbounds/InboundTable.tsx`
- 修改文件: `web/src/components/certificates/CertificateTable.tsx`
- 修改文件: `web/src/components/certificates/CertificateFormModal.tsx`
- 修改文件: `web/src/pages/Core.tsx`
- 修改文件: `web/src/pages/Subscriptions.tsx`
- 修改文件: `web/src/pages/Traffic.tsx`
- 修改文件: `web/src/pages/Inbounds.tsx`
- 修改文件: `web/src/pages/Login.tsx`
- 修改文件: `web/src/pages/Setup.tsx`
