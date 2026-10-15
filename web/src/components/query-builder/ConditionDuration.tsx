import { FieldHelp } from "@/components/ui/field-help"
import { Input } from "@/components/ui/input"
import type { RuleCondition } from "@/types"
import { useId } from "react"
import { useTranslation } from "react-i18next"

export function ConditionDuration({ condition, onChange }: {
  condition: RuleCondition
  onChange: (condition: RuleCondition) => void
}) {
  const { t } = useTranslation("automations")
  const id = useId()
  return (
    <div className="flex shrink-0 items-center gap-1.5 text-xs text-muted-foreground">
      <label htmlFor={id}>{t("queryBuilder.holdFor.label")}</label>
      <FieldHelp>{t("queryBuilder.holdFor.help")}</FieldHelp>
      <Input
        id={id}
        type="number"
        min={0}
        max={2592000}
        step={1}
        className="h-7 w-20 text-xs"
        value={condition.holdForSeconds ?? 0}
        onChange={event => {
          const seconds = Number(event.target.value)
          if (Number.isInteger(seconds) && seconds >= 0) {
            onChange({ ...condition, holdForSeconds: seconds || undefined })
          }
        }}
      />
    </div>
  )
}
