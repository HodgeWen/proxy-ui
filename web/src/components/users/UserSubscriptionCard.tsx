"use client"

import { useState } from "react"
import { QRCodeSVG } from "qrcode.react"
import { Copy, ChevronDown, ChevronUp, RotateCcw } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { formatBytes } from "@/lib/format"
import type { SubscriptionNode } from "@/components/users/UserTable"

export type UserSubscriptionCardProps = {
  user: {
    name: string
    subscription_url: string
    subscription_nodes?: SubscriptionNode[]
    traffic_used: number
    traffic_limit: number
    expire_at: string | null
  }
  onReset: () => Promise<void>
}

async function copyToClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success("已复制到剪贴板")
  } catch {
    toast.error("复制失败")
  }
}

export function UserSubscriptionCard({ user, onReset }: UserSubscriptionCardProps) {
  const [showQR, setShowQR] = useState(false)
  const [resetting, setResetting] = useState(false)

  const fullSubscriptionUrl =
    user.subscription_url.startsWith("http") ||
    user.subscription_url.startsWith("//")
      ? user.subscription_url
      : `${typeof window !== "undefined" ? window.location.origin : ""}${user.subscription_url.startsWith("/") ? user.subscription_url : `/${user.subscription_url}`}`

  const trafficText =
    user.traffic_limit > 0
      ? `${formatBytes(user.traffic_used)} / ${formatBytes(user.traffic_limit)}`
      : "无限制"

  const expireText = user.expire_at
    ? new Date(user.expire_at).toISOString().slice(0, 10)
    : "永不过期"

  const handleReset = async () => {
    setResetting(true)
    try {
      await onReset()
    } finally {
      setResetting(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-2 text-sm sm:grid-cols-2">
          <div>
            <span className="text-muted-foreground">用户名：</span>
            <span>{user.name}</span>
          </div>
          <div>
            <span className="text-muted-foreground">剩余流量：</span>
            <span>{trafficText}</span>
          </div>
          <div>
            <span className="text-muted-foreground">到期时间：</span>
            <span>{expireText}</span>
          </div>
        </div>

        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground shrink-0">订阅链接：</span>
            <code className="flex-1 truncate rounded bg-muted px-2 py-1 text-xs font-mono">
              {fullSubscriptionUrl}
            </code>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => copyToClipboard(fullSubscriptionUrl)}
              aria-label="复制订阅链接"
            >
              <Copy className="size-4" />
              复制
            </Button>
          </div>
        </div>

        {user.subscription_nodes && user.subscription_nodes.length > 0 && (
          <div className="space-y-2">
            <p className="text-sm text-muted-foreground">节点列表</p>
            <div className="space-y-2">
              {user.subscription_nodes.map((node) => (
                <div
                  key={node.name}
                  className="flex items-center gap-2 rounded-md bg-muted/80 px-3 py-2"
                >
                  <span className="flex-1 truncate text-sm">{node.name}</span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="shrink-0"
                    onClick={() => copyToClipboard(node.link)}
                    aria-label={`复制 ${node.name} 链接`}
                  >
                    <Copy className="size-4" />
                  </Button>
                </div>
              ))}
            </div>
          </div>
        )}

        <div className="space-y-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setShowQR((v) => !v)}
            aria-expanded={showQR}
          >
            {showQR ? (
              <>
                <ChevronUp className="size-4" />
                隐藏 QR 码
              </>
            ) : (
              <>
                <ChevronDown className="size-4" />
                显示 QR 码
              </>
            )}
          </Button>
          {showQR && (
            <div className="flex justify-center rounded-lg border border-border bg-card p-4">
              <QRCodeSVG value={fullSubscriptionUrl} size={200} />
            </div>
          )}
        </div>

        <div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handleReset}
            disabled={resetting}
            className="text-destructive hover:bg-destructive/10 hover:text-destructive"
          >
            <RotateCcw className="size-4" />
            {resetting ? "重置中..." : "重置订阅"}
          </Button>
          <p className="mt-1 text-xs text-muted-foreground">
            重置后旧订阅链接将立即失效
          </p>
        </div>
    </div>
  )
}
