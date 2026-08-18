/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

export interface Instance {
  id: number
  name: string
  host: string
  username: string
  hasApiKey?: boolean
  basicUsername?: string
  tlsSkipVerify: boolean
  hasLocalFilesystemAccess: boolean
  // Hardlink mode settings (per-instance)
  useHardlinks: boolean
  hardlinkBaseDir: string
  hardlinkDirPreset: "flat" | "by-tracker" | "by-instance"
  // Reflink mode (copy-on-write) - mutually exclusive with hardlink mode
  useReflinks: boolean
  // Fallback to regular mode when reflink/hardlink fails
  fallbackToRegularMode: boolean
  // Collect and record daily traffic statistics for this instance
  dailyTrafficEnabled: boolean
  countryCode: string
  sortOrder: number
  isActive: boolean
  reannounceSettings: InstanceReannounceSettings
}

export interface InstanceFormData {
  name: string
  host: string
  username?: string
  password?: string
  apiKey?: string
  basicUsername?: string
  basicPassword?: string
  tlsSkipVerify: boolean
  hasLocalFilesystemAccess: boolean
  // Hardlink mode settings (per-instance)
  useHardlinks?: boolean
  hardlinkBaseDir?: string
  hardlinkDirPreset?: "flat" | "by-tracker" | "by-instance"
  // Reflink mode (copy-on-write) - mutually exclusive with hardlink mode
  useReflinks?: boolean
  // Fallback to regular mode when reflink/hardlink fails
  fallbackToRegularMode?: boolean
  dailyTrafficEnabled?: boolean
  countryCode?: string
  reannounceSettings: InstanceReannounceSettings
  /** When set, creating the instance copies credentials from this source. */
  cloneInstanceId?: number
}

export interface InstanceReannounceSettings {
  enabled: boolean
  initialWaitSeconds: number
  reannounceIntervalSeconds: number
  maxAgeSeconds: number
  maxRetries: number
  aggressive: boolean
  monitorAll: boolean
  excludeCategories: boolean
  categories: string[]
  excludeTags: boolean
  tags: string[]
  excludeTrackers: boolean
  trackers: string[]
}

// Reannounce settings constraints - shared across components
export const REANNOUNCE_CONSTRAINTS = {
  MIN_INITIAL_WAIT: 5,
  MIN_INTERVAL: 5,
  MIN_MAX_AGE: 60,
  MIN_MAX_RETRIES: 1,
  MAX_MAX_RETRIES: 50,
  DEFAULT_MAX_RETRIES: 50,
} as const

export interface InstanceCrossSeedCompletionSettings {
  instanceId: number
  enabled: boolean
  categories: string[]
  tags: string[]
  excludeCategories: string[]
  excludeTags: string[]
  indexerIds: number[]
  bypassTorznabCache: boolean
  delaySeconds: number
}

/**
 * A torrent match found by the backend using proper release metadata parsing (rls library).
 */
export interface LocalCrossSeedMatch {
  instanceId: number
  instanceName: string
  hash: string
  name: string
  size: number
  progress: number
  savePath: string
  contentPath: string
  category: string
  tags: string
  state: string
  tracker: string
  trackerHealth?: string
  matchType: "content_path" | "hardlink" | "reflink" | "name" | "release"
}

export interface InstanceReannounceActivity {
  instanceId: number
  hash: string
  torrentName?: string
  trackers?: string
  outcome: "skipped" | "failed" | "succeeded" | "started"
  reason?: string
  timestamp: string
}

export interface InstanceReannounceCandidate {
  instanceId: number
  hash: string
  torrentName?: string
  trackers?: string
  timeActiveSeconds?: number
  category?: string
  tags?: string
  state: "watching" | "reannouncing" | "cooldown"
  hasTrackerProblem: boolean
  waitingForInitial: boolean
}

export interface InstanceError {
  id: number
  instanceId: number
  errorType: string
  errorMessage: string
  occurredAt: string
}

export interface InstanceResponse extends Instance {
  connected: boolean
  hasDecryptionError: boolean
  recentErrors?: InstanceError[]
  connectionStatus?: string
}

export interface InstanceCapabilities {
  supportsTorrentCreation: boolean
  supportsTorrentExport: boolean
  supportsSetTags: boolean
  supportsSetComment: boolean
  supportsTrackerHealth: boolean
  supportsTrackerEditing: boolean
  supportsRenameTorrent: boolean
  supportsRenameFile: boolean
  supportsRenameFolder: boolean
  supportsFilePriority: boolean
  supportsSubcategories: boolean
  subcategoriesAlwaysEnabled: boolean
  supportsTorrentTmpPath: boolean
  supportsPathAutocomplete: boolean
  supportsFreeSpacePathSource: boolean
  supportsSetRSSFeedURL: boolean
  supportsShareLimitsAction: boolean
  supportsShareLimitsMode?: boolean
  webAPIVersion?: string
}

export type DailyTrafficDataSource = "session" | "alltime" | "restart" | "reinstall"

export interface InstanceDailyTraffic {
  id: number
  instanceId: number
  date: string // YYYY-MM-DD in UI timezone
  uploaded: number
  downloaded: number
  peakUlSpeed: number
  peakDlSpeed: number
  dataSource: DailyTrafficDataSource
  createdAt: string
  updatedAt: string
  baselineSessionUploaded: number
  baselineSessionDownloaded: number
  baselineAlltimeUploaded: number
  baselineAlltimeDownloaded: number
  baselineAt: string // RFC3339, first sample time of the day
}

export interface InstanceDailyTrafficResponse {
  items: InstanceDailyTraffic[]
  total: number
}
