import * as React from "react"
import { cn } from "@/lib/utils"

type DropdownMenuContextValue = {
  open: boolean
  setOpen: (open: boolean) => void
}

const DropdownMenuContext = React.createContext<DropdownMenuContextValue | null>(null)

function useDropdownMenuContext() {
  const context = React.useContext(DropdownMenuContext)
  if (!context) {
    throw new Error("Dropdown menu components must be used within DropdownMenu")
  }
  return context
}

export function DropdownMenu({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = React.useState(false)
  const rootRef = React.useRef<HTMLDivElement | null>(null)

  React.useEffect(() => {
    if (!open) return

    const handleClick = (event: MouseEvent) => {
      if (rootRef.current?.contains(event.target as Node)) return
      setOpen(false)
    }

    document.addEventListener("mousedown", handleClick)
    return () => document.removeEventListener("mousedown", handleClick)
  }, [open])

  return (
    <DropdownMenuContext.Provider value={{ open, setOpen }}>
      <div ref={rootRef} className="relative inline-flex">
        {children}
      </div>
    </DropdownMenuContext.Provider>
  )
}

export function DropdownMenuTrigger({
  asChild,
  children,
}: {
  asChild?: boolean
  children: React.ReactElement
}) {
  const { open, setOpen } = useDropdownMenuContext()
  const child = children as React.ReactElement<{
    onClick?: (event: React.MouseEvent<HTMLElement>) => void
    "aria-expanded"?: boolean
  }>

  if (asChild && React.isValidElement(children)) {
    return React.cloneElement(child, {
      "aria-expanded": open,
      onClick: (event: React.MouseEvent<HTMLElement>) => {
        child.props.onClick?.(event)
        if (!event.defaultPrevented) {
          setOpen(!open)
        }
      },
    })
  }

  return React.cloneElement(child, {
    "aria-expanded": open,
    onClick: () => setOpen(!open),
  })
}

export function DropdownMenuContent({
  className,
  align = "center",
  children,
}: React.HTMLAttributes<HTMLDivElement> & {
  align?: "start" | "center" | "end"
}) {
  const { open } = useDropdownMenuContext()

  if (!open) return null

  const alignClass =
    align === "start"
      ? "left-0"
      : align === "end"
        ? "right-0"
        : "left-1/2 -translate-x-1/2"

  return (
    <div
      role="menu"
      className={cn(
        "absolute top-full z-50 mt-2 min-w-40 rounded-[calc(var(--radius)*1.5)] border border-[color:var(--border)] bg-[color:var(--overlay)] p-1.5 shadow-[var(--overlay-shadow)]",
        alignClass,
        className
      )}
    >
      {children}
    </div>
  )
}

export function DropdownMenuItem({
  className,
  inset,
  variant,
  onClick,
  children,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  inset?: boolean
  variant?: "default" | "destructive"
}) {
  const { setOpen } = useDropdownMenuContext()

  return (
    <button
      type="button"
      role="menuitem"
      className={cn(
        "flex w-full items-center gap-2 rounded-[calc(var(--radius)*1.1)] px-3 py-2 text-left text-sm transition-colors hover:bg-[color:var(--surface-secondary)]",
        inset && "pl-8",
        variant === "destructive" && "text-[color:var(--danger)]",
        className
      )}
      onClick={(event) => {
        onClick?.(event)
        if (!event.defaultPrevented) {
          setOpen(false)
        }
      }}
      {...props}
    >
      {children}
    </button>
  )
}
