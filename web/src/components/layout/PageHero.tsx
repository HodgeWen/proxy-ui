import type { ReactNode } from "react"
import { cn } from "@/lib/utils"

type HeroMetric = {
  label: string
  value: string
}

type PageHeroProps = {
  title: string
  description: string
  metrics?: HeroMetric[]
  actions?: ReactNode
  children?: ReactNode
  className?: string
}

export function PageHero({
  title,
  description,
  metrics = [],
  actions,
  children,
  className,
}: PageHeroProps) {
  return (
    <div className={cn("space-y-4", className)}>
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
          <p className="text-sm text-foreground-500">{description}</p>
        </div>
        {actions && (
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            {actions}
          </div>
        )}
      </div>

      {(metrics.length > 0 || children) && (
        <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
          {metrics.map((m) => (
            <div key={m.label} className="text-sm">
              <span className="text-foreground-500">{m.label}</span>{" "}
              <span className="font-semibold">{m.value}</span>
            </div>
          ))}
          {children}
        </div>
      )}
    </div>
  )
}
