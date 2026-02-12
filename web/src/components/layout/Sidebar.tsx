import { Link, useLocation } from "react-router-dom"
import {
  LayoutDashboard,
  Radio,
  Users,
  Link2,
  Shield,
  Gauge,
  Box,
} from "lucide-react"
import {
  Sidebar as ShadcnSidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"

const navItems: Array<{
  to: string
  label: string
  icon: typeof LayoutDashboard
  disabled?: boolean
}> = [
  { to: "/", label: "仪表盘", icon: LayoutDashboard },
  { to: "/inbounds", label: "入站", icon: Radio },
  { to: "/users", label: "用户", icon: Users, disabled: true },
  { to: "/subscriptions", label: "订阅", icon: Link2, disabled: true },
  { to: "/certificates", label: "证书", icon: Shield },
  { to: "/traffic", label: "流量", icon: Gauge, disabled: true },
  { to: "/core", label: "核心", icon: Box, disabled: true },
]

export function Sidebar() {
  const location = useLocation()

  return (
    <ShadcnSidebar side="left" collapsible="none" className="border-0">
      <SidebarHeader className="p-4">
        <span className="text-lg font-semibold text-sidebar-foreground">
          s-ui
        </span>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {navItems.map((item) => {
                const isActive =
                  item.to === "/"
                    ? location.pathname === "/"
                    : location.pathname.startsWith(item.to)
                return (
                  <SidebarMenuItem key={item.to}>
                      <SidebarMenuButton
                        asChild
                        isActive={isActive}
                        disabled={item.disabled ?? false}
                      >
                      <Link
                        to={item.to}
                        className={
                          item.disabled ? "pointer-events-none opacity-50" : ""
                        }
                      >
                        <item.icon className="size-4" />
                        <span>{item.label}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                )
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
    </ShadcnSidebar>
  )
}
