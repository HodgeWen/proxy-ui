import * as React from "react"
import { cn } from "@/lib/utils"

export type InputProps = React.InputHTMLAttributes<HTMLInputElement>

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type = "text", ...props }, ref) => {
    return (
      <input
        ref={ref}
        type={type}
        className={cn(
          "flex h-10 w-full rounded-[calc(var(--field-radius))] border border-[color:var(--field-border)] bg-[color:var(--field-background)] px-3 py-2 text-sm text-[color:var(--field-foreground)] shadow-[var(--field-shadow)] transition-colors placeholder:text-[color:var(--field-placeholder)] focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50",
          className
        )}
        {...props}
      />
    )
  }
)

Input.displayName = "Input"
