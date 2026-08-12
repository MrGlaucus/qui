import { useEffect, useMemo, useRef } from "react"
import * as echarts from "echarts/core"
import { LineChart } from "echarts/charts"
import {
  GridComponent,
  TooltipComponent,
  LegendComponent
} from "echarts/components"
import { CanvasRenderer } from "echarts/renderers"
import { converter } from "culori"
import { useTranslation } from "react-i18next"
import { useTheme } from "@/hooks/useTheme"
import { formatBytes } from "@/lib/utils"

echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

const oklchToRgb = converter("rgb")

// resolveCssColor reads a CSS custom property value and converts oklch to a
// hex/rgb string echarts' canvas renderer can consume. Falls back to a neutral
// color when the variable is missing or unparseable.
function resolveCssColor(varName: string): string {
  const value = getComputedStyle(document.documentElement).getPropertyValue(varName).trim()
  if (!value) return "#888888"
  try {
    const rgb = oklchToRgb(value)
    if (!rgb) return "#888888"
    const { r, g, b } = rgb
    return `rgb(${Math.round(r * 255)}, ${Math.round(g * 255)}, ${Math.round(b * 255)})`
  } catch {
    return "#888888"
  }
}

export interface TrafficDataPoint {
  date: string // YYYY-MM-DD
  uploaded: number
  downloaded: number
}

interface TrafficLineChartProps {
  data: TrafficDataPoint[]
  height?: number
}

// TrafficLineChart renders a 7-day upload/download line chart backed by echarts.
// Colors are resolved from the active theme's CSS variables on each theme change
// so the chart stays visually consistent with the rest of the dashboard.
export function TrafficLineChart({ data, height = 160 }: TrafficLineChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<echarts.ECharts | null>(null)
  const { mode, theme } = useTheme()
  const { t } = useTranslation("dashboard")

  // The API returns rows newest-first (ORDER BY date DESC); always plot in
  // chronological order so the x-axis runs oldest -> newest.
  const sorted = useMemo(
    () => [...data].sort((a, b) => a.date.localeCompare(b.date)),
    [data]
  )

  useEffect(() => {
    if (!containerRef.current) return
    const chart = echarts.init(containerRef.current)
    chartRef.current = chart
    const resizeObserver = new ResizeObserver(() => chart.resize())
    resizeObserver.observe(containerRef.current)
    return () => {
      resizeObserver.disconnect()
      chart.dispose()
      chartRef.current = null
    }
  }, [])

  useEffect(() => {
    const chart = chartRef.current
    if (!chart) return

    const uploadColor = resolveCssColor("--chart-3")
    const downloadColor = resolveCssColor("--chart-2")
    const textColor = resolveCssColor("--muted-foreground")
    const borderColor = resolveCssColor("--border")
    const axisLabelColor = resolveCssColor("--muted-foreground")

    const xLabels = sorted.map((d) => d.date.slice(5)) // MM-DD

    chart.setOption({
      animation: false,
      grid: { left: 8, right: 8, top: 28, bottom: 4, containLabel: true },
      tooltip: {
        trigger: "axis",
        backgroundColor: resolveCssColor("--card"),
        borderColor,
        textStyle: { color: resolveCssColor("--foreground"), fontSize: 12 },
        formatter: (params: unknown) => {
          const items = params as Array<{
            axisValue: string
            seriesName: string
            marker: string
            value: number
          }>
          if (!Array.isArray(items) || items.length === 0) return ""
          const date = sorted.find((d) => d.date.slice(5) === items[0].axisValue)?.date ?? items[0].axisValue
          const rows = items.map((item) => {
            const value = typeof item.value === "number" ? item.value : 0
            return `${item.marker}${item.seriesName}: ${formatBytes(value)}`
          })
          return [`<strong>${date}</strong>`, ...rows].join("<br/>")
        },
      },
      legend: {
        top: 0,
        right: 8,
        itemWidth: 10,
        itemHeight: 10,
        textStyle: { color: textColor, fontSize: 11 },
        data: [t("dailyTraffic.upload"), t("dailyTraffic.download")],
      },
      xAxis: {
        type: "category",
        data: xLabels,
        boundaryGap: false,
        axisLine: { lineStyle: { color: borderColor } },
        axisLabel: { color: axisLabelColor, fontSize: 10 },
        axisTick: { show: false },
      },
      yAxis: {
        type: "value",
        axisLabel: {
          color: axisLabelColor,
          fontSize: 10,
          formatter: (value: number) => formatBytes(Math.round(value)),
        },
        splitLine: { lineStyle: { color: borderColor, opacity: 0.4 } },
      },
      series: [
        {
          name: t("dailyTraffic.upload"),
          type: "line",
          smooth: true,
          showSymbol: sorted.length <= 14,
          symbolSize: 5,
          lineStyle: { width: 2, color: uploadColor },
          itemStyle: { color: uploadColor },
          areaStyle: { color: uploadColor, opacity: 0.12 },
          data: sorted.map((d) => d.uploaded),
        },
        {
          name: t("dailyTraffic.download"),
          type: "line",
          smooth: true,
          showSymbol: sorted.length <= 14,
          symbolSize: 5,
          lineStyle: { width: 2, color: downloadColor },
          itemStyle: { color: downloadColor },
          areaStyle: { color: downloadColor, opacity: 0.12 },
          data: sorted.map((d) => d.downloaded),
        },
      ],
    })
  }, [sorted, mode, theme, t])

  return <div ref={containerRef} style={{ height }} className="w-full" />
}
