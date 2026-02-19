# Phase 6: Traffic Statistics - Context

**Gathered:** 2026-02-12
**Status:** Ready for planning

<domain>
## Phase Boundary

面板展示每个用户和每个入站的流量使用量。系统记录并展示每个用户的上行/下行流量，以及每个入站节点的总流量。流量趋势图表、历史查询不在本阶段范围内。

</domain>

<decisions>
## Implementation Decisions

### 入站流量展示
- 流量数据直接内嵌在现有 Inbounds 列表表格中，增加上行/下行流量列
- 每个入站仅展示上行总量 + 下行总量，纯数字展示，无图表
- 入站列表支持按流量升降序排序

### 用户流量展示
- 用户列表表格中增加流量列（上行/下行），与入站保持一致的展示风格
- 流量超限与状态提示由 Claude 决定合理方案

### Claude's Discretion
- 数据更新体验：自动轮询 vs 手动刷新、刷新频率
- 后端采集频率与实时性策略（根据 sing-box API 能力决定）
- 用户流量重置与入站统计的联动关系
- 流量数字的格式化（B/KB/MB/GB 自动换算）
- 用户接近或超过流量限制时的界面提示方式

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

*Phase: 06-traffic-statistics*
*Context gathered: 2026-02-12*
