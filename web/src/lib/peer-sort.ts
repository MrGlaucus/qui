/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { SortedPeer } from "@/types"

export type PeerSortField = "up_speed" | "dl_speed" | "uploaded" | "downloaded" | "progress" | "key" | "client" | "isp"
export interface PeerSort {
  field: PeerSortField
  direction: "asc" | "desc"
}

export function sortPeers(peers: readonly SortedPeer[], sort: PeerSort, ispData: Record<string, string | null>): SortedPeer[] {
  const value = (peer: SortedPeer) => {
    if (sort.field === "isp") {
      const isp = peer.ip ? ispData[peer.ip] : undefined
      return isp === "loading" ? undefined : isp
    }
    return peer[sort.field]
  }
  return [...peers].sort((a, b) => {
    const left = value(a)
    const right = value(b)
    const leftMissing = left == null || left === ""
    const rightMissing = right == null || right === ""
    // Unknown values stay last in either direction; zero is a valid value.
    if (leftMissing || rightMissing) return Number(leftMissing) - Number(rightMissing)
    const difference = typeof left === "number" && typeof right === "number"
      ? left - right
      : String(left).localeCompare(String(right), undefined, { numeric: true })
    return sort.direction === "asc" ? difference : -difference
  })
}
