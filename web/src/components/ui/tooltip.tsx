import * as React from "react"
import { cn } from "@/lib/utils"

type TooltipContextValue = {
  open: boolean
  setOpen: (open: boolean) => void
}

const TooltipContext = React.createContext<TooltipContextValue | null>(null)

function useTooltipContext() {
  const context = React.useContext(TooltipContext)
  if (!context) {
    throw new Error("Tooltip components must be used within Tooltip")
  }
  return context
}

export function TooltipProvider({ children }: { children: React.ReactNode }) {
  return <>{children}</>
}

export function Tooltip({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = React.useState(false)

  return (
    <TooltipContext.Provider value={{ open, setOpen }}>
      <div className="relative inline-flex">{children}</div>
    </TooltipContext.Provider>
  )
}

export function TooltipTrigger({
  asChild,
  children,
}: {
  asChild?: boolean
  children: React.ReactElement
}) {
  const { setOpen } = useTooltipContext()
  const child = children as React.ReactElement<{
    onMouseEnter?: (event: React.MouseEvent<HTMLElement>) => void
    onMouseLeave?: (event: React.MouseEvent<HTMLElement>) => void
    onFocus?: (event: React.FocusEvent<HTMLElement>) => void
    onBlur?: (event: React.FocusEvent<HTMLElement>) => void
  }>

  if (asChild && React.isValidElement(children)) {
    return React.cloneElement(child, {
      onMouseEnter: (event: React.MouseEvent<HTMLElement>) => {
        child.props.onMouseEnter?.(event)
        setOpen(true)
      },
      onMouseLeave: (event: React.MouseEvent<HTMLElement>) => {
        child.props.onMouseLeave?.(event)
        setOpen(false)
      },
      onFocus: (event: React.FocusEvent<HTMLElement>) => {
        child.props.onFocus?.(event)
        setOpen(true)
      },
      onBlur: (event: React.FocusEvent<HTMLElement>) => {
        child.props.onBlur?.(event)
        setOpen(false)
      },
    })
  }

  return children
}

export function TooltipContent({
  className,
  children,
}: React.HTMLAttributes<HTMLDivElement>) {
  const { open } = useTooltipContext()

  if (!open) return null

  return (
    <div
      className={cn(
        "absolute bottom-full left-1/2 z-50 mb-2 -translate-x-1/2 whitespace-nowrap rounded-xl bg-[color:var(--overlay)] px-3 py-1.5 text-xs text-[color:var(--overlay-foreground)] shadow-[var(--overlay-shadow)]",
        className
      )}
    >
      {children}
    </div>
  )
}
