/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { describe, expect, it } from "vitest"

import {
  DASHBOARD_STATS_FALLBACK_ORDER,
  DASHBOARD_STATS_FALLBACK_SORT,
  createDashboardStatsFallbackQueryKey,
  hasDashboardStatsPayload,
  mergeDashboardInstanceMeta,
  mergeDashboardStatsSnapshot,
  mergeLiveTodayTraffic,
  resolveDashboardDataStatusKind,
  resolveDashboardStreamError,
  resolveDashboardTorrentCounts,
  shouldUseDashboardStatsFallback
} from "@/lib/dashboard-stream"
import type { InstanceDailyTraffic, InstanceResponse, TodayTraffic, TorrentCounts } from "@/types"

function makeCounts(overrides: Partial<TorrentCounts> = {}): TorrentCounts {
  return {
    status: { all: 1 },
    categories: {},
    tags: {},
    trackers: { "tracker.example": 1 },
    trackerTransfers: {
      "tracker.example": {
        uploaded: 10,
        downloaded: 20,
        uploadedSession: 5,
        downloadedSession: 7,
        totalSize: 30,
        count: 1,
      },
    },
    total: 1,
    ...overrides,
  }
}

function makeInstance(overrides: Partial<InstanceResponse> = {}): InstanceResponse {
  return {
    id: 7,
    name: "qbit",
    host: "http://localhost:8080",
    username: "user",
    tlsSkipVerify: false,
    hasLocalFilesystemAccess: true,
    useHardlinks: false,
    hardlinkBaseDir: "",
    hardlinkDirPreset: "flat",
    useReflinks: false,
    fallbackToRegularMode: true,
    dailyTrafficEnabled: true,
    countryCode: "",
    sortOrder: 0,
    isActive: true,
    reannounceSettings: {
      enabled: false,
      initialWaitSeconds: 5,
      reannounceIntervalSeconds: 60,
      maxAgeSeconds: 3600,
      maxRetries: 3,
      aggressive: false,
      monitorAll: false,
      excludeCategories: false,
      categories: [],
      excludeTags: false,
      tags: [],
      excludeTrackers: false,
      trackers: [],
    },
    connected: false,
    hasDecryptionError: true,
    recentErrors: [{
      id: 1,
      instanceId: 7,
      errorType: "auth",
      errorMessage: "old",
      occurredAt: "2026-06-29T00:00:00Z",
    }],
    connectionStatus: "disconnected",
    ...overrides,
  }
}

describe("resolveDashboardTorrentCounts", () => {
  it("preserves hydrated counts when a lightweight stream tick omits counts", () => {
    const currentCounts = makeCounts()

    expect(resolveDashboardTorrentCounts(undefined, currentCounts)).toBe(currentCounts)
  })

  it("uses incoming counts when the stream includes them", () => {
    const currentCounts = makeCounts()
    const incomingCounts = makeCounts({ status: { all: 0 }, trackers: {}, trackerTransfers: {}, total: 0 })

    expect(resolveDashboardTorrentCounts(incomingCounts, currentCounts)).toBe(incomingCounts)
  })
})

describe("shouldUseDashboardStatsFallback", () => {
  it("keeps fallback active when the stream is connected but has no fresh stream stats yet", () => {
    expect(shouldUseDashboardStatsFallback(true, false)).toBe(true)
  })

  it("keeps fallback active after reconnect even when cached stats exist", () => {
    const cachedData = {
      stats: { totalDownloadSpeed: 1 },
    }

    expect(hasDashboardStatsPayload(cachedData)).toBe(true)
    expect(shouldUseDashboardStatsFallback(true, false)).toBe(true)
  })

  it("uses live stream data once a fresh connected payload exists", () => {
    expect(shouldUseDashboardStatsFallback(true, true)).toBe(false)
  })

  it("treats counts-only snapshots as usable dashboard data", () => {
    expect(hasDashboardStatsPayload({
      counts: makeCounts(),
    })).toBe(true)
  })
})

describe("resolveDashboardStreamError", () => {
  it("clears stale stream errors after fallback data succeeds", () => {
    expect(resolveDashboardStreamError("stream down", true, { torrents: [], total: 0 })).toBeNull()
  })

  it("keeps stream errors until fallback has usable data", () => {
    expect(resolveDashboardStreamError("stream down", true, undefined)).toBe("stream down")
  })

  it("keeps stream errors while the stream is active", () => {
    expect(resolveDashboardStreamError("stream down", false, { torrents: [], total: 0 })).toBe("stream down")
  })
})

describe("resolveDashboardDataStatusKind", () => {
  it("reports fallback before live when fallback data is active on a connected stream", () => {
    expect(resolveDashboardDataStatusKind({
      streamError: null,
      fallbackActive: true,
      cacheMetadata: null,
      isFirstLoad: false,
      streamConnected: true,
    })).toBe("fallback")
  })

  it("reports cached before live when fallback-active data came from cache", () => {
    expect(resolveDashboardDataStatusKind({
      streamError: null,
      fallbackActive: true,
      cacheMetadata: { source: "cache", age: 0, isStale: false },
      isFirstLoad: false,
      streamConnected: true,
    })).toBe("cached")
  })

  it("keeps initial fallback-active no-data state from reporting live", () => {
    expect(resolveDashboardDataStatusKind({
      streamError: null,
      fallbackActive: true,
      cacheMetadata: null,
      isFirstLoad: true,
      streamConnected: true,
    })).toBeNull()
  })

  it("reports live when connected and no fallback data is active", () => {
    expect(resolveDashboardDataStatusKind({
      streamError: null,
      fallbackActive: false,
      cacheMetadata: null,
      isFirstLoad: false,
      streamConnected: true,
    })).toBe("live")
  })
})

