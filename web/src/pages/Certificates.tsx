import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import {
  CertificateTable,
  type Certificate,
} from "@/components/certificates/CertificateTable"
import {
  CertificateFormModal,
  type CertificateForEdit,
} from "@/components/certificates/CertificateFormModal"

async function fetchCertificates(): Promise<{ data: Certificate[] }> {
  const res = await fetch("/api/certs", { credentials: "include" })
  if (!res.ok) throw new Error("获取证书列表失败")
  return res.json()
}

async function fetchCertificate(id: number): Promise<CertificateForEdit> {
  const res = await fetch(`/api/certs/${id}`, { credentials: "include" })
  if (!res.ok) throw new Error("获取证书详情失败")
  return res.json()
}

export function Certificates() {
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["certificates"],
    queryFn: fetchCertificates,
  })

  const { data: editingCert } = useQuery({
    queryKey: ["certificate", editingId],
    queryFn: () => fetchCertificate(editingId!),
    enabled: !!editingId,
  })

  const handleAdd = () => {
    setEditingId(null)
    setFormOpen(true)
  }

  const handleEdit = (id: number) => {
    setEditingId(id)
    setFormOpen(true)
  }

  const handleEditSuccess = () => {
    setEditingId(null)
  }

  const handleDelete = async (id: number) => {
    if (!window.confirm("确定要删除此证书吗？")) return
    const res = await fetch(`/api/certs/${id}`, {
      method: "DELETE",
      credentials: "include",
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      toast.error(err.error || "删除失败")
      return
    }
    toast.success("证书已删除")
    queryClient.invalidateQueries({ queryKey: ["certificates"] })
  }

  if (isLoading) {
    return (
      <div className="p-6 space-y-6">
        <h1 className="text-2xl font-bold">证书管理</h1>
        <p className="text-muted-foreground">加载中...</p>
      </div>
    )
  }

  if (isError) {
    return (
      <div className="p-6 space-y-6">
        <h1 className="text-2xl font-bold">证书管理</h1>
        <p className="text-destructive">{error?.message ?? "加载失败"}</p>
      </div>
    )
  }

  return (
    <div className="p-6 space-y-6">
      <div className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold">证书管理</h1>
        <Button onClick={handleAdd}>添加证书</Button>
      </div>
      </div>
      <div className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none" style={{ animationDelay: '75ms' }}>
      <CertificateTable
        certificates={data?.data ?? []}
        onEdit={handleEdit}
        onDelete={handleDelete}
      />
      </div>
      <CertificateFormModal
        open={formOpen}
        onOpenChange={(open) => {
          if (!open) setEditingId(null)
          setFormOpen(open)
        }}
        certificate={editingCert ?? undefined}
        onSuccess={handleEditSuccess}
      />
    </div>
  )
}
