import { Pencil, MoreHorizontal, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { formatBytes } from "@/lib/format"
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
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>标签</TableHead>
          <TableHead>协议</TableHead>
          <TableHead>端口</TableHead>
          <TableHead>TLS</TableHead>
          <TableHead>传输</TableHead>
          <TableHead>监听地址</TableHead>
          <TableHead>用户数</TableHead>
          <TableHead>上行</TableHead>
          <TableHead>下行</TableHead>
          <TableHead>创建时间</TableHead>
          <TableHead>操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {inbounds.map((ib) => (
          <TableRow key={ib.id}>
            <TableCell>{ib.tag}</TableCell>
            <TableCell>{ib.protocol}</TableCell>
            <TableCell>{ib.listen_port}</TableCell>
            <TableCell>{ib.tls_type}</TableCell>
            <TableCell>{ib.transport_type}</TableCell>
            <TableCell>{ib.listen}</TableCell>
            <TableCell>{ib.user_count}</TableCell>
            <TableCell>{formatBytes(ib.traffic_uplink ?? 0)}</TableCell>
            <TableCell>{formatBytes(ib.traffic_downlink ?? 0)}</TableCell>
            <TableCell>{ib.created_at}</TableCell>
            <TableCell>
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-8"
                  onClick={() => onEdit?.(ib.id)}
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
                      onClick={() => onDelete?.(ib.id)}
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
