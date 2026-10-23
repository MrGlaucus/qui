/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

/**
 * Window event asking the torrents page to drop every active filter.
 *
 * The filter toggles live in the header and in the mobile bottom bar, both of
 * which render outside the page that owns the filter state. They dispatch this
 * instead of threading a callback through every layout layer.
 */
export const CLEAR_TORRENT_FILTERS_EVENT = "qui-clear-torrent-filters"

export function requestClearTorrentFilters(): void {
  window.dispatchEvent(new Event(CLEAR_TORRENT_FILTERS_EVENT))
}
