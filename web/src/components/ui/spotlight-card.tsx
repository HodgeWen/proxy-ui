import type { HTMLAttributes } from "react"
import { cn } from "@/lib/utils"

type SpotlightCardProps = HTMLAttributes<HTMLDivElement>

export function SpotlightCard({
  className,
  children,
  ...props
}: SpotlightCardProps) {
  return (
    <div {...props} className={cn("group relative", className)}>
      <div className="pointer-events-none absolute inset-0 rounded-[calc(var(--radius)*2)] bg-[radial-gradient(circle_at_top,_var(--accent),_transparent_45%)] opacity-0 blur-2xl transition-opacity duration-300 group-hover:opacity-15" />
      <div className="relative">{children}</div>
    </div>
  )
}
