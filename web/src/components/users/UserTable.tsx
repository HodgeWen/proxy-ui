import { Pencil, MoreHorizontal, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Badge } from "@/components/ui/badge"
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
  selectedIds: number[]
  onSelectionChange: (ids: number[]) => void
  isLoading?: boolean
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

function formatTraffic(used: number, limit: number): string {
  if (limit === 0) return "无限制"
  const usedStr = formatBytes(used)
  const limitStr = formatBytes(limit)
  const pct = limit > 0 ? Math.round((used / limit) * 100) : 0
  return `${usedStr} / ${limitStr} (${pct}%)`
}

function formatExpire(expireAt: string | null): string {
  if (!expireAt) return "永不过期"
  const d = new Date(expireAt)
  return d.toISOString().slice(0, 10)
}

function getStatusBadge(user: User): { label: string; variant: "default" | "secondary" | "destructive" | "outline" } {
  const now = new Date()
  const expired = user.expire_at && new Date(user.expire_at) <= now
  const overLimit = user.traffic_limit > 0 && user.traffic_used >= user.traffic_limit

  if (!user.enabled) return { label: "禁用", variant: "secondary" }
  if (expired) return { label: "过期", variant: "destructive" }
  if (overLimit) return { label: "超限", variant: "destructive" }
  return { label: "启用", variant: "default" }
}

export function UserTable({
  users,
  onEdit,
  onDelete,
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
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="w-10">
            <Checkbox
              checked={allSelected ? true : someSelected ? "indeterminate" : false}
              onCheckedChange={handleHeaderCheckbox}
              aria-label="全选当前页"
            />
          </TableHead>
          <TableHead>名称</TableHead>
          <TableHead>备注</TableHead>
          <TableHead>状态</TableHead>
          <TableHead>流量</TableHead>
          <TableHead>到期</TableHead>
          <TableHead>节点</TableHead>
          <TableHead>操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {isLoading ? (
          <TableRow>
            <TableCell colSpan={8} className="text-center text-muted-foreground py-8">
              加载中...
            </TableCell>
          </TableRow>
        ) : users.length === 0 ? (
          <TableRow>
            <TableCell colSpan={8} className="text-center text-muted-foreground py-8">
              暂无用户
            </TableCell>
          </TableRow>
        ) : (
          users.map((user) => {
            const status = getStatusBadge(user)
            return (
              <TableRow key={user.id}>
                <TableCell>
                  <Checkbox
                    checked={selectedIds.includes(user.id)}
                    onCheckedChange={(c) => handleRowCheckbox(user.id, c)}
                    aria-label={`选择 ${user.name}`}
                  />
                </TableCell>
                <TableCell>{user.name}</TableCell>
                <TableCell>{user.remark || "—"}</TableCell>
                <TableCell>
                  <Badge variant={status.variant}>{status.label}</Badge>
                </TableCell>
                <TableCell>{formatTraffic(user.traffic_used, user.traffic_limit)}</TableCell>
                <TableCell>{formatExpire(user.expire_at)}</TableCell>
                <TableCell>{user.inbound_tags?.length ? user.inbound_tags.join(", ") : "—"}</TableCell>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8"
                      onClick={() => onEdit?.(user.id)}
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
                          onClick={() => onDelete?.(user.id)}
                        >
                          <Trash2 className="size-4" />
                          删除
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </TableCell>
              </TableRow>
            )
          })
        )}
      </TableBody>
    </Table>
  )
}
