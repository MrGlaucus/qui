import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"

/** Persisted dashboard domains, including trackers with no remaining torrents. */
export function useHistoricalTrackers(enabled = true) {
  const now = new Date()
  const date = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`
  return useQuery({
    queryKey: ["historical-tracker-domains", date],
    queryFn: async () => {
      const traffic = await api.getTrackerTraffic(date)
      return [...new Set([...traffic.totals, ...traffic.items]
        .filter(row => row.trackerKey.startsWith("domain:"))
        .map(row => row.trackerKey.slice("domain:".length))
        .filter(Boolean))]
    },
    enabled,
    staleTime: 60_000,
  })
}
