import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { UserTable, type User } from "@/components/users/UserTable"
import { UserFormModal, type UserForEdit } from "@/components/users/UserFormModal"

async function fetchUsers(): Promise<{ data: User[] }> {
  const res = await fetch("/api/users", { credentials: "include" })
  if (!res.ok) throw new Error("获取用户列表失败")
  return res.json()
}

export function Users() {
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [selectedIds, setSelectedIds] = useState<number[]>([])

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["users"],
    queryFn: fetchUsers,
  })

  const { data: editingUser } = useQuery({
    queryKey: ["user", editingId],
    queryFn: async (): Promise<UserForEdit> => {
      const res = await fetch(`/api/users/${editingId}`, { credentials: "include" })
      if (!res.ok) throw new Error("获取用户详情失败")
      return res.json()
    },
    enabled: !!editingId,
  })

  const handleAddUser = () => {
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
    if (!window.confirm("确定要删除此用户吗？")) return
    const res = await fetch(`/api/users/${id}`, {
      method: "DELETE",
      credentials: "include",
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      toast.error(err.error || "删除失败")
      return
    }
    toast.success("用户已删除")
    queryClient.invalidateQueries({ queryKey: ["users"] })
    setSelectedIds((prev) => prev.filter((i) => i !== id))
  }

  if (isLoading) {
    return (
      <div className="p-6 space-y-6">
        <h1 className="text-2xl font-bold">用户管理</h1>
        <p className="text-muted-foreground">加载中...</p>
      </div>
    )
  }

  if (isError) {
    return (
      <div className="p-6 space-y-6">
        <h1 className="text-2xl font-bold">用户管理</h1>
        <p className="text-destructive">{error?.message ?? "加载失败"}</p>
      </div>
    )
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold">用户管理</h1>
        <Button onClick={handleAddUser}>添加用户</Button>
      </div>
      <UserTable
        users={data?.data ?? []}
        onEdit={handleEdit}
        onDelete={handleDelete}
        selectedIds={selectedIds}
        onSelectionChange={setSelectedIds}
        isLoading={isLoading}
      />
      <UserFormModal
        open={formOpen}
        onOpenChange={(open) => {
          if (!open) setEditingId(null)
          setFormOpen(open)
        }}
        user={editingUser ?? undefined}
        onSuccess={handleEditSuccess}
      />
    </div>
  )
}
