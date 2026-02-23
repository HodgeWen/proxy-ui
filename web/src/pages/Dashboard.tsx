import { useQuery } from "@tanstack/react-query"
import { Radio, Users, ArrowUpDown } from "lucide-react"
import { formatBytes } from "@/lib/format"
import { useCountUp } from "@/hooks/use-count-up"
import { SpotlightCard } from "@/components/ui/spotlight-card"
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

  const ready = !isLoading
  const inboundCount = useCountUp({ to: stats?.inbound_count ?? 0, startWhen: ready })
  const userCount = useCountUp({ to: stats?.user_count ?? 0, startWhen: ready })
  const activeUserCount = useCountUp({ to: stats?.active_user_count ?? 0, startWhen: ready })
  const uplinkBytes = useCountUp({ to: stats?.total_uplink ?? 0, startWhen: ready })
  const downlinkBytes = useCountUp({ to: stats?.total_downlink ?? 0, startWhen: ready })

  const cards = [
    {
      title: "入站数",
      icon: Radio,
      value: inboundCount.formatted,
      sub: null,
    },
    {
      title: "用户数",
      icon: Users,
      value: userCount.formatted,
      sub: `活跃 ${activeUserCount.formatted}`,
    },
    {
      title: "总流量",
      icon: ArrowUpDown,
      value: `↑ ${formatBytes(uplinkBytes.value)}`,
      sub: `↓ ${formatBytes(downlinkBytes.value)}`,
      smallValue: true,
    },
  ] as const

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-bold">仪表盘</h1>

      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
        {cards.map((card, index) => (
          <div
            key={card.title}
            className="animate-in fade-in zoom-in-95 duration-300 fill-mode-both motion-reduce:animate-none"
            style={{ animationDelay: `${index * 75}ms` }}
          >
            <SpotlightCard>
              <Card className="transition-shadow hover:shadow-md">
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
            </SpotlightCard>
          </div>
        ))}
      </div>
    </div>
  )
}
