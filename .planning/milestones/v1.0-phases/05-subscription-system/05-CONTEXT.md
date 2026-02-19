# Phase 5: Subscription System - Context

**Gathered:** 2026-02-12
**Status:** Ready for planning

<domain>
## Phase Boundary

为每个用户生成唯一订阅链接，支持 Base64（V2ray 标准分享链接）和 ClashMeta YAML 格式输出。管理员可在后台查看用户订阅信息、复制链接、展示 QR 码。订阅 API 端点无需认证，知道链接即可访问。

流量统计属于 Phase 6，不在本阶段范围内。

</domain>

<decisions>
## Implementation Decisions

### 订阅链接设计
- 使用短 token（如 `/sub/abc123def`），非 UUID，需额外生成并存储在数据库
- 格式切换采用双模式：默认 UA 自动检测（Clash 客户端返回 YAML，其他返回 Base64），同时支持 `?format=clash` query param 强制指定
- 管理员可重置用户的订阅 token，重置后旧链接立即失效
- 订阅链接无额外认证，知道链接即可访问（最佳客户端兼容性）

### 信息展示（管理后台内）
- 订阅信息仅在管理后台展示，不作为公开信息页（订阅链接本身不附带信息页）
- 展示内容丰富：用户名、剩余流量、到期时间、节点列表、各节点状态、每节点一键复制链接
- 暗色卡片风格，与管理面板整体设计保持一致
- 响应式布局，桌面端和移动端均可用

### QR 码
- 订阅链接级别生成一个 QR 码（不是每个节点各一个）
- QR 码展示位置：用户详情弹窗内
- 展示方式：默认隐藏，点击按钮弹出/展开
- QR 码旁附带一键复制订阅链接功能

### 订阅内容生成
- 禁用或过期用户访问订阅链接时返回 HTTP 403
- Base64 格式：标准 V2ray 分享链接（`vless://uuid@host:port?params#name`）的 Base64 编码
- Clash 格式：仅 ClashMeta 格式（支持 VLESS 和 Hysteria2，普通 Clash 不支持这些协议）
- 响应携带 `subscription-userinfo` header，包含流量用量和到期信息（主流客户端均支持）

### Claude's Discretion
- 短 token 的具体长度和字符集
- 订阅 API 的具体路由结构
- QR 码的尺寸和样式细节
- 用户详情弹窗中订阅信息区域的具体布局
- UA 检测的具体规则（哪些 UA 匹配 Clash）

</decisions>

<specifics>
## Specific Ideas

- 信息展示集成在用户详情弹窗中，不需要独立的公开信息页——管理员查看用户时一目了然
- subscription-userinfo header 使客户端能直接显示用户剩余流量和到期时间，无需额外查询

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 05-subscription-system*
*Context gathered: 2026-02-12*
