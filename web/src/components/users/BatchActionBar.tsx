import { Button } from "@/components/ui/button"

type BatchActionBarProps = {
  selectedCount: number
  onDelete: () => void
  onEnable: () => void
  onDisable: () => void
  onResetTraffic: () => void
  onClearSelection: () => void
}

export function BatchActionBar({
  selectedCount,
  onDelete,
  onEnable,
  onDisable,
  onResetTraffic,
  onClearSelection,
}: BatchActionBarProps) {
  const handleDelete = () => {
    if (window.confirm(`确定删除 ${selectedCount} 个用户？`)) {
      onDelete()
    }
  }

  return (
    <div className="flex items-center gap-2 rounded-md border bg-muted/50 px-4 py-2">
      <span className="text-sm font-medium">已选 {selectedCount} 个用户</span>
      <Button variant="outline" size="sm" onClick={handleDelete}>
        删除
      </Button>
      <Button variant="outline" size="sm" onClick={onEnable}>
        启用
      </Button>
      <Button variant="outline" size="sm" onClick={onDisable}>
        禁用
      </Button>
      <Button variant="outline" size="sm" onClick={onResetTraffic}>
        流量重置
      </Button>
      <Button variant="ghost" size="sm" onClick={onClearSelection}>
        取消选择
      </Button>
    </div>
  )
}
