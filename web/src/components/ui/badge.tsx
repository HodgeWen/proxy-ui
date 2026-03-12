import * as React from "react"
import { cn } from "@/lib/utils"

type BadgeVariant = "default" | "secondary" | "destructive" | "outline"

export type BadgeProps = React.ComponentPropsWithoutRef<"span"> & {
  variant?: BadgeVariant
}

const variantClasses: Record<BadgeVariant, string> = {
  default: "bg-[color:var(--accent)]/14 text-[color:var(--accent)]",
  secondary: "bg-[color:var(--surface-secondary)] text-[color:var(--foreground)]",
  destructive: "bg-[color:var(--danger)]/14 text-[color:var(--danger)]",
  outline: "border border-[color:var(--border)] text-[color:var(--foreground)]",
}

export function Badge({
  className,
  variant = "default",
  ...props
}: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium",
        variantClasses[variant],
        className
      )}
      {...props}
    />
  )
}
