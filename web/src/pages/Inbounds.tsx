import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Button, Chip, Dropdown } from "@heroui/react"
import { ArrowUpDown, Plus } from "lucide-react"
import { InboundTable, type Inbound } from "@/components/inbounds/InboundTable"
import {
  InboundFormModal,
  type InboundForEdit,
} from "@/components/inbounds/InboundFormModal"
import { PageHero } from "@/components/layout/PageHero"

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
        <p className="text-[color:var(--muted)]">加载中...</p>
      </div>
    )
  }

  if (isError) {
    return (
      <div className="p-6 space-y-6">
        <h1 className="text-2xl font-bold">入站管理</h1>
        <p className="text-[color:var(--danger)]">{error?.message ?? "加载失败"}</p>
      </div>
    )
  }

  return (
    <div className="p-6 space-y-6">
      <PageHero
          title="协议入口编排"
          description="把协议、TLS 与流量排序操作集中在首屏控制栏，让入站列表更像运维控制台，而不是单独的表格页。"
          metrics={[
            { label: "总入站", value: String(data?.data.length ?? 0) },
            {
              label: "排序",
              value:
                sort === "created"
                  ? "默认"
                  : sort === "traffic_asc"
                    ? "流量升序"
                    : "流量降序",
            },
          ]}
          actions={
            <>
              <Dropdown.Root>
                <Dropdown.Trigger>
                  <Button variant="secondary">
                    <ArrowUpDown className="size-4" />
                    排序
                  </Button>
                </Dropdown.Trigger>
                <Dropdown.Popover placement="bottom end">
                  <Dropdown.Menu>
                    <Dropdown.Item onAction={() => setSort("created")}>
                      默认
                    </Dropdown.Item>
                    <Dropdown.Item onAction={() => setSort("traffic_asc")}>
                      流量升序
                    </Dropdown.Item>
                    <Dropdown.Item onAction={() => setSort("traffic_desc")}>
                      流量降序
                    </Dropdown.Item>
                  </Dropdown.Menu>
                </Dropdown.Popover>
              </Dropdown.Root>
              <Button variant="primary" onPress={handleAddInbound}>
                <Plus className="size-4" />
                添加入站
              </Button>
            </>
          }
        >
          <div className="space-y-2">
            <p className="text-[11px] uppercase tracking-[0.22em] text-[color:var(--muted)]">
              Sort Preset
            </p>
            <Chip variant="secondary" className="border-[color:var(--border)] bg-[color:var(--surface-secondary)]/74">
              {sort === "created" ? "按创建时间" : sort === "traffic_asc" ? "按低流量优先" : "按高流量优先"}
            </Chip>
          </div>
        </PageHero>

      <InboundTable
          inbounds={data?.data ?? []}
          onEdit={handleEdit}
          onDelete={handleDelete}
        />

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
