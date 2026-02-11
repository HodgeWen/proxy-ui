"use client"

import { useState, useEffect } from "react"
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
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"

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

function buildConfigJson(
  values: VLESSFormValues | Hysteria2FormValues,
  protocol: "vless" | "hysteria2"
): Record<string, unknown> {
  if (protocol === "vless") {
    const v = values as VLESSFormValues
    const config: Record<string, unknown> = { users: [] }

    if (v.tls_type === "tls" && (v.tls_server_name || v.tls_certificate_path)) {
      config.tls = {
        enabled: true,
        server_name: v.tls_server_name || "",
        certificate_path: v.tls_certificate_path || "",
        key_path: v.tls_key_path || "",
      }
    } else if (v.tls_type === "reality") {
      config.tls = {
        enabled: true,
        reality: {
          enabled: true,
          handshake: {
            server: v.reality_server || "google.com",
            server_port: v.reality_server_port || 443,
          },
          private_key: v.reality_private_key || "",
          short_id: v.reality_short_id ? [v.reality_short_id] : [],
        },
      }
    } else {
      config.tls = { enabled: false }
    }

    if (v.transport_type === "ws") {
      config.transport = {
        type: "ws",
        path: v.transport_path || "/vless",
      }
    } else if (v.transport_type === "grpc") {
      config.transport = {
        type: "grpc",
        service_name: v.transport_service_name || "TunService",
      }
    } else if (v.transport_type === "http") {
      config.transport = {
        type: "http",
        host: v.transport_http_host || "",
        path: v.transport_http_path || "",
      }
    }

    return config
  } else {
    const v = values as Hysteria2FormValues
    const config: Record<string, unknown> = {
      users: [],
      tls: {
        enabled: true,
        server_name: v.tls_server_name || "",
        certificate_path: v.tls_certificate_path || "",
        key_path: v.tls_key_path || "",
      },
    }
    if (v.up_mbps) config.up_mbps = v.up_mbps
    if (v.down_mbps) config.down_mbps = v.down_mbps
    if (v.obfs_password) {
      config.obfs = { type: "salamander", password: v.obfs_password }
    }
    return config
  }
}

function parseConfigFromJson(
  configJson: string | undefined
): Partial<VLESSFormValues> & Partial<Hysteria2FormValues> {
  if (!configJson) return {}
  try {
    const cfg = JSON.parse(configJson) as Record<string, unknown>
    const out: Record<string, unknown> = {}

    const tls = cfg.tls as Record<string, unknown> | undefined
    if (tls) {
      const reality = tls.reality as Record<string, unknown> | undefined
      if (reality) {
        out.tls_type = "reality"
        const hs = reality.handshake as Record<string, unknown>
        if (hs) {
          out.reality_server = hs.server
          out.reality_server_port = hs.server_port
        }
        out.reality_private_key = reality.private_key
        const sid = reality.short_id
        out.reality_short_id = Array.isArray(sid) ? sid[0] : sid
      } else if (tls.enabled) {
        out.tls_type = "tls"
        out.tls_server_name = tls.server_name
        out.tls_certificate_path = tls.certificate_path
        out.tls_key_path = tls.key_path
      } else {
        out.tls_type = "none"
      }
    }

    const transport = cfg.transport as Record<string, unknown> | undefined
    if (transport) {
      const t = transport.type as string
      out.transport_type = t || "tcp"
      if (t === "ws") out.transport_path = transport.path
      if (t === "grpc") out.transport_service_name = transport.service_name
      if (t === "http") {
        out.transport_http_host = transport.host
        out.transport_http_path = transport.path
      }
    }

    out.up_mbps = cfg.up_mbps
    out.down_mbps = cfg.down_mbps
    const obfs = cfg.obfs as Record<string, unknown> | undefined
    if (obfs?.password) out.obfs_password = obfs.password as string

    return out as Partial<VLESSFormValues> & Partial<Hysteria2FormValues>
  } catch {
    return {}
  }
}

