"use client"

import { useState, useEffect } from "react"
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
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"

const baseSchema = z.object({
  tag: z.string().min(1, "标签不能为空"),
  listen: z.string(),
  listen_port: z.coerce.number().min(1).max(65535),
})

const vlessSchema = baseSchema.extend({
  tls_type: z.enum(["none", "tls", "reality"]),
  tls_server_name: z.string().optional(),
  tls_certificate_path: z.string().optional(),
  tls_key_path: z.string().optional(),
  reality_server: z.string().optional(),
  reality_server_port: z.coerce.number().optional(),
  reality_private_key: z.string().optional(),
  reality_short_id: z.string().optional(),
  transport_type: z.enum(["tcp", "ws", "grpc", "http"]),
  transport_path: z.string().optional(),
  transport_service_name: z.string().optional(),
  transport_http_host: z.string().optional(),
  transport_http_path: z.string().optional(),
  flow: z.string().optional(),
})

const hysteria2Schema = baseSchema.extend({
  up_mbps: z.coerce.number().min(1).optional(),
  down_mbps: z.coerce.number().min(1).optional(),
  obfs_password: z.string().optional(),
  tls_server_name: z.string().optional(),
  tls_certificate_path: z.string().optional(),
  tls_key_path: z.string().optional(),
})

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
  config_json?: string
}

type InboundFormModalProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  inbound?: InboundForEdit
  onSuccess?: () => void
}

