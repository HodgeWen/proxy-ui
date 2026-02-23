import { useQuery } from "@tanstack/react-query"
import { ArrowUpDown, Radio, Users } from "lucide-react"
import { formatBytes } from "@/lib/format"
import { useCountUp } from "@/hooks/use-count-up"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"

type Inbound = {
  id: number
  tag: string
  protocol: string
  traffic_uplink: number
  traffic_downlink: number
}

type UserTraffic = {
  id: number
  name: string
  traffic_uplink: number
  traffic_downlink: number
  traffic_limit: number
  traffic_used: number
  expire_at: string | null
  enabled: boolean
}

function getUserStatus(u: UserTraffic): { label: string; variant: "default" | "secondary" | "destructive" | "outline" } {
  if (!u.enabled) return { label: "禁用", variant: "secondary" }
  if (u.expire_at && new Date(u.expire_at) <= new Date()) return { label: "过期", variant: "destructive" }
  if (u.traffic_limit > 0 && u.traffic_used >= u.traffic_limit) return { label: "超限", variant: "destructive" }
  return { label: "活跃", variant: "default" }
}

export function Traffic() {
  const { data: inboundsData, isLoading: inboundsLoading } = useQuery<{ data: Inbound[] }>({
    queryKey: ["inbounds"],
    queryFn: async () => {
      const res = await fetch("/api/inbounds", { credentials: "include" })
      if (!res.ok) throw new Error("获取入站列表失败")
      return res.json()
    },
    refetchInterval: 60000,
  })

  const { data: usersData, isLoading: usersLoading } = useQuery<{ data: UserTraffic[] }>({
    queryKey: ["users"],
    queryFn: async () => {
      const res = await fetch("/api/users", { credentials: "include" })
      if (!res.ok) throw new Error("获取用户列表失败")
      return res.json()
    },
    refetchInterval: 60000,
  })

  const inbounds = inboundsData?.data ?? []
  const users = usersData?.data ?? []

  const totalUplink = inbounds.reduce((sum, ib) => sum + (ib.traffic_uplink ?? 0), 0)
  const totalDownlink = inbounds.reduce((sum, ib) => sum + (ib.traffic_downlink ?? 0), 0)
  const activeUsers = users.filter((u) => {
    if (!u.enabled) return false
    if (u.expire_at && new Date(u.expire_at) <= new Date()) return false
    if (u.traffic_limit > 0 && u.traffic_used >= u.traffic_limit) return false
    return true
  }).length

  const inboundsReady = !inboundsLoading
  const usersReady = !usersLoading

  const uplinkCount = useCountUp({ to: totalUplink, startWhen: inboundsReady })
  const downlinkCount = useCountUp({ to: totalDownlink, startWhen: inboundsReady })
  const activeUsersCount = useCountUp({ to: activeUsers, startWhen: usersReady })
  const totalUsersCount = useCountUp({ to: users.length, startWhen: usersReady })
  const inboundLenCount = useCountUp({ to: inbounds.length, startWhen: inboundsReady })

  const statCards = [
    {
      title: "总流量",
      icon: ArrowUpDown,
      value: `↑ ${formatBytes(uplinkCount.value)}`,
      sub: `↓ ${formatBytes(downlinkCount.value)}`,
      loading: inboundsLoading,
      smallValue: true,
    },
    {
      title: "活跃用户",
      icon: Users,
      value: activeUsersCount.formatted,
      sub: `共 ${totalUsersCount.formatted} 个用户`,
      loading: usersLoading,
    },
    {
      title: "入站数",
      icon: Radio,
      value: inboundLenCount.formatted,
      sub: null,
      loading: inboundsLoading,
    },
  ] as const

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-bold">流量</h1>

      <div className="grid gap-4 md:grid-cols-3">
        {statCards.map((card, index) => (
          <div
            key={card.title}
            className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none"
            style={{ animationDelay: `${index * 75}ms` }}
          >
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">{card.title}</CardTitle>
                <card.icon className="size-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className={card.smallValue ? "text-xl font-bold" : "text-2xl font-bold"}>
                  {card.value}
                </div>
                {card.sub && (
                  <p className="text-sm text-muted-foreground">{card.sub}</p>
                )}
              </CardContent>
            </Card>
          </div>
        ))}
      </div>

      <div
        className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none"
        style={{ animationDelay: "225ms" }}
      >
        <Tabs defaultValue="inbounds">
          <TabsList>
            <TabsTrigger value="inbounds">按入站</TabsTrigger>
            <TabsTrigger value="users">按用户</TabsTrigger>
          </TabsList>

          <TabsContent value="inbounds" className="mt-4">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>标签</TableHead>
                  <TableHead>协议</TableHead>
                  <TableHead>上行</TableHead>
                  <TableHead>下行</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {inboundsLoading ? (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-muted-foreground py-8">
                      加载中...
                    </TableCell>
                  </TableRow>
                ) : inbounds.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-muted-foreground py-8">
                      暂无入站
                    </TableCell>
                  </TableRow>
                ) : (
                  inbounds.map((ib) => (
                    <TableRow key={ib.id}>
                      <TableCell className="font-medium">{ib.tag}</TableCell>
                      <TableCell>{ib.protocol}</TableCell>
                      <TableCell>{formatBytes(ib.traffic_uplink ?? 0)}</TableCell>
                      <TableCell>{formatBytes(ib.traffic_downlink ?? 0)}</TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </TabsContent>

          <TabsContent value="users" className="mt-4">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>用户名</TableHead>
                  <TableHead>上行</TableHead>
                  <TableHead>下行</TableHead>
                  <TableHead>状态</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {usersLoading ? (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-muted-foreground py-8">
                      加载中...
                    </TableCell>
                  </TableRow>
                ) : users.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-muted-foreground py-8">
                      暂无用户
                    </TableCell>
                  </TableRow>
                ) : (
                  users.map((u) => {
                    const status = getUserStatus(u)
                    return (
                      <TableRow key={u.id}>
                        <TableCell className="font-medium">{u.name}</TableCell>
                        <TableCell>{formatBytes(u.traffic_uplink ?? 0)}</TableCell>
                        <TableCell>{formatBytes(u.traffic_downlink ?? 0)}</TableCell>
                        <TableCell>
                          <Badge variant={status.variant}>{status.label}</Badge>
                        </TableCell>
                      </TableRow>
                    )
                  })
                )}
              </TableBody>
            </Table>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}
