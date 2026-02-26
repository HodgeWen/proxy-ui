import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Play, Square, RefreshCw, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import { SpotlightCard } from "@/components/ui/spotlight-card"
import { useState } from "react"
import { useCoreUpdateStream } from "@/hooks/use-core-update-stream"
import {
  ACTIONS_BY_STATE,
  type CoreState,
  getStateMeta,
  normalizeCoreState,
} from "@/lib/core-status"

type CoreLastError = {
  message: string
  occurredAt: string
  stage: string
  source: string
}

type CoreStatus = {
  state: CoreState
  actions: string[]
  lastError: CoreLastError | null
  running: boolean
  version: string
  binaryPath: string
  configPath: string
}

type Release = {
  tag: string
  version: string
  prerelease: boolean
}

type VersionsResponse = {
  releases: Release[]
}

type CoreAction = "start" | "stop" | "restart"
type LifecycleAction = "install" | "start" | "stop" | "restart" | "retry_start" | "view_logs"

const CORE_ACTION_ENDPOINTS: Record<CoreAction, string> = {
  start: "/api/core/start",
  stop: "/api/core/stop",
  restart: "/api/core/restart",
}

type CoreActionErrorPayload = {
  code?: string
  message?: string
  detail?: string
}

type CoreLogsResponse = {
  path: string
  count: number
  entries: string[]
}

type CoreLogsErrorPayload = {
  code?: string
  message?: string
  detail?: string
}

type CoreUpdateErrorPayload = {
  code?: string
  message?: string
  detail?: string
  error?: string
}

type CoreUpdateError = Error & {
  status?: number
  code?: string
}

function mapActionErrorMessage(action: CoreAction, error: CoreActionErrorPayload): string {
  switch (error.code) {
    case "CORE_NOT_INSTALLED":
      return "未检测到 sing-box 核心，请先下载安装。"
    case "CORE_ALREADY_RUNNING":
      return "核心已在运行，无需重复启动。"
    case "CORE_ALREADY_STOPPED":
      return "核心已停止。"
    case "CORE_START_FAILED":
      return error.detail
        ? `核心启动失败：${error.detail}`
        : "核心启动失败，请查看日志并重试。"
    case "CORE_CONFIG_NOT_FOUND":
      return "未找到核心配置文件，请先保存配置。"
    default: {
      const actionLabelMap: Record<CoreAction, string> = {
        start: "启动",
        stop: "停止",
        restart: "重启",
      }
      return error.message || `核心${actionLabelMap[action]}失败`
    }
  }
}

function mapLogErrorMessage(error: CoreLogsErrorPayload): string {
  switch (error.code) {
    case "CORE_LOG_NOT_FOUND":
      return "暂无日志文件，可先重试启动后再查看。"
    case "CORE_LOG_READ_FAILED":
      return error.detail ? `读取日志失败：${error.detail}` : "读取日志失败，请稍后重试。"
    case "CORE_INVALID_LOG_LINES":
      return "日志参数无效，请刷新页面后重试。"
    default:
      return error.message || "读取日志失败，请稍后重试。"
  }
}

async function fetchStatus(): Promise<CoreStatus> {
  const res = await fetch("/api/core/status", { credentials: "include" })
  if (!res.ok) throw new Error("获取状态失败")
  const data = (await res.json()) as Partial<CoreStatus> & { state?: string }
  const state = normalizeCoreState(data.state, data.running)
  return {
    state,
    actions: Array.isArray(data.actions) ? data.actions : [...ACTIONS_BY_STATE[state]],
    lastError: data.lastError ?? null,
    running: Boolean(data.running),
    version: data.version || "",
    binaryPath: data.binaryPath || "",
    configPath: data.configPath || "",
  }
}

async function fetchVersions(): Promise<VersionsResponse> {
  const res = await fetch("/api/core/versions", { credentials: "include" })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error((data as { error?: string }).error || "获取版本列表失败")
  }
  return res.json()
}

async function fetchConfigFile(): Promise<string> {
  const res = await fetch("/api/core/config-file", { credentials: "include" })
  if (res.status === 404) return ""
  if (!res.ok) throw new Error("获取配置文件失败")
  const data = await res.json()
  return JSON.stringify(data, null, 2)
}

async function executeCoreAction(action: CoreAction): Promise<void> {
  const res = await fetch(CORE_ACTION_ENDPOINTS[action], {
    method: "POST",
    credentials: "include",
  })
  if (!res.ok) {
    const payload = (await res.json().catch(() => ({}))) as CoreActionErrorPayload
    throw new Error(mapActionErrorMessage(action, payload))
  }
}

