import { useQuery } from "@tanstack/react-query"
import { ArrowUpDown, Radio, Users } from "lucide-react"
import { Card } from "@heroui/react"
import { formatBytes } from "@/lib/format"
import { useCountUp } from "@/hooks/use-count-up"
import { PageHero } from "@/components/layout/PageHero"

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
      sub: "当前已创建并可用于分配用户的入站数量。",
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
  ] satisfies Array<{
    title: string
    icon: typeof Radio
    value: string
    sub: string
    smallValue?: boolean
  }>

  return (
    <div className="p-6 space-y-6">
      <PageHero
        title="运行总览"
        description="首屏强调当前规模、活跃度和总流量，方便快速确认整套面板是否处于健康区间。"
        metrics={[
          { label: "入站", value: inboundCount.formatted },
          { label: "用户", value: userCount.formatted },
          { label: "活跃", value: activeUserCount.formatted },
        ]}
      />

      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
        {cards.map((card) => (
          <Card key={card.title}>
            <Card.Content className="space-y-4 p-5">
              <div className="flex items-center justify-between">
                <h2 className="text-sm font-medium text-foreground-500">{card.title}</h2>
                <card.icon className="size-4 text-primary" />
              </div>
              <div>
                <div className={card.smallValue ? "text-xl font-bold" : "text-3xl font-semibold"}>
                  {card.value}
                </div>
                <p className="mt-1 text-sm text-foreground-500">{card.sub}</p>
              </div>
            </Card.Content>
          </Card>
        ))}
      </div>
    </div>
  )
}
