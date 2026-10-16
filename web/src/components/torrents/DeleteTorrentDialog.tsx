/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from "@/components/ui/alert-dialog"
import { CrossSeedWarning } from "./CrossSeedWarning"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { api } from "@/lib/api"
import { useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import type { CrossSeedWarningResult } from "@/hooks/useCrossSeedWarning"
import { Checkbox } from "@/components/ui/checkbox"
import { useTranslation } from "react-i18next"
import { Folder, Tags, Lock, Unlock } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { useInstances } from "@/hooks/useInstances"
import { useDateTimeFormatters } from "@/hooks/useDateTimeFormatters"
import { flagClass } from "@/lib/countryFlags"
import { getStateLabel } from "@/lib/torrent-state-utils"
import { formatBytes } from "@/lib/utils"
import type { CrossInstanceTorrent, Torrent } from "@/types"

interface DeleteTorrentDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  count: number
  torrents: Torrent[]
  instanceId?: number
  totalSize: number
  formattedSize: string
  preserveCrossSeeds?: boolean
  onPreserveCrossSeedsChange?: (checked: boolean) => void
  includeCrossSeeds?: boolean
  onIncludeCrossSeedsChange?: (checked: boolean) => void
  selectionRequest?: Omit<Parameters<typeof api.bulkAction>[1], "action">
  deleteFiles: boolean
  onDeleteFilesChange: (checked: boolean) => void
  isDeleteFilesLocked: boolean
  onToggleDeleteFilesLock: () => void
  deleteCrossSeeds: boolean
  onDeleteCrossSeedsChange: (checked: boolean) => void
  showBlockCrossSeeds: boolean
  blockCrossSeeds: boolean
  onBlockCrossSeedsChange: (checked: boolean) => void
  crossSeedWarning?: CrossSeedWarningResult | null
  onConfirm: () => void
}

