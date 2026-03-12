# 前端全面迁移到 HeroUI v3 Beta

> 状态: 未执行

## 目标

基于当前 `web/` 代码现状，为整个前端建立一套可执行的 HeroUI v3.0.0 beta 迁移路径：覆盖全部页面、共享布局、表格、表单与模态框，尽量把现有 Tailwind 手写外观压缩到 HeroUI 不提供能力的少量布局类，最终让界面结构、状态展示和交互容器都回到 HeroUI 原生组件体系。

## 内容

### 阶段 1：补齐迁移基线，先统一全局接入方式

1. 检查并补齐 HeroUI v3 beta 的全局接入：
   - `web/src/main.tsx`
   - `web/src/components/theme-provider.tsx`
   - `web/src/index.css`
2. 明确三条基线规则：
   - 页面和业务组件优先使用 HeroUI 组件组合，不再先写 HTML 容器再补 Tailwind 皮肤。
   - `className` 仅保留布局类、尺寸约束类和 HeroUI 暂无对应 API 的极少数样式。
   - 所有状态颜色、边框、表面层级都统一走 HeroUI semantic token 或组件 prop，不再手写视觉语义。
3. 清理当前全局样式中不属于设计系统的内容，只保留导入、字体、文档根节点、高度基线和必要业务动画。
4. 产出一份迁移映射表，约束常见替换方向：
   - 页面外层容器：手写 `div + p-6 + space-y-*` -> HeroUI section/card/header 组合
   - 手写标题区 -> 统一的 HeroUI 页面头部骨架
   - 手写加载/错误/空态文案 -> HeroUI `Spinner` / `Alert` / 表格空态
   - 手写确认框 `window.confirm` -> HeroUI `Modal` 确认流

完成标准：
`main.tsx`、主题接入和全局 CSS 的职责清晰，项目有统一的“什么必须用 HeroUI、什么允许保留 Tailwind”的迁移边界。

### 阶段 2：先收敛布局骨架，避免每个页面继续各写一套壳

1. 重构全局布局层：
   - `web/src/components/layout/AppLayout.tsx`
   - `web/src/components/layout/Sidebar.tsx`
   - `web/src/components/layout/PageHero.tsx`
2. 目标不是简单“换颜色”，而是把这些组件改造成可复用的 HeroUI 页面框架：
   - 侧边导航改成 HeroUI 风格的导航列表/抽屉模式，移动端遮罩和面板不再手写。
   - 页面头部统一提供标题、描述、指标、操作区和状态区插槽，减少页面内重复排版。
   - 主内容容器的边框、阴影、表面层级由 HeroUI 控制，避免 `bg-content1`、`rounded-xl`、`shadow-*` 到处手写。
3. 同时整理布局层与主题切换、路由跳转之间的关系，保证后续页面迁移时不再改壳。

完成标准：
所有业务页都能复用同一套 HeroUI 页面框架；布局文件不再承担大段视觉样式拼接。

### 阶段 3：按页面类型分批迁移，优先清理最常见的数据页模式

#### 批次 A：认证与初始化页

涉及文件：
- `web/src/pages/Login.tsx`
- `web/src/pages/Setup.tsx`

实施要点：
- 统一使用 HeroUI 的 `Card`、输入组件、按钮、错误提示与提交状态。
- 去掉登录卡片和初始化卡片上多余的手写对齐、阴影、尺寸和品牌块样式。
- 让表单结构回到 HeroUI 自身的 header/body/footer 节奏，而不是 `Card + 自定义 div` 套娃。

#### 批次 B：信息总览页

涉及文件：
- `web/src/pages/Dashboard.tsx`
- `web/src/pages/Traffic.tsx`

实施要点：
- 指标卡片统一成 HeroUI 统计卡片模式，弱化手写网格、标题、数值字号和说明文字类名。
- `Traffic.tsx` 的标签页、表格和状态标签全部回归 HeroUI 语义 API。
- 加载中与空数据状态不再直接塞进 `Table.Cell` 文本，而是统一使用 HeroUI 推荐的 loading / empty 表达方式。

#### 批次 C：标准 CRUD 数据页

涉及文件：
- `web/src/pages/Users.tsx`
- `web/src/pages/Inbounds.tsx`
- `web/src/pages/Certificates.tsx`
- `web/src/pages/Subscriptions.tsx`

实施要点：
- 搜索栏、排序、批量操作、新增按钮统一进入 HeroUI 风格的页头操作区。
- 页内摘要信息和“Query State / Sort Preset”这类手写说明块，整理成 HeroUI `Chip`、`Snippet`、`Card` 或更轻的描述组件。
- 所有删除确认从 `window.confirm` 迁移到 HeroUI `Modal`，避免交互割裂。
- 表格中的操作区、链接展示、二维码弹窗、状态标签全部统一为 HeroUI 组合，不再保留手写代码块底色容器。

#### 批次 D：复杂运维页

涉及文件：
- `web/src/pages/Core.tsx`

