# s-ui

## What This Is

s-ui 是一个基于 sing-box 核心的现代代理管理面板。它提供美观的暗色主题 Web 界面，用于管理 VLESS 和 Hysteria2 入站代理、用户、流量和订阅链接。面向开源社区，任何人都可以通过 Docker、脚本或单二进制文件一键部署到服务器。v1.0 已交付完整的入站/用户/证书/订阅/流量/核心管理功能。

## Core Value

提供一个**界面美观、体验流畅、部署简单**的 sing-box 管理面板——让代理服务器管理不再忍受丑陋的 UI。

## Requirements

### Validated

- ✓ 入站配置管理（VLESS/Hysteria2 弹框表单 CRUD，TLS，传输选项）— v1.0
- ✓ 用户管理（创建/编辑/删除，分配节点，流量限制/到期，批量操作）— v1.0
- ✓ 流量统计（gRPC V2Ray API 采集，按用户/入站展示）— v1.0
- ✓ 订阅链接生成（Base64/ClashMeta 双格式，QR 码）— v1.0
- ✓ TLS 证书管理（手动路径指定，关联入站，更新后自动配置联动）— v1.0
- ✓ 现代暗色主题 UI（shadcn/ui + Tailwind CSS）— v1.0
- ✓ 智能默认值（最佳实践预填配置）— v1.0
- ✓ 字段提示系统（info 图标 + 悬浮说明）— v1.0
- ✓ 纯中文界面（专业术语保留英文）— v1.0
- ✓ 三种部署方式（Docker / bash / 单二进制）— v1.0

### Active

- [ ] 一键 Let's Encrypt 证书申请（ACME 集成）
- [ ] 证书自动续期
- [ ] 在线客户端数实时显示
- [ ] 实时仪表盘/图表（流量趋势、用量排行）
- [ ] 周期性流量限制（日/周/月重置）
- [ ] 外部订阅链接合并
- [ ] 数据库备份/恢复

### Out of Scope

- 服务器系统状态监控（CPU/内存/带宽）— 不是核心功能，保持面板聚焦
- VLESS 和 Hysteria2 之外的协议 — v1 聚焦两个协议，后续可扩展
- 多语言国际化 — 仅支持中文，降低复杂度
- 移动端 App — Web 优先
- Telegram Bot — 与核心价值无关
- 多管理员角色 — v1 单管理员
- SaaS 功能（套餐/订单/支付）— 不同产品形态

## Context

**Current state:** 已发布 v1.0 MVP，~10,179 LOC (Go 4,094 + TypeScript 6,085)，208 个文件。
**Tech stack:** Go (Chi + GORM + glebarez/sqlite) + React (Vite + shadcn/ui + Tailwind v4 + TanStack Query)。
**Deployment:** Docker Compose、bash 安装脚本（systemd）、单二进制 + 配置文件。GitHub Actions CI/CD 自动构建 amd64/arm64。

**v1.0 Tech debt:**
- react-bits 已安装未使用（shadcn 过渡足够）
- Tag 默认值静态
- UUID 搜索未实现
- QR 码订阅链接级别（非逐节点）

## Constraints

- **Tech Stack**: Go 后端 + React 前端（shadcn/ui + Tailwind CSS）
- **依赖版本**: 所有依赖必须使用最新版本
- **前端包管理**: bun
- **UI**: 暗色主题为主
- **Dependencies**: sing-box 核心（外部依赖）
- **Deployment**: Docker / bash / 单二进制三种方式
- **Language**: 界面仅中文

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go 后端 | 与 sing-box 同语言，可编译为单二进制 | ✓ Good — CGO-free 跨平台构建成功 |
| React + shadcn/ui | 组件灵活可定制，Tailwind 适合暗色主题 | ✓ Good — 7 个页面组件体系清晰 |
| react-bits 微交互 | 增强界面交互体验 | ⚠️ Revisit — 已安装未使用，shadcn 过渡已足够 |
| bun 包管理 | 速度快，现代工具链 | ✓ Good |
| 所有依赖最新版本 | 硬约束 | ✓ Good |
| 仅 VLESS + Hysteria2 | v1 聚焦两个协议 | ✓ Good — 覆盖最主流需求 |
| 弹框表单模式 | 所有配置通过 Modal 完成 | ✓ Good — 单页体验流畅 |
| 仅中文 | 目标用户明确 | ✓ Good |
| ConfigGenerator 从 DB 派生 | 不直接编辑配置文件 | ✓ Good — 数据一致性保证 |
| CGO-free SQLite (glebarez) | 跨平台单二进制 | ✓ Good — Docker 和 CI 构建简化 |
| V2Ray API gRPC 流量采集 | 实时增量统计 | ✓ Good — 60s 间隔精度足够 |
| 公开订阅端点 | /sub/{token} 无认证 | ✓ Good — 客户端直接导入 |

---
*Last updated: 2026-02-19 after v1.0 milestone*
