import { Info } from "lucide-react"
import { Label, Tooltip } from "@heroui/react"

type FieldLabelProps = {
  label: string
  tooltip?: string
  htmlFor?: string
}

export function FieldLabel({ label, tooltip, htmlFor }: FieldLabelProps) {
  return (
    <div className="flex items-center gap-1.5">
      <Label htmlFor={htmlFor}>{label}</Label>
      {tooltip ? (
        <Tooltip.Root>
          <Tooltip.Trigger>
            <Info className="size-4 cursor-help text-foreground-500" />
          </Tooltip.Trigger>
          <Tooltip.Content className="max-w-xs">
            <p className="text-xs">{tooltip}</p>
          </Tooltip.Content>
        </Tooltip.Root>
      ) : null}
    </div>
  )
}
