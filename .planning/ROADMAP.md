# Roadmap: s-ui

## Overview

从零构建一个 sing-box 管理面板：先建立基础（认证、数据库、sing-box 集成、UI 框架），再实现入站管理、证书管理、用户管理、订阅系统、流量统计、核心版本管理，最后完成三种部署方式与 HTTPS 支持。每个阶段交付可验证的用户能力。

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

- [x] **Phase 1: Foundation** - 项目脚手架、认证、sing-box 集成、UI 框架 ✓ (2026-02-11)
- [x] **Phase 2: Inbound Management** - VLESS/Hysteria2 入站 CRUD、TLS、传输选项 ✓ (2026-02-11)
- [x] **Phase 3: Certificate Management** - 手动证书路径、关联入站 TLS ✓ (2026-02-12)
- [x] **Phase 4: User Management** - 用户 CRUD、分配节点、流量/到期、批量操作 ✓ (2026-02-12)
- [x] **Phase 5: Subscription System** - 订阅链接、Base64/Clash 格式、QR 码、信息页 ✓ (2026-02-12)
- [x] **Phase 6: Traffic Statistics** - 按用户/按入站流量统计 ✓ (2026-02-12)
- [ ] **Phase 7: sing-box Core Management** - 核心更新/回滚
- [ ] **Phase 8: Deployment & Production** - HTTPS、Docker、bash、单二进制

## Phase Details

### Phase 1: Foundation

**Goal**: 管理员可以登录面板，面板能与 sing-box 交互，UI 具备暗色主题与弹框表单框架

**Depends on**: Nothing (first phase)

**Requirements**: SYS-01, COR-01, COR-02, UX-01, UX-02, UX-03, UX-04, UX-05

**Success Criteria** (what must be TRUE):
  1. 管理员可通过用户名/密码登录面板（首次部署必须设置，无默认密码）
  2. 面板启动时自动检测并显示 sing-box 版本
  3. 面板可应用配置变更并重启 sing-box 进程
  4. 界面使用暗色主题、shadcn/ui、中文语言
  5. 配置操作通过弹框（Modal）表单完成，不跳转页面

**Plans:** 3 plans

Plans:
- [x] 01-01-PLAN.md — 项目脚手架 + 认证（Setup/Login、Session、Admin）
- [x] 01-02-PLAN.md — sing-box 集成（版本检测、配置应用、重启）
- [x] 01-03-PLAN.md — UI 框架 + 仪表盘（暗色主题、Sidebar、sing-box 状态）

---

### Phase 2: Inbound Management

**Goal**: 用户可完整管理 VLESS 和 Hysteria2 入站（添加、编辑、删除、TLS、传输选项）

**Depends on**: Phase 1

**Requirements**: INB-01, INB-02, INB-03, INB-04, INB-05, INB-06, INB-07, INB-08, INB-09, INB-10

**Success Criteria** (what must be TRUE):
  1. 用户可通过弹框表单添加 VLESS 入站配置
  2. 用户可通过弹框表单添加 Hysteria2 入站配置
  3. 用户可编辑、删除已有入站配置
  4. 用户可配置端口、监听地址、标签、TLS 选项、高级传输（WebSocket、gRPC、HTTP/2）
  5. 表单字段有智能默认值，且每个字段旁有 info 图标悬浮说明
  6. 配置变更后自动触发 `sing-box check` 验证，通过后应用并重启

**Plans:** 6 plans

Plans:
- [x] 02-01-PLAN.md — Inbound model + DB layer (GORM, CRUD)
- [x] 02-02-PLAN.md — ConfigGenerator (DB → sing-box JSON)
- [x] 02-03-PLAN.md — Inbound API (CRUD, apply config, restart)
- [x] 02-04-PLAN.md — Inbound list UI (table, Edit, Delete)
- [x] 02-05-PLAN.md — Inbound form modal (VLESS/Hysteria2, TLS, transport)
- [x] 02-06-PLAN.md — Gap closure: protocol sync, FieldLabel tooltips

---

### Phase 3: Certificate Management

**Goal**: 管理员可指定 TLS 证书路径并将其关联到入站

**Depends on**: Phase 2

**Requirements**: CRT-01, CRT-02

**Success Criteria** (what must be TRUE):
  1. 管理员可手动指定 TLS 证书路径（fullchain + privkey）
  2. 管理员可将证书关联到入站配置的 TLS 选项

**Plans:** 3 plans

Plans:
- [x] 03-01-PLAN.md — Certificate model + DB + ConfigGenerator resolve cert_id
- [x] 03-02-PLAN.md — Cert CRUD API + certificates list/form UI
- [x] 03-03-PLAN.md — InboundFormModal TLS cert selector

---

### Phase 4: User Management

