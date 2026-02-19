import { Link, useLocation, useNavigate } from "react-router-dom"
import {
  LayoutDashboard,
  Radio,
  Users,
  Link2,
  Shield,
  Gauge,
  Box,
  LogOut,
} from "lucide-react"
import {
  Sidebar as ShadcnSidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { Button } from "@/components/ui/button"

const navItems: Array<{
  to: string
  label: string
  icon: typeof LayoutDashboard
}> = [
  { to: "/", label: "仪表盘", icon: LayoutDashboard },
  { to: "/inbounds", label: "入站", icon: Radio },
  { to: "/users", label: "用户", icon: Users },
  { to: "/subscriptions", label: "订阅", icon: Link2 },
  { to: "/certificates", label: "证书", icon: Shield },
  { to: "/traffic", label: "流量", icon: Gauge },
  { to: "/core", label: "核心", icon: Box },
]

export function Sidebar() {
  const location = useLocation()
  const navigate = useNavigate()

  const handleLogout = async () => {
    await fetch("/api/logout", { method: "POST", credentials: "include" })
    navigate("/login", { replace: true })
  }

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
                    <SidebarMenuButton asChild isActive={isActive}>
                      <Link to={item.to}>
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
      <SidebarFooter className="p-4">
        <Button
          variant="ghost"
          size="sm"
          className="w-full justify-start gap-2"
          onClick={handleLogout}
        >
          <LogOut className="size-4" />
          <span>退出登录</span>
        </Button>
      </SidebarFooter>
    </ShadcnSidebar>
  )
}
