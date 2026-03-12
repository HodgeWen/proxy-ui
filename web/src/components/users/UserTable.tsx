import { Pencil, MoreHorizontal, Trash2, Link } from "lucide-react"
import { Button, Checkbox, Chip, Dropdown, Table } from "@heroui/react"
import { formatBytes } from "@/lib/format"

export type SubscriptionNode = {
  name: string
  link: string
}

export type User = {
  id: number
  name: string
  remark: string
  uuid: string
  password: string
  traffic_limit: number
  traffic_used: number
  traffic_uplink: number
  traffic_downlink: number
  expire_at: string | null
  enabled: boolean
  created_at: string
  inbound_ids: number[]
  inbound_tags: string[]
  subscription_url?: string
  subscription_nodes?: SubscriptionNode[]
}

type UserTableProps = {
  users: User[]
  onEdit?: (id: number) => void
  onDelete?: (id: number) => void
  onSubscription?: (id: number) => void
  selectedIds: number[]
  onSelectionChange: (ids: number[]) => void
  isLoading?: boolean
}

function formatExpire(expireAt: string | null): string {
  if (!expireAt) return "永不过期"
  const d = new Date(expireAt)
  return d.toISOString().slice(0, 10)
}

function getStatusBadge(user: User): { label: string; color: "danger" | "default" | "success" } {
  const now = new Date()
  const expired = user.expire_at && new Date(user.expire_at) <= now
  const overLimit = user.traffic_limit > 0 && user.traffic_used >= user.traffic_limit

  if (!user.enabled) return { label: "禁用", color: "default" }
  if (expired) return { label: "过期", color: "danger" }
  if (overLimit) return { label: "超限", color: "danger" }
  return { label: "启用", color: "success" }
}

export function UserTable({
  users,
  onEdit,
  onDelete,
  onSubscription,
  selectedIds,
  onSelectionChange,
  isLoading = false,
}: UserTableProps) {
  const allSelected = users.length > 0 && users.every((u) => selectedIds.includes(u.id))
  const someSelected = selectedIds.length > 0

  const handleHeaderCheckbox = (checked: boolean | "indeterminate") => {
    if (checked === true) {
      onSelectionChange(users.map((u) => u.id))
    } else {
      onSelectionChange([])
    }
  }

  const handleRowCheckbox = (id: number, checked: boolean | "indeterminate") => {
    if (checked === true) {
      onSelectionChange([...selectedIds, id])
    } else {
      onSelectionChange(selectedIds.filter((i) => i !== id))
    }
  }

  return (
    <Table.Root aria-label="用户列表">
      <Table.ScrollContainer>
        <Table.Content>
      <Table.Header>
        <Table.Column className="w-10">
          <Checkbox.Root
            slot="selection"
            aria-label="全选当前页"
            isSelected={allSelected}
            isIndeterminate={!allSelected && someSelected}
            onChange={handleHeaderCheckbox}
          >
            <Checkbox.Control>
              <Checkbox.Indicator />
            </Checkbox.Control>
          </Checkbox.Root>
        </Table.Column>
          <Table.Column>名称</Table.Column>
          <Table.Column>备注</Table.Column>
          <Table.Column>状态</Table.Column>
          <Table.Column>上行</Table.Column>
          <Table.Column>下行</Table.Column>
          <Table.Column>到期</Table.Column>
          <Table.Column>节点</Table.Column>
          <Table.Column>操作</Table.Column>
      </Table.Header>
      <Table.Body>
        {isLoading ? (
          <Table.Row>
            <Table.Cell colSpan={9} className="text-center text-foreground-500 py-8">
              加载中...
            </Table.Cell>
          </Table.Row>
        ) : users.length === 0 ? (
          <Table.Row>
            <Table.Cell colSpan={9} className="text-center text-foreground-500 py-8">
              暂无用户
            </Table.Cell>
          </Table.Row>
        ) : (
          users.map((user) => {
            const status = getStatusBadge(user)
            return (
              <Table.Row key={user.id}>
                <Table.Cell>
                  <Checkbox.Root
                    slot="selection"
                    aria-label={`选择 ${user.name}`}
                    isSelected={selectedIds.includes(user.id)}
                    onChange={(checked) => handleRowCheckbox(user.id, checked)}
                  >
                    <Checkbox.Control>
                      <Checkbox.Indicator />
                    </Checkbox.Control>
                  </Checkbox.Root>
                </Table.Cell>
                <Table.Cell>{user.name}</Table.Cell>
                <Table.Cell>{user.remark || "—"}</Table.Cell>
                <Table.Cell>
                  <Chip color={status.color}>{status.label}</Chip>
                </Table.Cell>
                <Table.Cell>{formatBytes(user.traffic_uplink ?? 0)}</Table.Cell>
                <Table.Cell>{formatBytes(user.traffic_downlink ?? 0)}</Table.Cell>
                <Table.Cell>{formatExpire(user.expire_at)}</Table.Cell>
                <Table.Cell>{user.inbound_tags?.length ? user.inbound_tags.join(", ") : "—"}</Table.Cell>
                <Table.Cell>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      isIconOnly
                      onPress={() => onEdit?.(user.id)}
                      aria-label="编辑"
                    >
                      <Pencil className="size-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      isIconOnly
                      onPress={() => onSubscription?.(user.id)}
                      aria-label="订阅"
                    >
                      <Link className="size-4" />
                    </Button>
                    <Dropdown.Root>
                      <Dropdown.Trigger>
                        <Button
                          variant="ghost"
                          isIconOnly
                          aria-label="更多操作"
                        >
                          <MoreHorizontal className="size-4" />
                        </Button>
                      </Dropdown.Trigger>
                      <Dropdown.Popover placement="bottom end">
                        <Dropdown.Menu>
                        <Dropdown.Item
                          onAction={() => onDelete?.(user.id)}
                          className="text-danger"
                        >
                          <Trash2 className="mr-2 inline size-4" />
                          删除
                        </Dropdown.Item>
                        </Dropdown.Menu>
                      </Dropdown.Popover>
                    </Dropdown.Root>
                  </div>
                </Table.Cell>
              </Table.Row>
            )
          })
        )}
      </Table.Body>
        </Table.Content>
      </Table.ScrollContainer>
    </Table.Root>
  )
}
