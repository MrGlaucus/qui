/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useLayoutRoute } from "@/contexts/LayoutRouteContext"
import { usePersistedFilters } from "@/hooks/usePersistedFilters"
import { usePersistedColumnFilters } from "@/hooks/usePersistedColumnFilters"
import { usePersistedUnifiedInstanceFilter } from "@/hooks/usePersistedUnifiedInstanceFilter"
import { toViewFilters } from "@/lib/filter-views"
import { ALL_INSTANCES_ID } from "@/lib/instances"
import { Torrents, type TorrentsSearch } from "@/pages/Torrents"
import { createFileRoute } from "@tanstack/react-router"
import { useEffect, useLayoutEffect } from "react"
import { useTranslation } from "react-i18next"
import { z } from "zod"

const unifiedSearchSchema = z.object({
  modal: z.enum(["add-torrent", "create-torrent", "tasks"]).optional(),
  torrent: z.string().optional(),
  tab: z.string().optional(),
  instance: z.coerce.number().optional(),
  trackers: z.array(z.string()).min(1).optional().catch(undefined),
})

export const Route = createFileRoute("/_authenticated/instances/")({
  validateSearch: unifiedSearchSchema,
  component: UnifiedInstanceTorrents,
  staticData: {
    titleKey: "unifiedScope.unified",
    titleNs: "common",
  },
})

function UnifiedInstanceTorrents() {
  const { t } = useTranslation("common")
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const { setLayoutRouteState, resetLayoutRouteState } = useLayoutRoute()
  const [, setFilters] = usePersistedFilters(ALL_INSTANCES_ID)
  const [, setColumnFilters] = usePersistedColumnFilters(ALL_INSTANCES_ID)
  const [, setUnifiedInstanceFilter] = usePersistedUnifiedInstanceFilter()
  const applyingTrackerFilter = search.trackers !== undefined

  // Apply navigation filters before mounting the list, so old filters and
  // instance subsets cannot hide torrents belonging to the selected site.
  useEffect(() => {
    if (!search.trackers) return
    setFilters(toViewFilters({ trackers: [...new Set(search.trackers)] }))
    setColumnFilters([])
    setUnifiedInstanceFilter([])
    void navigate({
      search: previous => ({ ...previous, trackers: undefined }),
      replace: true,
    })
  }, [search.trackers, setFilters, setColumnFilters, setUnifiedInstanceFilter, navigate])

  useLayoutEffect(() => {
    setLayoutRouteState({
      // The header also writes search parameters when its controls mount.
      // Wait until the navigation filter has been consumed before enabling it.
      showInstanceControls: !applyingTrackerFilter,
      instanceId: ALL_INSTANCES_ID,
    })

    return () => {
      resetLayoutRouteState()
    }
  }, [applyingTrackerFilter, resetLayoutRouteState, setLayoutRouteState])

  const handleSearchChange = (newSearch: TorrentsSearch) => {
    navigate({
      search: newSearch,
      replace: true,
    })
  }

  if (applyingTrackerFilter) return <div role="status">{t("mobileNav.loading")}</div>

  return (
    <Torrents
      instanceId={ALL_INSTANCES_ID}
      instanceName={t("unifiedScope.unified")}
      isAllInstancesView
      search={search}
      onSearchChange={handleSearchChange}
    />
  )
}
