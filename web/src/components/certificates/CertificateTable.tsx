import { Pencil, MoreHorizontal, Trash2 } from "lucide-react"
import { Button, Dropdown, Table } from "@heroui/react"

export type Certificate = {
  id: number
  name: string
  fullchain_path: string
  privkey_path: string
  created_at: string
}

type CertificateTableProps = {
  certificates: Certificate[]
  onEdit?: (id: number) => void
  onDelete?: (id: number) => void
}

export function CertificateTable({
  certificates,
  onEdit,
  onDelete,
}: CertificateTableProps) {
  return (
    <Table.Root aria-label="证书列表" className="rounded-xl border border-[color:var(--border)] bg-[color:var(--surface)] shadow-[var(--surface-shadow)]">
      <Table.ScrollContainer>
        <Table.Content>
      <Table.Header>
        <Table.Column>名称</Table.Column>
        <Table.Column>证书路径</Table.Column>
        <Table.Column>私钥路径</Table.Column>
        <Table.Column>创建时间</Table.Column>
        <Table.Column>操作</Table.Column>
      </Table.Header>
      <Table.Body>
        {certificates.map((cert) => (
          <Table.Row key={cert.id}>
            <Table.Cell>{cert.name}</Table.Cell>
            <Table.Cell>{cert.fullchain_path}</Table.Cell>
            <Table.Cell>{cert.privkey_path}</Table.Cell>
            <Table.Cell>{cert.created_at}</Table.Cell>
            <Table.Cell>
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  isIconOnly
                  onPress={() => onEdit?.(cert.id)}
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
                      onAction={() => onDelete?.(cert.id)}
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
