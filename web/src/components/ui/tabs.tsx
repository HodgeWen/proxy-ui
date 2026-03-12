import * as React from "react"
import { cn } from "@/lib/utils"

type TabsContextValue = {
  value: string
  setValue: (value: string) => void
}

const TabsContext = React.createContext<TabsContextValue | null>(null)

function useTabsContext() {
  const context = React.useContext(TabsContext)
  if (!context) {
    throw new Error("Tabs components must be used within Tabs")
  }
  return context
}

export function Tabs({
  value,
  defaultValue,
  onValueChange,
  children,
  className,
}: {
  value?: string
  defaultValue?: string
  onValueChange?: (value: string) => void
  children: React.ReactNode
  className?: string
}) {
  const [internalValue, setInternalValue] = React.useState(defaultValue ?? "")
  const currentValue = value ?? internalValue

  const setValue = React.useCallback(
    (nextValue: string) => {
      if (value === undefined) {
        setInternalValue(nextValue)
      }
      onValueChange?.(nextValue)
    },
    [onValueChange, value]
  )

  return (
    <TabsContext.Provider value={{ value: currentValue, setValue }}>
      <div className={cn("w-full", className)}>{children}</div>
    </TabsContext.Provider>
  )
}

export function TabsList({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "inline-flex rounded-[calc(var(--radius)*1.6)] bg-[color:var(--surface-secondary)] p-1",
        className
      )}
      {...props}
    />
  )
}

export function TabsTrigger({
  className,
  value,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  value: string
}) {
  const context = useTabsContext()
  const active = context.value === value

  return (
    <button
      type="button"
      className={cn(
        "rounded-[calc(var(--radius)*1.2)] px-4 py-2 text-sm font-medium transition-colors",
        active
          ? "bg-[color:var(--surface)] text-[color:var(--foreground)] shadow-[var(--surface-shadow)]"
          : "text-[color:var(--muted)] hover:text-[color:var(--foreground)]",
        className
      )}
      onClick={() => context.setValue(value)}
      {...props}
    />
  )
}

export function TabsContent({
  className,
  value,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & {
  value: string
}) {
  const context = useTabsContext()

  if (context.value !== value) return null

  return <div className={cn("w-full", className)} {...props} />
}
