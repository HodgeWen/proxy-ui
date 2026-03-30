import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Modal,  Button  } from "@heroui/react"
import { Plus } from "lucide-react"
import {
  CertificateTable,
  type Certificate,
} from "@/components/certificates/CertificateTable"
import {
  CertificateFormModal,
  type CertificateForEdit,
} from "@/components/certificates/CertificateFormModal"
import { PageHero } from "@/components/layout/PageHero"

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

  
  const [deleteModalOpen, setDeleteModalOpen] = useState(false)
  const [deletingId, setDeletingId] = useState<number | null>(null)

  const handleDeleteClick = (id: number) => {
    setDeletingId(id)
    setDeleteModalOpen(true)
  }

  const confirmDelete = async () => {
    if (deletingId === null) return
    const id = deletingId
    setDeleteModalOpen(false)
    setDeletingId(null)

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
        <p className="text-default-500">加载中...</p>
      
      <Modal.Root isOpen={deleteModalOpen} onOpenChange={setDeleteModalOpen}>
        <Modal.Backdrop>
          <Modal.Container>
            <Modal.Dialog>
              <Modal.Header>
                <Modal.Heading>确认删除</Modal.Heading>
              </Modal.Header>
              <Modal.Body>
                <p className="text-sm text-default-500">确定要删除此证书吗？此操作无法撤销。</p>
              </Modal.Body>
              <Modal.Footer>
                <Button variant="outline" onPress={() => setDeleteModalOpen(false)}>取消</Button>
                <Button variant="danger" onPress={confirmDelete}>确认删除</Button>
              </Modal.Footer>
            </Modal.Dialog>
          </Modal.Container>
        </Modal.Backdrop>
      </Modal.Root>
    </div>
  )
}
  if (isError) {
    return (
      <div className="p-6 space-y-6">
        <h1 className="text-2xl font-bold">证书管理</h1>
        <p className="text-danger">{error?.message ?? "加载失败"}</p>
      </div>
    )
  }

  return (
    <div className="p-6 space-y-6">
      <PageHero
          title="TLS 证书台账"
          description="将证书路径维护从单行按钮页升级为带状态说明的台账面板，方便后续在入站配置里复用。"
          metrics={[
            { label: "证书数", value: String(data?.data.length ?? 0) },
            { label: "用途", value: "TLS / Reality" },
          ]}
          actions={
            <Button variant="primary" onPress={handleAdd}>
              <Plus className="size-4" />
              添加证书
            </Button>
          }
        />

      <CertificateTable
          certificates={data?.data ?? []}
          onEdit={handleEdit}
          onDelete={handleDeleteClick}
        />

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
