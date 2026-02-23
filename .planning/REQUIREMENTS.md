# Requirements: s-ui

**Defined:** 2026-02-19
**Core Value:** 提供一个界面美观、体验流畅、部署简单的 sing-box 管理面板

## v1.1 Requirements

Requirements for v1.1 UI与核心优化。Each maps to roadmap phases.

### UI 动效

- [x] **UIANM-01**: 仪表盘统计数字使用 CountUp 动画展示（react-bits CountUp 组件）
- [x] **UIANM-02**: 所有页面卡片使用 AnimatedContent 入场动效（react-bits AnimatedContent + tw-animate-css）
- [x] **UIANM-03**: 核心状态卡片使用 SpotlightCard 聚光悬停效果（react-bits SpotlightCard）
- [x] **UIANM-04**: 全局统一组件悬停交互效果（按钮、卡片、表格行等一致的 hover 过渡）

### UI 一致性

- [ ] **UICON-01**: 全局间距和色彩一致性审查与修正（页面间距统一、色彩变量规范、暗色主题一致性）
- [ ] **UICON-02**: 核心状态指示器动效（运行中绿色脉冲、已停止灰色、未安装黄色提示、异常红色闪烁）

### 核心进程管理

- [ ] **CORE-01**: 拆分现有"重启"为独立的启动/停止/重启按钮，根据当前状态显示可用操作
- [ ] **CORE-02**: 核心二进制不存在时面板启动不自动启动核心，UI 提示用户下载安装
- [ ] **CORE-03**: 启动失败时不显示语义错误的"重启"按钮，改为显示上下文正确的操作（如"重试启动"或"查看日志"）
- [ ] **CORE-04**: 增强状态 API，区分未安装/已停止/运行中/异常四种状态，前端根据状态渲染不同 UI

### 核心更新

- [ ] **UPDT-01**: SSE 实时推送核心更新下载进度，前端使用 Progress 组件展示进度条
- [ ] **UPDT-02**: 并发更新保护（sync.Mutex + TryLock），防止多次点击重复触发更新，返回 HTTP 409

## Future Requirements

Deferred to future release. Tracked but not in current roadmap.

### 证书增强

- **CERT-01**: 一键 Let's Encrypt 证书申请（ACME 集成）
- **CERT-02**: 证书自动续期

### 数据与监控

- **DATA-01**: 在线客户端数实时显示
- **DATA-02**: 实时仪表盘/图表（流量趋势、用量排行）
- **DATA-03**: 周期性流量限制（日/周/月重置）

### 扩展功能

- **EXT-01**: 外部订阅链接合并
- **EXT-02**: 数据库备份/恢复

### UI 增强（已评估，延后）

- **UIFUT-01**: Skeleton 加载态替换"加载中..."文字
- **UIFUT-02**: 统一删除确认对话框（替换 window.confirm）
- **UIFUT-03**: 仪表盘卡片交错入场动画
- **UIFUT-04**: 多步骤 Stepper 可视化更新流程
- **UIFUT-05**: 更新失败详细错误展示与恢复操作

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| 路由页面过渡动画 | 复杂度高，与 React Router 深度耦合，收益不明显 |
| 实时日志流 | 需要大量后端基础设施，属于新功能而非优化 |
| 更新到指定版本 | 需要版本选择 UI 和 API 参数变更，范围过大 |
| Framer Motion / motion 库 | 50KB+ 包体积，react-bits + tw-animate-css 已足够 |
| 自动更新核心 | 运维关注点，需要配置选项和定时任务，属于不同功能域 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| UIANM-01 | Phase 11 | Complete |
| UIANM-02 | Phase 11 | Complete |
| UIANM-03 | Phase 11 | Complete |
| UIANM-04 | Phase 11 | Complete |
| CORE-01 | Phase 12 | Pending |
| CORE-02 | Phase 12 | Pending |
| CORE-03 | Phase 12 | Pending |
| CORE-04 | Phase 12 | Pending |
| UPDT-01 | Phase 13 | Pending |
| UPDT-02 | Phase 13 | Pending |
| UICON-01 | Phase 14 | Pending |
| UICON-02 | Phase 14 | Pending |

**Coverage:**
- v1.1 requirements: 12 total
- Mapped to phases: 12 ✓
- Unmapped: 0

---
*Requirements defined: 2026-02-19*
*Last updated: 2026-02-19 after roadmap creation*
