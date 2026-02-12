import { Pencil, MoreHorizontal, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

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
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>名称</TableHead>
          <TableHead>证书路径</TableHead>
          <TableHead>私钥路径</TableHead>
          <TableHead>创建时间</TableHead>
          <TableHead>操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {certificates.map((cert) => (
          <TableRow key={cert.id}>
            <TableCell>{cert.name}</TableCell>
            <TableCell>{cert.fullchain_path}</TableCell>
            <TableCell>{cert.privkey_path}</TableCell>
            <TableCell>{cert.created_at}</TableCell>
            <TableCell>
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-8"
                  onClick={() => onEdit?.(cert.id)}
                  aria-label="编辑"
                >
                  <Pencil className="size-4" />
                </Button>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8"
                      aria-label="更多操作"
                    >
                      <MoreHorizontal className="size-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem
                      variant="destructive"
                      onClick={() => onDelete?.(cert.id)}
                    >
                      <Trash2 className="size-4" />
                      删除
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
