# Phase 4: User Management - Context

**Gathered:** 2026-02-12
**Status:** Ready for planning

<domain>
## Phase Boundary

管理员可创建、编辑、删除代理用户，将用户分配到入站节点，设置流量上限与到期时间，支持搜索筛选和批量操作。系统自动为 VLESS 用户生成 UUID。订阅系统和流量统计属于后续阶段。

</domain>

<decisions>
## Implementation Decisions

### 用户表单与节点分配
- 创建/编辑用户在同一弹框表单中完成，包含用户基本信息和节点分配
- 节点分配使用多选下拉框（适合节点数不多的场景）
- UUID（VLESS）和密码（Hysteria2）由系统自动生成，用户详情中只读展示 + 一键复制
- 同一用户分配到多个 VLESS 节点时，所有节点共享同一个 UUID

### 流量与到期展示
- 流量用量纯文字展示（如「2.3 / 10 GB (23%)」），紧凑省空间
- 到期时间使用绝对日期展示（如「2026-03-15」）

### 流量与到期处理
- 用户流量达到上限时，系统自动禁用该用户（从 sing-box 配置中移除）
- 用户到期时，系统自动禁用该用户（从 sing-box 配置中移除）
- ConfigGenerator 需过滤掉被禁用/超限/过期的用户

### 批量操作
- 每行左侧 Checkbox 选中用户，勾选后顶部出现批量操作栏
- 表头 Checkbox 全选当前页
- 批量操作包括：删除、启用/禁用、流量重置（来自 USR-09）
- 危险操作（如批量删除）使用简单确认弹框（「确定删除 5 个用户？」）
- 操作完成后 Toast 提示 + 列表自动刷新

### Claude's Discretion
- 用户列表的具体列选择和信息密度
- 状态标签的颜色和样式设计
- 搜索筛选的具体范围和交互方式
- 表单字段的具体排列和分组
- 空状态的展示设计

</decisions>

<specifics>
## Specific Ideas

- 自动禁用意味着 ConfigGenerator 需要在生成配置时感知用户状态（启用/禁用、流量是否超限、是否过期），只为有效用户生成 sing-box 用户配置
- UUID 全节点共享简化了用户管理，也便于后续订阅系统生成链接（Phase 5）

</specifics>

<deferred>
## Deferred Ideas

None — 讨论保持在阶段范围内

</deferred>

---

*Phase: 04-user-management*
*Context gathered: 2026-02-12*
