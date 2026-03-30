import { useState } from "react"
import { Button, Card, Modal } from "@heroui/react"

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
  const [confirmOpen, setConfirmOpen] = useState(false)

  const handleConfirmDelete = () => {
    setConfirmOpen(false)
    onDelete()
  }

  return (
    <>
      <Card className="fixed bottom-6 left-1/2 z-50 w-[90%] -translate-x-1/2 shadow-medium md:w-auto border border-divider">
        <Card.Content className="flex flex-col gap-3 px-6 py-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex items-center gap-3">
            <span className="text-sm font-medium">已选 {selectedCount} 个用户</span>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" size="sm" onPress={() => setConfirmOpen(true)}>
              删除
            </Button>
            <Button variant="outline" size="sm" onPress={onEnable}>
              启用
            </Button>
            <Button variant="outline" size="sm" onPress={onDisable}>
              禁用
            </Button>
            <Button variant="outline" size="sm" onPress={onResetTraffic}>
              流量重置
            </Button>
            <Button variant="ghost" size="sm" onPress={onClearSelection}>
              取消选择
            </Button>
          </div>
        </Card.Content>
      </Card>

      <Modal.Root isOpen={confirmOpen} onOpenChange={setConfirmOpen}>
        <Modal.Backdrop>
          <Modal.Container>
            <Modal.Dialog>
              <Modal.Header>
                <Modal.Heading>确认删除</Modal.Heading>
              </Modal.Header>
              <Modal.Body>
                <p className="text-sm text-foreground-500">
                  确定删除 {selectedCount} 个用户吗？此操作无法撤销。
                </p>
              </Modal.Body>
              <Modal.Footer>
                <Button variant="outline" onPress={() => setConfirmOpen(false)}>
                  取消
                </Button>
                <Button variant="primary" onPress={handleConfirmDelete}>
                  确定删除
                </Button>
              </Modal.Footer>
            </Modal.Dialog>
          </Modal.Container>
        </Modal.Backdrop>
      </Modal.Root>
    </>
  )
}
