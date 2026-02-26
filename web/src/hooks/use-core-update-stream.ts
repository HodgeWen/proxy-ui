import { useEffect, useState } from "react"

type CoreUpdateStreamPayload = {
  inProgress?: boolean
  percent?: number
  updatedAt?: string
  error?: string
}

export type CoreUpdateStreamState = {
  isUpdating: boolean
  percent: number
  updatedAt: string | null
  error: string | null
}

const INITIAL_STATE: CoreUpdateStreamState = {
  isUpdating: false,
  percent: 0,
  updatedAt: null,
  error: null,
}

function normalizePercent(value: number): number {
  if (!Number.isFinite(value)) return 0
  if (value < 0) return 0
  if (value > 100) return 100
  return Math.round(value)
}

function mergeSnapshot(
  prev: CoreUpdateStreamState,
  payload: CoreUpdateStreamPayload,
): CoreUpdateStreamState {
  const next: CoreUpdateStreamState = { ...prev }

  if (typeof payload.inProgress === "boolean") {
    next.isUpdating = payload.inProgress
    if (payload.inProgress) {
      // A new stream cycle starts from server snapshot/incremental events.
      next.error = null
    }
  }

  if (typeof payload.percent === "number") {
    next.percent = normalizePercent(payload.percent)
  }

  if (typeof payload.updatedAt === "string" && payload.updatedAt.length > 0) {
    next.updatedAt = payload.updatedAt
  }

  if (typeof payload.error === "string") {
    next.error = payload.error || null
  }

  return next
}

export function useCoreUpdateStream(): CoreUpdateStreamState {
  const [state, setState] = useState<CoreUpdateStreamState>(INITIAL_STATE)

  useEffect(() => {
    const source = new EventSource("/api/core/update/stream")

    source.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data) as CoreUpdateStreamPayload
        setState((prev) => mergeSnapshot(prev, payload))
      } catch {
        setState((prev) => ({
          ...prev,
          error: "更新进度消息解析失败",
        }))
      }
    }

    source.onerror = () => {
      // EventSource will auto-reconnect. Keep UI stable and avoid hard-fail state.
      setState((prev) => ({
        ...prev,
        error: prev.error ?? "更新进度连接中断，正在重连",
      }))
    }

    return () => {
      source.close()
    }
  }, [])

  return state
}
