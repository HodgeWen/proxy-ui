import { useQuery } from "@tanstack/react-query"
import { Radio, Users, ArrowUpDown } from "lucide-react"
import { formatBytes } from "@/lib/format"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"

type StatsSummary = {
  inbound_count: number
  user_count: number
  active_user_count: number
  total_uplink: number
  total_downlink: number
}

export function Dashboard() {
  const { data: stats, isLoading } = useQuery<StatsSummary>({
    queryKey: ["stats", "summary"],
    queryFn: async () => {
      const res = await fetch("/api/stats/summary", { credentials: "include" })
      if (!res.ok) throw new Error("获取统计数据失败")
      return res.json()
    },
    refetchInterval: 30000,
  })

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-bold">仪表盘</h1>

      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
        <Card className="transition-shadow hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">入站数</CardTitle>
            <Radio className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {isLoading ? "..." : (stats?.inbound_count ?? 0)}
            </div>
          </CardContent>
        </Card>

        <Card className="transition-shadow hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">用户数</CardTitle>
            <Users className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {isLoading ? "..." : (stats?.user_count ?? 0)}
            </div>
            <p className="text-sm text-muted-foreground">
              {isLoading ? "" : `活跃 ${stats?.active_user_count ?? 0}`}
            </p>
          </CardContent>
        </Card>

        <Card className="transition-shadow hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">总流量</CardTitle>
            <ArrowUpDown className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-xl font-bold">
              {isLoading ? "..." : `↑ ${formatBytes(stats?.total_uplink ?? 0)}`}
            </div>
            <p className="text-sm text-muted-foreground">
              {isLoading ? "" : `↓ ${formatBytes(stats?.total_downlink ?? 0)}`}
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
