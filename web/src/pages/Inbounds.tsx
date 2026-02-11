import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { InboundTable, type Inbound } from "@/components/inbounds/InboundTable"
import { InboundFormModal } from "@/components/inbounds/InboundFormModal"

async function fetchInbounds(): Promise<{ data: Inbound[] }> {
  const res = await fetch("/api/inbounds", { credentials: "include" })
  if (!res.ok) throw new Error("获取入站列表失败")
  return res.json()
}

export function Inbounds() {
  const [formOpen, setFormOpen] = useState(false)
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["inbounds"],
    queryFn: fetchInbounds,
  })

  const handleAddInbound = () => {
    setFormOpen(true)
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
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold">入站管理</h1>
        <Button onClick={handleAddInbound}>添加入站</Button>
      </div>
      <InboundTable
        inbounds={data?.data ?? []}
        onEdit={(id) => {
          /* Plan 05: wire to form modal */
        }}
        onDelete={(id) => {
          /* Plan 05: wire to delete flow */
        }}
      />
      <InboundFormModal
        open={formOpen}
        onOpenChange={setFormOpen}
        onSuccess={() => {}}
      />
    </div>
  )
}
