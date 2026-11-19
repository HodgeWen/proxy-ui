import { useState, useEffect, useMemo } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Modal,  Button, Input  } from "@heroui/react"
import { Search, UserPlus } from "lucide-react"
import { UserTable, type User } from "@/components/users/UserTable"
import { UserFormModal } from "@/components/users/UserFormModal"
import { UserSubscriptionModal } from "@/components/users/UserSubscriptionModal"
import { BatchActionBar } from "@/components/users/BatchActionBar"
import { PageHero } from "@/components/layout/PageHero"

async function fetchUsers(q?: string): Promise<{ data: User[] }> {
  const url = q ? `/api/users?q=${encodeURIComponent(q)}` : "/api/users"
  const res = await fetch(url, { credentials: "include" })
  if (!res.ok) throw new Error("获取用户列表失败")
  return res.json()
}

export function Users() {
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [subOpen, setSubOpen] = useState(false)
  const [subUserId, setSubUserId] = useState<number | undefined>()
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [searchInput, setSearchInput] = useState("")
  const [searchQ, setSearchQ] = useState("")

  // Debounce: only update query key 300ms after last keystroke
  useEffect(() => {
    const timer = setTimeout(() => setSearchQ(searchInput), 300)
    return () => clearTimeout(timer)
  }, [searchInput])

  const { data, isLoading, isFetching, isError, error } = useQuery({
    queryKey: ["users", searchQ],
    queryFn: () => fetchUsers(searchQ || undefined),
    placeholderData: (prev) => prev, // keep previous data while fetching new query key
    refetchInterval: 60000,
  })

  // Look up the editing user directly from the list (no separate fetch)
  const editingUser = useMemo(() => {
    if (!editingId || !data?.data) return undefined
    return data.data.find((u) => u.id === editingId)
  }, [editingId, data])

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

  const handleSubscription = (id: number) => {
    setSubUserId(id)
    setSubOpen(true)
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


  const runBatchAction = async (action: string) => {
    const res = await fetch("/api/users/batch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ action, ids: selectedIds }),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      toast.error(err.error || "批量操作失败")
      return
    }
    toast.success("操作成功")
    queryClient.invalidateQueries({ queryKey: ["users"] })
    setSelectedIds([])
  }

  const handleBatchDelete = () => runBatchAction("delete")

  return (
    <div className="p-6 space-y-6">
      <PageHero
          title="用户控制台"
          description="把搜索、批量操作和新增入口收进同一条控制轨道，避免数据页继续停留在单薄的 CRUD 头部。"
          metrics={[
            { label: "总用户", value: String(data?.data.length ?? 0) },
            { label: "已选", value: String(selectedIds.length) },
            { label: "搜索词", value: searchQ ? "已启用" : "未筛选" },
          ]}
          actions={
            <>
              <Input
                startContent={<Search className="size-4 text-default-500" />}
                placeholder="搜索用户（名称 / 备注 / UUID）"
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                className="w-full sm:w-80"
              />
              <Button variant="primary" onPress={handleAddUser}>
                <UserPlus className="size-4" />
                添加用户
              </Button>
            </>
          }
        >
          <div className="space-y-2">
            <p className="text-[11px] uppercase tracking-[0.22em] text-default-500">
              Query State
            </p>
            <p className="text-sm font-medium">
              {isFetching ? "正在刷新用户列表" : "用户列表已同步"}
            </p>
            <p className="text-sm text-default-500">
              当前搜索：{searchQ || "全部用户"}
            </p>
          </div>
        </PageHero>

      {selectedIds.length > 0 && (
        <BatchActionBar
            selectedCount={selectedIds.length}
            onDelete={handleBatchDelete}
            onEnable={() => runBatchAction("enable")}
            onDisable={() => runBatchAction("disable")}
            onResetTraffic={() => runBatchAction("reset_traffic")}
            onClearSelection={() => setSelectedIds([])}
          />
      )}

      {isError ? (
          <p className="text-danger">{error?.message ?? "加载失败"}</p>
        ) : (
          <UserTable
            users={data?.data ?? []}
            onEdit={handleEdit}
            onDelete={handleDeleteClick}
            onSubscription={handleSubscription}
            selectedIds={selectedIds}
            onSelectionChange={setSelectedIds}
            isLoading={isLoading}
          />
        )}

      <UserFormModal
        open={formOpen}
        onOpenChange={(open) => {
          if (!open) setEditingId(null)
          setFormOpen(open)
        }}
        user={editingUser}
        onSuccess={handleEditSuccess}
      />
      <UserSubscriptionModal
        open={subOpen}
        onOpenChange={(open) => {
          if (!open) setSubUserId(undefined)
          setSubOpen(open)
        }}
        userId={subUserId}
      />
    
      <Modal.Root isOpen={deleteModalOpen} onOpenChange={setDeleteModalOpen}>
        <Modal.Backdrop>
          <Modal.Container>
            <Modal.Dialog>
              <Modal.Header>
                <Modal.Heading>确认删除</Modal.Heading>
              </Modal.Header>
              <Modal.Body>
                <p className="text-sm text-default-500">确定要删除此用户吗？此操作无法撤销。</p>
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