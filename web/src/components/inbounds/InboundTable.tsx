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
  created_at: string
}

type InboundTableProps = {
  inbounds: Inbound[]
}

export function InboundTable({ inbounds }: InboundTableProps) {
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
            <TableCell>{ib.created_at}</TableCell>
            <TableCell />
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
