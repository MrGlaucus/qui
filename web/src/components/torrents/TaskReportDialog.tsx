import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle
} from "@/components/ui/dialog"
import { Progress } from "@/components/ui/progress"
import { Separator } from "@/components/ui/separator"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { TrackerIconImage } from "@/components/ui/tracker-icon"
import { useIspData } from "@/contexts/IspDataContext"
import { useDateTimeFormatters } from "@/hooks/useDateTimeFormatters"
import { useIsMobile } from "@/hooks/useMediaQuery"
import { useTrackerIcons } from "@/hooks/useTrackerIcons"
import { api } from "@/lib/api"
import { getCountryName } from "@/lib/countryNames"
import { formatSpeedWithUnit, useSpeedUnits, type SpeedUnit } from "@/lib/speedUnits"
import { getStateLabel } from "@/lib/torrent-state-utils"
import { cn, formatBytes } from "@/lib/utils"
import type { SortedPeer, Torrent, TorrentProperties } from "@/types"
import { useQuery } from "@tanstack/react-query"
import "flag-icons/css/flag-icons.min.css"
import { Camera, Loader2, Pause, Play, X, ArrowLeft } from "lucide-react"
import { memo, useCallback, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

interface TaskReportDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  instanceId: number
  torrent: Torrent
}

function formatDate(ts: number, formatters: ReturnType<typeof useDateTimeFormatters>): string {
  if (!ts || ts <= 0) return "—"
  try {
    return formatters.formatDate(new Date(ts * 1000))
  } catch {
    return "—"
  }
}

