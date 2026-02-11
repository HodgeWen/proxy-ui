"use client"

import { useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

const baseSchema = z.object({
  tag: z.string().min(1, "标签不能为空"),
  listen: z.string(),
  listen_port: z.coerce.number().min(1).max(65535),
})

type BaseFormValues = z.infer<typeof baseSchema>

export type InboundForEdit = {
  id: number
  tag: string
  protocol: string
  listen: string
  listen_port: number
  tls_type: string
  transport_type: string
  user_count: number
  created_at: string
}

type InboundFormModalProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  inbound?: InboundForEdit
  onSuccess?: () => void
}

export function InboundFormModal({
  open,
  onOpenChange,
  inbound,
  onSuccess,
}: InboundFormModalProps) {
  const [protocol, setProtocol] = useState<"vless" | "hysteria2">(
    inbound?.protocol === "hysteria2" ? "hysteria2" : "vless"
  )
  const [checkError, setCheckError] = useState<string | null>(null)

  const form = useForm<BaseFormValues>({
    resolver: zodResolver(baseSchema),
    mode: "onBlur",
    defaultValues: {
      tag: inbound?.tag ?? "vless-in-1",
      listen: inbound?.listen ?? "::",
      listen_port: inbound?.listen_port ?? 443,
    },
  })

  const handleSubmit = form.handleSubmit(async (values) => {
    setCheckError(null)
    // Submit flow will be implemented in Task 3
    console.log("submit", values, protocol)
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{inbound ? "编辑入站" : "添加入站"}</DialogTitle>
        </DialogHeader>

        {checkError && (
          <div className="rounded-lg bg-destructive/10 text-destructive p-3 text-sm">
            <pre className="whitespace-pre-wrap">{checkError}</pre>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-6">
          {/* 基本设置 */}
          <section className="space-y-4">
            <h3 className="text-sm font-semibold">基本设置</h3>
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="protocol">协议</Label>
                <Select
                  value={protocol}
                  onValueChange={(v) => setProtocol(v as "vless" | "hysteria2")}
                >
                  <SelectTrigger id="protocol">
                    <SelectValue placeholder="选择协议" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="vless">VLESS</SelectItem>
                    <SelectItem value="hysteria2">Hysteria2</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="tag">标签</Label>
                <Input
                  id="tag"
                  {...form.register("tag")}
                  placeholder="vless-in-1"
                />
                {form.formState.errors.tag && (
                  <p className="text-sm text-destructive">
                    {form.formState.errors.tag.message}
                  </p>
                )}
              </div>
              <div className="space-y-2">
                <Label htmlFor="listen">监听地址</Label>
                <Input
                  id="listen"
                  {...form.register("listen")}
                  placeholder="::"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="listen_port">端口</Label>
                <Input
                  id="listen_port"
                  type="number"
                  {...form.register("listen_port")}
                  placeholder="443"
                />
                {form.formState.errors.listen_port && (
                  <p className="text-sm text-destructive">
                    {form.formState.errors.listen_port.message}
                  </p>
                )}
              </div>
            </div>
          </section>

          {/* TLS - conditional placeholder for Task 2 */}
          {(protocol === "vless" || protocol === "hysteria2") && (
            <section className="space-y-4">
              <h3 className="text-sm font-semibold">TLS</h3>
              <p className="text-sm text-muted-foreground">
                TLS 配置将在下一步添加
              </p>
            </section>
          )}

          {/* 传输 - VLESS only, placeholder for Task 2 */}
          {protocol === "vless" && (
            <section className="space-y-4">
              <h3 className="text-sm font-semibold">传输</h3>
              <p className="text-sm text-muted-foreground">
                传输协议配置将在下一步添加
              </p>
            </section>
          )}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              取消
            </Button>
            <Button type="submit" disabled={form.formState.isSubmitting}>
              {inbound ? "保存" : "添加"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
