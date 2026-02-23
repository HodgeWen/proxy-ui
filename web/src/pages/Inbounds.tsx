import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { InboundTable, type Inbound } from "@/components/inbounds/InboundTable"
import {
  InboundFormModal,
  type InboundForEdit,
} from "@/components/inbounds/InboundFormModal"

type SortOption = "created" | "traffic_asc" | "traffic_desc"

async function fetchInbounds(sort?: SortOption): Promise<{ data: Inbound[] }> {
  const opts: RequestInit = { credentials: "include" }
  const url =
    sort && sort !== "created"
      ? `/api/inbounds?sort=${sort}`
      : "/api/inbounds"
  const res = await fetch(url, opts)
  if (!res.ok) throw new Error("获取入站列表失败")
  return res.json()
}

async function fetchInbound(id: number): Promise<InboundForEdit> {
  const res = await fetch(`/api/inbounds/${id}`, { credentials: "include" })
  if (!res.ok) throw new Error("获取入站详情失败")
  const data = await res.json()
  return {
    ...data,
    config_json:
      typeof data.config_json === "string"
        ? data.config_json
        : JSON.stringify(data.config_json ?? {}),
  }
}

export function Inbounds() {
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [sort, setSort] = useState<SortOption>("created")

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["inbounds", sort],
    queryFn: () => fetchInbounds(sort),
    refetchInterval: 60000,
  })

  const { data: editingInbound } = useQuery({
    queryKey: ["inbound", editingId],
    queryFn: () => fetchInbound(editingId!),
    enabled: !!editingId,
  })

  const handleAddInbound = () => {
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
    if (!window.confirm("确定要删除此入站吗？配置将重新应用。")) return
    const res = await fetch(`/api/inbounds/${id}`, {
      method: "DELETE",
      credentials: "include",
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      toast.error(err.error || "删除失败")
      return
    }
    toast.success("入站已删除")
    queryClient.invalidateQueries({ queryKey: ["inbounds"] })
  }

  if (isLoading) {
    return (
      <div className="p-6 space-y-6">
        <h1 className="text-2xl font-bold">入站管理</h1>
        <p className="text-muted-foreground">加载中...</p>
      </div>
    )
  }

  if (isError) {
    return (
      <div className="p-6 space-y-6">
        <h1 className="text-2xl font-bold">入站管理</h1>
        <p className="text-destructive">{error?.message ?? "加载失败"}</p>
      </div>
    )
  }

  return (
    <div className="p-6 space-y-6">
      <div className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold">入站管理</h1>
        <div className="flex items-center gap-4">
          <Select
            value={sort}
            onValueChange={(v) => setSort(v as SortOption)}
          >
            <SelectTrigger className="w-32">
              <SelectValue placeholder="排序" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="created">默认</SelectItem>
              <SelectItem value="traffic_asc">流量升序</SelectItem>
              <SelectItem value="traffic_desc">流量降序</SelectItem>
            </SelectContent>
          </Select>
          <Button onClick={handleAddInbound}>添加入站</Button>
        </div>
      </div>
      </div>
      <div className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none" style={{ animationDelay: '75ms' }}>
      <InboundTable
        inbounds={data?.data ?? []}
        onEdit={handleEdit}
        onDelete={handleDelete}
      />
      </div>
      <InboundFormModal
        open={formOpen}
        onOpenChange={(open) => {
          if (!open) setEditingId(null)
          setFormOpen(open)
        }}
        inbound={editingInbound ?? undefined}
        onSuccess={handleEditSuccess}
      />
    </div>
  )
}
