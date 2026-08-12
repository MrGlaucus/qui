import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { api } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Button } from "@/components/ui/button"
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog"
import { useDateTimeFormatters } from "@/hooks/useDateTimeFormatters"
import type { CrossSeedLogEntry } from "@/types"

const PAGE_SIZE = 50

const CLEANUP_OPTIONS = [
  { labelKey: "log.cleanup.day1", hours: 24 },
  { labelKey: "log.cleanup.week1", hours: 168 },
  { labelKey: "log.cleanup.month1", hours: 720 },
] as const

export function CrossSeedLogTab() {
  const { t } = useTranslation("crossseed")
  const { formatDate } = useDateTimeFormatters()
  const queryClient = useQueryClient()
  const [offset, setOffset] = useState(0)
  const [cleanupHours, setCleanupHours] = useState(24)

  const { data, isLoading } = useQuery({
    queryKey: ["cross-seed-log", offset],
    queryFn: () => api.listCrossSeedLog(PAGE_SIZE, offset),
    refetchInterval: 30_000,
  })

  const cleanupMutation = useMutation({
    mutationFn: (hours: number) => api.cleanCrossSeedLog(hours),
    onSuccess: (result) => {
      toast.success(t("log.cleanup.success", { count: result.deleted }))
      queryClient.invalidateQueries({ queryKey: ["cross-seed-log"] })
    },
    onError: () => {
      toast.error(t("log.cleanup.error"))
    },
  })

  const entries = data?.entries ?? []
  const total = data?.total ?? 0
  const hasPrev = offset > 0
  const hasNext = offset + PAGE_SIZE < total

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>{t("log.title")}</CardTitle>
          <div className="flex items-center gap-2">
            <select
              value={cleanupHours}
              onChange={(e) => setCleanupHours(Number(e.target.value))}
              className="h-9 rounded-md border border-input bg-background px-3 text-sm"
            >
              {CLEANUP_OPTIONS.map((opt) => (
                <option key={opt.hours} value={opt.hours}>{t(opt.labelKey)}</option>
              ))}
            </select>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="outline" size="sm" disabled={cleanupMutation.isPending}>
                  {t("log.cleanup.button")}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t("log.cleanup.confirmTitle")}</AlertDialogTitle>
                  <AlertDialogDescription>
                    {t("log.cleanup.confirmDescription", { age: t(CLEANUP_OPTIONS.find(o => o.hours === cleanupHours)?.labelKey ?? "") })}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t("log.cleanup.cancel")}</AlertDialogCancel>
                  <AlertDialogAction onClick={() => cleanupMutation.mutate(cleanupHours)}>
                    {t("log.cleanup.button")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <p className="text-sm text-muted-foreground">{t("common:loading")}</p>
        ) : entries.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("common:empty")}</p>
        ) : (
          <>
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("log.instance")}</TableHead>
                    <TableHead>{t("log.infoHash")}</TableHead>
                    <TableHead>{t("log.torrentName")}</TableHead>
                    <TableHead>{t("log.indexer")}</TableHead>
                    <TableHead>{t("log.createdAt")}</TableHead>
                    <TableHead>{t("log.publishDate")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {entries.map((entry: CrossSeedLogEntry) => (
                    <TableRow key={entry.infoHash}>
                      <TableCell className="whitespace-nowrap">{entry.instanceName}</TableCell>
                      <TableCell>
                        <code className="text-xs">{entry.infoHash.substring(0, 16)}...</code>
                      </TableCell>
                      <TableCell className="max-w-[300px] truncate" title={entry.torrentName}>
                        {entry.torrentName}
                      </TableCell>
                      <TableCell className="whitespace-nowrap">{entry.sourceIndexer || "-"}</TableCell>
                      <TableCell className="whitespace-nowrap">{formatDate(new Date(entry.createdAt))}</TableCell>
                      <TableCell className="whitespace-nowrap">
                        {entry.publishDate ? formatDate(new Date(entry.publishDate)) : "-"}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            <div className="flex items-center justify-between mt-4">
              <p className="text-sm text-muted-foreground">
                {offset + 1}-{Math.min(offset + PAGE_SIZE, total)} / {total}
              </p>
              <div className="flex gap-2">
                <Button variant="outline" size="sm" disabled={!hasPrev} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
                  {t("common:previous")}
                </Button>
                <Button variant="outline" size="sm" disabled={!hasNext} onClick={() => setOffset(offset + PAGE_SIZE)}>
                  {t("common:next")}
                </Button>
              </div>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}
