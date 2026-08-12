import { useTranslation } from "react-i18next"
import type { InstanceDailyTraffic, DailyTrafficDataSource } from "@/types"
import { formatBytes } from "@/lib/utils"
import { formatSpeedWithUnit, useSpeedUnits } from "@/lib/speedUnits"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { ArrowDown, ArrowUp } from "lucide-react"
import { TrafficLineChart } from "./TrafficLineChart"

interface DailyTrafficCardProps {
  items: InstanceDailyTraffic[]
  isLoading?: boolean
}

function baselinePair(item: InstanceDailyTraffic): { uploaded: number; downloaded: number } | null {
  if (!item.baselineAt) return null
  if (item.dataSource === "session") {
    return { uploaded: item.baselineSessionUploaded, downloaded: item.baselineSessionDownloaded }
  }
  return { uploaded: item.baselineAlltimeUploaded, downloaded: item.baselineAlltimeDownloaded }
}

function formatBaselineTime(value: string): string {
  if (!value) return "—"
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return "—"
  return d.toLocaleString(undefined, {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function PeakSpeeds({ peakDlSpeed, peakUlSpeed }: { peakDlSpeed: number; peakUlSpeed: number }) {
  const { t } = useTranslation("dashboard")
  const [speedUnit] = useSpeedUnits()
  return (
    <>
      <p className="whitespace-nowrap">
        {t("dailyTraffic.peakDl")}: {formatSpeedWithUnit(peakDlSpeed, speedUnit)}
      </p>
      <p className="whitespace-nowrap">
        {t("dailyTraffic.peakUl")}: {formatSpeedWithUnit(peakUlSpeed, speedUnit)}
      </p>
    </>
  )
}

export function DailyTrafficCard({ items, isLoading = false }: DailyTrafficCardProps) {
  const { t } = useTranslation("dashboard")

  if (isLoading) {
    return <div className="text-sm text-muted-foreground py-4">{t("dailyTraffic.loading")}</div>
  }

  if (items.length === 0) {
    return <div className="text-sm text-muted-foreground py-4">{t("dailyTraffic.empty")}</div>
  }

  const chartData = items.map((i) => ({
    date: i.date,
    uploaded: i.uploaded,
    downloaded: i.downloaded,
  }))

  return (
    <div className="space-y-3">
      <TrafficLineChart data={chartData} />
      <div className="border rounded-lg overflow-x-auto">
        <table className="w-full text-[12px]">
          <thead>
            <tr className="bg-muted/50">
              <th className="text-left px-2 py-1.5 font-medium text-muted-foreground whitespace-nowrap">{t("dailyTraffic.date")}</th>
              <th className="text-right px-2 py-1.5 font-medium text-chart-2 whitespace-nowrap">{t("dailyTraffic.downloaded")}</th>
              <th className="text-right px-2 py-1.5 font-medium text-chart-3 whitespace-nowrap">{t("dailyTraffic.uploaded")}</th>
              <th className="text-right px-2 py-1.5 font-medium text-muted-foreground whitespace-nowrap">{t("dailyTraffic.source")}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((i) => {
              const baseline = baselinePair(i)
              return (
                <tr key={i.date} className="border-b border-border/50 last:border-0">
                  <td className="px-2 py-1.5 font-medium tabular-nums whitespace-nowrap">{i.date}</td>
                  <td className="text-right px-2 py-1.5 font-mono text-chart-2 whitespace-nowrap">
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="cursor-help">{formatBytes(i.downloaded)}</span>
                      </TooltipTrigger>
                      <TooltipContent>
                        <PeakSpeeds peakDlSpeed={i.peakDlSpeed} peakUlSpeed={i.peakUlSpeed} />
                      </TooltipContent>
                    </Tooltip>
                  </td>
                  <td className="text-right px-2 py-1.5 font-mono text-chart-3 whitespace-nowrap">
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="cursor-help">{formatBytes(i.uploaded)}</span>
                      </TooltipTrigger>
                      <TooltipContent>
                        <PeakSpeeds peakDlSpeed={i.peakDlSpeed} peakUlSpeed={i.peakUlSpeed} />
                      </TooltipContent>
                    </Tooltip>
                  </td>
                  <td className="text-right px-2 py-1.5">
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="inline-flex px-1.5 py-0.5 rounded text-[10px] bg-muted text-muted-foreground whitespace-nowrap cursor-help">
                          {t(dailyTrafficSourceLabel(i.dataSource))}
                        </span>
                      </TooltipTrigger>
                      <TooltipContent>
                        <p className="whitespace-nowrap">
                          {t("dailyTraffic.source")}: {t(dailyTrafficSourceLabel(i.dataSource))}
                        </p>
                        {baseline && (
                          <p className="whitespace-nowrap">
                            {t("dailyTraffic.baseline")}: <ArrowUp className="inline h-3 w-3" /> {formatBytes(baseline.uploaded)} <ArrowDown className="inline h-3 w-3" /> {formatBytes(baseline.downloaded)}
                          </p>
                        )}
                        {i.baselineAt && <p className="whitespace-nowrap">{t("dailyTraffic.baselineAt")}: {formatBaselineTime(i.baselineAt)}</p>}
                      </TooltipContent>
                    </Tooltip>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export function dailyTrafficSourceLabel(source: DailyTrafficDataSource): string {
  switch (source) {
    case "session":
      return "dailyTraffic.sourceSession"
    case "alltime":
      return "dailyTraffic.sourceAlltime"
    case "restart":
      return "dailyTraffic.sourceRestart"
    case "reinstall":
      return "dailyTraffic.sourceReinstall"
    default:
      return "dailyTraffic.sourceSession"
  }
}