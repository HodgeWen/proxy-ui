# Requirements: s-ui

**Defined:** 2026-02-11
**Core Value:** 提供一个界面美观、体验流畅、部署简单的 sing-box 管理面板

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### 入站管理 (Inbound)

- [ ] **INB-01**: 用户可以通过弹框表单添加 VLESS 入站配置
- [ ] **INB-02**: 用户可以通过弹框表单添加 Hysteria2 入站配置
- [ ] **INB-03**: 用户可以编辑已有的入站配置
- [ ] **INB-04**: 用户可以删除入站配置
- [ ] **INB-05**: 用户可以配置入站的端口、监听地址、标签
- [ ] **INB-06**: 用户可以配置入站的 TLS 选项（启用/禁用、关联证书）
- [ ] **INB-07**: 用户可以配置高级传输选项（WebSocket, gRPC, HTTP/2）
- [ ] **INB-08**: 表单字段使用智能默认值预填（flow、transport、端口等最佳实践）
- [ ] **INB-09**: 每个表单字段旁有 info 图标，悬浮显示该字段的用途说明
- [ ] **INB-10**: 配置变更后自动触发 `sing-box check` 验证，通过后应用并重启

### 用户管理 (User)

- [ ] **USR-01**: 管理员可以通过弹框表单创建用户（用户名、备注）
- [ ] **USR-02**: 管理员可以编辑用户信息
- [ ] **USR-03**: 管理员可以删除用户
- [ ] **USR-04**: 管理员可以将用户分配到一个或多个入站节点
- [ ] **USR-05**: 管理员可以为用户设置流量上限（字节数）
- [ ] **USR-06**: 管理员可以为用户设置到期时间
- [ ] **USR-07**: 系统自动为 VLESS 用户生成 UUID，支持一键复制
- [ ] **USR-08**: 管理员可以通过关键字搜索和筛选用户列表
- [ ] **USR-09**: 管理员可以批量选择用户执行删除、启用/禁用、流量重置操作

### 订阅系统 (Subscription)

- [ ] **SUB-01**: 系统为每个用户生成唯一的订阅链接
- [ ] **SUB-02**: 订阅链接返回 Base64 编码的代理列表（V2ray 标准格式）
- [ ] **SUB-03**: 订阅链接支持 Clash/ClashMeta YAML 格式输出
- [ ] **SUB-04**: 用户可以为每个节点生成 QR 码（支持扫码导入客户端）
- [ ] **SUB-05**: 订阅链接附带信息页，显示用户用量、到期时间、节点列表

### 证书管理 (Certificate)

- [ ] **CRT-01**: 管理员可以手动指定 TLS 证书路径（fullchain + privkey）
- [ ] **CRT-02**: 管理员可以将证书关联到入站配置的 TLS 选项

### 流量统计 (Traffic)

- [ ] **TRF-01**: 系统记录并展示每个用户的上行/下行流量使用量
- [ ] **TRF-02**: 系统记录并展示每个入站节点的总流量使用量

### sing-box 核心管理 (Core)

- [ ] **COR-01**: 面板可以应用配置变更并重启 sing-box 进程
- [ ] **COR-02**: 面板启动时自动检测 sing-box 版本并显示
- [ ] **COR-03**: 管理员可以在面板中将 sing-box 核心更新到最新版本
- [ ] **COR-04**: 管理员可以将 sing-box 核心回滚到上一个已安装版本

### 面板系统 (System)

- [ ] **SYS-01**: 管理员通过用户名/密码登录面板（首次部署必须设置，无默认密码）
- [ ] **SYS-02**: 面板支持 HTTPS 访问
- [ ] **SYS-03**: 支持 Docker Compose 一条命令部署
- [ ] **SYS-04**: 支持 bash 安装脚本部署（curl | bash）
- [ ] **SYS-05**: 支持下载单二进制文件 + 配置文件运行

### 界面体验 (UX)

