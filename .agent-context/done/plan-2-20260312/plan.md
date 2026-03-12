# 前端重构 — 基础设施迁移

> 状态: 已执行

## 目标

将前端 UI 基础从 shadcn/ui (Radix) 迁移到 HeroUI 3.0.0-beta.8 (React Aria)，完成依赖替换、主题系统重建、基础组件替换。为后续页面级迁移提供完整的组件基础设施。

## 内容

### 步骤 1：依赖管理

**移除：**
- shadcn/ui 相关：`radix-ui`、各 `@radix-ui/react-*` 包、`class-variance-authority`、`tw-animate-css`
- shadcn CLI 开发依赖
- `sonner`（HeroUI v3 有内置 Toast）

**安装：**
- `@heroui/react@beta`（组件库）
- `@heroui/styles@beta`（样式系统）

**保留：**
- `@tanstack/react-query`（数据请求，HeroUI 不替代）
- `react-hook-form` + `zod` + `@hookform/resolvers`（表单验证逻辑，与 HeroUI Form 组件集成）
- `react-router-dom`（路由）
- `lucide-react`（图标库）
- `qrcode.react`（二维码）
- `clsx` + `tailwind-merge`（`cn()` 工具）

### 步骤 2：主题系统重建

- 移除 `web/src/index.css` 中 shadcn 的 oklch CSS 变量和 `@import "shadcn/tailwind.css"`
- 引入 `@heroui/styles` 的 CSS
- 重建暗/亮模式主题变量，适配 HeroUI 的语义化 token（primary, secondary, danger 等）
- 更新 `ThemeProvider` 以兼容 HeroUI 的主题机制

### 步骤 3：删除 shadcn/ui 基础组件

删除 `web/src/components/ui/` 下所有 shadcn 生成的组件文件：
badge, button, card, checkbox, dialog, dropdown-menu, input, label, popover, progress, radio-group, select, separator, sheet, sidebar, skeleton, sonner, spotlight-card, table, tabs, tooltip

### 步骤 4：建立组件映射层

创建 HeroUI 组件的项目级封装或 re-export（如需自定义默认 props），确保：

| shadcn 组件 | HeroUI v3 对应 |
|-------------|----------------|
| Button | Button |
| Input | TextField / Input |
| Select | Select |
| Checkbox | Checkbox |
| RadioGroup | RadioGroup |
| Dialog | Modal |
| DropdownMenu | Dropdown |
| Table | Table |
| Card | Card |
| Badge | Badge |
| Tabs | Tabs |
| Tooltip | Tooltip |
| Popover | Popover |
| Progress | Spinner / Skeleton |
| Skeleton | Skeleton |
| Sheet | Modal (side variant) |
| Separator | Separator |
| Label | Label |
| Toast (sonner) | Toast |

### 步骤 5：更新布局系统

- 重构 `AppLayout.tsx`：移除 shadcn Sidebar 依赖，使用 HeroUI 组件或自定义侧边栏
- 重构 `Sidebar.tsx`：使用 HeroUI 的 Link、Button、Separator 等重建导航

### 步骤 6：验证

- `bun run lint` 通过
- `bun run dev` 能正常启动
- 布局和主题切换正常工作

## 影响范围

- `web/package.json`
- `web/eslint.config.js`
- `web/vite.config.ts`
- `web/tsconfig.app.json`
- `web/tsconfig.json`
- `web/src/index.css`
- `web/src/main.tsx`
- `web/src/shims/sonner.ts`
- `web/src/components/theme-provider.tsx`
- `web/src/components/layout/AppLayout.tsx`
- `web/src/components/layout/Sidebar.tsx`
- `web/src/components/ui/badge.tsx`
- `web/src/components/ui/button.tsx`
- `web/src/components/ui/card.tsx`
- `web/src/components/ui/checkbox.tsx`
- `web/src/components/ui/dialog.tsx`
- `web/src/components/ui/dropdown-menu.tsx`
- `web/src/components/ui/input.tsx`
- `web/src/components/ui/label.tsx`
- `web/src/components/ui/popover.tsx`
- `web/src/components/ui/progress.tsx`
- `web/src/components/ui/radio-group.tsx`
- `web/src/components/ui/select.tsx`
- `web/src/components/ui/separator.tsx`
- `web/src/components/ui/skeleton.tsx`
- `web/src/components/ui/sheet.tsx`
- `web/src/components/ui/sidebar.tsx`
- `web/src/components/ui/sonner.tsx`
- `web/src/components/ui/spotlight-card.tsx`
- `web/src/components/ui/table.tsx`
- `web/src/components/ui/tabs.tsx`
- `web/src/components/ui/tooltip.tsx`
- `web/src/components/certificates/CertificateFormModal.tsx`
- `web/src/components/inbounds/InboundFormModal.tsx`
- `web/src/components/users/UserFormModal.tsx`
- `web/src/pages/Dashboard.tsx`
- `web/src/pages/Traffic.tsx`

## 历史补丁
