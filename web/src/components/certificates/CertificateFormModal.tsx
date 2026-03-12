"use client"

import { useEffect, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import {
  Button,
  Input,
  Modal,
} from "@heroui/react"
import { FieldLabel } from "@/components/form/FieldLabel"

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

export function CertificateFormModal({
  open,
  onOpenChange,
  certificate,
  onSuccess,
}: CertificateFormModalProps) {
  const queryClient = useQueryClient()
  const [checkError, setCheckError] = useState<string | null>(null)
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
      if (res.status === 400 && data.error) {
        setCheckError(data.error)
      } else {
        toast.error(data.error || "请求失败")
      }
      return
    }

    toast.success(certificate ? "证书已更新" : "证书已添加")
    onSuccess?.()
    onOpenChange(false)
    queryClient.invalidateQueries({ queryKey: ["certificates"] })
  })

  return (
    <Modal.Root isOpen={open} onOpenChange={onOpenChange}>
      <Modal.Backdrop>
        <Modal.Container size="lg">
          <Modal.Dialog>
            <Modal.Header>
              <Modal.Heading>{certificate ? "编辑证书" : "添加证书"}</Modal.Heading>
            </Modal.Header>
            <Modal.Body>

        {checkError && (
          <div className="rounded-lg bg-[color:var(--danger)]/10 text-[color:var(--danger)] p-3 text-sm">
            <pre className="whitespace-pre-wrap">{checkError}</pre>
          </div>
        )}

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
                <p className="text-sm text-[color:var(--danger)]">
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
                <p className="text-sm text-[color:var(--danger)]">
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
                <p className="text-sm text-[color:var(--danger)]">
              {form.formState.errors.privkey_path.message}
                </p>
              )}
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
              {certificate ? "保存" : "添加"}
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
