# 前端重构 — 页面组件迁移

> 状态: 未执行

## 目标

将所有业务页面及其子组件从 shadcn/ui 迁移到 HeroUI 3.0.0-beta.8，完成整个前端的 UI 库切换。参照 `docs/api.md` 确保所有功能完整覆盖。

## 内容

### 步骤 1：认证页面

- **Login** (`pages/Login.tsx`)：使用 HeroUI 的 TextField、Button、Card、Form 重构登录表单
- **Setup** (`pages/Setup.tsx`)：使用 HeroUI 组件重构首次设置页面

### 步骤 2：Dashboard

- **Dashboard** (`pages/Dashboard.tsx`)：使用 HeroUI Card 重构统计卡片，替换 spotlight-card

### 步骤 3：入站管理

- **Inbounds** (`pages/Inbounds.tsx`)：使用 HeroUI Table、Select、Button 重构列表页
- **InboundTable** (`components/inbounds/InboundTable.tsx`)：迁移到 HeroUI Table + Dropdown
- **InboundFormModal** (`components/inbounds/InboundFormModal.tsx`)：迁移到 HeroUI Modal + Form 组件（VLESS / Hysteria2 双表单），集成 react-hook-form + zod

### 步骤 4：用户管理

- **Users** (`pages/Users.tsx`)：使用 HeroUI 组件重构搜索和用户列表
- **UserTable** (`components/users/UserTable.tsx`)：迁移到 HeroUI Table + Dropdown + Checkbox
- **UserFormModal** (`components/users/UserFormModal.tsx`)：迁移到 HeroUI Modal + Form
- **UserSubscriptionModal** (`components/users/UserSubscriptionModal.tsx`)：迁移到 HeroUI Modal
- **UserSubscriptionCard** (`components/users/UserSubscriptionCard.tsx`)：迁移到 HeroUI Card
- **BatchActionBar** (`components/users/BatchActionBar.tsx`)：使用 HeroUI Button、Alert

### 步骤 5：订阅管理

- **Subscriptions** (`pages/Subscriptions.tsx`)：使用 HeroUI Table、Badge、Modal 重构

### 步骤 6：证书管理

- **Certificates** (`pages/Certificates.tsx`)：使用 HeroUI 组件重构列表页
- **CertificateTable** (`components/certificates/CertificateTable.tsx`)：迁移到 HeroUI Table + Dropdown
- **CertificateFormModal** (`components/certificates/CertificateFormModal.tsx`)：迁移到 HeroUI Modal + Form

### 步骤 7：流量统计

- **Traffic** (`pages/Traffic.tsx`)：使用 HeroUI Tabs、Table、Badge 重构

### 步骤 8：核心管理

- **Core** (`pages/Core.tsx`)：使用 HeroUI Card、Badge、Button、Modal、Select 重构。SSE 更新进度保持现有 hook 逻辑，UI 层替换。

### 步骤 9：全局与工具

- 替换全局 Toaster（sonner → HeroUI Toast）
- 确认 `cn()` 工具函数与 HeroUI class 兼容
- 更新 `useIsMobile` hook（如有 HeroUI 响应式方案可替换）

### 步骤 10：验证

- 所有页面可正常渲染
- 表单提交、数据加载、操作反馈正常
- 暗/亮主题切换正常
- 移动端响应式正常
- `bun run lint` 通过
- `bun run dev` 全功能验证

## 影响范围

## 历史补丁
