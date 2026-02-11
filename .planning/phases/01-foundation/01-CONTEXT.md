# Phase 1: Foundation - Context

**Gathered:** 2026-02-11
**Status:** Ready for planning

<domain>
## Phase Boundary

搭建管理面板基础框架：管理员认证登录、sing-box 进程集成（版本检测、配置应用、进程重启）、暗色主题 UI 框架（shadcn/ui + react-bits 微交互）。不包含任何业务功能（入站管理、用户管理等属于后续阶段）。

</domain>

<decisions>
## Implementation Decisions

### 登录与首次设置流程
- 首次部署使用独立 Setup 页面，包含设置用户名、密码、确认密码
- 登录后 Session 有效期 7 天，支持「记住我」选项
- 登录页采用居中极简风格：纯色背景 + 居中登录卡片（Vercel Dashboard 风格）
- 登录失败显示简单提示「用户名或密码错误」，不做重试次数限制

### 面板整体布局
- 固定左侧边栏导航（Linear/Vercel 风格）
- 侧边栏预留所有将来菜单项（入站、用户、订阅、证书、流量、核心），未实现的置灰/禁用
- 仪表盘（首页）显示 sing-box 状态卡片 + 预留统计概览卡片（入站数、用户数等，Phase 1 显示 0）
- Phase 1 仅考虑桌面端，不做移动端响应式适配

### sing-box 状态与操作反馈
- sing-box 运行状态仅在仪表盘卡片中展示（运行/停止状态、版本号、运行时长）
- 配置应用成功/失败使用 Toast 通知（右上角弹出，几秒后自动消失）
- 重启 sing-box 时按钮变为 Loading 状态，完成后恢复
- sing-box check 验证失败时弹出 Modal 显示原始错误输出（代码块格式）

### 视觉风格与微交互
- 暗色主题使用深灰系配色（类似 Linear/GitHub Dark，#1a1a2e/#111827 色域），非纯黑
- 强调色使用紫色系（类似 Linear 紫色强调），用于按钮、链接、活动状态
- react-bits 微交互程度适中：页面切换、列表项出现、卡片 hover、Modal 弹出等均有过渡动画
- 无边框设计：用背景色差和阴影区分层次（Vercel 风格），不使用显式边框线

### Claude's Discretion
- 具体的色值定义与 CSS 变量命名
- 登录页 Logo/品牌展示方式
- Toast 通知的精确位置和持续时间
- Loading 骨架屏的具体样式
- 侧边栏图标选择
- Setup 页面的具体表单校验规则

</decisions>

<specifics>
## Specific Ideas

- 登录页风格参考 Vercel Dashboard 的居中卡片式设计
- 整体暗色主题参考 Linear 的深灰色调（非纯黑），配合紫色强调色
- 层次区分参考 Vercel 的无边框设计（背景色差 + 阴影）
- 侧边栏预留骨架的设计意图：让用户在 Phase 1 就能看到产品的完整导航结构，有「产品成型」的感觉

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-foundation*
*Context gathered: 2026-02-11*
