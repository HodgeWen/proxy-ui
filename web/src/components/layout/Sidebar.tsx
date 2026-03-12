import { Link, useLocation, useNavigate } from "react-router-dom"
import {
  Box,
  Gauge,
  Link2,
  LogOut,
  Moon,
  Radio,
  Shield,
  Sun,
  Users,
  LayoutDashboard,
  X,
} from "lucide-react"
import { Button } from "@heroui/react"
import { cn } from "@/lib/utils"
import { useTheme } from "@/components/theme-provider"

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

type SidebarProps = {
  mobile?: boolean
  open?: boolean
  onClose?: () => void
}

export function Sidebar({
  mobile = false,
  open = true,
  onClose,
}: SidebarProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const { resolvedTheme, setTheme } = useTheme()

  const handleLogout = async () => {
    await fetch("/api/logout", { method: "POST", credentials: "include" })
    navigate("/login", { replace: true })
  }

  const toggleTheme = () => {
    setTheme(resolvedTheme === "dark" ? "light" : "dark")
  }

  return (
    <aside
      className={cn(
        "flex h-full w-full max-w-72 flex-col rounded-2xl border border-[color:var(--border)] bg-[color:var(--surface)]/92 p-4 shadow-[var(--surface-shadow)] backdrop-blur-xl",
        mobile &&
          "rounded-none border-0 bg-[color:var(--surface)] px-5 pb-5 pt-4 shadow-none"
      )}
      data-open={open}
    >
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="flex size-12 items-center justify-center rounded-[1.4rem] bg-[color:var(--accent)]/12 text-lg font-semibold text-[color:var(--accent)]">
            S
          </div>
          <div>
            <p className="text-xs uppercase tracking-[0.24em] text-[color:var(--muted)]">
              sing-box panel
            </p>
            <h1 className="text-2xl font-semibold tracking-tight">s-ui</h1>
          </div>
        </div>
        {mobile && (
          <Button variant="ghost" isIconOnly onPress={onClose} aria-label="关闭导航">
            <X className="size-4" />
          </Button>
        )}
      </div>

      <nav className="mt-6 flex flex-1 flex-col gap-1.5">
        {navItems.map((item) => {
          const isActive =
            item.to === "/"
              ? location.pathname === "/"
              : location.pathname.startsWith(item.to)

          return (
            <Link
              key={item.to}
              to={item.to}
              onClick={onClose}
              className={cn(
                "flex items-center gap-3 rounded-lg px-3 py-3 text-sm font-medium transition-all duration-200",
                isActive
                  ? "bg-[color:var(--accent)]/10 text-[color:var(--accent)]"
                  : "text-[color:var(--muted)] hover:bg-[color:var(--surface-secondary)] hover:text-[color:var(--foreground)]"
              )}
            >
              <item.icon className="size-4" />
              <span>{item.label}</span>
            </Link>
          )
        })}
      </nav>

      <div className="mt-auto space-y-2">
        <Button
          variant="secondary"
          className="w-full justify-start gap-2"
          onPress={toggleTheme}
        >
          {resolvedTheme === "dark" ? (
            <Sun className="size-4" />
          ) : (
            <Moon className="size-4" />
          )}
          <span>{resolvedTheme === "dark" ? "切到亮色" : "切到暗色"}</span>
        </Button>
        <Button
          variant="ghost"
          className="w-full justify-start gap-2"
          onPress={handleLogout}
        >
          <LogOut className="size-4" />
          <span>退出登录</span>
        </Button>
      </div>
    </aside>
  )
}
