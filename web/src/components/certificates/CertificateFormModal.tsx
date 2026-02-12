"use client"

import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Info } from "lucide-react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { Label } from "@/components/ui/label"

const schema = z.object({
  name: z.string().min(1, "名称不能为空"),
  fullchain_path: z.string().min(1, "证书路径不能为空"),
  privkey_path: z.string().min(1, "私钥路径不能为空"),
})

type FormValues = z.infer<typeof schema>

export type CertificateForEdit = {
  id: number
  name: string
  fullchain_path: string
  privkey_path: string
  created_at: string
}

type CertificateFormModalProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  certificate?: CertificateForEdit
  onSuccess?: () => void
}

function FieldLabel({
  label,
  tooltip,
  htmlFor,
}: {
  label: string
  tooltip: string
  htmlFor?: string
}) {
  return (
    <div className="flex items-center gap-1.5">
      <Label htmlFor={htmlFor}>{label}</Label>
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger asChild>
            <Info className="size-4 text-muted-foreground cursor-help" />
          </TooltipTrigger>
          <TooltipContent className="max-w-xs">
            <p className="text-xs">{tooltip}</p>
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </div>
  )
}

export function CertificateFormModal({
  open,
  onOpenChange,
  certificate,
  onSuccess,
}: CertificateFormModalProps) {
  const queryClient = useQueryClient()
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    mode: "onBlur",
    defaultValues: {
      name: "",
      fullchain_path: "",
      privkey_path: "",
    },
  })

  useEffect(() => {
    if (certificate) {
      form.reset({
        name: certificate.name,
        fullchain_path: certificate.fullchain_path,
        privkey_path: certificate.privkey_path,
      })
    } else {
      form.reset({
        name: "",
        fullchain_path: "",
        privkey_path: "",
      })
    }
  }, [certificate, open, form])

  const handleSubmit = form.handleSubmit(async (values) => {
    const url = certificate ? `/api/certs/${certificate.id}` : "/api/certs"
    const method = certificate ? "PUT" : "POST"

    const res = await fetch(url, {
      method,
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({
        name: values.name,
        fullchain_path: values.fullchain_path,
        privkey_path: values.privkey_path,
      }),
    })

    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      toast.error(data.error || "请求失败")
      return
    }

    toast.success(certificate ? "证书已更新" : "证书已添加")
    onSuccess?.()
    onOpenChange(false)
    queryClient.invalidateQueries({ queryKey: ["certificates"] })
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{certificate ? "编辑证书" : "添加证书"}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-6">
          <div className="grid gap-4 sm:grid-cols-1">
            <div className="space-y-2">
              <FieldLabel
                label="名称"
                tooltip="证书显示名称，用于在入站中选择时识别。"
                htmlFor="name"
              />
              <Input
                id="name"
                {...form.register("name")}
                placeholder="example.com"
              />
              {form.formState.errors.name && (
                <p className="text-sm text-destructive">
                  {form.formState.errors.name.message}
                </p>
              )}
            </div>
            <div className="space-y-2">
              <FieldLabel
                label="证书路径"
                tooltip="完整证书链文件路径。典型值：/etc/certs/fullchain.pem"
                htmlFor="fullchain_path"
              />
              <Input
                id="fullchain_path"
                {...form.register("fullchain_path")}
                placeholder="/etc/certs/fullchain.pem"
              />
              {form.formState.errors.fullchain_path && (
                <p className="text-sm text-destructive">
                  {form.formState.errors.fullchain_path.message}
                </p>
              )}
            </div>
            <div className="space-y-2">
              <FieldLabel
                label="私钥路径"
                tooltip="私钥文件路径。典型值：/etc/certs/privkey.pem"
                htmlFor="privkey_path"
              />
              <Input
                id="privkey_path"
                {...form.register("privkey_path")}
                placeholder="/etc/certs/privkey.pem"
              />
              {form.formState.errors.privkey_path && (
                <p className="text-sm text-destructive">
                  {form.formState.errors.privkey_path.message}
                </p>
              )}
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
              {certificate ? "保存" : "添加"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