- [ ] **UX-01**: 暗色主题 UI（Vercel/Linear 风格）
- [ ] **UX-02**: 使用 shadcn/ui 组件库 + Tailwind CSS
- [ ] **UX-03**: 使用 react-bits 库增强微交互（动画、过渡效果）
- [ ] **UX-04**: 界面语言为中文（专业术语如 VLESS、TLS、Hysteria2 保留英文）
- [ ] **UX-05**: 所有配置操作通过弹框（Modal）表单完成，不跳转页面

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### 证书增强

- **CRT-10**: 一键 Let's Encrypt 证书申请（ACME 集成）
- **CRT-11**: 证书自动续期（到期前 30 天自动更新）

### 流量增强

- **TRF-10**: 在线客户端数实时显示
- **TRF-11**: 实时仪表盘/图表（流量趋势、用量排行）
- **TRF-12**: 周期性流量限制（日/周/月重置）

### 订阅增强

- **SUB-10**: 外部订阅链接合并
- **SUB-11**: 订阅 token 轮换

### 系统增强

- **SYS-10**: REST API（自动化集成）
- **SYS-11**: 数据库备份/恢复
- **SYS-12**: 反向代理最佳实践文档和自动检测

### 用户增强

- **USR-10**: 周期性流量重置策略

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| 服务器系统监控（CPU/内存/带宽） | 不是核心功能，保持面板聚焦代理管理 |
| VLESS 和 Hysteria2 之外的协议 | v1 聚焦两个协议，降低复杂度，后续可扩展 |
| 多语言国际化 | 仅支持中文，减少开发成本 |
| Telegram Bot | 范围蔓延，与核心价值无关 |
| 移动端 App | Web 优先 |
| 多管理员角色 | v1 单管理员，降低复杂度 |
| SaaS 功能（套餐/订单/支付） | 不同产品形态 |
| CLI 工具 | Web 面板为主 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| INB-01 | Phase 2 | Pending |
| INB-02 | Phase 2 | Pending |
| INB-03 | Phase 2 | Pending |
| INB-04 | Phase 2 | Pending |
| INB-05 | Phase 2 | Pending |
| INB-06 | Phase 2 | Pending |
| INB-07 | Phase 2 | Pending |
| INB-08 | Phase 2 | Pending |
| INB-09 | Phase 2 | Pending |
| INB-10 | Phase 2 | Pending |
| USR-01 | Phase 4 | Pending |
| USR-02 | Phase 4 | Pending |
| USR-03 | Phase 4 | Pending |
| USR-04 | Phase 4 | Pending |
| USR-05 | Phase 4 | Pending |
| USR-06 | Phase 4 | Pending |
| USR-07 | Phase 4 | Pending |
| USR-08 | Phase 4 | Pending |
| USR-09 | Phase 4 | Pending |
| SUB-01 | Phase 5 | Pending |
| SUB-02 | Phase 5 | Pending |
| SUB-03 | Phase 5 | Pending |
| SUB-04 | Phase 5 | Pending |
| SUB-05 | Phase 5 | Pending |
| CRT-01 | Phase 3 | Pending |
| CRT-02 | Phase 3 | Pending |
| TRF-01 | Phase 6 | Pending |
| TRF-02 | Phase 6 | Pending |
| COR-01 | Phase 1 | Pending |
| COR-02 | Phase 1 | Pending |
| COR-03 | Phase 7 | Pending |
| COR-04 | Phase 7 | Pending |
| SYS-01 | Phase 1 | Pending |
| SYS-02 | Phase 8 | Pending |
| SYS-03 | Phase 8 | Pending |
| SYS-04 | Phase 8 | Pending |
| SYS-05 | Phase 8 | Pending |
| UX-01 | Phase 1 | Pending |
| UX-02 | Phase 1 | Pending |
| UX-03 | Phase 1 | Pending |
| UX-04 | Phase 1 | Pending |
| UX-05 | Phase 1 | Pending |

**Coverage:**
- v1 requirements: 42 total
- Mapped to phases: 42
- Unmapped: 0 ✓

---
*Requirements defined: 2026-02-11*
*Last updated: 2026-02-11 after roadmap creation*
