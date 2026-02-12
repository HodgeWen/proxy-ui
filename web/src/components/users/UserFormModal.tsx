"use client"

import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Copy, ChevronDown } from "lucide-react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"

const schema = z.object({
  name: z.string().min(1, "名称不能为空"),
  remark: z.string().optional(),
  inbound_ids: z.array(z.number()),
  traffic_limit: z.coerce.number().min(0),
  expire_at: z.string().optional(),
})

type FormValues = z.infer<typeof schema>

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

type Inbound = {
  id: number
  tag: string
  protocol: string
}

async function fetchInbounds(): Promise<{ data: Inbound[] }> {
  const res = await fetch("/api/inbounds", { credentials: "include" })
  if (!res.ok) throw new Error("获取入站列表失败")
  return res.json()
}

type UserFormModalProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  user?: UserForEdit
  onSuccess?: () => void
}

async function copyToClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success("已复制到剪贴板")
  } catch {
    toast.error("复制失败")
  }
}

export function UserFormModal({
  open,
  onOpenChange,
  user,
  onSuccess,
}: UserFormModalProps) {
  const queryClient = useQueryClient()
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    mode: "onBlur",
    defaultValues: {
      name: "",
      remark: "",
      inbound_ids: [],
      traffic_limit: 0,
      expire_at: "",
    },
  })

  const { data: inboundData } = useQuery({
    queryKey: ["inbounds"],
    queryFn: fetchInbounds,
    enabled: open,
  })
  const inbounds = inboundData?.data ?? []

  const bytesToGB = (bytes: number) => (bytes > 0 ? Math.round(bytes / (1024 * 1024 * 1024)) : 0)
  const gbToBytes = (gb: number) => (gb > 0 ? gb * 1024 * 1024 * 1024 : 0)

  useEffect(() => {
    if (user) {
      form.reset({
        name: user.name,
        remark: user.remark || "",
        inbound_ids: user.inbound_ids ?? [],
        traffic_limit: bytesToGB(user.traffic_limit ?? 0),
        expire_at: user.expire_at ? user.expire_at.slice(0, 10) : "",
      })
    } else {
      form.reset({
        name: "",
        remark: "",
        inbound_ids: [],
        traffic_limit: 0,
        expire_at: "",
      })
    }
  }, [user, open, form])

  const handleSubmit = form.handleSubmit(async (values) => {
    const expireAt = values.expire_at
      ? `${values.expire_at}T23:59:59.999Z`
      : undefined

    const trafficLimitBytes = gbToBytes(values.traffic_limit)

    const body = {
      name: values.name,
      remark: values.remark || "",
      inbound_ids: values.inbound_ids,
      traffic_limit: trafficLimitBytes === 0 ? null : trafficLimitBytes,
      expire_at: expireAt ?? null,
    }

    const url = user ? `/api/users/${user.id}` : "/api/users"
    const method = user ? "PUT" : "POST"

    const res = await fetch(url, {
      method,
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify(body),
    })

    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      toast.error(data.error || "请求失败")
      return
    }

    toast.success(user ? "用户已更新" : "用户已添加")
    onSuccess?.()
    onOpenChange(false)
    queryClient.invalidateQueries({ queryKey: ["users"] })
  })

  const selectedIds = form.watch("inbound_ids")
  const selectedLabels = inbounds
    .filter((ib) => selectedIds.includes(ib.id))
    .map((ib) => ib.tag)
  const displayText = selectedLabels.length > 0 ? selectedLabels.join(", ") : "选择节点"

  const toggleInbound = (id: number) => {
    const current = form.getValues("inbound_ids")
    const next = current.includes(id)
      ? current.filter((i) => i !== id)
      : [...current, id]
    form.setValue("inbound_ids", next)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{user ? "编辑用户" : "添加用户"}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-6">
          {user && (
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label>UUID</Label>
                <div className="flex gap-2">
                  <Input
                    readOnly
                    value={user.uuid}
                    className="font-mono text-sm"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    onClick={() => copyToClipboard(user.uuid)}
                    aria-label="复制 UUID"
                  >
                    <Copy className="size-4" />
                  </Button>
                </div>
              </div>
              <div className="space-y-2">
                <Label>密码</Label>
                <div className="flex gap-2">
                  <Input
                    readOnly
                    value={user.password}
                    className="font-mono text-sm"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    onClick={() => copyToClipboard(user.password)}
                    aria-label="复制密码"
                  >
                    <Copy className="size-4" />
                  </Button>
                </div>
              </div>
            </div>
          )}

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="name">名称</Label>
              <Input
                id="name"
                {...form.register("name")}
                placeholder="用户名"
              />
              {form.formState.errors.name && (
                <p className="text-sm text-destructive">
                  {form.formState.errors.name.message}
                </p>
              )}
            </div>
            <div className="space-y-2">
              <Label htmlFor="remark">备注</Label>
              <Input
                id="remark"
                {...form.register("remark")}
                placeholder="可选"
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label>节点</Label>
            <Popover>
              <PopoverTrigger asChild>
                <Button
                  variant="outline"
                  className="w-full justify-between font-normal"
                >
                  <span className="truncate">{displayText}</span>
                  <ChevronDown className="size-4 shrink-0 opacity-50" />
                </Button>
              </PopoverTrigger>
              <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-2" align="start">
                <div className="max-h-60 overflow-y-auto space-y-2">
                  {inbounds.length === 0 ? (
                    <p className="text-sm text-muted-foreground py-2">暂无入站</p>
                  ) : (
                    inbounds.map((ib) => (
                      <div
                        key={ib.id}
                        className="flex items-center space-x-2 cursor-pointer rounded-sm py-2 px-2 hover:bg-accent"
                        onClick={() => toggleInbound(ib.id)}
                      >
                        <Checkbox
                          checked={selectedIds.includes(ib.id)}
                          onCheckedChange={() => toggleInbound(ib.id)}
                        />
                        <Label className="flex-1 cursor-pointer text-sm font-normal">
                          {ib.tag}
                        </Label>
                      </div>
                    ))
                  )}
                </div>
              </PopoverContent>
            </Popover>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="traffic_limit">流量上限 (GB)</Label>
              <Input
                id="traffic_limit"
                type="number"
                min={0}
                {...form.register("traffic_limit")}
                placeholder="0 表示无限制"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="expire_at">到期时间</Label>
              <Input
                id="expire_at"
                type="date"
                {...form.register("expire_at")}
              />
              <p className="text-xs text-muted-foreground">留空表示永不过期</p>
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              取消
            </Button>
            <Button type="submit" disabled={form.formState.isSubmitting}>
              {user ? "保存" : "添加"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