**Goal**: 管理员可创建、编辑、删除用户，分配节点，设置流量与到期，支持搜索与批量操作

**Depends on**: Phase 2

**Requirements**: USR-01, USR-02, USR-03, USR-04, USR-05, USR-06, USR-07, USR-08, USR-09

**Success Criteria** (what must be TRUE):
  1. 管理员可通过弹框表单创建、编辑、删除用户
  2. 管理员可将用户分配到一个或多个入站节点
  3. 管理员可为用户设置流量上限和到期时间
  4. 系统自动为 VLESS 用户生成 UUID，支持一键复制
  5. 管理员可通过关键字搜索和筛选用户列表
  6. 管理员可批量选择用户执行删除、启用/禁用、流量重置操作

**Plans:** 4 plans

Plans:
- [x] 04-01-PLAN.md — User model + ConfigGenerator (derive users from DB)
- [x] 04-02-PLAN.md — User CRUD API + batch endpoint
- [x] 04-03-PLAN.md — Users page + table + UserFormModal
- [x] 04-04-PLAN.md — Search + batch UI + verify

---

### Phase 5: Subscription System

**Goal**: 每个用户有唯一订阅链接，支持 Base64/Clash 格式、QR 码、信息页

**Depends on**: Phase 4

**Requirements**: SUB-01, SUB-02, SUB-03, SUB-04, SUB-05

**Success Criteria** (what must be TRUE):
  1. 系统为每个用户生成唯一的订阅链接
  2. 订阅链接返回 Base64 编码的代理列表（V2ray 标准格式）
  3. 订阅链接支持 Clash/ClashMeta YAML 格式输出
  4. 用户可为每个节点生成 QR 码（支持扫码导入客户端）
  5. 订阅链接附带信息页，显示用户用量、到期时间、节点列表

**Plans:** 3 plans

Plans:
- [x] 05-01-PLAN.md — Subscription token + generator + GET /sub/{token}
- [x] 05-02-PLAN.md — Admin API: reset token, subscription_url/nodes in GetUser
- [x] 05-03-PLAN.md — UserSubscriptionCard (QR, copy, reset) in UserSubscriptionModal

---

### Phase 6: Traffic Statistics

**Goal**: 面板展示每个用户和每个入站的流量使用量

**Depends on**: Phase 4

**Requirements**: TRF-01, TRF-02

**Success Criteria** (what must be TRUE):
  1. 系统记录并展示每个用户的上行/下行流量使用量
  2. 系统记录并展示每个入站节点的总流量使用量

**Plans:** 3 plans

Plans:
- [x] 06-01-PLAN.md — Stats backend (proto, StatsClient, DB columns, ConfigGenerator v2ray_api, cron)
- [x] 06-02-PLAN.md — API traffic responses + sort + reset_traffic
- [x] 06-03-PLAN.md — Frontend InboundTable + UserTable traffic columns

---

### Phase 7: sing-box Core Management

**Goal**: 管理员可在面板中更新或回滚 sing-box 核心版本

**Depends on**: Phase 1

**Requirements**: COR-03, COR-04

**Success Criteria** (what must be TRUE):
  1. 管理员可在面板中将 sing-box 核心更新到最新版本
  2. 管理员可将 sing-box 核心回滚到上一个已安装版本

**Plans**: TBD (预计 2 plans)

Plans:
- [ ] 07-01: TBD
- [ ] 07-02: TBD

---

### Phase 8: Deployment & Production

**Goal**: 用户可通过 Docker、bash 脚本或单二进制三种方式部署面板，并支持 HTTPS

**Depends on**: Phase 1

**Requirements**: SYS-02, SYS-03, SYS-04, SYS-05

**Success Criteria** (what must be TRUE):
  1. 面板支持 HTTPS 访问
  2. 支持 Docker Compose 一条命令部署
  3. 支持 bash 安装脚本部署（curl | bash）
  4. 支持下载单二进制文件 + 配置文件运行

**Plans**: TBD (预计 4–5 plans)

Plans:
- [ ] 08-01: TBD
- [ ] 08-02: TBD
- [ ] 08-03: TBD

---

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 3/3 | ✓ Complete | 2026-02-11 |
| 2. Inbound Management | 6/6 | ✓ Complete | 2026-02-11 |
| 3. Certificate Management | 3/3 | ✓ Complete | 2026-02-12 |
| 4. User Management | 4/4 | ✓ Complete | 2026-02-12 |
| 5. Subscription System | 3/3 | ✓ Complete | 2026-02-12 |
| 6. Traffic Statistics | 3/3 | ✓ Complete | 2026-02-12 |
| 7. sing-box Core Management | 0/— | Not started | - |
| 8. Deployment & Production | 0/— | Not started | - |
