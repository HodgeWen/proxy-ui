import { useMemo, useState } from "react"
import { Outlet, useLocation } from "react-router-dom"
import { Menu } from "lucide-react"
import { Button } from "@heroui/react"
import { Sidebar } from "./Sidebar"

const pageMeta: Record<string, string> = {
  "/": "仪表盘",
  "/inbounds": "入站管理",
  "/users": "用户管理",
  "/subscriptions": "订阅",
  "/certificates": "证书管理",
  "/traffic": "流量",
  "/core": "核心",
}

export function AppLayout() {
  const location = useLocation()
  const [mobileOpen, setMobileOpen] = useState(false)

  const currentPage = useMemo(() => {
    const exact = pageMeta[location.pathname]
    if (exact) return exact
    const matched = Object.entries(pageMeta).find(([path]) =>
      path !== "/" ? location.pathname.startsWith(path) : false
    )
    return matched?.[1] ?? "控制台"
  }, [location.pathname])

  return (
    <div className="relative h-screen overflow-hidden px-4 py-4 md:px-6 md:py-6">
      <div className="mx-auto flex h-[calc(100vh-2rem)] max-w-[1600px] gap-6 md:h-[calc(100vh-3rem)]">
        <div className="hidden w-72 shrink-0 lg:block">
          <Sidebar />
        </div>

        <div className="flex min-w-0 flex-1 flex-col gap-4">
          <header className="flex shrink-0 items-center gap-3 px-2 py-3">
            <Button
              variant="ghost"
              isIconOnly
              className="lg:hidden"
              onPress={() => setMobileOpen(true)}
              aria-label="打开导航"
            >
              <Menu className="size-4" />
            </Button>
            <h2 className="text-lg font-semibold">{currentPage}</h2>
          </header>

          <main className="min-h-0 flex-1 overflow-y-auto rounded-xl ">
            <Outlet />
          </main>
        </div>
      </div>

      {mobileOpen && (
        <div className="fixed inset-0 z-50 flex lg:hidden">
          <button
            type="button"
            className="flex-1 bg-black/50 backdrop-blur-sm"
            onClick={() => setMobileOpen(false)}
            aria-label="关闭导航遮罩"
          />
          <div className="w-[86vw] max-w-xs">
            <Sidebar mobile open={mobileOpen} onClose={() => setMobileOpen(false)} />
          </div>
        </div>
      )}
    </div>
  )
}
