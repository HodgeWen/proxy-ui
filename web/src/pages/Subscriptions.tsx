import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { toast } from "sonner"
import { Copy, QrCode } from "lucide-react"
import { QRCodeSVG } from "qrcode.react"
import { Button, Chip, Modal, Table } from "@heroui/react"
import { PageHero } from "@/components/layout/PageHero"

type UserSub = {
  id: number
  name: string
  subscription_url: string
  traffic_limit: number
  traffic_used: number
  expire_at: string | null
  enabled: boolean
}

function getStatus(u: UserSub): { label: string; color: "danger" | "default" | "success" } {
  if (!u.enabled) return { label: "禁用", color: "default" }
  if (u.expire_at && new Date(u.expire_at) <= new Date()) return { label: "过期", color: "danger" }
  if (u.traffic_limit > 0 && u.traffic_used >= u.traffic_limit) return { label: "超限", color: "danger" }
  return { label: "活跃", color: "success" }
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
      <PageHero
        title="订阅分发面板"
        description="统一查看订阅链接、状态和二维码入口，避免操作藏在稀疏的列表项里。"
        metrics={[
          { label: "用户数", value: String(users.length) },
          {
            label: "有效订阅",
            value: String(users.filter((user) => Boolean(user.subscription_url)).length),
          },
        ]}
      />

      <Table.Root aria-label="订阅列表">
        <Table.ScrollContainer>
          <Table.Content>
            <Table.Header>
              <Table.Column>用户名</Table.Column>
              <Table.Column>订阅链接</Table.Column>
              <Table.Column>状态</Table.Column>
              <Table.Column>操作</Table.Column>
            </Table.Header>
            <Table.Body>
              {isLoading ? (
                <Table.Row>
                  <Table.Cell className="py-8 text-center text-foreground-500" colSpan={4}>
                    加载中...
                  </Table.Cell>
                </Table.Row>
              ) : users.length === 0 ? (
                <Table.Row>
                  <Table.Cell className="py-8 text-center text-foreground-500" colSpan={4}>
                    暂无用户
                  </Table.Cell>
                </Table.Row>
              ) : (
                users.map((u) => {
                  const status = getStatus(u)
                  const url = u.subscription_url ? fullUrl(u.subscription_url) : ""
                  return (
                    <Table.Row key={u.id}>
                      <Table.Cell className="font-medium">{u.name}</Table.Cell>
                      <Table.Cell className="max-w-xs">
                        {url ? (
                          <code className="block truncate rounded-lg bg-content2 px-2.5 py-1.5 text-xs font-mono">
                            {url}
                          </code>
                        ) : (
                          <span className="text-foreground-500">—</span>
                        )}
                      </Table.Cell>
                      <Table.Cell>
                        <Chip color={status.color}>{status.label}</Chip>
                      </Table.Cell>
                      <Table.Cell>
                        <div className="flex items-center gap-1">
                          <Button
                            variant="ghost"
                            isIconOnly
                            isDisabled={!url}
                            onPress={() => copyToClipboard(url)}
                            aria-label="复制链接"
                          >
                            <Copy className="size-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            isIconOnly
                            isDisabled={!url}
                            onPress={() => setQrUser(u)}
                            aria-label="查看 QR 码"
                          >
                            <QrCode className="size-4" />
                          </Button>
                        </div>
                      </Table.Cell>
                    </Table.Row>
                  )
                })
              )}
            </Table.Body>
          </Table.Content>
        </Table.ScrollContainer>
      </Table.Root>

      <Modal.Root isOpen={!!qrUser} onOpenChange={(open) => !open && setQrUser(null)}>
        <Modal.Backdrop>
          <Modal.Container size="md">
            <Modal.Dialog>
              <Modal.Header>
                <Modal.Heading>{qrUser?.name} — 订阅 QR 码</Modal.Heading>
              </Modal.Header>
          {qrUser?.subscription_url && (
            <Modal.Body>
              <div className="flex flex-col items-center gap-4">
                <div className="rounded-lg border border-divider bg-content1 p-4">
                <QRCodeSVG value={fullUrl(qrUser.subscription_url)} size={200} />
                </div>
                <code className="break-all px-4 text-center text-xs font-mono text-foreground-500">
                  {fullUrl(qrUser.subscription_url)}
                </code>
              </div>
            </Modal.Body>
          )}
            </Modal.Dialog>
          </Modal.Container>
        </Modal.Backdrop>
      </Modal.Root>
    </div>
  )
}
