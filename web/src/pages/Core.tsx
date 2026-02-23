import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { RefreshCw, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
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

type CoreStatus = {
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

async function fetchStatus(): Promise<CoreStatus> {
  const res = await fetch("/api/core/status", { credentials: "include" })
  if (!res.ok) throw new Error("获取状态失败")
  return res.json()
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

async function restartCore(): Promise<void> {
  const res = await fetch("/api/core/restart", {
    method: "POST",
    credentials: "include",
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || "重启失败")
  }
}

async function updateCore(): Promise<void> {
  const res = await fetch("/api/core/update", {
    method: "POST",
    credentials: "include",
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error((data as { error?: string }).error || "更新失败")
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
  const [errorModalOpen, setErrorModalOpen] = useState(false)
  const [errorDetail, setErrorDetail] = useState("")
  const [updateConfirmOpen, setUpdateConfirmOpen] = useState(false)
  const [rollbackConfirmOpen, setRollbackConfirmOpen] = useState(false)
  const [versionsListOpen, setVersionsListOpen] = useState(false)

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

  const restartMutation = useMutation({
    mutationFn: restartCore,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["core", "status"] })
      toast.success("sing-box 已重启")
    },
    onError: (err: Error) => {
      const msg = err.message
      toast.error(msg)
      if (msg.includes("\n") || msg.includes("check") || msg.length > 100) {
        setErrorDetail(msg)
        setErrorModalOpen(true)
      }
    },
  })

  const updateMutation = useMutation({
    mutationFn: updateCore,
    onSuccess: () => {
      setUpdateConfirmOpen(false)
      queryClient.invalidateQueries({ queryKey: ["core", "status"] })
      queryClient.invalidateQueries({ queryKey: ["core", "versions"] })
      toast.success("更新成功")
    },
    onError: (err: Error) => {
      const msg = err.message
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
                    status.running ? "bg-emerald-500" : "bg-muted-foreground"
                  }`}
                />
                <span className="text-lg font-medium">
                  {status.running ? "运行中" : "已停止"}
                </span>
              </div>
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
                <Button
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
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => setUpdateConfirmOpen(true)}
                  disabled={updateMutation.isPending}
                >
                  {updateMutation.isPending ? (
                    <>
                      <Loader2 className="size-4 animate-spin" />
                      更新中...
                    </>
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
              disabled={updateMutation.isPending}
            >
              {updateMutation.isPending ? (
                <>
                  <Loader2 className="size-4 animate-spin" />
                  更新中...
                </>
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
            <DialogTitle>配置验证失败</DialogTitle>
            <DialogDescription>sing-box check 输出如下：</DialogDescription>
          </DialogHeader>
          <pre className="mt-2 p-4 rounded-lg bg-muted text-sm overflow-auto max-h-[60vh] font-mono whitespace-pre-wrap">
            {errorDetail}
          </pre>
        </DialogContent>
      </Dialog>
    </div>
  )
}
