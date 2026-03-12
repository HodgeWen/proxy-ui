import * as React from "react"
import { Check } from "lucide-react"
import { cn } from "@/lib/utils"

export type CheckedState = boolean | "indeterminate"

export type CheckboxProps = Omit<
  React.InputHTMLAttributes<HTMLInputElement>,
  "checked" | "defaultChecked" | "onChange"
> & {
  checked?: CheckedState
  defaultChecked?: CheckedState
  onCheckedChange?: (checked: CheckedState) => void
}

export const Checkbox = React.forwardRef<HTMLInputElement, CheckboxProps>(
  (
    {
      checked,
      defaultChecked,
      onCheckedChange,
      className,
      children,
      disabled,
      ...props
    },
    ref
  ) => {
    const innerRef = React.useRef<HTMLInputElement | null>(null)

    React.useImperativeHandle(ref, () => innerRef.current as HTMLInputElement)

    React.useEffect(() => {
      if (!innerRef.current) return
      innerRef.current.indeterminate = checked === "indeterminate"
    }, [checked])

    return (
      <label
        className={cn(
          "inline-flex items-center gap-2 text-sm text-[color:var(--foreground)]",
          disabled && "cursor-not-allowed opacity-50",
          className
        )}
      >
        <span className="relative inline-flex size-4 items-center justify-center overflow-hidden rounded-[4px] border border-[color:var(--field-border)] bg-[color:var(--field-background)]">
          <input
            {...props}
            ref={innerRef}
            type="checkbox"
            disabled={disabled}
            checked={checked === "indeterminate" ? false : checked}
            defaultChecked={defaultChecked === true}
            className="peer absolute inset-0 opacity-0"
            onChange={(event) => onCheckedChange?.(event.target.checked)}
          />
          <span className="absolute inset-0 hidden items-center justify-center bg-[color:var(--accent)] text-[color:var(--accent-foreground)] peer-checked:flex">
            <Check className="size-3" />
          </span>
          {checked === "indeterminate" && (
            <span className="absolute inset-0 flex items-center justify-center bg-[color:var(--accent)] text-[color:var(--accent-foreground)]">
              <span className="h-0.5 w-2 rounded-full bg-current" />
            </span>
          )}
        </span>
        {children ? <span>{children}</span> : null}
      </label>
    )
  }
)

Checkbox.displayName = "Checkbox"
