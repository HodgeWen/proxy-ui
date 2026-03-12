import * as React from "react"
import { ChevronDown } from "lucide-react"
import { cn } from "@/lib/utils"

type SelectItemData = {
  value: string
  label: string
}

type SelectContextValue = {
  value: string
  setValue: (value: string) => void
  options: SelectItemData[]
  placeholder?: string
  disabled?: boolean
}

const SelectContext = React.createContext<SelectContextValue | null>(null)

function useSelectContext() {
  const context = React.useContext(SelectContext)
  if (!context) {
    throw new Error("Select components must be used within Select")
  }
  return context
}

function extractText(node: React.ReactNode): string {
  if (typeof node === "string" || typeof node === "number") {
    return String(node)
  }
  if (Array.isArray(node)) {
    return node.map(extractText).join("")
  }
  if (React.isValidElement(node)) {
    return extractText((node.props as { children?: React.ReactNode }).children)
  }
  return ""
}

function collectFromChildren(children: React.ReactNode): {
  options: SelectItemData[]
  placeholder?: string
} {
  const options: SelectItemData[] = []
  let placeholder: string | undefined

  const walk = (node: React.ReactNode) => {
    React.Children.forEach(node, (child) => {
      if (!React.isValidElement(child)) return
      const props = child.props as {
        value?: string
        placeholder?: string
        children?: React.ReactNode
      }

      if (child.type === SelectItem) {
        options.push({
          value: String(props.value),
          label: extractText(props.children),
        })
      }

      if (child.type === SelectValue && props.placeholder) {
        placeholder = props.placeholder
      }

      if (props.children) {
        walk(props.children)
      }
    })
  }

  walk(children)

  return { options, placeholder }
}

export function Select({
  value,
  defaultValue,
  onValueChange,
  disabled,
  children,
}: {
  value?: string
  defaultValue?: string
  onValueChange?: (value: string) => void
  disabled?: boolean
  children: React.ReactNode
}) {
  const [internalValue, setInternalValue] = React.useState(defaultValue ?? "")
  const currentValue = value ?? internalValue
  const { options, placeholder } = React.useMemo(
    () => collectFromChildren(children),
    [children]
  )

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
    <SelectContext.Provider
      value={{
        value: currentValue,
        setValue,
        options,
        placeholder,
        disabled,
      }}
    >
      {children}
    </SelectContext.Provider>
  )
}

export function SelectTrigger({
  className,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  const { value, setValue, options, placeholder, disabled } = useSelectContext()

  return (
    <div className={cn("relative", className)} {...props}>
      <select
        className="h-10 w-full appearance-none rounded-[calc(var(--field-radius))] border border-[color:var(--field-border)] bg-[color:var(--field-background)] px-3 pr-10 text-sm text-[color:var(--field-foreground)] shadow-[var(--field-shadow)] outline-none transition-colors"
        value={value}
        disabled={disabled}
        onChange={(event) => setValue(event.target.value)}
      >
        {placeholder && <option value="">{placeholder}</option>}
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      <ChevronDown className="pointer-events-none absolute right-3 top-1/2 size-4 -translate-y-1/2 text-[color:var(--muted)]" />
      {children}
    </div>
  )
}

export function SelectValue(_valueProps: { placeholder?: string }) {
  return null
}

export function SelectContent({ children }: { children: React.ReactNode }) {
  return <>{children}</>
}

export function SelectItem(_itemProps: {
  value: string
  children: React.ReactNode
}) {
  return null
}
