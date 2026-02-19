# Phase 7: sing-box Core Management - Context

**Gathered:** 2026-02-12
**Status:** Ready for planning

<domain>
## Phase Boundary

管理员可在面板中更新或回滚 sing-box 核心版本。从 GitHub Releases 获取版本信息，一键更新到最新版，更新前自动备份当前二进制以支持回滚。不包括自动更新调度或配置迁移。

</domain>

<decisions>
## Implementation Decisions

### 更新流程
- 从 sing-box 官方 GitHub Releases API 获取可用版本列表
- 一键更新到最新版本（无需从列表中手动选择特定版本）
- 展示所有版本（包括预发布版），但清晰标记 stable / pre-release
- 服务端自动检测 OS/ARCH，下载对应平台的二进制文件

### 版本管理界面
- 嵌入仪表盘页面，作为 Dashboard 的一个板块展示
- 展示当前安装版本 + 最新可用版本
- 点击更新按钮后弹出确认对话框，确认后执行
- Sidebar 或仪表盘上显示新版本可用的徽标提示

### Claude's Discretion
- 回滚机制：保留几个旧版本、回滚触发方式、自动 vs 手动回滚
- 更新安全：更新过程中 sing-box 服务的处理流程（停止→替换→启动）
- 进度展示：步骤进度 vs spinner
- 失败处理：自动回滚 + 通知 vs 仅通知
- 更新后验证：是否检查新版本启动成功和配置有效

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 07-core-management*
*Context gathered: 2026-02-12*
