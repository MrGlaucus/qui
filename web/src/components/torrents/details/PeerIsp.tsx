/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { Loader2, Network } from "lucide-react"
import { useTranslation } from "react-i18next"

interface PeerIspProps {
  ip?: string
  value: string | null | undefined
  incognitoMode: boolean
  onLookup?: (ip: string) => void
  className?: string
  textOnly?: boolean
}

export function PeerIsp({ ip, value, incognitoMode, onLookup, className, textOnly = false }: PeerIspProps) {
  const { t } = useTranslation("torrents")
  if (!ip || incognitoMode) return <span className="text-muted-foreground">-</span>

  const loading = value === "loading"
  const label = value === null ? t("peersTable.retry") : textOnly ? t("peersTable.query") : t("detailsPanel.queryIsp")

  return (
    <span className={cn("min-w-0 text-xs", className)}>
      {value && !loading ? (
        <span className="block truncate" title={value}>{value}</span>
      ) : (
        <Button
          type="button"
          variant={textOnly ? "link" : "ghost"}
          size="xs"
          className={cn(textOnly ? "h-auto p-0 font-normal" : "h-8 px-1.5", "text-primary", value === null && "text-destructive")}
          disabled={loading || !onLookup}
          aria-label={`${label} ${ip}`}
          aria-busy={loading}
          onClick={event => { event.stopPropagation(); onLookup?.(ip) }}
        >
          {!textOnly && (loading ? <Loader2 className="size-3 animate-spin" /> : <Network className="size-3" />)}
          {label}
        </Button>
      )}
    </span>
  )
}
