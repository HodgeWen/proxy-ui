# Phase 2: Inbound Management - Context

**Gathered:** 2026-02-11
**Status:** Ready for planning

<domain>
## Phase Boundary

用户可完整管理 VLESS 和 Hysteria2 入站（添加、编辑、删除）。支持 TLS（含 Reality）、传输协议选项（WebSocket、gRPC、HTTP/2）。表单字段有智能默认值和帮助提示。配置变更后自动 sing-box check 验证并重启。

不包含：证书管理（Phase 3）、用户分配到入站（Phase 4）、流量统计（Phase 6）。

</domain>

<decisions>
## Implementation Decisions

### 表单结构与交互
- 统一表单 — 先选协议类型（VLESS / Hysteria2），表单动态切换对应字段
- 分段显示 — 表单按「基本设置 / TLS / 传输」分块，每块有标题
- 所有字段始终可见，不折叠高级选项
- 使用宽 Modal 弹框，给表单充足空间，减少滚动

### 入站列表展示
- 表格布局，每行一个入站
- 丰富信息列：标签、协议类型、端口、TLS 状态、传输协议、监听地址、用户数、创建时间
- 状态用图标表示（如 TLS 开启/关闭、传输协议类型）
- 操作按钮组合方式：常用操作（编辑）内联显示，更多操作（删除等）放下拉菜单

### TLS 与传输配置体验
- TLS 用下拉选择：无 TLS / TLS / Reality 三选一
- VLESS Reality 纳入本阶段，选择 Reality 后动态显示 Reality 专属字段（server_name、public_key 等）
- 传输协议用单选按钮展示（TCP / WebSocket / gRPC / HTTP/2），选中后下方即时显示对应配置字段
- 协议/TLS 切换时字段即时切换，不使用过渡动画

### 智能默认值与帮助提示
- 智能默认值 — 根据协议类型自动填充常见配置（VLESS 自动生成 UUID、Hysteria2 自动生成密码、端口等）
- info 图标悬浮提示内容详细：字段说明 + 典型值举例 + 何时需要修改
- 表单验证在字段失焦时触发（如端口范围、必填项）
- sing-box check 验证失败时在 Modal 内顶部显示错误信息，不关闭弹框，用户可直接修改

### Claude's Discretion
- 表格列的具体宽度与排列顺序
- 表单字段的具体排列顺序
- info tooltip 的具体文案措辞
- 空状态（无入站时）的展示方式
- 删除确认的交互方式

</decisions>

<specifics>
## Specific Ideas

- Hysteria2 的字段与 VLESS 差异较大（如 up/down 带宽、obfs 混淆），表单需要根据协议类型有明显不同的字段集
- Reality 是 VLESS 独有的 TLS 方案，Hysteria2 不支持 Reality，TLS 下拉选项需要根据协议类型动态调整
- 用户数列为后续 Phase 4 预留，本阶段可显示为 0 或 —

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-inbound-management*
*Context gathered: 2026-02-11*
