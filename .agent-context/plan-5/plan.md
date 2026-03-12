# 前端全面迁移 HeroUI + 样式重构

> 状态: 已执行

## 目标

将前端全面迁移到 HeroUI 3.0.0-beta 组件体系，移除所有废弃的自定义 UI 组件和未使用的依赖（react-bits、shadcn 遗留组件），并以 HeroUI 原生设计语言重构全部页面样式——去除花里胡哨的渐变、动画、装饰元素，做到干净、专业、统一。

## 现状摘要

- `components/ui/`：18 个文件，全部未被任何业务代码引用（shadcn 遗留死代码）
- `react-bits`：`package.json` 中的依赖，源码中零引用
- `PageHero`：自定义页面头部组件，使用 Card + 渐变背景 + Chip 装饰
- `PanelReveal`：自定义 CSS 入场动画包装器（panel-reveal keyframes）
- `OptionPicker`：基于 HeroUI Dropdown+Button 的自封装下拉，应使用 HeroUI Select
- `FieldLabel`：`form/FieldLabel.tsx` 与 `CertificateFormModal` 内部重复定义
- `index.css`：自定义 panel-reveal/status-blink 动画、body 网格背景、大量装饰性 CSS
- Login/Setup 页面：渐变背景、装饰圆、Sparkles 图标等花哨元素
- AppLayout 页头：Workspace 面包屑、Live session Chip 等过度装饰
- `sonner` 别名：已有 shim 包装 HeroUI toast，保留可用

## 内容

### 步骤 1：删除废弃依赖与死代码

**目标**：清除不再使用的代码和依赖，减小项目体积。

- 删除整个 `web/src/components/ui/` 目录（18 个文件：badge, button, card, checkbox, dialog, dropdown-menu, input, label, popover, progress, radio-group, select, separator, skeleton, spotlight-card, table, tabs, tooltip）
- 从 `package.json` 移除 `react-bits` 依赖
- 运行 `bun install` 更新 lockfile
- 如果有其他文件引用了 `components/ui/` 的导入路径，一并清除

**完成标准**：`components/ui/` 目录不存在，`react-bits` 不在依赖列表中，`bun install` 无报错。

### 步骤 2：移除 PanelReveal 动画系统

**目标**：去除自定义入场动画，让内容直接显示。

- 删除 `web/src/components/layout/PanelReveal.tsx`
- 在以下 7 个页面中移除 PanelReveal 导入和包裹：
  - `Dashboard.tsx`, `Users.tsx`, `Inbounds.tsx`, `Certificates.tsx`, `Traffic.tsx`, `Subscriptions.tsx`, `Core.tsx`
- 从 `index.css` 移除：
  - `@keyframes panel-reveal` 定义
  - `.panel-reveal` 及 `[data-delay]` 相关规则
  - `@keyframes status-blink`（如未使用）
- 从 `index.css` 移除 `body::before` 伪元素（32px 网格背景装饰）

**完成标准**：项目中无 PanelReveal 引用，无 panel-reveal CSS，无网格背景。

### 步骤 3：重构 PageHero 为简洁页面标题

**目标**：将 PageHero 从花哨的 Card+渐变组件简化为干净的页面标题区域。

- 重写 `PageHero.tsx`：
  - 移除 Card 包裹和渐变背景
  - 移除 eyebrow Chip
  - 保留：标题（h1/h2）、描述文本、actions 插槽、可选的 metrics 区域
  - 样式使用纯 Tailwind + HeroUI 设计变量，简洁大方
- 更新所有引用 PageHero 的页面，适配新的 props 接口（如有变化）

**完成标准**：PageHero 渲染为简洁的标题+描述+操作区，无渐变、无 Card、无装饰 Chip。

### 步骤 4：用 HeroUI Select 替换 OptionPicker

**目标**：废弃自封装的 OptionPicker，使用 HeroUI 原生 Select 组件。

- 删除 `web/src/components/form/OptionPicker.tsx`
- 在 `InboundFormModal.tsx` 中将所有 OptionPicker 用法替换为 HeroUI `Select`（含 `Label`, `ListBox`, `Select.Trigger` 等）
- 确保协议选择、TLS 类型选择、证书选择等下拉功能正常

**完成标准**：OptionPicker 文件已删除，所有下拉选择使用 HeroUI Select，功能不变。

### 步骤 5：统一表单组件与模式

**目标**：统一表单辅助组件，消除重复代码。

- 评估 `FieldLabel` 是否仍有必要——如果 HeroUI 的 `<Label>` + `<Description>` 可直接满足需求，则废弃 FieldLabel，直接使用 HeroUI 原生组件
- 如果保留 FieldLabel：统一 `CertificateFormModal.tsx` 中的内联 FieldLabel 为使用 `@/components/form/FieldLabel`
- 确保所有表单弹窗（UserFormModal、InboundFormModal、CertificateFormModal）使用一致的 HeroUI Form 模式：
  - `<Form>` 包裹
  - `<TextField>` / `<NumberField>` / `<Select>` 等 HeroUI 表单组件
  - `<Label>` + `<Description>` + `<FieldError>` 三件套

**完成标准**：无重复的表单辅助组件，所有表单弹窗使用统一的 HeroUI Form 模式。

### 步骤 6：重构 Login 与 Setup 页面

**目标**：去除花哨的视觉装饰，使用干净的 HeroUI 风格。

