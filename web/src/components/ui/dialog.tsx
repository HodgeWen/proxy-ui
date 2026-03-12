import * as React from "react"
import { createPortal } from "react-dom"
import { X } from "lucide-react"
import { cn } from "@/lib/utils"

type DialogContextValue = {
  open: boolean
  setOpen: (open: boolean) => void
}

const DialogContext = React.createContext<DialogContextValue | null>(null)

function useDialogContext() {
  const context = React.useContext(DialogContext)
  if (!context) {
    throw new Error("Dialog components must be used within Dialog")
  }
  return context
}

type DialogProps = {
  open?: boolean
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
  children: React.ReactNode
}

export function Dialog({
  open,
  defaultOpen = false,
  onOpenChange,
  children,
}: DialogProps) {
  const [internalOpen, setInternalOpen] = React.useState(defaultOpen)
  const isControlled = open !== undefined
  const currentOpen = isControlled ? open : internalOpen

  const setOpen = React.useCallback(
    (nextOpen: boolean) => {
      if (!isControlled) {
        setInternalOpen(nextOpen)
      }
      onOpenChange?.(nextOpen)
    },
    [isControlled, onOpenChange]
  )

  return (
    <DialogContext.Provider value={{ open: currentOpen, setOpen }}>
      {children}
    </DialogContext.Provider>
  )
}

export function DialogTrigger({
  asChild,
  children,
}: {
  asChild?: boolean
  children: React.ReactElement
}) {
  const { setOpen } = useDialogContext()
  const child = children as React.ReactElement<{ onClick?: (event: React.MouseEvent<HTMLElement>) => void }>

  if (asChild && React.isValidElement(children)) {
    return React.cloneElement(child, {
      onClick: (event: React.MouseEvent<HTMLElement>) => {
        child.props.onClick?.(event)
        if (!event.defaultPrevented) {
          setOpen(true)
        }
      },
    })
  }

  return React.cloneElement(child, {
    onClick: (event: React.MouseEvent<HTMLElement>) => {
      child.props.onClick?.(event)
      if (!event.defaultPrevented) {
        setOpen(true)
      }
    },
  })
}

export function DialogContent({
  className,
  children,
}: React.HTMLAttributes<HTMLDivElement>) {
  const { open, setOpen } = useDialogContext()
  const [mounted, setMounted] = React.useState(false)

  React.useEffect(() => {
    setMounted(true)
  }, [])

  React.useEffect(() => {
    if (!open) return

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false)
      }
    }

    const originalOverflow = document.body.style.overflow
    document.body.style.overflow = "hidden"
    window.addEventListener("keydown", onKeyDown)

    return () => {
      document.body.style.overflow = originalOverflow
      window.removeEventListener("keydown", onKeyDown)
    }
  }, [open, setOpen])

  if (!mounted || !open) return null

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/55 px-4 py-8 backdrop-blur-sm"
      onMouseDown={() => setOpen(false)}
    >
      <div
        className={cn(
          "relative flex w-full max-w-lg flex-col gap-4 overflow-hidden rounded-[calc(var(--radius)*2)] border border-[color:var(--border)] bg-[color:var(--overlay)] p-6 text-[color:var(--overlay-foreground)] shadow-[var(--overlay-shadow)]",
          className
        )}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <button
          type="button"
          className="absolute right-4 top-4 inline-flex size-8 items-center justify-center rounded-full text-[color:var(--muted)] transition-colors hover:bg-[color:var(--surface-secondary)] hover:text-[color:var(--foreground)]"
          onClick={() => setOpen(false)}
          aria-label="关闭"
        >
          <X className="size-4" />
        </button>
        {children}
      </div>
    </div>,
    document.body
  )
}

export function DialogHeader({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("space-y-1.5 pr-8", className)} {...props} />
}

export function DialogFooter({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "flex flex-col-reverse gap-3 pt-2 sm:flex-row sm:justify-end",
        className
      )}
      {...props}
    />
  )
}

export function DialogTitle({
  className,
  ...props
}: React.HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h2
      className={cn("text-lg font-semibold tracking-tight", className)}
      {...props}
    />
  )
}

export function DialogDescription({
  className,
  ...props
}: React.HTMLAttributes<HTMLParagraphElement>) {
  return (
    <p
      className={cn("text-sm text-[color:var(--muted)]", className)}
      {...props}
    />
  )
}

export function DialogClose({
  asChild,
  children,
}: {
  asChild?: boolean
  children: React.ReactElement
}) {
  const { setOpen } = useDialogContext()
  const child = children as React.ReactElement<{ onClick?: (event: React.MouseEvent<HTMLElement>) => void }>

  if (asChild && React.isValidElement(children)) {
    return React.cloneElement(child, {
      onClick: (event: React.MouseEvent<HTMLElement>) => {
        child.props.onClick?.(event)
        if (!event.defaultPrevented) {
          setOpen(false)
        }
      },
    })
  }

  return React.cloneElement(child, {
    onClick: () => setOpen(false),
  })
}
