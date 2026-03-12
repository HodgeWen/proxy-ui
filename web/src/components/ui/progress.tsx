import { cn } from "@/lib/utils"

type ProgressProps = {
  value?: number
  className?: string
}

export function Progress({ value = 0, className }: ProgressProps) {
  const safeValue = Math.max(0, Math.min(100, value))

  return (
    <div
      className={cn(
        "relative h-2.5 w-full overflow-hidden rounded-full bg-[color:var(--surface-secondary)]",
        className
      )}
    >
      <div
        className="h-full rounded-full bg-[color:var(--accent)] transition-[width] duration-300"
        style={{ width: `${safeValue}%` }}
      />
    </div>
  )
}
