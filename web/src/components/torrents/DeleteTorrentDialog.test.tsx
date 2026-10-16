/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { cleanup, fireEvent, render, within, waitFor } from "@testing-library/react"
import type { ComponentProps } from "react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { TooltipProvider } from "@/components/ui/tooltip"
import type { CrossInstanceTorrent, Torrent } from "@/types"
import { api } from "@/lib/api"
import { makeFilters } from "@/test/mockFilters"
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

vi.mock("@/lib/api", () => ({ api: { previewTorrentDeletion: vi.fn() } }))
afterEach(() => { cleanup(); vi.clearAllMocks() })

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
  it("offers four deletion modes and clears legacy cross-seed expansion", () => {
    const change = vi.fn(), files = vi.fn(), peers = vi.fn()
    const view = mount([torrent], { onPreserveCrossSeedsChange: change, onDeleteFilesChange: files, onDeleteCrossSeedsChange: peers })
    expect(view.getAllByRole("radio")).toHaveLength(4)
    fireEvent.click(view.getByLabelText(/deleteDialog.modes.preserve.label/))
    expect(change).toHaveBeenCalledWith(true)
    expect(files).not.toHaveBeenCalled()
    expect(peers).toHaveBeenCalledWith(false)
  })
  it("selects include mode without changing the remembered file deletion preference", () => {
    const include = vi.fn(), preserve = vi.fn(), files = vi.fn(), legacy = vi.fn()
    const view = mount([torrent], { onIncludeCrossSeedsChange: include, onPreserveCrossSeedsChange: preserve, onDeleteFilesChange: files, onDeleteCrossSeedsChange: legacy })
    fireEvent.click(view.getByLabelText(/deleteDialog.modes.include.label/))
    expect(include).toHaveBeenCalledWith(true)
    expect(preserve).toHaveBeenCalledWith(false)
    expect(legacy).toHaveBeenCalledWith(false)
    expect(files).not.toHaveBeenCalled()
  })
  it("previews the full filter selection and lists extra related torrents", async () => {
    vi.mocked(api.previewTorrentDeletion).mockResolvedValue({ selected: 23, related: 1, total: 24, relatedTorrents: [{ ...torrent, instanceId: 2, hash: "extra", name: "Synthetic Related Torrent", category: "Related category", tags: "Related tag", ratio: -1 }] })
    const view = mount([torrent], { includeCrossSeeds: true, selectionRequest: { hashes: [], selectAll: true, filters: makeFilters() } })
    await waitFor(() => expect(view.getByText("Synthetic Related Torrent")).not.toBeNull())
    const related = within(view.getByRole("list", { name: "deleteDialog.relatedTorrents" }))
    for (const value of ["Seed B", "Related category", "Related tag", "2 GiB", "46%", "∞"]) {
      expect(related.getByText(value)).not.toBeNull()
    }
    for (const field of ["state", "addedOn"]) {
      expect(related.getByText(`tableColumns.${field}`)).not.toBeNull()
    }
    expect(api.previewTorrentDeletion).toHaveBeenCalledWith(1, expect.objectContaining({ includeCrossSeeds: true, selectAll: true, hashes: [] }))
    expect(view.getByRole("status").textContent).toContain("\"total\":24")
    expect((view.getByRole("button", { name: "deleteDialog.delete" }) as HTMLButtonElement).disabled).toBe(false)
  })
  it("blocks include mode when verification fails", async () => {
    vi.mocked(api.previewTorrentDeletion).mockRejectedValue(new Error("unreadable"))
    const view = mount([torrent], { includeCrossSeeds: true, selectionRequest: { hashes: [torrent.hash] } })
    await waitFor(() => expect(view.getByRole("status").textContent).toContain("deleteDialog.planError"))
    expect((view.getByRole("button", { name: "deleteDialog.delete" }) as HTMLButtonElement).disabled).toBe(true)
  })
  it("waits for the full selection preview before allowing preserving deletion", async () => {
    let resolve: (value: { keepFiles: number; deleteFiles: number }) => void = () => {}
    vi.mocked(api.previewTorrentDeletion).mockReturnValue(new Promise(done => { resolve = done }))
    const filters = makeFilters({ categories: ["Movies"] })
    const view = mount([torrent], { preserveCrossSeeds: true, selectionRequest: { hashes: [], selectAll: true, filters } })
    const button = view.getByRole("button", { name: "deleteDialog.delete" }) as HTMLButtonElement
    expect(button.disabled).toBe(true)
    expect(api.previewTorrentDeletion).toHaveBeenCalledWith(1, expect.objectContaining({ selectAll: true, hashes: [], action: "delete", filters }))
    resolve({ keepFiles: 0, deleteFiles: 23 })
    await waitFor(() => expect(button.disabled).toBe(false))
    expect(view.getByRole("status").textContent).toContain("\"deleteFiles\":23")
  })
  it("blocks deletion after a failed shared-file check", async () => {
    vi.mocked(api.previewTorrentDeletion).mockRejectedValue(new Error("offline"))
    const view = mount([torrent], { preserveCrossSeeds: true, selectionRequest: { hashes: [torrent.hash] } })
    await waitFor(() => expect(view.getByRole("status").textContent).toContain("deleteDialog.planError"))
    expect((view.getByRole("button", { name: "deleteDialog.delete" }) as HTMLButtonElement).disabled).toBe(true)
  })
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
