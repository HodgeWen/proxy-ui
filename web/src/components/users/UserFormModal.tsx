"use client"

import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Copy, ChevronDown } from "lucide-react"
import { Button, Checkbox, Input, Modal, Popover } from "@heroui/react"
import type { User } from "@/components/users/UserTable"
import { FieldLabel } from "@/components/form/FieldLabel"

const schema = z.object({
  name: z.string().min(1, "名称不能为空"),
  remark: z.string().optional(),
  inbound_ids: z.array(z.number()),
  traffic_limit: z.coerce.number().min(0),
  expire_at: z.string().optional(),
})

type FormValues = z.infer<typeof schema>

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
  user?: User
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
    resolver: zodResolver(schema) as never,
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
    <Modal.Root isOpen={open} onOpenChange={onOpenChange}>
      <Modal.Backdrop>
        <Modal.Container size="lg" className="max-h-[90vh]">
          <Modal.Dialog>
            <Modal.Header>
              <Modal.Heading>{user ? "编辑用户" : "添加用户"}</Modal.Heading>
            </Modal.Header>
            <Modal.Body>

        <form onSubmit={handleSubmit} className="space-y-6">
          {user && (
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <FieldLabel label="UUID" />
                <div className="flex gap-2">
                  <Input
                    readOnly
                    value={user.uuid}
                    className="font-mono text-sm"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    isIconOnly
                    onPress={() => copyToClipboard(user.uuid)}
                    aria-label="复制 UUID"
                  >
                    <Copy className="size-4" />
                  </Button>
                </div>
              </div>
              <div className="space-y-2">
                <FieldLabel label="密码" />
                <div className="flex gap-2">
                  <Input
                    readOnly
                    value={user.password}
                    className="font-mono text-sm"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    isIconOnly
                    onPress={() => copyToClipboard(user.password)}
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
              <FieldLabel label="名称" htmlFor="name" />
              <Input
                id="name"
                {...form.register("name")}
                placeholder="用户名"
              />
              {form.formState.errors.name && (
                <p className="text-sm text-danger">
                  {form.formState.errors.name.message}
                </p>
              )}
            </div>
            <div className="space-y-2">
              <FieldLabel label="备注" htmlFor="remark" />
              <Input
                id="remark"
                {...form.register("remark")}
                placeholder="可选"
              />
            </div>
          </div>

          <div className="space-y-2">
            <FieldLabel label="节点" />
            <Popover.Root>
              <Popover.Trigger>
                <Button
                  variant="outline"
                  className="w-full justify-between font-normal"
                >
                  <span className="truncate">{displayText}</span>
                  <ChevronDown className="size-4 shrink-0 opacity-50" />
                </Button>
              </Popover.Trigger>
              <Popover.Content className="w-[22rem] p-2" placement="bottom start">
                <Popover.Dialog>
                <div className="max-h-60 overflow-y-auto space-y-2">
                  {inbounds.length === 0 ? (
                    <p className="text-sm text-foreground-500 py-2">暂无入站</p>
                  ) : (
                    inbounds.map((ib) => (
                      <div
                        key={ib.id}
                        className="flex items-center space-x-2 cursor-pointer rounded-sm py-2 px-2 hover:bg-accent"
                        onClick={() => toggleInbound(ib.id)}
                      >
                        <Checkbox.Root
                          isSelected={selectedIds.includes(ib.id)}
                          onChange={() => toggleInbound(ib.id)}
                        >
                          <Checkbox.Control>
                            <Checkbox.Indicator />
                          </Checkbox.Control>
                          <Checkbox.Content className="flex-1 cursor-pointer text-sm font-normal">
                            {ib.tag}
                          </Checkbox.Content>
                        </Checkbox.Root>
                      </div>
                    ))
                  )}
                </div>
                </Popover.Dialog>
              </Popover.Content>
            </Popover.Root>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <FieldLabel label="流量上限 (GB)" htmlFor="traffic_limit" />
              <Input
                id="traffic_limit"
                type="number"
                min={0}
                {...form.register("traffic_limit")}
                placeholder="0 表示无限制"
              />
            </div>
            <div className="space-y-2">
              <FieldLabel label="到期时间" htmlFor="expire_at" />
              <Input
                id="expire_at"
                type="date"
                {...form.register("expire_at")}
              />
              <p className="text-xs text-foreground-500">留空表示永不过期</p>
            </div>
          </div>

              <Modal.Footer>
            <Button
              type="button"
              variant="outline"
              onPress={() => onOpenChange(false)}
            >
              取消
            </Button>
            <Button type="submit" isDisabled={form.formState.isSubmitting}>
              {user ? "保存" : "添加"}
            </Button>
              </Modal.Footer>
        </form>
            </Modal.Body>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal.Root>
  )
}
