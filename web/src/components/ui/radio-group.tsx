import * as React from "react"
import { Radio as HeroRadio, RadioGroup as HeroRadioGroup } from "@heroui/react"
import { cn } from "@/lib/utils"

export type RadioGroupProps = Omit<
  React.ComponentPropsWithoutRef<typeof HeroRadioGroup>,
  "onChange"
> & {
  onValueChange?: (value: string) => void
}
export type RadioGroupItemProps = React.ComponentPropsWithoutRef<typeof HeroRadio>

export function RadioGroup({
  className,
  onValueChange,
  ...props
}: RadioGroupProps) {
  return (
    <HeroRadioGroup
      {...props}
      className={cn("gap-3", className)}
      onChange={onValueChange as never}
    />
  )
}

export function RadioGroupItem({
  className,
  children,
  ...props
}: RadioGroupItemProps) {
  return (
    <HeroRadio {...props} className={cn("mr-2", className)}>
      {children}
    </HeroRadio>
  )
}