export function DeleteTorrentDialog({
  open,
  onOpenChange,
  count,
  torrents,
  instanceId,
  totalSize,
  formattedSize,
  preserveCrossSeeds = false,
  onPreserveCrossSeedsChange,
  includeCrossSeeds = false,
  onIncludeCrossSeedsChange,
  selectionRequest,
  deleteFiles,
  onDeleteFilesChange,
  isDeleteFilesLocked,
  onToggleDeleteFilesLock,
  deleteCrossSeeds,
  onDeleteCrossSeedsChange,
  showBlockCrossSeeds,
  blockCrossSeeds,
  onBlockCrossSeedsChange,
  crossSeedWarning,
  onConfirm,
}: DeleteTorrentDialogProps) {
  const { t } = useTranslation("torrents")
  const { instances } = useInstances()
  const { formatAddedOn } = useDateTimeFormatters()
  const [preview, setPreview] = useState<{ key: string; data?: Awaited<ReturnType<typeof api.previewTorrentDeletion>>; error?: boolean }>()
  const selectionKey = JSON.stringify({ instanceId, ...selectionRequest, includeCrossSeeds })
  const hasSelection = selectionRequest !== undefined
  const needsPreview = preserveCrossSeeds || includeCrossSeeds
  useEffect(() => {
    if (!open || !needsPreview || !hasSelection || instanceId === undefined) return
    setPreview(undefined)
    let cancelled = false
    const { instanceId: owner, ...request } = JSON.parse(selectionKey)
    void api.previewTorrentDeletion(owner, { ...request, action: "delete" }).then(data => {
      if (!cancelled) setPreview({ key: selectionKey, data })
    }).catch(() => {
      if (!cancelled) setPreview({ key: selectionKey, error: true })
    })
    return () => { cancelled = true }
  }, [open, needsPreview, selectionKey, instanceId, hasSelection])
  const currentPreview = preview?.key === selectionKey ? preview : undefined
  const mode = includeCrossSeeds ? "include" : preserveCrossSeeds ? "preserve" : deleteFiles ? "files" : "keep"
  // Include cross-seeds in the displayed count when selected
  const crossSeedCount = !needsPreview && deleteCrossSeeds ? (crossSeedWarning?.affectedTorrents.length ?? 0) : 0
  const displayCount = includeCrossSeeds ? currentPreview?.data?.total ?? count : count + crossSeedCount

  const renderTorrent = (torrent: Pick<Torrent, "hash" | "name" | "category" | "tags" | "size" | "state" | "progress" | "added_on" | "ratio"> & Partial<Pick<CrossInstanceTorrent, "instanceId" | "instanceName">>) => {
    const ownerId = (torrent as Partial<CrossInstanceTorrent>).instanceId ?? instanceId
    const owner = instances?.find(instance => instance.id === ownerId)
    const ownerName = owner?.name || (torrent as Partial<CrossInstanceTorrent>).instanceName || t("deleteDialog.instanceFallback", { id: ownerId ?? "—" })
    const flag = flagClass(owner?.countryCode)
    const tags = torrent.tags.split(",").map(tag => tag.trim()).filter(Boolean)
    return (
      <li key={`${ownerId}:${torrent.hash}`} className="space-y-2 px-3 py-3 text-left">
        <p className="text-sm font-medium leading-snug [overflow-wrap:anywhere]">{torrent.name}</p>
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge variant="outline" className="min-h-5 max-w-full gap-1 border-solid border-primary/35 bg-transparent px-1.5 py-0.5 text-[11px] font-medium text-primary shadow-none">
            {flag && <span className={`${flag} shrink-0 rounded-sm text-[11px]`} />}
            <span className="whitespace-normal [overflow-wrap:anywhere]">{ownerName}</span>
          </Badge>
          <Badge variant="outline" className="min-h-5 max-w-full gap-1 px-1.5 py-0.5 text-[11px] font-normal text-muted-foreground">
            <Folder className="size-3 shrink-0" aria-label={t("reportDialog.category")} />
            <span className="whitespace-normal [overflow-wrap:anywhere]">{torrent.category || t("reportDialog.uncategorized")}</span>
          </Badge>
          {(tags.length ? tags : [t("reportDialog.noTags")]).map(tag => (
            <Badge key={tag} variant="outline" className="min-h-5 max-w-full gap-1 px-1.5 py-0.5 text-[11px] font-normal text-muted-foreground">
              <Tags className="size-3 shrink-0" aria-label={t("reportDialog.tags")} />
              <span className="whitespace-normal [overflow-wrap:anywhere]">{tag}</span>
            </Badge>
          ))}
        </div>
        <dl className="grid grid-cols-3 gap-x-3 gap-y-2 text-xs sm:grid-cols-[1fr_1fr_1fr_2fr_1fr]">
          <div className="min-w-0">
            <dt className="text-muted-foreground">{t("tableColumns.size")}</dt>
            <dd className="mt-0.5 tabular-nums">{formatBytes(torrent.size)}</dd>
          </div>
          <div className="min-w-0">
            <dt className="text-muted-foreground">{t("tableColumns.state")}</dt>
            <dd className="mt-0.5 [overflow-wrap:anywhere]">{getStateLabel(torrent.state, t)}</dd>
          </div>
          <div className="min-w-0">
            <dt className="text-muted-foreground">{t("tableColumns.progress")}</dt>
            <dd className="mt-0.5 tabular-nums">{torrent.progress >= 0.99 && torrent.progress < 1
              ? (Math.floor(torrent.progress * 1000) / 10).toFixed(1)
              : Math.round(torrent.progress * 100)}%</dd>
          </div>
          <div className="col-span-2 min-w-0 sm:col-span-1">
            <dt className="text-muted-foreground">{t("tableColumns.addedOn")}</dt>
            <dd className="mt-0.5 tabular-nums">{formatAddedOn(torrent.added_on)}</dd>
          </div>
          <div className="min-w-0">
            <dt className="text-muted-foreground">{t("tableColumns.ratio")}</dt>
            <dd className="mt-0.5 tabular-nums">{torrent.ratio === -1 ? "∞" : torrent.ratio.toFixed(2)}</dd>
          </div>
        </dl>
      </li>
    )
  }

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:!max-w-2xl">
        <AlertDialogHeader className="text-left">
          <AlertDialogTitle>{t("deleteDialog.title", { count: displayCount })}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("deleteDialog.description")}
            {totalSize > 0 && (
              <span className="block mt-2 text-xs text-muted-foreground">
                {includeCrossSeeds && <>{t("deleteDialog.selectedTorrents")} · </>}
                {t("deleteDialog.totalSize", { size: formattedSize })}
              </span>
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <section className="min-w-0 overflow-hidden rounded-lg border" aria-label={t("deleteDialog.selectedTorrents")}>
          <div className="flex items-center justify-between gap-3 border-b bg-muted/40 px-3 py-2 text-xs">
            <span className="font-medium">{t("deleteDialog.selectedTorrents")}</span>
            <span className="tabular-nums text-muted-foreground">{torrents.length} / {count}</span>
          </div>
          <ul className="max-h-[min(18rem,40dvh)] overflow-y-auto overscroll-contain divide-y" tabIndex={0} aria-label={t("deleteDialog.selectedTorrents")}>
            {torrents.map(renderTorrent)}
          </ul>
          {torrents.length < count && (
            <p className="border-t bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
              {t("deleteDialog.partialPreview", { shown: torrents.length, total: count })}
            </p>
          )}
        </section>
        <fieldset className="min-w-0 space-y-2">
          <div className="flex items-center justify-between gap-2">
            <span className="text-sm font-medium">{t("deleteDialog.modeLabel")}</span>
            {!needsPreview && <Button type="button" variant="ghost" size="icon" className="size-7" onClick={onToggleDeleteFilesLock}
              aria-label={t(isDeleteFilesLocked ? "deleteFilesPreference.lockAriaLabel.stop" : "deleteFilesPreference.lockAriaLabel.remember")} aria-pressed={isDeleteFilesLocked}>
              {isDeleteFilesLocked ? <Lock className="size-3.5" /> : <Unlock className="size-3.5" />}
            </Button>}
          </div>
          <RadioGroup aria-label={t("deleteDialog.modeLabel")} value={mode} onValueChange={value => {
            onPreserveCrossSeedsChange?.(value === "preserve")
            onIncludeCrossSeedsChange?.(value === "include")
            if (value === "include") onBlockCrossSeedsChange(false)
            if (value !== "preserve" && value !== "include") onDeleteFilesChange(value === "files")
            onDeleteCrossSeedsChange(false)
          }} className="gap-2">
            {["keep", "files", "preserve", "include"].map(value => <label key={value} htmlFor={`delete-mode-${value}`} className="flex cursor-pointer items-start gap-2 rounded-md border px-3 py-2 text-left has-[[data-state=checked]]:border-primary/40">
              <RadioGroupItem id={`delete-mode-${value}`} value={value} className="mt-0.5 shrink-0" />
              <span className="min-w-0"><span className="block text-sm font-medium">{t(`deleteDialog.modes.${value}.label`)}</span>
                <span className="mt-0.5 block text-xs text-muted-foreground">{t(`deleteDialog.modes.${value}.description`)}</span></span>
            </label>)}
          </RadioGroup>
        </fieldset>
        {needsPreview && <div role="status" className="rounded-md border bg-muted/30 px-3 py-2 text-xs leading-relaxed">
          {currentPreview?.error ? t("deleteDialog.planError") : currentPreview?.data
            ? <>{t(includeCrossSeeds ? "deleteDialog.includePlanSummary" : "deleteDialog.planSummary", currentPreview.data)}<span className="mt-1 block text-muted-foreground">{t(includeCrossSeeds ? "deleteDialog.includePlanRecheck" : "deleteDialog.planRecheck")}</span></>
            : t("deleteDialog.planChecking")}
        </div>}
        {includeCrossSeeds && !!currentPreview?.data?.relatedTorrents?.length && (
          <section className="min-w-0 rounded-md border border-destructive/30">
            <h3 className="border-b px-3 py-2 text-xs font-medium">{t("deleteDialog.relatedTorrents")}</h3>
            <ul className="max-h-[min(18rem,40dvh)] overflow-y-auto overscroll-contain divide-y" tabIndex={0} aria-label={t("deleteDialog.relatedTorrents")}>
              {currentPreview.data.relatedTorrents.map(renderTorrent)}
            </ul>
          </section>
        )}
        {!needsPreview && crossSeedWarning && (
          <CrossSeedWarning
            affectedTorrents={crossSeedWarning.affectedTorrents}
            searchState={crossSeedWarning.searchState}
            hasWarning={crossSeedWarning.hasWarning}
            deleteFiles={deleteFiles}
            deleteCrossSeeds={deleteCrossSeeds}
            onDeleteCrossSeedsChange={onDeleteCrossSeedsChange}
            onSearch={crossSeedWarning.search}
            totalToCheck={crossSeedWarning.totalToCheck}
            checkedCount={crossSeedWarning.checkedCount}
          />
        )}
        {!includeCrossSeeds && showBlockCrossSeeds && (
          <div className="mt-3 flex items-center gap-2">
            <Checkbox
              id="blockCrossSeeds"
              checked={blockCrossSeeds}
              onCheckedChange={(checked) => onBlockCrossSeedsChange(checked === true)}
            />
            <label
              htmlFor="blockCrossSeeds"
              className="text-xs cursor-pointer select-none"
            >
              {t("deleteDialog.blockCrossSeeds")}
            </label>
          </div>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel>{t("deleteDialog.cancel")}</AlertDialogCancel>
          <AlertDialogAction
            disabled={needsPreview && !currentPreview?.data}
            onClick={onConfirm}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          >
            {t("deleteDialog.delete")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
