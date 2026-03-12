import { Pencil, MoreHorizontal, Trash2 } from "lucide-react"
import { Button, Dropdown, Table } from "@heroui/react"
import { formatBytes } from "@/lib/format"

export type Inbound = {
  id: number
  tag: string
  protocol: string
  listen: string
  listen_port: number
  tls_type: string
  transport_type: string
  user_count: number
  traffic_uplink: number
  traffic_downlink: number
  created_at: string
}

type InboundTableProps = {
  inbounds: Inbound[]
  onEdit?: (id: number) => void
  onDelete?: (id: number) => void
}

export function InboundTable({
  inbounds,
  onEdit,
  onDelete,
}: InboundTableProps) {
  return (
    <Table.Root aria-label="入站列表" className="rounded-xl border border-[color:var(--border)] bg-[color:var(--surface)] shadow-[var(--surface-shadow)]">
      <Table.ScrollContainer>
        <Table.Content>
      <Table.Header>
        <Table.Column>标签</Table.Column>
        <Table.Column>协议</Table.Column>
        <Table.Column>端口</Table.Column>
        <Table.Column>TLS</Table.Column>
        <Table.Column>传输</Table.Column>
        <Table.Column>监听地址</Table.Column>
        <Table.Column>用户数</Table.Column>
        <Table.Column>上行</Table.Column>
        <Table.Column>下行</Table.Column>
        <Table.Column>创建时间</Table.Column>
        <Table.Column>操作</Table.Column>
      </Table.Header>
      <Table.Body>
        {inbounds.map((ib) => (
          <Table.Row key={ib.id}>
            <Table.Cell>{ib.tag}</Table.Cell>
            <Table.Cell>{ib.protocol}</Table.Cell>
            <Table.Cell>{ib.listen_port}</Table.Cell>
            <Table.Cell>{ib.tls_type}</Table.Cell>
            <Table.Cell>{ib.transport_type}</Table.Cell>
            <Table.Cell>{ib.listen}</Table.Cell>
            <Table.Cell>{ib.user_count}</Table.Cell>
            <Table.Cell>{formatBytes(ib.traffic_uplink ?? 0)}</Table.Cell>
            <Table.Cell>{formatBytes(ib.traffic_downlink ?? 0)}</Table.Cell>
            <Table.Cell>{ib.created_at}</Table.Cell>
            <Table.Cell>
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  isIconOnly
                  onPress={() => onEdit?.(ib.id)}
                  aria-label="编辑"
                >
                  <Pencil className="size-4" />
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
                      onAction={() => onDelete?.(ib.id)}
                      className="text-[color:var(--danger)]"
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
        ))}
      </Table.Body>
        </Table.Content>
      </Table.ScrollContainer>
    </Table.Root>
  )
}