async function startCore(): Promise<void> {
  return executeCoreAction("start")
}

async function stopCore(): Promise<void> {
  return executeCoreAction("stop")
}

async function restartCore(): Promise<void> {
  return executeCoreAction("restart")
}

async function fetchCoreLogs(lines = 200): Promise<CoreLogsResponse> {
  const res = await fetch(`/api/core/logs?lines=${lines}`, { credentials: "include" })
  if (!res.ok) {
    const payload = (await res.json().catch(() => ({}))) as CoreLogsErrorPayload
    throw new Error(mapLogErrorMessage(payload))
  }
  const data = (await res.json()) as Partial<CoreLogsResponse>
  return {
    path: data.path || "",
    count: typeof data.count === "number" ? data.count : data.entries?.length || 0,
    entries: Array.isArray(data.entries) ? data.entries : [],
  }
}

async function updateCore(): Promise<void> {
  const res = await fetch("/api/core/update", {
    method: "POST",
    credentials: "include",
  })
  const data = (await res.json().catch(() => ({}))) as CoreUpdateErrorPayload
  if (!res.ok) {
    const message = data.message || data.error || "更新失败"
    const err = new Error(message) as CoreUpdateError
    err.status = res.status
    err.code = data.code
    throw err
  }
}

async function rollbackCore(): Promise<void> {
  const res = await fetch("/api/core/rollback", {
    method: "POST",
    credentials: "include",
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error((data as { error?: string }).error || "回滚失败")
  }
}

export function Core() {
  const queryClient = useQueryClient()
  const updateStream = useCoreUpdateStream()
  const [errorModalOpen, setErrorModalOpen] = useState(false)
  const [errorDetail, setErrorDetail] = useState("")
  const [updateConfirmOpen, setUpdateConfirmOpen] = useState(false)
  const [rollbackConfirmOpen, setRollbackConfirmOpen] = useState(false)
  const [versionsListOpen, setVersionsListOpen] = useState(false)
  const [logsOpen, setLogsOpen] = useState(false)
  const [logsData, setLogsData] = useState<CoreLogsResponse | null>(null)
  const [logsError, setLogsError] = useState("")

  const { data: status, isLoading } = useQuery({
    queryKey: ["core", "status"],
    queryFn: fetchStatus,
    refetchInterval: 5000,
  })

  const { data: versionsData } = useQuery({
    queryKey: ["core", "versions"],
    queryFn: fetchVersions,
  })

  const { data: configContent, isLoading: configLoading } = useQuery({
    queryKey: ["core", "config-file"],
    queryFn: fetchConfigFile,
  })

  const latestStable = versionsData?.releases?.find((r) => !r.prerelease)
  const latestOverall = versionsData?.releases?.[0]
  const latestVersion = latestStable?.version ?? latestOverall?.version
  const updateAvailable =
    !!status?.version &&
    !!latestStable &&
    status.version !== latestStable.version
  const stateMeta = status ? getStateMeta(status.state) : null
  const lifecycleActions = status
    ? ([...ACTIONS_BY_STATE[status.state]].filter(
        (action) => status.actions.length === 0 || status.actions.includes(action)
      ) as LifecycleAction[])
    : []

  const invalidateCoreStatus = () =>
    queryClient.invalidateQueries({ queryKey: ["core", "status"] })

  const handleCoreActionError = (err: Error) => {
    const msg = err.message
    toast.error(msg)
    if (msg.includes("：")) {
      setErrorDetail(msg)
      setErrorModalOpen(true)
    }
  }

  const renderLifecycleAction = (action: LifecycleAction) => {
    switch (action) {
      case "start":
        return (
          <Button
            key={action}
            size="sm"
            onClick={() => startMutation.mutate()}
            disabled={startMutation.isPending}
          >
            {startMutation.isPending ? (
              <>
                <Loader2 className="size-4 animate-spin" />
                启动中...
              </>
            ) : (
              <>
                <Play className="size-4" />
                启动
              </>
            )}
          </Button>
        )
      case "retry_start":
        return (
          <Button
            key={action}
            size="sm"
            onClick={() => retryStartMutation.mutate()}
            disabled={retryStartMutation.isPending}
          >
            {retryStartMutation.isPending ? (
              <>
                <Loader2 className="size-4 animate-spin" />
                重试中...
              </>
            ) : (
              <>
                <Play className="size-4" />
                重试启动
              </>
            )}
          </Button>
        )
      case "stop":
        return (
          <Button
            key={action}
            size="sm"
            variant="secondary"
            onClick={() => stopMutation.mutate()}
            disabled={stopMutation.isPending}
          >
            {stopMutation.isPending ? (
              <>
                <Loader2 className="size-4 animate-spin" />
                停止中...
              </>
            ) : (
              <>
                <Square className="size-4" />
                停止
              </>
            )}
          </Button>
        )
      case "restart":
        return (
          <Button
            key={action}
            size="sm"
            onClick={() => restartMutation.mutate()}
            disabled={restartMutation.isPending}
          >
            {restartMutation.isPending ? (
              <>
                <RefreshCw className="size-4 animate-spin" />
                重启中...
              </>
            ) : (
              <>
                <RefreshCw className="size-4" />
                重启
              </>
            )}
          </Button>
        )
      case "install":
        return (
          <Button key={action} size="sm" asChild>
            <a
              href="https://github.com/SagerNet/sing-box/releases"
              target="_blank"
              rel="noreferrer"
            >
              下载并安装 sing-box
            </a>
          </Button>
        )
      case "view_logs":
        return (
          <Button
            key={action}
            size="sm"
            variant="outline"
            onClick={() => {
              setLogsOpen(true)
              setLogsError("")
              setLogsData(null)
              logsMutation.mutate()
            }}
          >
            查看日志
          </Button>
        )
      default:
        return null
    }
  }

  const startMutation = useMutation({
    mutationFn: startCore,
    onSuccess: () => {
      void invalidateCoreStatus()
      toast.success("sing-box 已启动")
    },
    onError: (err: Error) => {
      handleCoreActionError(err)
    },
  })

  const stopMutation = useMutation({
    mutationFn: stopCore,
    onSuccess: () => {
      void invalidateCoreStatus()
      toast.success("sing-box 已停止")
    },
    onError: (err: Error) => {
      handleCoreActionError(err)
    },
  })

  const restartMutation = useMutation({
    mutationFn: restartCore,
    onSuccess: () => {
      void invalidateCoreStatus()
      toast.success("sing-box 已重启")
    },
    onError: (err: Error) => {
      handleCoreActionError(err)
    },
  })

  const retryStartMutation = useMutation({
    mutationFn: startCore,
    onSuccess: () => {
      void invalidateCoreStatus()
      toast.success("重试启动成功")
    },
    onError: (err: Error) => {
      handleCoreActionError(err)
    },
  })

  const updateMutation = useMutation({
    mutationFn: updateCore,
    onSuccess: () => {
      setUpdateConfirmOpen(false)
      void queryClient.invalidateQueries({ queryKey: ["core", "status"] })
      void queryClient.invalidateQueries({ queryKey: ["core", "versions"] })
    },
    onError: (err: Error) => {
      setUpdateConfirmOpen(false)
      const updateErr = err as CoreUpdateError
      if (updateErr.status === 409 || updateErr.code === "CORE_UPDATE_CONFLICT") {
        void queryClient.invalidateQueries({ queryKey: ["core", "status"] })
        return
      }

      const msg = updateErr.message
      toast.error(msg)
      if (msg.includes("请设置") || msg.includes("SINGBOX_BINARY_PATH")) {
        toast.error("核心更新需要设置 SINGBOX_BINARY_PATH", { duration: 6000 })
      }
    },
  })

  const rollbackMutation = useMutation({
    mutationFn: rollbackCore,
    onSuccess: () => {
      setRollbackConfirmOpen(false)
      queryClient.invalidateQueries({ queryKey: ["core", "status"] })
      queryClient.invalidateQueries({ queryKey: ["core", "versions"] })
      toast.success("回滚成功")
    },
    onError: (err: Error) => {
      toast.error(err.message)
    },
  })

  const logsMutation = useMutation({
    mutationFn: () => fetchCoreLogs(200),
    onSuccess: (data) => {
      setLogsData(data)
    },
    onError: (err: Error) => {
      setLogsError(err.message)
    },
  })

  const showUpdateProgress = updateMutation.isPending || updateStream.isUpdating
  const updatePercent = updateStream.isUpdating ? updateStream.percent : 0
  const updateActionDisabled = updateMutation.isPending || updateStream.isUpdating

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-bold">核心管理</h1>

      {/* Status & Version */}
      <div className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none">
      <SpotlightCard>
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <CardTitle>sing-box 状态</CardTitle>
            {updateAvailable && (
              <Badge variant="secondary" className="text-xs">
                有新版本
              </Badge>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="h-16 flex items-center text-muted-foreground">
              加载中...
            </div>
          ) : status ? (
            <div className="space-y-4">
              <div className="flex items-center gap-2">
                <span
                  className={`inline-block w-2.5 h-2.5 rounded-full ${
                    stateMeta?.dotClassName || "bg-muted-foreground"
                  }`}
                />
                <span className="text-lg font-medium">
                  {stateMeta?.label || "状态未知"}
                </span>
              </div>
              <p className="text-sm text-muted-foreground">
                {stateMeta?.description || "当前状态暂不可用。"}
              </p>
              {status.state === "running" && (
                <p className="text-sm">核心运行正常，可执行停止或重启操作。</p>
              )}
              {status.state === "stopped" && (
                <p className="text-sm">核心已停止，点击“启动”即可恢复服务。</p>
              )}
              {status.state === "not_installed" && (
                <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm space-y-2">
                  <p className="font-medium text-amber-700 dark:text-amber-300">
                    检测到核心未安装，请先下载并安装 sing-box 二进制。
                  </p>
                  <p className="text-muted-foreground">
                    建议安装到当前路径：<span className="font-mono break-all">{status.binaryPath}</span>
                  </p>
                </div>
              )}
              {status.state === "error" && (
                <div className="rounded-lg border border-red-500/40 bg-red-500/10 p-3 text-sm space-y-1">
                  <p className="font-medium text-red-700 dark:text-red-300">
                    最近一次启动异常，可重试启动或查看日志定位问题。
                  </p>
                  {status.lastError?.message && (
                    <p className="text-muted-foreground break-all">
                      错误信息：{status.lastError.message}
                    </p>
                  )}
                </div>
              )}
              {showUpdateProgress && (
                <div className="space-y-2 rounded-lg border bg-muted/20 p-3">
                  <p className="text-sm font-medium">更新中 {updatePercent}%</p>
                  <Progress value={updatePercent} />
                </div>
              )}
              <div className="grid gap-2 sm:grid-cols-2 text-sm">
                {status.version && (
                  <div>
                    <span className="text-muted-foreground">当前版本：</span>
                    <span className="font-mono">{status.version}</span>
                  </div>
                )}
                {latestVersion && (
                  <div>
                    <span className="text-muted-foreground">最新版本：</span>
                    <span className="font-mono">{latestVersion}</span>
                  </div>
                )}
                {status.binaryPath && (
                  <div>
                    <span className="text-muted-foreground">二进制路径：</span>
                    <span className="font-mono text-xs break-all">{status.binaryPath}</span>
                  </div>
                )}
                {status.configPath && (
                  <div>
                    <span className="text-muted-foreground">配置路径：</span>
                    <span className="font-mono text-xs break-all">{status.configPath}</span>
                  </div>
                )}
              </div>
              <Button
                variant="link"
                size="sm"
                className="h-auto p-0 text-muted-foreground"
                onClick={() => setVersionsListOpen(true)}
              >
                查看所有版本
              </Button>
              <div className="flex flex-wrap gap-2">
                {lifecycleActions.map((action) => renderLifecycleAction(action))}
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => setUpdateConfirmOpen(true)}
                  disabled={updateActionDisabled}
                >
                  {updateMutation.isPending ? (
                    <>
                      <Loader2 className="size-4 animate-spin" />
                      更新中...
                    </>
                  ) : updateStream.isUpdating ? (
                    "更新中..."
                  ) : (
                    "更新"
                  )}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setRollbackConfirmOpen(true)}
                  disabled={rollbackMutation.isPending}
                >
                  {rollbackMutation.isPending ? (
                    <>
                      <Loader2 className="size-4 animate-spin" />
                      回滚中...
                    </>
                  ) : (
                    "回滚"
                  )}
                </Button>
              </div>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">无法获取状态</p>
          )}
        </CardContent>
      </Card>
      </SpotlightCard>
      </div>

      {/* Config File Viewer */}
      <div className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none" style={{ animationDelay: '75ms' }}>
      <Card>
        <CardHeader>
          <CardTitle>sing-box 配置文件</CardTitle>
          <CardDescription>当前生成的 sing-box 配置（只读）</CardDescription>
        </CardHeader>
        <CardContent>
          {configLoading ? (
            <div className="text-muted-foreground text-sm">加载中...</div>
          ) : configContent ? (
            <pre className="bg-muted rounded-lg p-4 text-sm font-mono overflow-auto max-h-[60vh] whitespace-pre-wrap">
              {configContent}
            </pre>
          ) : (
            <p className="text-sm text-muted-foreground">
              暂无配置文件 — 添加入站后自动生成
            </p>
          )}
        </CardContent>
      </Card>
      </div>

      {/* Update Confirm Dialog */}
      <Dialog open={updateConfirmOpen} onOpenChange={setUpdateConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认更新</DialogTitle>
            <DialogDescription>
              确定要将 sing-box 更新到最新版本吗？更新过程中服务将短暂停止。
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-2 mt-4">
            <Button
              variant="outline"
              onClick={() => setUpdateConfirmOpen(false)}
              disabled={updateMutation.isPending}
            >
              取消
            </Button>
            <Button
              onClick={() => updateMutation.mutate()}
              disabled={updateActionDisabled}
            >
              {updateMutation.isPending ? (
                <>
                  <Loader2 className="size-4 animate-spin" />
                  更新中...
                </>
              ) : updateStream.isUpdating ? (
                "更新中..."
              ) : (
                "确定"
              )}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Rollback Confirm Dialog */}
      <Dialog open={rollbackConfirmOpen} onOpenChange={setRollbackConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认回滚</DialogTitle>
            <DialogDescription>确定要回滚到上一版本吗？</DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-2 mt-4">
            <Button
              variant="outline"
              onClick={() => setRollbackConfirmOpen(false)}
              disabled={rollbackMutation.isPending}
            >
              取消
            </Button>
            <Button
              onClick={() => rollbackMutation.mutate()}
              disabled={rollbackMutation.isPending}
            >
              {rollbackMutation.isPending ? (
                <>
                  <Loader2 className="size-4 animate-spin" />
                  回滚中...
                </>
              ) : (
                "确定"
              )}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Versions List Dialog */}
      <Dialog open={versionsListOpen} onOpenChange={setVersionsListOpen}>
        <DialogContent className="max-w-md max-h-[80vh] flex flex-col">
          <DialogHeader>
            <DialogTitle>所有版本</DialogTitle>
            <DialogDescription>
              来自 GitHub Releases，stable 为正式版，pre-release 为预发布版
            </DialogDescription>
          </DialogHeader>
          <div className="overflow-y-auto max-h-[60vh] -mx-6 px-6">
            <ul className="space-y-2">
              {versionsData?.releases?.map((r) => (
                <li
                  key={r.tag}
                  className="flex items-center justify-between gap-2 py-1.5 border-b border-border last:border-0"
                >
                  <span className="font-mono text-sm">{r.tag}</span>
                  <Badge
                    variant={r.prerelease ? "outline" : "default"}
                    className={
                      r.prerelease
                        ? "bg-amber-500/20 text-amber-700 dark:text-amber-400 border-amber-500/50"
                        : "bg-emerald-600 text-white border-transparent"
                    }
                  >
                    {r.prerelease ? "pre-release" : "stable"}
                  </Badge>
                </li>
              ))}
            </ul>
          </div>
        </DialogContent>
      </Dialog>

      {/* Error Detail Dialog */}
      <Dialog open={errorModalOpen} onOpenChange={setErrorModalOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-hidden flex flex-col">
          <DialogHeader>
            <DialogTitle>核心操作失败</DialogTitle>
            <DialogDescription>错误详情如下：</DialogDescription>
          </DialogHeader>
          <pre className="mt-2 p-4 rounded-lg bg-muted text-sm overflow-auto max-h-[60vh] font-mono whitespace-pre-wrap">
            {errorDetail}
          </pre>
        </DialogContent>
      </Dialog>

      {/* Core Logs Dialog */}
      <Dialog
        open={logsOpen}
        onOpenChange={(open) => {
          setLogsOpen(open)
          if (!open) {
            setLogsError("")
            setLogsData(null)
            logsMutation.reset()
          }
        }}
      >
        <DialogContent className="max-w-3xl max-h-[80vh] overflow-hidden flex flex-col">
          <DialogHeader>
            <DialogTitle>核心日志</DialogTitle>
            <DialogDescription>
              {logsData?.path ? `日志路径：${logsData.path}` : "展示最近 200 行日志"}
            </DialogDescription>
          </DialogHeader>
          {logsMutation.isPending ? (
            <div className="h-40 flex items-center justify-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              正在读取日志...
            </div>
          ) : logsError ? (
            <div className="h-40 flex items-center justify-center text-sm text-red-600 dark:text-red-400">
              {logsError}
            </div>
          ) : logsData && logsData.entries.length > 0 ? (
            <pre className="mt-2 p-4 rounded-lg bg-muted text-sm overflow-auto max-h-[60vh] font-mono whitespace-pre-wrap">
              {logsData.entries.join("\n")}
            </pre>
          ) : (
            <div className="h-40 flex items-center justify-center text-sm text-muted-foreground">
              暂无日志内容
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