实施要点：
- 这是当前最重页面，迁移时单独处理，不和其他页面混在一起。
- 把状态卡、升级进度、错误详情、版本列表、日志查看、确认弹窗统一到 HeroUI 卡片/告警/进度/模态框体系。
- 去掉手写进度条、手写状态点、手写空态区块，只保留 HeroUI 不覆盖时必须的布局类。

完成标准：
9 个页面都完成 HeroUI 化，页面级代码主要在表达业务结构，不再承担大量视觉样式实现。

### 阶段 4：迁移共享业务组件，建立可复用的 HeroUI 业务积木

涉及文件：
- `web/src/components/form/FieldLabel.tsx`
- `web/src/components/users/BatchActionBar.tsx`
- `web/src/components/users/UserTable.tsx`
- `web/src/components/users/UserSubscriptionCard.tsx`
- `web/src/components/users/UserSubscriptionModal.tsx`
- `web/src/components/inbounds/InboundTable.tsx`
- `web/src/components/certificates/CertificateTable.tsx`

实施要点：
1. 统一表格体系：
   - 表头、空态、加载态、行内操作、状态 `Chip`、多选列全部按 HeroUI 能力收敛。
2. 统一详情与订阅展示：
   - 订阅链接、节点列表、二维码、复制按钮改成 HeroUI 原生信息容器，不再手写 `code + bg-content2` 风格块。
3. 统一字段标签和提示：
   - `FieldLabel` 只保留语义，不再负责手写间距和 tooltip 外观。
4. 统一批量操作条：
   - 从“自定义 Card 样式条”收敛成可复用的 HeroUI 工具条/操作面板。

完成标准：
通用业务组件可以被多个页面直接复用；同类交互不再在不同页面各写一套 className。

### 阶段 5：单独攻克高复杂度表单与模态框

涉及文件：
- `web/src/components/users/UserFormModal.tsx`
- `web/src/components/inbounds/InboundFormModal.tsx`
- `web/src/components/certificates/CertificateFormModal.tsx`

实施要点：
1. 统一模态框骨架：
   - header/body/footer 分层固定下来，滚动区、宽度、高度限制走同一套策略。
2. 统一表单字段体系：
   - 输入框、单选、复选、选择器、下拉浮层、帮助提示、校验错误都尽量改用 HeroUI 原生表单能力。
3. 去掉表单中的手写块级结构：
   - `space-y-*`、`grid gap-*` 只保留必要布局；
   - 各种“基本设置 / TLS / 传输”分组改成 HeroUI `Card` / `Accordion` / `Tabs` 等更稳定的容器。
4. 优先解决两个最高复杂度文件：
   - `InboundFormModal.tsx`（911 行）
   - `UserFormModal.tsx`（315 行）
5. 对 `react-hook-form + zod` 的绑定方式做一次抽象收敛，避免迁移 HeroUI 组件后每个字段都重复写一遍胶水代码。

完成标准：
复杂模态框里的绝大多数 UI 交互都使用 HeroUI 原生组件；表单代码聚焦于数据结构和校验，而不是样式维护。

### 阶段 6：交互收尾与样式减法

1. 全面清理剩余的视觉类噪点：
   - 逐文件审查 `className`
   - 删除只为“看起来像组件”而存在的边框、背景、阴影、圆角类
2. 重点清理以下模式：
   - 手写 `text-danger` / `text-foreground-500` 错误文案块
   - 手写 `rounded-lg bg-content2` 信息块
   - 手写 `border-divider bg-content1 shadow-*` 容器
   - 手写移动端遮罩和抽屉结构
3. 只保留三类 Tailwind：
   - 页面/区域级布局
   - HeroUI 没有直接暴露 prop 的尺寸约束
   - 与业务强绑定的极小量样式（例如日志等宽文本区域）
4. 用可量化方式验收减法结果：
   - 优先关注当前高密度文件：`Core.tsx`、`InboundFormModal.tsx`、`UserFormModal.tsx`
   - 对比迁移前后的 `className` 数量和自定义状态块数量，确认明显下降

完成标准：
项目中的自定义样式只剩“布局必需品”，而不是继续承担 HeroUI 本该负责的外观职责。

### 阶段 7：验收与回归检查

1. 基础检查：
   - `web/src/` 中不存在新的原生确认框、手写状态色块和无必要样式容器扩散
   - 页面和共享组件的视觉语义统一
2. 运行检查：
   - `cd web && bun run build`
   - 如环境允许，再执行 `cd web && bun run lint`
3. 人工走查范围：
   - 登录 / 初始化
   - 仪表盘 / 流量
   - 入站 / 用户 / 订阅 / 证书
   - 核心页的日志、更新、回滚、版本列表、错误弹窗
   - 亮色与暗色主题
   - 桌面端与移动端侧边导航
4. 验收口径：
   - 页面与组件以 HeroUI 结构为主
   - 自定义样式显著减少
   - 交互一致性比当前版本更强
   - 不牺牲现有业务能力

## 影响范围

- `web/src/main.tsx`
- `web/src/index.css`
- `web/src/components/theme-provider.tsx`
- `web/src/components/layout/*`
- `web/src/components/form/*`
- `web/src/components/users/*`
- `web/src/components/inbounds/*`
- `web/src/components/certificates/*`
- `web/src/pages/*`

## 历史补丁
