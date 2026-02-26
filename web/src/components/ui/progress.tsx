import * as React from "react"

import { cn } from "@/lib/utils"

type ProgressProps = React.ComponentProps<"div"> & {
  value?: number
}

function normalizeProgress(value?: number): number {
  if (typeof value !== "number" || Number.isNaN(value)) return 0
  if (value < 0) return 0
  if (value > 100) return 100
  return value
}

function Progress({ className, value = 0, ...props }: ProgressProps) {
  const progress = normalizeProgress(value)

  return (
    <div
      data-slot="progress"
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={progress}
      className={cn(
        "bg-secondary relative h-2 w-full overflow-hidden rounded-full",
        className,
      )}
      {...props}
    >
      <div
        data-slot="progress-indicator"
        className="bg-primary h-full w-full flex-1 transition-all duration-300 ease-out"
        style={{ transform: `translateX(-${100 - progress}%)` }}
      />
    </div>
  )
}

export { Progress }