- Login.tsx：
  - 移除渐变背景（`radial-gradient`, `linear-gradient`）
  - 移除装饰性伪元素和 Sparkles/ShieldCheck 图标装饰
  - 使用简洁的居中 Card + HeroUI Form 组件
  - 保留基本的品牌标识（logo + 标题）
- Setup.tsx：
  - 同样移除渐变、装饰圆、Sparkles 图标
  - 使用简洁的步骤式卡片布局
  - 使用 HeroUI Form 组件

**完成标准**：Login/Setup 页面视觉干净、无渐变装饰，符合 HeroUI 设计语言。

### 步骤 7：重构 AppLayout 与 Sidebar

**目标**：简化应用框架布局，符合 HeroUI 设计规范。

- AppLayout.tsx：
  - 简化页头：移除 Workspace 面包屑装饰、Live session Chip、时间 Chip
  - 保留：页面标题、移动端菜单按钮
  - 移动端遮罩：将原生 `<button>` 替换为合适的 HeroUI 组件
- Sidebar.tsx：
  - 简化 Control Deck 卡片样式
  - 清理导航项的 active 状态样式，使用 HeroUI 标准模式
  - 保留核心功能：logo、导航、主题切换、登出

**完成标准**：AppLayout 和 Sidebar 视觉简洁，无过度装饰，功能完整。

### 步骤 8：重构所有业务页面样式

**目标**：统一所有页面的视觉风格为 HeroUI 原生风格。

- Dashboard.tsx：简化统计卡片样式，使用 HeroUI Card 标准用法
- Core.tsx（600 行）：简化核心管理面板，移除花哨的状态展示样式
- Traffic.tsx：简化流量监控页面，使用 HeroUI Table/Card 标准样式
- Subscriptions.tsx：简化订阅列表样式
- Users.tsx：确认 UserTable、BatchActionBar 样式符合 HeroUI 风格
- Inbounds.tsx：确认入站管理样式统一
- Certificates.tsx：确认证书管理样式统一
- 将 `BatchActionBar` 中的 `window.confirm` 替换为 HeroUI Modal 确认对话框

**完成标准**：所有页面视觉风格统一，使用标准 HeroUI 组件和样式，无自定义花哨装饰。

### 步骤 9：清理 index.css 与最终验证

**目标**：精简 CSS，确保构建通过。

- 清理 `index.css`：
  - 移除所有已废弃的 keyframes 动画
  - 简化 body 背景（移除 radial-gradient 装饰）
  - 审查 CSS 自定义变量，移除未使用的变量
  - 保留 HeroUI 必需的主题变量和 Tailwind 导入
- 运行 `bun run build` 确保构建通过
- 运行 `bun run lint` 确保无 lint 错误
- 启动 dev server 验证所有页面渲染正常

**完成标准**：`bun run build` 成功，`bun run lint` 无错误，所有页面正常渲染。

## 影响范围

**删除的文件：**
- `web/src/components/ui/` — 18 个 shadcn 遗留组件（badge, button, card, checkbox, dialog, dropdown-menu, input, label, popover, progress, radio-group, select, separator, skeleton, spotlight-card, table, tabs, tooltip）
- `web/src/components/layout/PanelReveal.tsx` — 入场动画包装器
- `web/src/components/form/OptionPicker.tsx` — 自封装下拉组件

**新增/重写的文件：**
- `web/src/components/layout/PageHero.tsx` — 简化为纯 Tailwind 标题区域
- `web/src/components/form/FieldLabel.tsx` — 统一表单标签组件

**修改的页面（移除 PanelReveal + eyebrow + 样式统一）：**
- `web/src/pages/Dashboard.tsx`
- `web/src/pages/Users.tsx`
- `web/src/pages/Inbounds.tsx`
- `web/src/pages/Certificates.tsx`
- `web/src/pages/Traffic.tsx`
- `web/src/pages/Subscriptions.tsx`
- `web/src/pages/Core.tsx`
- `web/src/pages/Login.tsx` — 去除渐变装饰
- `web/src/pages/Setup.tsx` — 去除渐变装饰，简化为单列布局

**修改的组件：**
- `web/src/components/layout/AppLayout.tsx` — 简化页头
- `web/src/components/layout/Sidebar.tsx` — 移除 Control Deck、简化导航样式
- `web/src/components/users/BatchActionBar.tsx` — window.confirm 替换为 HeroUI Modal
- `web/src/components/users/UserFormModal.tsx` — shadcn 类名替换
- `web/src/components/users/UserTable.tsx` — shadcn 类名替换
- `web/src/components/users/UserSubscriptionCard.tsx` — shadcn 类名替换
- `web/src/components/users/UserSubscriptionModal.tsx` — shadcn 类名替换
- `web/src/components/certificates/CertificateFormModal.tsx` — 统一使用共享 FieldLabel、shadcn 类名替换
- `web/src/components/inbounds/InboundFormModal.tsx` — OptionPicker 替换为 HeroUI Select、shadcn 类名替换
- `web/src/components/inbounds/InboundTable.tsx` — shadcn 类名替换
- `web/src/components/certificates/CertificateTable.tsx` — shadcn 类名替换

**修改的样式/配置：**
- `web/src/index.css` — 移除 panel-reveal 动画、网格背景、body 渐变装饰
- `web/package.json` — 移除 react-bits 依赖
- `web/bun.lock` — 更新 lockfile

## 历史补丁

- patch-1: HeroUI 迁移运行时错误与样式修复
