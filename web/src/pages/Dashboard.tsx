import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { RefreshCw, Radio, Users } from "lucide-react"
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

async function fetchStatus(): Promise<CoreStatus> {
  const res = await fetch("/api/core/status", { credentials: "include" })
  if (!res.ok) throw new Error("获取状态失败")
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

export function Dashboard() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [errorModalOpen, setErrorModalOpen] = useState(false)
  const [errorDetail, setErrorDetail] = useState("")

  const { data: status, isLoading } = useQuery({
    queryKey: ["core", "status"],
    queryFn: fetchStatus,
    refetchInterval: 5000,
  })

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
        {/* sing-box 状态卡片 */}
        <Card className="transition-shadow hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">sing-box 状态</CardTitle>
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
                {status.version && (
                  <CardDescription>版本 {status.version}</CardDescription>
                )}
                <Button
                  size="sm"
                  className="mt-2"
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
