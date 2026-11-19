export const CORE_STATES = [
  "not_installed",
  "stopped",
  "running",
  "error",
] as const

export type CoreState = (typeof CORE_STATES)[number]

export const ACTIONS_BY_STATE: Record<CoreState, readonly string[]> = {
  running: ["stop", "restart"],
  stopped: ["start"],
  not_installed: ["install"],
  error: ["retry_start", "view_logs"],
}

type CoreStateMeta = {
  label: string
  description: string
  dotClassName: string
}

const STATE_META: Record<CoreState, CoreStateMeta> = {
  running: {
    label: "运行中",
    description: "sing-box 正在运行，可停止或重启。",
    dotClassName: "bg-emerald-500 animate-pulse motion-reduce:animate-none",
  },
  stopped: {
    label: "已停止",
    description: "sing-box 当前未运行，可手动启动。",
    dotClassName: "bg-muted-foreground",
  },
  not_installed: {
    label: "未安装",
    description: "未检测到 sing-box 二进制，请先下载安装。",
    dotClassName: "bg-amber-500",
  },
  error: {
    label: "启动异常",
    description: "最近一次启动失败，可重试启动或查看日志。",
    dotClassName: "bg-red-500 animate-[status-blink_1s_steps(2,end)_infinite] motion-reduce:animate-none",
  },
}

export function isCoreState(value: string): value is CoreState {
  return CORE_STATES.includes(value as CoreState)
}

export function normalizeCoreState(value?: string, running?: boolean): CoreState {
  if (value && isCoreState(value)) {
    return value
  }
  return running ? "running" : "stopped"
}

export function getStateMeta(state: CoreState): CoreStateMeta {
  return STATE_META[state]
}
