import * as React from "react"
import { Label as HeroLabel } from "@heroui/react"

export const Label = React.forwardRef<
  HTMLLabelElement,
  React.LabelHTMLAttributes<HTMLLabelElement>
>(({ children, ...props }, ref) => {
  return (
    <HeroLabel ref={ref as never} {...props}>
      {children}
    </HeroLabel>
  )
})

Label.displayName = "Label"