describe("mergeDashboardInstanceMeta", () => {
  it("uses live SSE auth and connection fields when stream metadata is connected", () => {
    const merged = mergeDashboardInstanceMeta(makeInstance(), true, {
      connected: true,
      hasDecryptionError: false,
      recentErrors: [],
      connectionStatus: "connected",
    })

    expect(merged.connected).toBe(true)
    expect(merged.hasDecryptionError).toBe(false)
    expect(merged.recentErrors).toEqual([])
    expect(merged.connectionStatus).toBe("connected")
  })

  it("ignores stale SSE metadata after the stream disconnects", () => {
    const instance = makeInstance({ connected: true, hasDecryptionError: false, connectionStatus: "connected" })

    expect(mergeDashboardInstanceMeta(instance, false, {
      connected: false,
      hasDecryptionError: true,
      connectionStatus: "disabled",
    })).toBe(instance)
  })
})

describe("createDashboardStatsFallbackQueryKey", () => {
  it("uses the same lightweight scope for stream snapshots and fallback probes", () => {
    expect(createDashboardStatsFallbackQueryKey(7)).toEqual([
      "dashboard-stats-fallback",
      7,
      DASHBOARD_STATS_FALLBACK_SORT,
      DASHBOARD_STATS_FALLBACK_ORDER,
    ])
  })
})

describe("mergeDashboardStatsSnapshot", () => {
  it("preserves cached tracker counts when an incoming lightweight snapshot omits counts", () => {
    const cachedCounts = makeCounts()
    const merged = mergeDashboardStatsSnapshot(
      {
        torrents: [],
        total: 1,
        stats: {
          totalDownloadSpeed: 1,
          totalUploadSpeed: 2,
          totalSize: 3,
          totalRemainingSize: 4,
          totalSeedingSize: 5,
          downloading: 6,
          seeding: 7,
          total: 8,
          paused: 9,
          error: 10,
        },
      },
      {
        torrents: [],
        total: 1,
        counts: cachedCounts,
      }
    )

    expect(merged.counts).toBe(cachedCounts)
    expect(merged.stats?.totalDownloadSpeed).toBe(1)
  })
})

describe("mergeLiveTodayTraffic", () => {
  function makeDailyItem(overrides: Partial<InstanceDailyTraffic> = {}): InstanceDailyTraffic {
    return {
      id: 1,
      instanceId: 7,
      date: "2026-08-10",
      uploaded: 100,
      downloaded: 200,
      peakUlSpeed: 10,
      peakDlSpeed: 20,
      dataSource: "session",
      createdAt: "",
      updatedAt: "",
      baselineSessionUploaded: 0,
      baselineSessionDownloaded: 0,
      baselineAlltimeUploaded: 0,
      baselineAlltimeDownloaded: 0,
      baselineAt: "2026-08-10T00:00:00Z",
      ...overrides,
    }
  }

  it("updates only the row matching the live date", () => {
    const today = makeDailyItem()
    const yesterday = makeDailyItem({ id: 2, date: "2026-08-09", uploaded: 55, downloaded: 66 })
    const live: TodayTraffic = { date: "2026-08-10", uploaded: 150, downloaded: 250 }

    const merged = mergeLiveTodayTraffic(
      { items: [today, yesterday], total: 2 },
      live
    )

    expect(merged).toBeDefined()
    expect(merged!.items[0].uploaded).toBe(150)
    expect(merged!.items[0].downloaded).toBe(250)
    expect(merged!.items[0].peakUlSpeed).toBe(10) // untouched fields survive
    expect(merged!.items[0].dataSource).toBe("session")
    expect(merged!.items[1]).toBe(yesterday) // history rows keep their identity
  })

  it("returns the same reference when the live values are unchanged", () => {
    const today = makeDailyItem({ uploaded: 150, downloaded: 250 })
    const response = { items: [today], total: 1 }
    const live: TodayTraffic = { date: "2026-08-10", uploaded: 150, downloaded: 250 }

    expect(mergeLiveTodayTraffic(response, live)).toBe(response)
  })

  it("returns current unchanged when live data is absent or has no matching row", () => {
    const response = { items: [makeDailyItem({ date: "2026-08-09" })], total: 1 }
    expect(mergeLiveTodayTraffic(response, null)).toBe(response)
    expect(mergeLiveTodayTraffic(response, undefined)).toBe(response)
    expect(mergeLiveTodayTraffic(response, { date: "2026-08-08", uploaded: 1, downloaded: 2 })).toBe(response)
    expect(mergeLiveTodayTraffic(undefined, { date: "2026-08-10", uploaded: 1, downloaded: 2 })).toBeUndefined()
  })
})
