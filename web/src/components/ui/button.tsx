import * as React from "react"
import { cn } from "@/lib/utils"

type ButtonVariant =
  | "default"
  | "destructive"
  | "outline"
  | "secondary"
  | "ghost"
  | "link"

type ButtonSize = "default" | "sm" | "lg" | "icon"

export type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  asChild?: boolean
  variant?: ButtonVariant
  size?: ButtonSize
}

const variantClasses: Record<ButtonVariant, string> = {
  default:
    "bg-[color:var(--accent)] text-[color:var(--accent-foreground)] hover:brightness-105",
  destructive:
    "bg-[color:var(--danger)] text-[color:var(--danger-foreground)] hover:brightness-105",
  outline:
    "border border-[color:var(--border)] bg-transparent text-[color:var(--foreground)] hover:bg-[color:var(--surface-secondary)]",
  secondary:
    "bg-[color:var(--surface-secondary)] text-[color:var(--foreground)] hover:bg-[color:var(--surface-tertiary)]",
  ghost:
    "bg-transparent text-[color:var(--foreground)] hover:bg-[color:var(--surface-secondary)]",
  link: "bg-transparent px-0 py-0 text-[color:var(--accent)] hover:underline",
}

const sizeClasses: Record<ButtonSize, string> = {
  default: "h-10 px-4 py-2 text-sm",
  sm: "h-9 rounded-[calc(var(--radius)*1.2)] px-3 py-2 text-sm",
  lg: "h-11 px-6 py-2.5 text-base",
  icon: "size-9 p-0",
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      asChild = false,
      variant = "default",
      size = "default",
      type = "button",
      children,
      ...props
    },
    ref
  ) => {
    const classes = cn(
      "inline-flex items-center justify-center gap-2 rounded-[calc(var(--radius)*1.4)] font-medium transition-all duration-150 focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50",
      variantClasses[variant],
      sizeClasses[size],
      variant !== "link" && "shadow-[var(--surface-shadow)]",
      className
    )

    if (asChild && React.isValidElement(children)) {
      const child = children as React.ReactElement<{ className?: string }>
      return React.cloneElement(child, {
        className: cn(classes, child.props.className),
      })
    }

    return (
      <button ref={ref} type={type} className={classes} {...props}>
        {children}
      </button>
    )
  }
)

Button.displayName = "Button"
