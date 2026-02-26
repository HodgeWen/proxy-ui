# Phase 13: Core Update Progress - Context

**Gathered:** 2026-02-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Provide real-time core update progress during download via SSE and prevent duplicate update triggers while an update is already in progress. This phase focuses on progress visibility and duplicate-trigger protection only.

</domain>

<decisions>
## Implementation Decisions

### 进度信息呈现
- 主文案偏好：简洁样式，使用“更新中 xx%”风格
- 进度条外显示百分比数字（固定可见）
- 不需要额外阶段提示（如准备中/下载中/完成），避免信息冗余
- 信息密度偏低，只保留关键进度信息

### 页面切换与返回
- 页面刷新后应尽量恢复到当前进度，保证连续感
- 若更新已完成，返回页面时直接回归普通状态，不额外展示成功提示
- 多标签页场景下，非发起标签页的更新操作偏好“静默禁用按钮”

### Claude's Discretion
- 用户离开“核心管理”页后再返回时，是否展示连续百分比细节由 Claude 决定（可在“连续感”与“界面简洁”之间权衡）
- 当按钮已被禁用但后端仍返回 409 时，是否补充轻量提示由 Claude 决定

</decisions>

<specifics>
## Specific Ideas

- 整体倾向：极简、低密度，但关键百分比必须清晰可见
- 进度反馈优先“连续感”，尤其是刷新后返回场景

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 13-core-update-progress*
*Context gathered: 2026-02-26*
