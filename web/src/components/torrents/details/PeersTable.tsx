/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuSeparator, ContextMenuTrigger } from "@/components/ui/context-menu"
import { Progress } from "@/components/ui/progress"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { canBanPeer, getPeerDisplayAddress } from "@/lib/torrent-peer-address"
import { getPeerFlagDetails } from "@/lib/torrent-peer-flags"
import { getCountryName } from "@/lib/countryNames"
import { cn, copyTextToClipboard, formatBytes } from "@/lib/utils"
import { formatSpeedWithUnit, type SpeedUnit } from "@/lib/speedUnits"
import type { SortedPeer } from "@/types"
import {
  createColumnHelper,
  flexRender,
  type SortFn,
  type SortingState,
  useTable
} from "@tanstack/react-table"
import { sortableDetailsTableFeatures } from "../tanstackTableFeatures"
import { SortIcon } from "@/components/ui/sort-icon"
import { Ban, Copy, Loader2 } from "lucide-react"
import { memo, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

interface PeersTableProps {
  peers: SortedPeer[] | undefined
  loading: boolean
  speedUnit: SpeedUnit
  showFlags: boolean
  incognitoMode: boolean
  onBanPeer?: (peer: SortedPeer) => void
  ispData: Record<string, string | null | "loading">
  onRetryIsp?: (ip: string) => void
}

const columnHelper = createColumnHelper<typeof sortableDetailsTableFeatures, SortedPeer>()

// Sorting function that pushes 0/null/undefined values to the bottom
const zeroLastSortingFn: SortFn<typeof sortableDetailsTableFeatures, SortedPeer> = (rowA, rowB, columnId) => {
  const a = (rowA.getValue(columnId) as number | undefined | null) ?? 0
  const b = (rowB.getValue(columnId) as number | undefined | null) ?? 0
  if (a === 0 && b !== 0) return 1
  if (b === 0 && a !== 0) return -1
  return a - b
}

// Sorting function for the ISP column: resolves to the ISP string, pushing
// missing/empty/loading values to the bottom.
const ispLastSortingFn: SortFn<typeof sortableDetailsTableFeatures, SortedPeer> = (rowA, rowB, columnId) => {
  const a = rowA.getValue(columnId) as string | null | undefined
  const b = rowB.getValue(columnId) as string | null | undefined
  const aEmpty = !a || a === "loading"
  const bEmpty = !b || b === "loading"
  if (aEmpty && bEmpty) return 0
  if (aEmpty) return 1
  if (bEmpty) return -1
  return a.localeCompare(b)
}

export const PeersTable = memo(function PeersTable({
  peers,
  loading,
  speedUnit,
  showFlags,
  incognitoMode,
  onBanPeer,
  ispData,
  onRetryIsp,
}: PeersTableProps) {
  const { t } = useTranslation("torrents")
  const [sorting, setSorting] = useState<SortingState>([{ id: "up_speed", desc: true }])

  const columns = useMemo(() => columnHelper.columns([
    columnHelper.accessor("country_code", {
      header: "",
      cell: (info) => {
        const code = info.getValue()?.toLowerCase()
        if (!code) return null
        const countryName = getCountryName(info.row.original.country_code ?? "", info.row.original.country, t)
        return (
          <span className="inline-flex items-center gap-1.5 min-w-0">
            <Tooltip>
              <TooltipTrigger asChild>
                <span className={cn("fi", `fi-${code}`, "rounded-sm")} />
              </TooltipTrigger>
              <TooltipContent>
                <p className="text-xs">{countryName}</p>
              </TooltipContent>
            </Tooltip>
            <span className="text-muted-foreground truncate">[{countryName}]</span>
          </span>
        )
      },
      size: 160,
      enableSorting: false,
    }),
    columnHelper.accessor((row) => row.key, {
      id: "address",
      header: t("peersTable.address"),
      cell: (info) => (
        <span className="font-mono text-xs">
          {getPeerDisplayAddress(info.row.original, incognitoMode)}
        </span>
      ),
      size: 150,
    }),
    columnHelper.accessor((row) => row.ip ? ispData[row.ip] ?? "" : "", {
      id: "isp",
      header: "ISP",
      cell: (info) => {
        const ip = info.row.original.ip
        if (!ip) return <span className="text-muted-foreground">-</span>
        if (incognitoMode) return <span className="text-muted-foreground">-</span>
        const status = ispData[ip]
        if (status === "loading") return <span className="text-muted-foreground">...</span>
        if (status === null) return (
          <span
            className="text-red-500 cursor-pointer hover:underline"
            onClick={(e) => { e.stopPropagation(); onRetryIsp?.(ip) }}
          >
            {t("peersTable.retry")}
          </span>
        )
        if (status) return <span className="truncate block max-w-[120px]" title={status}>{status}</span>
        return <span className="text-muted-foreground">-</span>
      },
      size: 120,
      sortUndefined: "last",
      sortFn: ispLastSortingFn,
    }),
    columnHelper.accessor("client", {
      header: t("peersTable.client"),
      cell: (info) => (
        <span className="truncate block max-w-[120px]" title={info.getValue()}>
          {info.getValue() || "-"}
        </span>
      ),
      size: 120,
    }),
    columnHelper.accessor("progress", {
      header: t("peersTable.progress"),
      cell: (info) => {
        const progress = info.getValue() * 100
        return (
          <div className="flex items-center gap-2">
            <Progress value={progress} className="h-1.5 w-16" />
            <span className="tabular-nums text-[10px] w-10">
              {progress.toFixed(1)}%
            </span>
          </div>
        )
      },
      size: 110,
    }),
    columnHelper.accessor("dl_speed", {
      header: t("peersTable.downSpeed"),
      cell: (info) => (
        <span className="tabular-nums text-green-500">
          {formatSpeedWithUnit(info.getValue() || 0, speedUnit)}
        </span>
      ),
      size: 90,
      sortUndefined: "last",
      sortFn: zeroLastSortingFn,
    }),
    columnHelper.accessor("up_speed", {
      header: t("peersTable.upSpeed"),
      cell: (info) => (
        <span className="tabular-nums text-blue-500">
          {formatSpeedWithUnit(info.getValue() || 0, speedUnit)}
        </span>
      ),
      size: 90,
      sortUndefined: "last",
      sortFn: zeroLastSortingFn,
    }),
    columnHelper.accessor("downloaded", {
      header: t("peersTable.downloaded"),
      cell: (info) => (
        <span className="tabular-nums">
          {formatBytes(info.getValue() || 0)}
        </span>
      ),
      size: 90,
      sortUndefined: "last",
      sortFn: zeroLastSortingFn,
    }),
    columnHelper.accessor("uploaded", {
      header: t("peersTable.uploaded"),
      cell: (info) => (
        <span className="tabular-nums">
          {formatBytes(info.getValue() || 0)}
        </span>
      ),
      size: 90,
      sortUndefined: "last",
      sortFn: zeroLastSortingFn,
    }),
    ...(showFlags ? [
      columnHelper.accessor("flags", {
        header: t("peersTable.flags"),
        cell: (info) => {
          const flags = info.getValue()
          if (!flags) return <span className="text-muted-foreground">-</span>
          const details = getPeerFlagDetails(flags, info.row.original.flags_desc)
          return (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="font-mono text-[10px] text-muted-foreground cursor-help">
                  {flags}
                </span>
              </TooltipTrigger>
              <TooltipContent side="left" className="max-w-[300px]">
                <div className="space-y-1 text-xs">
                  {details.map((d, i) => (
                    <div key={i}>
                      <span className="font-mono font-bold">{d.flag}</span>: {d.description}
                    </div>
                  ))}
                </div>
              </TooltipContent>
            </Tooltip>
          )
        },
        size: 60,
      }),
    ] : []),
  ]), [speedUnit, showFlags, incognitoMode, t, ispData, onRetryIsp])

  const data = useMemo(() => peers || [], [peers])

  const table = useTable({
    features: sortableDetailsTableFeatures,
    data,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
  })

  const handleCopyAddress = (peer: SortedPeer) => {
    if (incognitoMode) return
    copyTextToClipboard(peer.key)
    toast.success(t("peersTable.toast.ipCopied"))
  }

  const handleCopyIsp = (peer: SortedPeer) => {
    if (incognitoMode) return
    const isp = peer.ip ? ispData[peer.ip] : null
    if (!isp || isp === "loading") return
    copyTextToClipboard(isp)
    toast.success(t("peersTable.toast.ispCopied"))
  }

  if (loading && !peers) {
    return (
      <div className="flex items-center justify-center h-full">
        <Loader2 className="h-5 w-5 animate-spin" />
      </div>
    )
  }

  if (!peers || peers.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
        {t("peersTable.noPeersConnected")}
      </div>
    )
  }

  return (
    <ScrollArea className="h-full">
      <div className="min-w-[700px]">
        <table className="w-full text-xs">
          <thead className="sticky top-0 z-10 bg-background border-b">
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <th
                    key={header.id}
                    className={cn(
                      "px-2 py-2 text-left font-medium text-muted-foreground select-none",
                      header.column.getCanSort() && "cursor-pointer hover:bg-muted/50"
                    )}
                    style={{ width: header.getSize() }}
                    onClick={header.column.getToggleSortingHandler()}
                  >
                    <div className="flex items-center gap-1">
                      {flexRender(header.column.columnDef.header, header.getContext())}
                      {header.column.getCanSort() && (
                        <SortIcon sorted={header.column.getIsSorted()} />
                      )}
                    </div>
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.map((row) => (
              <ContextMenu key={row.id}>
                <ContextMenuTrigger asChild>
                  <tr className="border-b border-border/50 hover:bg-muted/30 cursor-default">
                    {row.getAllCells().map((cell) => (
                      <td
                        key={cell.id}
                        className="px-2 py-1.5"
                        style={{ width: cell.column.getSize() }}
                      >
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    ))}
                  </tr>
                </ContextMenuTrigger>
                <ContextMenuContent>
                  <ContextMenuItem
                    onClick={() => handleCopyAddress(row.original)}
                    disabled={incognitoMode}
                  >
                    <Copy className="h-3.5 w-3.5 mr-2" />
                    {t("peersTable.copyIpAddress")}
                  </ContextMenuItem>
                  <ContextMenuItem
                    onClick={() => handleCopyIsp(row.original)}
                    disabled={incognitoMode}
                  >
                    <Copy className="h-3.5 w-3.5 mr-2" />
                    {t("peersTable.copyIsp")}
                  </ContextMenuItem>
                  {onBanPeer && canBanPeer(row.original) && (
                    <>
                      <ContextMenuSeparator />
                      <ContextMenuItem
                        onClick={() => onBanPeer(row.original)}
                        className="text-destructive focus:text-destructive"
                      >
                        <Ban className="h-3.5 w-3.5 mr-2" />
                        {t("peersTable.banPeer")}
                      </ContextMenuItem>
                    </>
                  )}
                </ContextMenuContent>
              </ContextMenu>
            ))}
          </tbody>
        </table>
      </div>
    </ScrollArea>
  )
})
