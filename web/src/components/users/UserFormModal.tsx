"use client"

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

export type UserForEdit = {
  id: number
  name: string
  remark: string
  uuid: string
  password: string
  traffic_limit: number
  traffic_used: number
  expire_at: string | null
  enabled: boolean
  created_at: string
  inbound_ids: number[]
  inbound_tags: string[]
}

type UserFormModalProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  user?: UserForEdit
  onSuccess?: () => void
}

export function UserFormModal({
  open,
  onOpenChange,
  user,
  onSuccess,
}: UserFormModalProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{user ? "编辑用户" : "添加用户"}</DialogTitle>
        </DialogHeader>
        <p className="text-muted-foreground">实现中...</p>
      </DialogContent>
    </Dialog>
  )
}
