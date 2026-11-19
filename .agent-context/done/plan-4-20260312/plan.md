# 前端样式优化与 HeroUI 收口

> 状态: 已执行

## 目标

完成前端剩余组件向 HeroUI 3.0 的迁移，移除业务页面对旧 `components/ui/*` 包装层的直接依赖；同时重做应用壳层与数据页首屏结构，提升侧边栏、页面顶部和模块进入时的视觉层次与动效一致性，并以 oxlint 作为本次验证手段。

## 内容

### 步骤 1：全局布局重构

- 重做 `AppLayout` 顶部区域，去掉与页面内容重复的标题条，改为承载全局信息与快捷操作的控制条
- 重做 `Sidebar` 顶部品牌区、状态区和导航激活态，增强层次、氛围和移动端表现

### 步骤 2：页面首屏与动效统一

- 为 Dashboard、Users、Inbounds、Certificates、Subscriptions、Traffic、Core 页面统一页面 hero / control rail 结构
- 抽离统一的进入动效节奏，替换零散的 `animate-in` 和内联 delay 写法

### 步骤 3：HeroUI 迁移收口

- 迁移仍直接依赖 `@/components/ui/*` 的业务页面和组件到 `@heroui/react`
- 优先覆盖表格、按钮、输入、弹窗、下拉、标签、选项卡、进度和提示类组件

### 步骤 4：验证与计划更新

- 使用 oxlint 检查前端代码
- 将实际变更文件回写到 `## 影响范围`
- 完成后把当前计划状态更新为 `已执行`

## 影响范围

- `.agent-context/plan-4/plan.md`
- `web/package.json`
- `web/bun.lock`
- `web/vite.config.ts`
- `web/src/index.css`
- `web/src/hooks/use-count-up.ts`
- `web/src/components/form/FieldLabel.tsx`
- `web/src/components/form/OptionPicker.tsx`
- `web/src/components/layout/AppLayout.tsx`
- `web/src/components/layout/Sidebar.tsx`
- `web/src/components/layout/PageHero.tsx`
- `web/src/components/layout/PanelReveal.tsx`
- `web/src/pages/Dashboard.tsx`
- `web/src/pages/Users.tsx`
- `web/src/pages/Inbounds.tsx`
- `web/src/pages/Certificates.tsx`
- `web/src/pages/Subscriptions.tsx`
- `web/src/pages/Traffic.tsx`
- `web/src/pages/Core.tsx`
- `web/src/components/users/BatchActionBar.tsx`
- `web/src/components/users/UserTable.tsx`
- `web/src/components/users/UserFormModal.tsx`
- `web/src/components/users/UserSubscriptionCard.tsx`
- `web/src/components/users/UserSubscriptionModal.tsx`
- `web/src/components/inbounds/InboundTable.tsx`
- `web/src/components/inbounds/InboundFormModal.tsx`
- `web/src/components/certificates/CertificateTable.tsx`
- `web/src/components/certificates/CertificateFormModal.tsx`

## 历史补丁
