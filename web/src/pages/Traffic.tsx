import { useQuery } from "@tanstack/react-query"
import { ArrowUpDown, Radio, Users } from "lucide-react"
import { Card, Chip, Table, Tabs } from "@heroui/react"
import { formatBytes } from "@/lib/format"
import { useCountUp } from "@/hooks/use-count-up"
import { PageHero } from "@/components/layout/PageHero"

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

function getUserStatus(u: UserTraffic): { label: string; color: "danger" | "default" | "success" } {
  if (!u.enabled) return { label: "禁用", color: "default" }
  if (u.expire_at && new Date(u.expire_at) <= new Date()) return { label: "过期", color: "danger" }
  if (u.traffic_limit > 0 && u.traffic_used >= u.traffic_limit) return { label: "超限", color: "danger" }
  return { label: "活跃", color: "success" }
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
  ] satisfies Array<{
    title: string
    icon: typeof Radio
    value: string
    sub: string | null
    loading: boolean
    smallValue?: boolean
  }>

  return (
    <div className="p-6 space-y-6">
      <PageHero
        title="流量监控台"
        description="首屏先看总流量、活跃用户和入站规模，再深入到按入站或按用户的明细表。"
        metrics={[
          { label: "总上行", value: formatBytes(uplinkCount.value) },
          { label: "总下行", value: formatBytes(downlinkCount.value) },
          { label: "活跃用户", value: activeUsersCount.formatted },
          { label: "入站数", value: inboundLenCount.formatted },
        ]}
      />

      <div className="grid gap-6 md:grid-cols-3">
        {statCards.map((card) => (
          <Card key={card.title}>
            <Card.Content className="space-y-4">
              <div className="flex items-center justify-between">
                <h2 className="text-sm font-medium text-default-500">{card.title}</h2>
                <card.icon className="size-4 text-primary" />
              </div>
              <div className={card.smallValue ? "text-xl font-bold" : "text-2xl font-bold"}>
                {card.value}
              </div>
              {card.sub && (
                <p className="text-sm text-default-500">{card.sub}</p>
              )}
            </Card.Content>
          </Card>
        ))}
      </div>

      <Tabs.Root defaultSelectedKey="inbounds" className="space-y-4">
        <Tabs.ListContainer>
          <Tabs.List>
            <Tabs.Tab id="inbounds">按入站</Tabs.Tab>
            <Tabs.Tab id="users">按用户</Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>

        <Tabs.Panel id="inbounds">
          <Table.Root aria-label="入站流量表">
            <Table.ScrollContainer>
              <Table.Content>
                <Table.Header>
                  <Table.Column>标签</Table.Column>
                  <Table.Column>协议</Table.Column>
                  <Table.Column>上行</Table.Column>
                  <Table.Column>下行</Table.Column>
                </Table.Header>
                <Table.Body>
                  {inboundsLoading ? (
                    <Table.Row>
                      <Table.Cell colSpan={4} className="py-8 text-center text-default-500">
                        加载中...
                      </Table.Cell>
                    </Table.Row>
                  ) : inbounds.length === 0 ? (
                    <Table.Row>
                      <Table.Cell colSpan={4} className="py-8 text-center text-default-500">
                        暂无入站
                      </Table.Cell>
                    </Table.Row>
                  ) : (
                    inbounds.map((ib) => (
                      <Table.Row key={ib.id}>
                        <Table.Cell className="font-medium">{ib.tag}</Table.Cell>
                        <Table.Cell>{ib.protocol}</Table.Cell>
                        <Table.Cell>{formatBytes(ib.traffic_uplink ?? 0)}</Table.Cell>
                        <Table.Cell>{formatBytes(ib.traffic_downlink ?? 0)}</Table.Cell>
                      </Table.Row>
                    ))
                  )}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table.Root>
        </Tabs.Panel>

        <Tabs.Panel id="users">
          <Table.Root aria-label="用户流量表">
            <Table.ScrollContainer>
              <Table.Content>
                <Table.Header>
                  <Table.Column>用户名</Table.Column>
                  <Table.Column>上行</Table.Column>
                  <Table.Column>下行</Table.Column>
                  <Table.Column>状态</Table.Column>
                </Table.Header>
                <Table.Body>
                  {usersLoading ? (
                    <Table.Row>
                      <Table.Cell colSpan={4} className="py-8 text-center text-default-500">
                        加载中...
                      </Table.Cell>
                    </Table.Row>
                  ) : users.length === 0 ? (
                    <Table.Row>
                      <Table.Cell colSpan={4} className="py-8 text-center text-default-500">
                        暂无用户
                      </Table.Cell>
                    </Table.Row>
                  ) : (
                    users.map((u) => {
                      const status = getUserStatus(u)
                      return (
                        <Table.Row key={u.id}>
                          <Table.Cell className="font-medium">{u.name}</Table.Cell>
                          <Table.Cell>{formatBytes(u.traffic_uplink ?? 0)}</Table.Cell>
                          <Table.Cell>{formatBytes(u.traffic_downlink ?? 0)}</Table.Cell>
                          <Table.Cell>
                            <Chip color={status.color}>{status.label}</Chip>
                          </Table.Cell>
                        </Table.Row>
                      )
                    })
                  )}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table.Root>
        </Tabs.Panel>
      </Tabs.Root>
    </div>
  )
}
