import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { RefreshCw, Radio, Users, Loader2 } from "lucide-react"
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
import { useState } from "react"

type CoreStatus = {
  running: boolean
  version: string
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
    const msg = (data as { error?: string }).error || "更新失败"
    throw new Error(msg)
  }
}

async function rollbackCore(): Promise<void> {
  const res = await fetch("/api/core/rollback", {
    method: "POST",
    credentials: "include",
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const msg = (data as { error?: string }).error || "回滚失败"
    throw new Error(msg)
  }
}

export function Dashboard() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [errorModalOpen, setErrorModalOpen] = useState(false)
  const [errorDetail, setErrorDetail] = useState("")
  const [updateConfirmOpen, setUpdateConfirmOpen] = useState(false)
  const [rollbackConfirmOpen, setRollbackConfirmOpen] = useState(false)

  const { data: status, isLoading } = useQuery({
    queryKey: ["core", "status"],
    queryFn: fetchStatus,
    refetchInterval: 5000,
  })

  const { data: versionsData } = useQuery({
    queryKey: ["core", "versions"],
    queryFn: fetchVersions,
  })

  const latestStable = versionsData?.releases?.find((r) => !r.prerelease)
  const latestOverall = versionsData?.releases?.[0]
  const latestVersion = latestStable?.version ?? latestOverall?.version

  const restartMutation = useMutation({
    mutationFn: restartCore,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["core", "status"] })
      toast.success("sing-box 已重启")
    },
    onError: (err: Error) => {
      const msg = err.message
      toast.error(msg)
      // If error looks like check output (multi-line, contains "sing-box" or "error")
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

  const handleLogout = async () => {
    await fetch("/api/logout", { method: "POST", credentials: "include" })
    navigate("/login", { replace: true })
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold">仪表盘</h1>
        <Button variant="outline" size="sm" onClick={handleLogout}>
          退出登录
        </Button>
      </div>

      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
        {/* sing-box 状态与版本 */}
        <Card className="transition-shadow hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              sing-box 状态与版本
            </CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="h-16 flex items-center text-muted-foreground">
                加载中...
              </div>
            ) : status ? (
              <div className="space-y-2">
                <div className="flex items-center gap-2">
                  <span
                    className={`inline-block w-2 h-2 rounded-full ${
                      status.running ? "bg-emerald-500" : "bg-muted-foreground"
                    }`}
                  />
                  <span className="text-sm">
                    {status.running ? "运行中" : "已停止"}
                  </span>
                </div>
                <div className="text-sm space-y-0.5">
                  {status.version && (
                    <p className="text-muted-foreground">
                      当前版本: {status.version}
                    </p>
                  )}
                  {latestVersion && (
                    <p className="text-muted-foreground">
                      最新版本: {latestVersion}
                    </p>
                  )}
                </div>
                <div className="flex flex-wrap gap-2 mt-2">
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

        {/* 统计概览占位 */}
        <Card className="transition-shadow hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">入站数</CardTitle>
            <Radio className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">0</div>
            <CardDescription>Phase 1 占位</CardDescription>
          </CardContent>
        </Card>

        <Card className="transition-shadow hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">用户数</CardTitle>
            <Users className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">0</div>
            <CardDescription>Phase 1 占位</CardDescription>
          </CardContent>
        </Card>
      </div>

      {/* 更新确认 */}
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

      {/* 回滚确认 */}
      <Dialog open={rollbackConfirmOpen} onOpenChange={setRollbackConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认回滚</DialogTitle>
            <DialogDescription>
              确定要回滚到上一版本吗？
            </DialogDescription>
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

      {/* 错误详情 Modal - check 失败时显示 */}
      <Dialog open={errorModalOpen} onOpenChange={setErrorModalOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-hidden flex flex-col">
          <DialogHeader>
            <DialogTitle>配置验证失败</DialogTitle>
            <DialogDescription>
              sing-box check 输出如下：
            </DialogDescription>
          </DialogHeader>
          <pre className="mt-2 p-4 rounded-lg bg-muted text-sm overflow-auto max-h-[60vh] font-mono whitespace-pre-wrap">
            {errorDetail}
          </pre>
        </DialogContent>
      </Dialog>
    </div>
  )
}
