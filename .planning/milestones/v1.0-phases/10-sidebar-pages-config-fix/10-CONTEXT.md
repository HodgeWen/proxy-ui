# Phase 10: Sidebar Pages & Config Fix - Context

**Gathered:** 2026-02-19
**Status:** Ready for planning

<domain>
## Phase Boundary

启用侧边栏中被禁用的三个页面（订阅、流量、核心），将现有功能从嵌入位置迁移到独立页面，重构仪表盘为统计概览，并全面修复配置路径不一致问题。不增加新业务功能。

</domain>

<decisions>
## Implementation Decisions

### 订阅页面
- 页面定位由 Claude 决定（用户订阅列表或概览均可）
- 用户页的 UserSubscriptionModal 保留作为快捷入口，订阅页提供完整视图
- 列表信息精简：用户名、订阅链接、状态（活跃/过期/超限）
- 只读操作：查看订阅信息、复制链接、查看 QR 码（不含重置 token 等修改操作）

### 流量页面
- Tab 切换布局："按入站"和"按用户"两个 Tab，各自一个表格
- 顶部需要流量概览卡片（总上行/下行、活跃用户数等汇总数据）
- 入站页/用户页原有的流量列保留不变，流量页提供聚合视图
- 暂不需要图表（折线图/柱状图），表格展示即可，后续阶段再加

### 核心页面
- 完整迁移 Dashboard 中的核心管理功能（状态、版本、重启、更新、回滚、版本列表）
- 增强内容：配置文件查看、运行日志等（具体范围由 Claude 在研究阶段确定可行性）

### 仪表盘重构
- 移除现有的入站数/用户数占位卡片（硬编码 0）
- 改为统计概览页：展示入站数、用户数、总流量等实际数据（调 API 获取）
- 核心管理功能全部迁出到独立核心页面
- "退出登录"按钮从仪表盘右上角移到侧边栏底部（全局可见）

### 配置路径修复
- 全面清理所有配置路径相关代码，确保一致性
- 修复 `ApplyConfig()` 中 `NewProcessManager()` 不读取 panel config 的问题
- 修复 `NewProcessManager()` 和 `ConfigPathFromEnv()` 默认路径 `./config.json` 与实际 sing-box 配置路径 `sing-box.json` 不一致
- 统一消除遗留的 env-only ProcessManager 用法
- Docker 配置（Dockerfile/docker-compose）路径也需检查和修正
- 启动时验证配置路径有效性（文件存在、目录可写等），无效时报警

### Claude's Discretion
- 订阅页面的具体定位和布局设计
- 配置路径修复的具体实现方式（传递 config 参数 vs 删除遗留函数 vs 两者结合）
- 核心页面增强功能的具体范围（配置查看、日志等需研究可行性）
- 仪表盘统计卡片的具体数据和布局
- 各页面的 loading/error/empty 状态处理

</decisions>

<specifics>
## Specific Ideas

- 侧边栏当前已有 订阅/流量/核心 三个菜单项（带图标），只是 `disabled: true`，启用即可
- 路由文件 `web/src/routes.tsx` 需要新增对应的路由配置
- 当前 `ApplyConfig()` 在 `internal/core/config.go` 中使用 `NewProcessManager()` 是已确认的 bug
- `ConfigPathFromEnv()` 返回 `./config.json` 也是已确认的默认值错误

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 10-sidebar-pages-config-fix*
*Context gathered: 2026-02-19*