function formatTime(seconds: number): string {
  if (!seconds || seconds <= 0) return "—"
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function formatProgress(progress: number): string {
  if (progress >= 0.99 && progress < 1) {
    return (Math.floor(progress * 1000) / 10).toFixed(1)
  }
  return String(Math.round(progress * 100))
}

interface PeerCountryStat {
  code: string
  name: string
  count: number
  pct: number
}

interface PeerIspStat {
  isp: string
  count: number
  pct: number
}

const MAX_BAR_PCT = 90

function ReportContent({
  torrent,
  properties,
  peers,
  trackerIcons,
  ispData,
  speedUnit,
  formatters,
  t,
}: {
  torrent: Torrent
  properties: TorrentProperties | undefined
  peers: SortedPeer[] | undefined
  trackerIcons: Record<string, string> | undefined
  ispData: Record<string, string | null | "loading">
  speedUnit: SpeedUnit
  formatters: ReturnType<typeof useDateTimeFormatters>
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: any
}) {
  const stateLabel = getStateLabel(torrent.state, t)
  const isDownloading = ["downloading", "forcedDL", "metaDL", "stalledDL", "checkingDL", "queuedDL", "allocating"].includes(torrent.state)
  const isCompleted = torrent.progress >= 1
  const tags = torrent.tags ? torrent.tags.split(",").map((s: string) => s.trim()).filter(Boolean) : []

  const countryStats = useMemo(() => {
    if (!peers || peers.length === 0) return [] as PeerCountryStat[]
    const map = new Map<string, { code: string; count: number }>()
    for (const p of peers) {
      const code = p.country_code || "??"
      const entry = map.get(code) || { code, count: 0 }
      entry.count++
      map.set(code, entry)
    }
    const total = peers.length
    return Array.from(map.entries())
      .map(([code, { count }]) => {
        const name = getCountryName(code, undefined, t)
        return { code, name, count, pct: (count / total) * 100 }
      })
      .sort((a, b) => b.count - a.count)
  }, [peers, t])

  const ispStats = useMemo(() => {
    if (!peers || peers.length === 0) return [] as PeerIspStat[]
    const peerIps = new Set(peers.map((p) => p.ip))
    const entries = Object.entries(ispData).filter(
      (e): e is [string, string] => peerIps.has(e[0]) && typeof e[1] === "string" && e[1] !== "loading"
    )
    if (entries.length === 0) return [] as PeerIspStat[]
    const map = new Map<string, number>()
    for (const [, isp] of entries) {
      map.set(isp, (map.get(isp) || 0) + 1)
    }
    const total = Array.from(map.values()).reduce((a, b) => a + b, 0)
    return Array.from(map.entries())
      .map(([isp, count]) => ({ isp, count, pct: (count / total) * 100 }))
      .sort((a, b) => b.count - a.count)
  }, [ispData, peers])

  const maxCountryPct = countryStats.length > 0 ? Math.max(...countryStats.map((c) => c.pct)) : 100
  const maxIspPct = ispStats.length > 0 ? Math.max(...ispStats.map((i) => i.pct)) : 100

  return (
    <div className="space-y-5">
      {/* Basic Info */}
      <section className="space-y-2">
        <div className="flex items-start justify-between gap-3">
          <div className="flex-1 min-w-0 flex items-center gap-2">
            {torrent.tracker && (
              <TrackerIconImage tracker={torrent.tracker} trackerIcons={trackerIcons ?? {}} />
            )}
            <h3 className="text-sm font-semibold break-words leading-snug min-w-0">{torrent.name}</h3>
          </div>
          <Badge variant={isDownloading ? "default" : isCompleted ? "secondary" : "outline"}>
            {stateLabel}
          </Badge>
        </div>
        <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span>
            <Badge variant="outline" className="text-[10px] px-1.5 py-0 font-normal">
              {torrent.category || t("reportDialog.uncategorized")}
            </Badge>
          </span>
          <span className="flex flex-wrap gap-1 items-center">
            {tags.length > 0 ? tags.map((tag: string) => (
              <Badge key={tag} variant="outline" className="text-[10px] px-1.5 py-0">{tag}</Badge>
            )) : (
              <Badge variant="outline" className="text-[10px] px-1.5 py-0 font-normal text-muted-foreground">
                {t("reportDialog.noTags")}
              </Badge>
            )}
          </span>
        </div>
        {!isCompleted && (
          <div className="flex items-center gap-2">
            <Progress value={(torrent.progress || 0) * 100} className="h-1.5 flex-1" />
            <span className="text-xs text-muted-foreground tabular-nums shrink-0">
              {formatProgress(torrent.progress || 0)}%
            </span>
          </div>
        )}
        <Separator className="opacity-30" />
        <div className="grid grid-cols-2 md:grid-cols-4 gap-x-4 gap-y-2 text-xs">
          <div>
            <span className="text-muted-foreground">{t("reportDialog.size")}</span>
            <p className="font-medium">{formatBytes(torrent.size || 0)}</p>
          </div>
          <div>
            <span className="text-muted-foreground">{t("reportDialog.downloaded")}</span>
            <p className="font-medium">{formatBytes(torrent.downloaded || 0)}</p>
          </div>
          <div>
            <span className="text-muted-foreground">{t("reportDialog.uploaded")}</span>
            <p className="font-medium">{formatBytes(torrent.uploaded || 0)}</p>
          </div>
          <div>
            <span className="text-muted-foreground">{t("reportDialog.ratio")}</span>
            <p className="font-medium" style={{ color: `var(--ratio-${(torrent.ratio || 0) >= 3 ? "best" : (torrent.ratio || 0) >= 1 ? "good" : (torrent.ratio || 0) >= 0.5 ? "almost" : "bad"})` }}>
              {(torrent.ratio || 0).toFixed(2)}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span>
            <span className="font-medium text-foreground">{t("reportDialog.addedOn")}:</span>{" "}
            {formatDate(torrent.added_on, formatters)}
          </span>
          <span>
            <span className="font-medium text-foreground">{t("reportDialog.completedOn")}:</span>{" "}
            {formatDate(torrent.completion_on, formatters)}
          </span>
          <span>
            <span className="font-medium text-foreground">{t("reportDialog.seedingTime")}:</span>{" "}
            {formatTime(torrent.seeding_time)}
          </span>
        </div>
      </section>

      {/* Speed */}
      <section className="space-y-2">
        <h4 className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
          {t("reportDialog.speed")}
        </h4>
        <div className="border rounded-lg overflow-hidden">
          <table className="w-full text-xs">
            <thead>
              <tr className="bg-muted/50">
                <th className="text-left px-2 sm:px-3 py-1.5 font-medium text-muted-foreground w-14 sm:w-16">
                  {t("reportDialog.speedType")}
                </th>
                <th className="text-right px-2 sm:px-3 py-1.5 font-medium text-chart-2">
                  {t("reportDialog.download")}
                </th>
                <th className="text-right px-2 sm:px-3 py-1.5 font-medium text-chart-3">
                  {t("reportDialog.upload")}
                </th>
              </tr>
            </thead>
            <tbody>
              {[
                { key: "current", dl: properties?.dl_speed ?? torrent.dlspeed ?? 0, ul: properties?.up_speed ?? torrent.upspeed ?? 0 },
                { key: "average", dl: properties?.dl_speed_avg ?? 0, ul: properties?.up_speed_avg ?? 0 },
                { key: "peak", dl: properties?.peak_dl_speed ?? 0, ul: properties?.peak_up_speed ?? 0 },
              ].map((row, i) => (
                <tr key={row.key} className={i < 2 ? "border-b border-border/50" : ""}>
                  <td className="px-2 sm:px-3 py-1.5 text-muted-foreground capitalize">{t(`reportDialog.${row.key}`)}</td>
                  <td className="text-right px-2 sm:px-3 py-1.5 font-mono font-medium">
                    {formatSpeedWithUnit(row.dl, speedUnit)}
                  </td>
                  <td className="text-right px-2 sm:px-3 py-1.5 font-mono font-medium">
                    {formatSpeedWithUnit(row.ul, speedUnit)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {/* Connections */}
      <section className="space-y-2">
        <h4 className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
          {t("reportDialog.connections")}
        </h4>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-x-4 gap-y-2 text-xs">
          <div>
            <span className="text-muted-foreground">{t("reportDialog.seeds")}</span>
            <p className="font-medium">
              {torrent.num_seeds || 0}
              {properties?.seeds_total ? ` / ${properties.seeds_total}` : ""}
            </p>
          </div>
          <div>
            <span className="text-muted-foreground">{t("reportDialog.peers")}</span>
            <p className="font-medium">
              {torrent.num_leechs || 0}
              {properties?.peers_total ? ` / ${properties.peers_total}` : ""}
            </p>
          </div>
          <div>
            <span className="text-muted-foreground">{t("reportDialog.connections")}</span>
            <p className="font-medium">{properties?.nb_connections ?? "—"}</p>
          </div>
          <div>
            <span className="text-muted-foreground">ETA</span>
            <p className="font-medium">{torrent.eta > 0 ? formatTime(torrent.eta) : "—"}</p>
          </div>
        </div>
      </section>

      {/* Peer Country Distribution */}
      {countryStats.length > 0 && (
        <section className="space-y-2">
          <h4 className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
            {t("reportDialog.countryDistribution")}
          </h4>
          <div className="text-[10px] flex items-center gap-2 px-1 text-muted-foreground font-medium uppercase tracking-wider mb-1">
            <span className="w-4 shrink-0" />
            <span className="w-20 sm:w-24 truncate shrink-0">{t("reportDialog.country")}</span>
            <span className="flex-1" />
            <span className="w-8 text-right">{t("reportDialog.count")}</span>
            <span className="w-12 text-right">{t("reportDialog.pct")}</span>
          </div>
          <div className="space-y-1.5">
            {countryStats.map((stat) => (
              <div key={stat.code} className="flex items-center gap-2 text-xs">
                <span className={cn("fi", `fi-${stat.code.toLowerCase()}`, "rounded-sm w-4 h-3 shrink-0")} />
                <span className="w-20 sm:w-24 truncate shrink-0 text-muted-foreground">{stat.name}</span>
                <div className="flex-1 h-3 bg-muted rounded-full overflow-hidden">
                  <div
                    className="h-full bg-chart-1 rounded-full transition-all"
                    style={{ width: `${Math.min((stat.pct / maxCountryPct) * MAX_BAR_PCT, MAX_BAR_PCT)}%` }}
                  />
                </div>
                <span className="w-8 text-right font-medium tabular-nums">{stat.count}</span>
                <span className="w-12 text-right text-muted-foreground tabular-nums">{stat.pct.toFixed(1)}%</span>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* ISP Distribution */}
      {ispStats.length > 0 && (
        <section className="space-y-2">
          <h4 className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
            {t("reportDialog.ispDistribution")}
          </h4>
          <div className="text-[10px] flex items-center gap-2 px-1 text-muted-foreground font-medium uppercase tracking-wider mb-1">
            <span className="flex-1 max-w-[150px] truncate shrink-0">{t("reportDialog.isp")}</span>
            <span className="flex-1" />
            <span className="w-8 text-right">{t("reportDialog.count")}</span>
            <span className="w-12 text-right">{t("reportDialog.pct")}</span>
          </div>
          <div className="space-y-1.5">
            {ispStats.map((stat) => (
              <div key={stat.isp} className="flex items-center gap-2 text-xs">
                <span className="flex-1 truncate text-muted-foreground max-w-[150px]" title={stat.isp}>
                  {stat.isp}
                </span>
                <div className="flex-1 h-3 bg-muted rounded-full overflow-hidden">
                  <div
                    className="h-full bg-chart-2 rounded-full transition-all"
                    style={{ width: `${Math.min((stat.pct / maxIspPct) * MAX_BAR_PCT, MAX_BAR_PCT)}%` }}
                  />
                </div>
                <span className="w-8 text-right font-medium tabular-nums">{stat.count}</span>
                <span className="w-12 text-right text-muted-foreground tabular-nums">{stat.pct.toFixed(1)}%</span>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  )
}

export const TaskReportDialog = memo(function TaskReportDialog({
  open,
  onOpenChange,
  instanceId,
  torrent,
}: TaskReportDialogProps) {
  const { t } = useTranslation("torrents")
  const isMobile = useIsMobile()
  const [speedUnit] = useSpeedUnits()

  const { ispData } = useIspData()
  const formatters = useDateTimeFormatters()
  const contentRef = useRef<HTMLDivElement>(null)
  const [copying, setCopying] = useState(false)
  const [captureDataUrl, setCaptureDataUrl] = useState<string | null>(null)
  const [capturedBlob, setCapturedBlob] = useState<Blob | null>(null)
  const [showCaptureDialog, setShowCaptureDialog] = useState(false)
  const [paused, setPaused] = useState(true)
  const frozenPeers = useRef<SortedPeer[] | undefined>(undefined)
  const frozenProperties = useRef<TorrentProperties | undefined>(undefined)
  const frozenTorrentSnapshot = useRef<Torrent>(torrent)
  const prevOpen = useRef(open)
  if (open && !prevOpen.current) {
    frozenTorrentSnapshot.current = torrent
  }
  prevOpen.current = open
  if (!paused) frozenTorrentSnapshot.current = torrent
  const reportTorrent = paused ? frozenTorrentSnapshot.current : torrent
  const { data: trackerIcons } = useTrackerIcons()

  const { data: properties } = useQuery({
    queryKey: ["torrent-properties", instanceId, torrent.hash],
    queryFn: () => api.getTorrentProperties(instanceId, torrent.hash),
    enabled: open,
    staleTime: 5000,
    refetchInterval: paused ? false : 5000,
  })
  if (properties !== undefined) {
    if (!paused || frozenProperties.current === undefined) {
      frozenProperties.current = properties
    }
  }

  const { data: peersData } = useQuery({
    queryKey: ["torrent-peers", instanceId, torrent.hash],
    queryFn: () => api.getTorrentPeers(instanceId, torrent.hash),
    enabled: open,
    staleTime: 5000,
    refetchInterval: paused ? false : 5000,
  })
  if (peersData?.sorted_peers) {
    if (!paused || frozenPeers.current === undefined) {
      frozenPeers.current = peersData.sorted_peers
    }
  }

  const handleCapture = useCallback(async () => {
    if (!contentRef.current) return
    setCopying(true)
    try {
      const { toPng } = await import("html-to-image")

      // Match what the user actually sees: pick the effective background by
      // walking up from the report element until a non-transparent color shows.
      let backgroundColor = "#ffffff"
      let el: HTMLElement | null = contentRef.current
      while (el) {
        const bg = getComputedStyle(el).backgroundColor
        if (bg && bg !== "rgba(0, 0, 0, 0)" && bg !== "transparent") {
          backgroundColor = bg
          break
        }
        el = el.parentElement
      }

      const dataUrl = await toPng(contentRef.current, {
        pixelRatio: isMobile ? 1 : 2,
        cacheBust: true,
        backgroundColor,
      })
      const blob = await (await fetch(dataUrl)).blob()
      setCaptureDataUrl(dataUrl)
      setCapturedBlob(blob)
      setShowCaptureDialog(true)
    } catch {
      toast.error(t("reportDialog.copyImageFailed"))
    } finally {
      setCopying(false)
    }
  }, [isMobile, t])

  const handleCopyFromCapture = useCallback(async () => {
    if (!capturedBlob) return
    setShowCaptureDialog(false)
    try {
      await navigator.clipboard.write([
        new ClipboardItem({ "image/png": capturedBlob }),
      ])
      toast.success(t("reportDialog.copyImageSuccess"))
    } catch {
      toast.error(t("reportDialog.copyImageFailed"))
    }
  }, [capturedBlob, t])

  const handleDownloadFromCapture = useCallback(() => {
    if (!captureDataUrl) return
    setShowCaptureDialog(false)
    const link = document.createElement("a")
    link.href = captureDataUrl
    link.download = "torrent-report.png"
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    toast.success(t("reportDialog.downloadImageSuccess"))
  }, [captureDataUrl, t])

  const handleTogglePause = useCallback(() => {
    setPaused((prev) => !prev)
  }, [])

  const reportContent = (
    <div ref={contentRef} className="p-3 sm:p-8">
      <ReportContent
        torrent={reportTorrent}
        properties={paused ? frozenProperties.current : properties}
        peers={paused ? frozenPeers.current : peersData?.sorted_peers}
        trackerIcons={trackerIcons}
        ispData={ispData}
        speedUnit={speedUnit}
        formatters={formatters}
        t={t}
      />
    </div>
  )

  const captureDialog = (
    <AlertDialog open={showCaptureDialog} onOpenChange={setShowCaptureDialog}>
      <AlertDialogContent className="z-[70]">
        <AlertDialogHeader>
          <AlertDialogTitle>{t("reportDialog.captureTitle")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("reportDialog.captureDescription")}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter className="flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <AlertDialogCancel>{t("reportDialog.cancel")}</AlertDialogCancel>
          <AlertDialogAction onClick={handleCopyFromCapture}>
            {t("reportDialog.copy")}
          </AlertDialogAction>
          <AlertDialogAction onClick={handleDownloadFromCapture}>
            {t("reportDialog.downloadImage")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )

  return (
    <>
      {isMobile ? (
        <Dialog open={open} onOpenChange={onOpenChange}>
          <DialogContent
            showCloseButton={false}
            overlayClassName="bg-transparent"
            className="top-0 right-0 bottom-0 left-0 z-[60] flex flex-col gap-0 rounded-none border-0 p-0 shadow-none translate-x-0 translate-y-0 w-full max-w-none sm:max-w-none h-dvh overflow-hidden"
          >
            <header className="shrink-0 relative z-20 flex items-center justify-between gap-2 border-b bg-background px-3 h-12">
              <div className="flex items-center min-w-0 gap-1">
              <Button
                variant="ghost"
                size="icon"
                className="size-8 shrink-0"
                onClick={() => onOpenChange(false)}
                aria-label={t("reportDialog.close")}
              >
                <ArrowLeft className="h-4 w-4" />
              </Button>
              <DialogTitle className="text-sm font-semibold truncate">{t("reportDialog.title")}</DialogTitle>
            </div>
            <div className="flex items-center gap-0.5 shrink-0">
              <Button
                variant="ghost"
                size="icon"
                className="size-8"
                onClick={handleTogglePause}
                aria-label={paused ? t("reportDialog.resume") : t("reportDialog.pause")}
              >
                {paused ? <Play className="h-4 w-4" /> : <Pause className="h-4 w-4" />}
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="size-8"
                onClick={handleCapture}
                disabled={copying}
                aria-label={t("reportDialog.copyImage")}
              >
                {copying ? <Loader2 className="h-4 w-4 animate-spin" /> : <Camera className="h-4 w-4" />}
              </Button>
            </div>
          </header>
          <div className="flex-1 min-h-0 overflow-y-auto overscroll-contain">
            {reportContent}
          </div>
        </DialogContent>
      </Dialog>
      ) : (
        <Dialog open={open} onOpenChange={onOpenChange}>
          <DialogContent showCloseButton={false} className="w-full sm:max-w-lg md:max-w-xl lg:max-w-2xl max-h-[85vh] overflow-y-auto p-3 sm:p-6">
            <DialogHeader className="flex flex-row items-center justify-between gap-2 pr-8">
              <DialogTitle className="text-sm">{t("reportDialog.title")}</DialogTitle>
          <div className="flex items-center gap-1">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  onClick={handleTogglePause}
                  tabIndex={-1}
                >
                  {paused ? <Play className="h-3.5 w-3.5" /> : <Pause className="h-3.5 w-3.5" />}
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                <p className="text-xs">{paused ? t("reportDialog.resume") : t("reportDialog.pause")}</p>
              </TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  onClick={handleCapture}
                  disabled={copying}
                  tabIndex={-1}
                >
                  {copying ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Camera className="h-3.5 w-3.5" />}
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                <p className="text-xs">{t("reportDialog.copyImage")}</p>
              </TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  onClick={() => onOpenChange(false)}
                  tabIndex={-1}
                >
                  <X className="h-3.5 w-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                <p className="text-xs">{t("reportDialog.close")}</p>
              </TooltipContent>
            </Tooltip>
          </div>
        </DialogHeader>

        {reportContent}
      </DialogContent>
    </Dialog>
      )}
      {captureDialog}
    </>
  )
})
