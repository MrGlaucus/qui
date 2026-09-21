import { api } from "@/lib/api"
import type { TrackerTrafficResponse } from "@/types"
import { useQuery } from "@tanstack/react-query"

export function useTrackerTraffic(date: string) {
  return useQuery<TrackerTrafficResponse>({
    queryKey: ["tracker-traffic", date],
    queryFn: () => api.getTrackerTraffic(date),
    refetchInterval: 5_000,
  })
}
