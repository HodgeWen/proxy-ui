"use client"

import { useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Modal } from "@heroui/react"
import type { User } from "@/components/users/UserTable"
import { UserSubscriptionCard } from "@/components/users/UserSubscriptionCard"

async function fetchUser(id: number): Promise<User> {
  const res = await fetch(`/api/users/${id}`, { credentials: "include" })
  if (!res.ok) throw new Error("获取用户详情失败")
  return res.json()
}

type UserSubscriptionModalProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId?: number
}

export function UserSubscriptionModal({
  open,
  onOpenChange,
  userId,
}: UserSubscriptionModalProps) {
  const queryClient = useQueryClient()

  const { data: user, isLoading } = useQuery({
    queryKey: ["users", "detail", userId],
    queryFn: () => fetchUser(userId!),
    enabled: open && !!userId,
  })

  const handleReset = async () => {
    if (!userId) return
    const res = await fetch(`/api/users/${userId}/reset-subscription`, {
      method: "POST",
      credentials: "include",
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      toast.error(data.error || "重置失败")
      return
    }
    toast.success("订阅已重置")
    queryClient.invalidateQueries({ queryKey: ["users"] })
    queryClient.invalidateQueries({ queryKey: ["users", "detail", userId] })
  }

  return (
    <Modal.Root isOpen={open} onOpenChange={onOpenChange}>
      <Modal.Backdrop>
        <Modal.Container size="lg">
          <Modal.Dialog>
            <Modal.Header>
              <Modal.Heading>订阅信息{user ? ` — ${user.name}` : ""}</Modal.Heading>
            </Modal.Header>
            <Modal.Body>
        {isLoading ? (
          <div className="py-8 text-center text-foreground-500">加载中...</div>
        ) : user ? (
          <UserSubscriptionCard
            user={{
              name: user.name,
              subscription_url: user.subscription_url ?? "",
              subscription_nodes: user.subscription_nodes,
              traffic_used: user.traffic_used ?? 0,
              traffic_limit: user.traffic_limit ?? 0,
              expire_at: user.expire_at ?? null,
            }}
            onReset={handleReset}
          />
        ) : (
          <div className="py-8 text-center text-foreground-500">用户未找到</div>
        )}
            </Modal.Body>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal.Root>
  )
}
