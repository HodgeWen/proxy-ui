import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { toast } from "sonner"
import { Copy, QrCode } from "lucide-react"
import { QRCodeSVG } from "qrcode.react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

type UserSub = {
  id: number
  name: string
  subscription_url: string
  traffic_limit: number
  traffic_used: number
  expire_at: string | null
  enabled: boolean
}

function getStatus(u: UserSub): { label: string; variant: "default" | "secondary" | "destructive" | "outline" } {
  if (!u.enabled) return { label: "禁用", variant: "secondary" }
  if (u.expire_at && new Date(u.expire_at) <= new Date()) return { label: "过期", variant: "destructive" }
  if (u.traffic_limit > 0 && u.traffic_used >= u.traffic_limit) return { label: "超限", variant: "destructive" }
  return { label: "活跃", variant: "default" }
}

function fullUrl(path: string): string {
  if (path.startsWith("http") || path.startsWith("//")) return path
  const origin = typeof window !== "undefined" ? window.location.origin : ""
  return origin + (path.startsWith("/") ? path : `/${path}`)
}

async function copyToClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success("已复制到剪贴板")
  } catch {
    toast.error("复制失败")
  }
}

export function Subscriptions() {
  const [qrUser, setQrUser] = useState<UserSub | null>(null)

  const { data, isLoading } = useQuery<{ data: UserSub[] }>({
    queryKey: ["users"],
    queryFn: async () => {
      const res = await fetch("/api/users", { credentials: "include" })
      if (!res.ok) throw new Error("获取用户列表失败")
      return res.json()
    },
    refetchInterval: 60000,
  })

  const users = data?.data ?? []

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-bold">订阅</h1>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>用户名</TableHead>
            <TableHead>订阅链接</TableHead>
            <TableHead>状态</TableHead>
            <TableHead>操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading ? (
            <TableRow>
              <TableCell colSpan={4} className="text-center text-muted-foreground py-8">
                加载中...
              </TableCell>
            </TableRow>
          ) : users.length === 0 ? (
            <TableRow>
              <TableCell colSpan={4} className="text-center text-muted-foreground py-8">
                暂无用户
              </TableCell>
            </TableRow>
          ) : (
            users.map((u) => {
              const status = getStatus(u)
              const url = u.subscription_url ? fullUrl(u.subscription_url) : ""
              return (
                <TableRow key={u.id}>
                  <TableCell className="font-medium">{u.name}</TableCell>
                  <TableCell className="max-w-xs">
                    {url ? (
                      <code className="truncate block rounded bg-muted px-2 py-1 text-xs font-mono">
                        {url}
                      </code>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <Badge variant={status.variant}>{status.label}</Badge>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-8"
                        disabled={!url}
                        onClick={() => copyToClipboard(url)}
                        aria-label="复制链接"
                      >
                        <Copy className="size-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-8"
                        disabled={!url}
                        onClick={() => setQrUser(u)}
                        aria-label="查看 QR 码"
                      >
                        <QrCode className="size-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              )
            })
          )}
        </TableBody>
      </Table>

      <Dialog open={!!qrUser} onOpenChange={(open) => !open && setQrUser(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{qrUser?.name} — 订阅 QR 码</DialogTitle>
          </DialogHeader>
          {qrUser?.subscription_url && (
            <div className="flex flex-col items-center gap-4">
              <div className="rounded-lg border border-muted-foreground/20 bg-white p-4">
                <QRCodeSVG value={fullUrl(qrUser.subscription_url)} size={200} />
              </div>
              <code className="text-xs font-mono text-muted-foreground break-all text-center px-4">
                {fullUrl(qrUser.subscription_url)}
              </code>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
