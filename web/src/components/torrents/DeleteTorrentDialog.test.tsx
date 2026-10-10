/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { cleanup, fireEvent, render, within } from "@testing-library/react"
import type { ComponentProps } from "react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { TooltipProvider } from "@/components/ui/tooltip"
import type { CrossInstanceTorrent, Torrent } from "@/types"
import { DeleteTorrentDialog } from "./DeleteTorrentDialog"

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string, values?: Record<string, unknown>) => values ? `${key} ${JSON.stringify(values)}` : key }),
}))
vi.mock("@/hooks/useInstances", () => ({
  useInstances: () => ({ instances: [{ id: 1, name: "Seed A", countryCode: "de" }, { id: 2, name: "Seed B", countryCode: "ca" }] }),
}))
vi.mock("@/hooks/useDateTimeFormatters", () => ({
  useDateTimeFormatters: () => ({ formatAddedOn: (timestamp: number) => `formatted-date:${timestamp}` }),
}))

afterEach(cleanup)

function mount(torrents: Torrent[], overrides: Partial<ComponentProps<typeof DeleteTorrentDialog>> = {}) {
  const noop = () => {}
  return render(<TooltipProvider><DeleteTorrentDialog
    open onOpenChange={noop} count={torrents.length} torrents={torrents} instanceId={1}
    totalSize={0} formattedSize="0 B" deleteFiles={false} onDeleteFilesChange={noop}
    isDeleteFilesLocked={false} onToggleDeleteFilesLock={noop}
    deleteCrossSeeds={false} onDeleteCrossSeedsChange={noop}
    showBlockCrossSeeds={false} blockCrossSeeds={false} onBlockCrossSeedsChange={noop}
    onConfirm={noop} {...overrides}
  /></TooltipProvider>)
}

const torrent = {
  hash: "same-hash", name: "Synthetic.Example.2026.1080p", category: "Movies", tags: "Archive,  Ready ",
  size: 2 * 1024 ** 3, state: "downloading", progress: 0.456, added_on: 1700000000, ratio: 1.234,
} as Torrent

describe("DeleteTorrentDialog selection preview", () => {
  it("shows category, trimmed tags and the owning instance for single-instance selection", () => {
    const view = mount([torrent])
    const row = within(view.getByRole("listitem"))
    for (const text of [torrent.name, "Movies", "Archive", "Ready", "Seed A"]) {
      expect(row.getByText(text)).not.toBeNull()
    }
  })

  it("keeps identical hashes in different instances separate and shows empty metadata", () => {
    const first = { ...torrent, instanceId: 1, instanceName: "Seed A" } as CrossInstanceTorrent
    const second = { ...torrent, instanceId: 2, instanceName: "Seed B", category: "", tags: "" } as CrossInstanceTorrent
    const view = mount([first, second], { instanceId: 0 })
    const rows = view.getAllByRole("listitem")
    expect(rows).toHaveLength(2)
    expect(within(rows[0]).getByText("Seed A")).not.toBeNull()
    const row = within(rows[1])
    expect(row.getByText("Seed B")).not.toBeNull()
    expect(row.getByText("reportDialog.uncategorized")).not.toBeNull()
    expect(row.getByText("reportDialog.noTags")).not.toBeNull()
    expect(rows[1].querySelector(".fi-ca")).not.toBeNull()
  })

  it("shows each torrent's size, localized state, progress, formatted added time and ratio", () => {
    const view = mount([torrent])
    const row = within(view.getByRole("listitem"))
    for (const text of ["2 GiB", "stateLabels.downloading {\"defaultValue\":\"Downloading\"}", "46%", "formatted-date:1700000000", "1.23"]) {
      expect(row.getByText(text)).not.toBeNull()
    }
  })

  it("does not round unfinished torrents to 100% and preserves the infinite ratio sentinel", () => {
    const view = mount([{ ...torrent, progress: 0.9999, ratio: -1 }])
    const row = within(view.getByRole("listitem"))
    expect(row.getByText("99.9%")).not.toBeNull()
    expect(row.getByText("∞")).not.toBeNull()
  })

  it("warns about unloaded selected torrents without changing the delete action", () => {
    const onConfirm = vi.fn()
    const view = mount([torrent], { count: 200, onConfirm })
    expect(view.getByText("deleteDialog.partialPreview {\"shown\":1,\"total\":200}")).not.toBeNull()
    expect(onConfirm).not.toHaveBeenCalled()
    fireEvent.click(view.getByRole("button", { name: "deleteDialog.delete" }))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })
})
