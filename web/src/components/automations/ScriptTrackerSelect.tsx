import { useHistoricalTrackers } from "@/hooks/useHistoricalTrackers"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { MultiSelect, type Option } from "@/components/ui/multi-select"
import { FieldHelp } from "@/components/ui/field-help"
import { buildTrackerCustomizationMaps, useTrackerCustomizations } from "@/hooks/useTrackerCustomizations"
import { api } from "@/lib/api"

export function ScriptTrackerSelect({ instanceIds, selected, onChange }: {
  instanceIds: number[]
  selected: string[]
  onChange: (domains: string[]) => void
}) {
  const { t } = useTranslation("automations")
  const history = useHistoricalTrackers(instanceIds.length > 0)
  const customizations = useTrackerCustomizations()
  const trackers = useQuery({
    queryKey: ["script-task-trackers", instanceIds],
    enabled: instanceIds.length > 0,
    queryFn: async () => {
      const results = await Promise.allSettled(instanceIds.map(id => api.getActiveTrackers(id)))
      return {
        domains: results.flatMap(result => result.status === "fulfilled" ? Object.keys(result.value) : []),
        failed: results.some(result => result.status === "rejected"),
      }
    },
    staleTime: 30000,
  })
  const { domainToCustomization } = buildTrackerCustomizationMaps(customizations.data)
  const options = new Map<string, Option>()
  for (const domain of [...(trackers.data?.domains ?? []), ...(history.data ?? []), ...selected]) {
    const custom = domainToCustomization.get(domain.toLowerCase())
    const value = custom?.domains.join(",") ?? domain
    options.set(value, { value, label: custom?.displayName ?? domain })
    // Preserve legacy subsets instead of silently expanding their scope.
    if (custom && selected.includes(domain) && !custom.domains.every(item => selected.includes(item))) {
      options.set(domain, { value: domain, label: `${custom.displayName} (${domain})` })
    }
  }
  const values = [...options.keys()].filter(value => value.split(",").every(domain => selected.includes(domain)))
  return <div className="space-y-2">
    <div className="flex items-center gap-2 text-sm font-medium">{t("scripts.trackers")}<FieldHelp>{t("scripts.trackersHelp")}</FieldHelp></div>
    <MultiSelect title={t("scripts.trackers")} options={[...options.values()].sort((a, b) => a.label.localeCompare(b.label))} selected={values}
      onChange={next => onChange([...new Set(next.flatMap(value => value.split(",")))])}
      placeholder={t("scripts.trackersPlaceholder")} disabled={instanceIds.length === 0} />
    {(trackers.data?.failed || trackers.isError || history.isError || customizations.isError) && <p role="alert" className="text-sm text-destructive">{t("scripts.trackersError")}</p>}
  </div>
}