type VLESSFormValues = z.infer<typeof vlessSchema>
type Hysteria2FormValues = z.infer<typeof hysteria2Schema>

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

  const vlessForm = useForm<VLESSFormValues>({
    resolver: zodResolver(vlessSchema),
    mode: "onBlur",
    defaultValues: {
      tag: inbound?.tag ?? "vless-in-1",
      listen: inbound?.listen ?? "::",
      listen_port: inbound?.listen_port ?? 443,
      tls_type: "none",
      transport_type: "tcp",
      transport_path: "/vless",
      transport_service_name: "TunService",
      transport_http_host: "",
      transport_http_path: "",
      flow: "",
    },
  })

  const hy2Form = useForm<Hysteria2FormValues>({
    resolver: zodResolver(hysteria2Schema),
    mode: "onBlur",
    defaultValues: {
      tag: inbound?.tag ?? "hy2-in-1",
      listen: inbound?.listen ?? "::",
      listen_port: inbound?.listen_port ?? 443,
      up_mbps: 100,
      down_mbps: 100,
      obfs_password: "",
      tls_server_name: "",
      tls_certificate_path: "",
      tls_key_path: "",
    },
  })

  const form = protocol === "vless" ? vlessForm : hy2Form

  useEffect(() => {
    if (inbound) {
      // Populate from inbound when editing - Task 3 will parse config_json
    }
  }, [inbound])

  const handleSubmit = form.handleSubmit(async (values) => {
    setCheckError(null)
    // Submit flow in Task 3
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
                  placeholder={protocol === "vless" ? "vless-in-1" : "hy2-in-1"}
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
              {protocol === "hysteria2" && (
                <>
                  <div className="space-y-2">
                    <Label htmlFor="up_mbps">上行带宽 (Mbps)</Label>
                    <Input
                      id="up_mbps"
                      type="number"
                      {...form.register("up_mbps")}
                      placeholder="100"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="down_mbps">下行带宽 (Mbps)</Label>
                    <Input
                      id="down_mbps"
                      type="number"
                      {...form.register("down_mbps")}
                      placeholder="100"
                    />
                  </div>
                  <div className="space-y-2 sm:col-span-2">
                    <Label htmlFor="obfs_password">Obfs 密码 (可选)</Label>
                    <Input
                      id="obfs_password"
                      type="password"
                      {...form.register("obfs_password")}
                      placeholder="留空则不使用混淆"
                    />
                  </div>
                </>
              )}
            </div>
          </section>

          {/* TLS */}
          <section className="space-y-4">
            <h3 className="text-sm font-semibold">TLS</h3>
            {protocol === "vless" ? (
              <>
                <div className="space-y-2">
                  <Label>TLS 类型</Label>
                  <Select
                    value={form.watch("tls_type")}
                    onValueChange={(v) =>
                      form.setValue("tls_type", v as "none" | "tls" | "reality")
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">无 TLS</SelectItem>
                      <SelectItem value="tls">TLS</SelectItem>
                      <SelectItem value="reality">Reality</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                {form.watch("tls_type") === "tls" && (
                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="space-y-2">
                      <Label htmlFor="tls_server_name">server_name</Label>
                      <Input
                        id="tls_server_name"
                        {...form.register("tls_server_name")}
                        placeholder="example.com"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="tls_certificate_path">证书路径</Label>
                      <Input
                        id="tls_certificate_path"
                        {...form.register("tls_certificate_path")}
                        placeholder="/etc/certs/fullchain.pem"
                      />
                    </div>
                    <div className="space-y-2 sm:col-span-2">
                      <Label htmlFor="tls_key_path">私钥路径</Label>
                      <Input
                        id="tls_key_path"
                        {...form.register("tls_key_path")}
                        placeholder="/etc/certs/privkey.pem"
                      />
                    </div>
                  </div>
                )}
                {form.watch("tls_type") === "reality" && (
                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="space-y-2">
                      <Label htmlFor="reality_server">handshake.server</Label>
                      <Input
                        id="reality_server"
                        {...form.register("reality_server")}
                        placeholder="google.com"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="reality_server_port">
                        handshake.server_port
                      </Label>
                      <Input
                        id="reality_server_port"
                        type="number"
                        {...form.register("reality_server_port")}
                        placeholder="443"
                      />
                    </div>
                    <div className="space-y-2 sm:col-span-2">
                      <Label htmlFor="reality_private_key">private_key</Label>
                      <Input
                        id="reality_private_key"
                        {...form.register("reality_private_key")}
                        placeholder="Reality 私钥"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="reality_short_id">short_id</Label>
                      <Input
                        id="reality_short_id"
                        {...form.register("reality_short_id")}
                        placeholder="0123456789abcdef"
                      />
                    </div>
                  </div>
                )}
              </>
            ) : (
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="tls_server_name">server_name</Label>
                  <Input
                    id="tls_server_name"
                    {...form.register("tls_server_name")}
                    placeholder="example.com"
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="tls_certificate_path">证书路径</Label>
                  <Input
                    id="tls_certificate_path"
                    {...form.register("tls_certificate_path")}
                    placeholder="/etc/certs/fullchain.pem"
                  />
                </div>
                <div className="space-y-2 sm:col-span-2">
                  <Label htmlFor="tls_key_path">私钥路径</Label>
                  <Input
                    id="tls_key_path"
                    {...form.register("tls_key_path")}
                    placeholder="/etc/certs/privkey.pem"
                  />
                </div>
              </div>
            )}
          </section>

          {/* 传输 - VLESS only */}
          {protocol === "vless" && (
            <section className="space-y-4">
              <h3 className="text-sm font-semibold">传输</h3>
              <RadioGroup
                value={form.watch("transport_type")}
                onValueChange={(v) =>
                  form.setValue("transport_type", v as "tcp" | "ws" | "grpc" | "http")
                }
                className="flex flex-wrap gap-4"
              >
                <div className="flex items-center gap-2">
                  <RadioGroupItem value="tcp" id="tcp" />
                  <Label htmlFor="tcp">TCP</Label>
                </div>
                <div className="flex items-center gap-2">
                  <RadioGroupItem value="ws" id="ws" />
                  <Label htmlFor="ws">WebSocket</Label>
                </div>
                <div className="flex items-center gap-2">
                  <RadioGroupItem value="grpc" id="grpc" />
                  <Label htmlFor="grpc">gRPC</Label>
                </div>
                <div className="flex items-center gap-2">
                  <RadioGroupItem value="http" id="http" />
                  <Label htmlFor="http">HTTP/2</Label>
                </div>
              </RadioGroup>
              {form.watch("transport_type") === "ws" && (
                <div className="space-y-2">
                  <Label htmlFor="transport_path">path</Label>
                  <Input
                    id="transport_path"
                    {...form.register("transport_path")}
                    placeholder="/vless"
                  />
                </div>
              )}
              {form.watch("transport_type") === "grpc" && (
                <div className="space-y-2">
                  <Label htmlFor="transport_service_name">service_name</Label>
                  <Input
                    id="transport_service_name"
                    {...form.register("transport_service_name")}
                    placeholder="TunService"
                  />
                </div>
              )}
              {form.watch("transport_type") === "http" && (
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="transport_http_host">host</Label>
                    <Input
                      id="transport_http_host"
                      {...form.register("transport_http_host")}
                      placeholder=""
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="transport_http_path">path</Label>
                    <Input
                      id="transport_http_path"
                      {...form.register("transport_http_path")}
                      placeholder=""
                    />
                  </div>
                </div>
              )}
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
