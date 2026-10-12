import { afterEach, describe, expect, it, vi } from "vitest"
import { cleanup, renderHook, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import type { ReactNode } from "react"
import { api } from "@/lib/api"
import { useHistoricalTrackers } from "./useHistoricalTrackers"

vi.mock("@/lib/api", () => ({ api: { getTrackerTraffic: vi.fn() } }))
afterEach(() => { cleanup(); vi.clearAllMocks() })
function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>{children}</QueryClientProvider>
}
describe("historical trackers", () => {
  it("includes old totals, deduplicates domains and ignores non-domain keys", async () => {
    const row = (trackerKey: string) => ({ trackerKey, date: "", uploaded: 0, downloaded: 0 })
    vi.mocked(api.getTrackerTraffic).mockResolvedValue({ date: "", totals: [row("domain:old.test"), row("customization:1")], items: [row("domain:old.test"), row("domain:today.test"), row("domain:")] })
    const { result } = renderHook(() => useHistoricalTrackers(), { wrapper })
    await waitFor(() => expect(result.current.data).toEqual(["old.test", "today.test"]))
    expect(api.getTrackerTraffic).toHaveBeenCalledWith(expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/))
  })
  it("does not fetch when the selector is closed", () => {
    renderHook(() => useHistoricalTrackers(false), { wrapper })
    expect(api.getTrackerTraffic).not.toHaveBeenCalled()
  })
})
