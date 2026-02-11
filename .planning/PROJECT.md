# s-ui

## What This Is

s-ui 是一个基于 sing-box 核心的现代代理管理面板。它提供美观的暗色主题 Web 界面，用于管理 VLESS 和 Hysteria2 入站代理、用户、流量和订阅链接。面向开源社区，任何人都可以通过 Docker、脚本或单二进制文件一键部署到服务器。

## Core Value

提供一个**界面美观、体验流畅、部署简单**的 sing-box 管理面板——让代理服务器管理不再忍受丑陋的 UI。

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] 入站配置管理（通过弹框表单添加/编辑/删除 VLESS、Hysteria2 节点）
- [ ] 用户管理（创建用户、分配节点、流量限制）
- [ ] 流量统计（按用户、按节点的使用量统计）
- [ ] 订阅链接生成（生成可导入客户端的订阅链接）
- [ ] TLS 证书管理（自动申请、自动续期）
- [ ] 现代暗色主题 UI（Vercel/Linear 风格，shadcn/ui + react-bits 微交互）
- [ ] 智能默认值（最佳实践预填配置，最小化用户填写量）
- [ ] 字段提示系统（info 图标 + 悬浮说明，降低使用门槛）
- [ ] 纯中文界面（专业术语如 VLESS、TLS、Hysteria2 等保留英文）
- [ ] 三种部署方式（Docker Compose / bash 安装脚本 / 单二进制文件）

### Out of Scope

- 服务器系统状态监控（CPU/内存/带宽）— 不是核心功能，保持面板聚焦
- VLESS 和 Hysteria2 之外的协议 — v1 聚焦两个协议，后续可扩展
- 多语言国际化 — 仅支持中文，降低复杂度
- 移动端 App — Web 优先

## Context

**动机：** 现有的 sing-box / xray 面板（如 3x-ui）界面粗糙、体验差。社区需要一个现代化、好看、易用的管理面板。

**sing-box 核心：** sing-box 是一个通用代理平台，支持多种入站/出站协议。s-ui 通过 Web 界面管理 sing-box 的配置文件和运行状态，不修改 sing-box 本身。

**UX 设计原则：**
- 弹框表单驱动所有配置操作（不跳转页面）
- react-bits 库增强微交互体验（动画、过渡）
- 每个配置字段提供 info 图标悬浮提示
- 尽可能使用智能默认值，减少用户需要填写的内容

## Constraints

- **Tech Stack**: Go 后端（与 sing-box 同语言，便于集成和单二进制部署）+ React 前端（shadcn/ui + Tailwind CSS）
- **依赖版本**: **所有依赖必须使用最新版本**（硬约束，不接受过时版本）
- **前端包管理**: 使用 bun 进行依赖管理（非 npm/yarn/pnpm）
- **UI**: 暗色主题为主，现代设计风格（对标 Vercel/Linear）
- **Dependencies**: sing-box 核心（外部依赖，面板调用其 API 或管理其配置）
- **Deployment**: 必须支持 Docker Compose、bash 安装脚本、单二进制文件三种部署方式
- **Language**: 界面仅中文（专业术语保留英文原文）

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go 后端 | 与 sing-box 同语言，可编译为单二进制，部署简单 | — Pending |
| React + shadcn/ui | 组件灵活可定制，Tailwind 基础适合暗色主题定制 | — Pending |
| react-bits 微交互 | 增强界面交互体验，区别于现有面板的"工具感" | — Pending |
| bun 包管理 | 速度快，现代工具链 | — Pending |
| 所有依赖最新版本 | 硬约束，不使用过时依赖 | — Pending |
| 仅 VLESS + Hysteria2 | v1 聚焦最实用的两个协议，降低复杂度 | — Pending |
| 弹框表单模式 | 所有配置通过 Modal 完成，保持单页体验 | — Pending |
| 仅中文 | 目标用户明确，减少 i18n 成本 | — Pending |
| sing-box 版本管理 | 支持在面板中更新/回滚 sing-box 核心 | — Pending |

---
*Last updated: 2026-02-11 after initialization*
