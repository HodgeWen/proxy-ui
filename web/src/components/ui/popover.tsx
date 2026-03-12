import * as React from "react"
import { cn } from "@/lib/utils"

type PopoverContextValue = {
  open: boolean
  setOpen: (open: boolean) => void
  triggerRef: React.RefObject<HTMLElement | null>
}

const PopoverContext = React.createContext<PopoverContextValue | null>(null)

function usePopoverContext() {
  const context = React.useContext(PopoverContext)
  if (!context) {
    throw new Error("Popover components must be used within Popover")
  }
  return context
}

export function Popover({
  children,
  open: controlledOpen,
  defaultOpen = false,
  onOpenChange,
}: {
  children: React.ReactNode
  open?: boolean
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
}) {
  const triggerRef = React.useRef<HTMLElement | null>(null)
  const [internalOpen, setInternalOpen] = React.useState(defaultOpen)
  const open = controlledOpen ?? internalOpen

  const setOpen = React.useCallback(
    (nextOpen: boolean) => {
      if (controlledOpen === undefined) {
        setInternalOpen(nextOpen)
      }
      onOpenChange?.(nextOpen)
    },
    [controlledOpen, onOpenChange]
  )

  React.useEffect(() => {
    if (!open) return

    const handleClick = (event: MouseEvent) => {
      const target = event.target as Node
      if (triggerRef.current?.parentElement?.contains(target)) return
      setOpen(false)
    }

    document.addEventListener("mousedown", handleClick)
    return () => document.removeEventListener("mousedown", handleClick)
  }, [open, setOpen])

  return (
    <PopoverContext.Provider value={{ open, setOpen, triggerRef }}>
      <div className="relative inline-flex">{children}</div>
    </PopoverContext.Provider>
  )
}

export function PopoverTrigger({
  asChild,
  children,
}: {
  asChild?: boolean
  children: React.ReactElement
}) {
  const { open, setOpen, triggerRef } = usePopoverContext()
  const child = children as React.ReactElement<{
    onClick?: (event: React.MouseEvent<HTMLElement>) => void
    ref?: React.Ref<HTMLElement>
    "aria-expanded"?: boolean
  }>

  if (asChild && React.isValidElement(children)) {
    return React.cloneElement(child, {
      ref: triggerRef,
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
    ref: triggerRef,
    "aria-expanded": open,
    onClick: () => setOpen(!open),
  })
}

export function PopoverContent({
  className,
  align = "center",
  children,
}: React.HTMLAttributes<HTMLDivElement> & {
  align?: "start" | "center" | "end"
}) {
  const { open } = usePopoverContext()

  if (!open) return null
  const alignClass =
    align === "start"
      ? "left-0"
      : align === "end"
        ? "right-0"
        : "left-1/2 -translate-x-1/2"

  return (
    <div
      className={cn(
        "absolute top-full z-50 mt-2 rounded-[calc(var(--radius)*1.6)] border border-[color:var(--border)] bg-[color:var(--overlay)] p-3 text-[color:var(--overlay-foreground)] shadow-[var(--overlay-shadow)]",
        alignClass,
        "min-w-full",
        className
      )}
    >
      {children}
    </div>
  )
}