export function InboundFormModal({
  open,
  onOpenChange,
  inbound,
  onSuccess,
}: InboundFormModalProps) {
  const queryClient = useQueryClient()
  const [protocol, setProtocol] = useState<"vless" | "hysteria2">(
    inbound?.protocol === "hysteria2" ? "hysteria2" : "vless"
  )
  const [checkError, setCheckError] = useState<string | null>(null)

  const vlessForm = useForm<VLESSFormValues>({
    resolver: zodResolver(vlessSchema),
    mode: "onBlur",
    defaultValues: {
      tag: "vless-in-1",
      listen: "::",
      listen_port: 443,
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
      tag: "hy2-in-1",
      listen: "::",
      listen_port: 443,
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
      setProtocol(inbound.protocol === "hysteria2" ? "hysteria2" : "vless")
      const base = {
        tag: inbound.tag,
        listen: inbound.listen,
        listen_port: inbound.listen_port,
      }
      const parsed = parseConfigFromJson(inbound.config_json)
      if (inbound.protocol === "vless") {
        vlessForm.reset({
          ...base,
          tls_type: (parsed.tls_type as "none" | "tls" | "reality") || "none",
          tls_server_name: parsed.tls_server_name || "",
          tls_certificate_path: parsed.tls_certificate_path || "",
          tls_key_path: parsed.tls_key_path || "",
          reality_server: parsed.reality_server || "",
          reality_server_port: parsed.reality_server_port || 443,
          reality_private_key: parsed.reality_private_key || "",
          reality_short_id: parsed.reality_short_id || "",
          transport_type: (parsed.transport_type as "tcp" | "ws" | "grpc" | "http") || "tcp",
          transport_path: parsed.transport_path || "/vless",
          transport_service_name: parsed.transport_service_name || "TunService",
          transport_http_host: parsed.transport_http_host || "",
          transport_http_path: parsed.transport_http_path || "",
          flow: parsed.flow || "",
        })
      } else {
        hy2Form.reset({
          ...base,
          up_mbps: parsed.up_mbps || 100,
          down_mbps: parsed.down_mbps || 100,
          obfs_password: parsed.obfs_password || "",
          tls_server_name: parsed.tls_server_name || "",
          tls_certificate_path: parsed.tls_certificate_path || "",
          tls_key_path: parsed.tls_key_path || "",
        })
      }
    } else {
      setProtocol("vless")
      vlessForm.reset({
        tag: "vless-in-1",
        listen: "::",
        listen_port: 443,
        tls_type: "none",
        transport_type: "tcp",
        transport_path: "/vless",
        transport_service_name: "TunService",
        transport_http_host: "",
        transport_http_path: "",
        flow: "",
      })
      hy2Form.reset({
        tag: "hy2-in-1",
        listen: "::",
        listen_port: 443,
        up_mbps: 100,
        down_mbps: 100,
        obfs_password: "",
        tls_server_name: "",
        tls_certificate_path: "",
        tls_key_path: "",
      })
    }
  }, [inbound, open])

  const handleSubmit = form.handleSubmit(async (values) => {
    setCheckError(null)
    const configJson = buildConfigJson(values, protocol)
    const body = {
      tag: values.tag,
      protocol,
      listen: values.listen || "::",
      listen_port: values.listen_port,
      config_json: configJson,
    }

    const url = inbound ? `/api/inbounds/${inbound.id}` : "/api/inbounds"
    const method = inbound ? "PUT" : "POST"

    const res = await fetch(url, {
      method,
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify(body),
    })

    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      if (res.status === 400 && data.error) {
        setCheckError(data.error)
      } else {
        toast.error(data.error || "请求失败")
      }
      return
    }

    toast.success(inbound ? "入站已更新" : "入站已添加")
    onSuccess?.()
    onOpenChange(false)
    queryClient.invalidateQueries({ queryKey: ["inbounds"] })
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
          <section className="space-y-4">
            <h3 className="text-sm font-semibold">基本设置</h3>
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <FieldLabel
                  label="协议"
                  tooltip="入站协议类型。VLESS 基于 XTLS，Hysteria2 基于 QUIC。选择后表单字段会切换。典型值：VLESS 或 Hysteria2。"
                  htmlFor="protocol"
                />
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
                <FieldLabel
                  label="标签"
                  tooltip="入站唯一标识符。典型值示例：vless-in-1、hy2-in-1。修改时需保证不与现有入站冲突。"
                  htmlFor="tag"
                />
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
                <FieldLabel
                  label="监听地址"
                  tooltip="监听地址。:: 表示 IPv4+IPv6 全通。典型值：::。修改用于绑定特定网卡。"
                  htmlFor="listen"
                />
                <Input
                  id="listen"
                  {...form.register("listen")}
                  placeholder="::"
                />
              </div>
              <div className="space-y-2">
                <FieldLabel
                  label="端口"
                  tooltip="监听端口。典型值：443。范围为 1 到 65535。修改时需与防火墙或反向代理一致。"
                  htmlFor="listen_port"
                />
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
                    <FieldLabel
                      label="上行带宽 (Mbps)"
                      tooltip="上行带宽限制。典型值：100。修改时根据服务器实际带宽调整。"
                    />
                    <Input
                      id="up_mbps"
                      type="number"
                      {...form.register("up_mbps")}
                      placeholder="100"
                    />
                  </div>
                  <div className="space-y-2">
                    <FieldLabel
                      label="下行带宽 (Mbps)"
                      tooltip="下行带宽限制。典型值：100。修改时根据服务器实际带宽调整。"
                    />
                    <Input
                      id="down_mbps"
                      type="number"
                      {...form.register("down_mbps")}
                      placeholder="100"
                    />
                  </div>
                  <div className="space-y-2 sm:col-span-2">
                    <FieldLabel
                      label="Obfs 密码 (可选)"
                      tooltip="Salamander 混淆密码。留空则不使用混淆。用于对抗流量检测。"
                    />
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

          <section className="space-y-4">
            <h3 className="text-sm font-semibold">TLS</h3>
            {protocol === "vless" ? (
              <>
                <div className="space-y-2">
                  <FieldLabel
                    label="TLS 类型"
                    tooltip="TLS 模式。无 TLS 表示明文；TLS 使用证书；Reality 使用 Reality 协议伪装。选择后下方显示对应配置字段。"
                  />
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
                      <FieldLabel
                        label="server_name"
                        tooltip="TLS 域名。典型值：example.com。需与证书匹配。"
                      />
                      <Input
                        id="tls_server_name"
                        {...form.register("tls_server_name")}
                        placeholder="example.com"
                      />
                    </div>
                    <div className="space-y-2">
                      <FieldLabel
                        label="证书路径"
                        tooltip="证书文件路径。典型值：/etc/certs/fullchain.pem。"
                      />
                      <Input
                        id="tls_certificate_path"
                        {...form.register("tls_certificate_path")}
                        placeholder="/etc/certs/fullchain.pem"
                      />
                    </div>
                    <div className="space-y-2 sm:col-span-2">
                      <FieldLabel
                        label="私钥路径"
                        tooltip="私钥文件路径。典型值：/etc/certs/privkey.pem。"
                      />
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
                      <FieldLabel
                        label="handshake.server"
                        tooltip="Reality 握手目标域名。典型值：google.com。"
                      />
                      <Input
                        id="reality_server"
                        {...form.register("reality_server")}
                        placeholder="google.com"
                      />
                    </div>
                    <div className="space-y-2">
                      <FieldLabel
                        label="handshake.server_port"
                        tooltip="握手目标端口。典型值：443。"
                      />
                      <Input
                        id="reality_server_port"
                        type="number"
                        {...form.register("reality_server_port")}
                        placeholder="443"
                      />
                    </div>
                    <div className="space-y-2 sm:col-span-2">
                      <FieldLabel
                        label="private_key"
                        tooltip="Reality 私钥。由 sing-box generate reality-keypair 生成。"
                      />
                      <Input
                        id="reality_private_key"
                        {...form.register("reality_private_key")}
                        placeholder="Reality 私钥"
                      />
                    </div>
                    <div className="space-y-2">
                      <FieldLabel
                        label="short_id"
                        tooltip="Reality short_id。16 位十六进制。"
                      />
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
                  <FieldLabel
                    label="server_name"
                    tooltip="TLS 域名。Hysteria2 必须使用 TLS。"
                  />
                  <Input
                    id="tls_server_name"
                    {...form.register("tls_server_name")}
                    placeholder="example.com"
                  />
                </div>
                <div className="space-y-2">
                  <FieldLabel
                    label="证书路径"
                    tooltip="证书文件路径。典型值：/etc/certs/fullchain.pem。"
                  />
                  <Input
                    id="tls_certificate_path"
                    {...form.register("tls_certificate_path")}
                    placeholder="/etc/certs/fullchain.pem"
                  />
                </div>
                <div className="space-y-2 sm:col-span-2">
                  <FieldLabel
                    label="私钥路径"
                    tooltip="私钥文件路径。典型值：/etc/certs/privkey.pem。"
                  />
                  <Input
                    id="tls_key_path"
                    {...form.register("tls_key_path")}
                    placeholder="/etc/certs/privkey.pem"
                  />
                </div>
              </div>
            )}
          </section>

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
                  <FieldLabel
                    label="path"
                    tooltip="WebSocket 路径。典型值：/vless。"
                  />
                  <Input
                    id="transport_path"
                    {...form.register("transport_path")}
                    placeholder="/vless"
                  />
                </div>
              )}
              {form.watch("transport_type") === "grpc" && (
                <div className="space-y-2">
                  <FieldLabel
                    label="service_name"
                    tooltip="gRPC 服务名。典型值：TunService。需 sing-box 支持 gRPC。"
                  />
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
                    <FieldLabel
                      label="host"
                      tooltip="HTTP/2 host 头。"
                    />
                    <Input
                      id="transport_http_host"
                      {...form.register("transport_http_host")}
                      placeholder=""
                    />
                  </div>
                  <div className="space-y-2">
                    <FieldLabel
                      label="path"
                      tooltip="HTTP/2 路径。"
                    />
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
