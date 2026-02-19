---
status: complete
phase: 10-sidebar-pages-config-fix
source: 10-01-PLAN.md, 10-02-PLAN.md, 10-03-PLAN.md
started: 2026-02-19T18:40:00Z
updated: 2026-02-19T18:45:00Z
---

## Current Test

[testing complete]

## Tests

### 1. 侧边栏导航项全部启用
expected: 侧边栏显示 7 个导航项（仪表盘、入站、用户、订阅、证书、流量、核心），全部可点击，没有灰色/禁用状态。
result: pass

### 2. 侧边栏退出登录按钮
expected: 侧边栏底部显示"退出登录"按钮，带 LogOut 图标，点击后跳转到登录页。
result: pass

### 3. 仪表盘统计概览
expected: 导航到 / 仪表盘页面，显示统计卡片：入站数、用户数（含活跃数）、总流量（上行/下行，formatBytes 格式）。无核心管理 UI（无重启/更新/回滚按钮）。
result: pass

### 4. 订阅页面
expected: 点击侧边栏"订阅"导航到 /subscriptions。页面显示用户订阅列表，包含：用户名、订阅链接（截断+复制按钮）、状态徽章（活跃/过期/超限/禁用）、QR 按钮。点击复制按钮可复制链接，点击 QR 按钮弹出二维码。
result: pass

### 5. 流量页面
expected: 点击侧边栏"流量"导航到 /traffic。顶部显示概览卡片（总上行/下行、活跃用户、入站数）。下方有两个 Tab（按入站 / 按用户），切换后各自显示对应数据表格。
result: pass

### 6. 核心管理页面
expected: 点击侧边栏"核心"导航到 /core。显示 sing-box 状态（运行/停止指示器）、当前版本、最新版本。有重启/更新/回滚按钮。点击更新/回滚弹出确认对话框。
result: pass

### 7. 核心配置文件查看
expected: /core 页面下方有"sing-box 配置文件"卡片。如果已生成配置，显示格式化的 JSON；如果没有配置文件，显示"暂无配置文件"空状态提示。
result: pass

## Summary

total: 7
passed: 7
issues: 0
pending: 0
skipped: 0

## Gaps

[none]
