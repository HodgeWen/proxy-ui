import * as React from "react"
import { Skeleton as HeroSkeleton } from "@heroui/react"
import { cn } from "@/lib/utils"

export function Skeleton({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof HeroSkeleton>) {
  return <HeroSkeleton {...props} className={cn("rounded-[calc(var(--radius)*1.2)]", className)} />
}
