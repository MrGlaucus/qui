import { api } from "@/lib/api"
import type { InstanceDailyTrafficResponse } from "@/types"
import { useQuery } from "@tanstack/react-query"

const STALE_MS = 30_000

export function useDailyTraffic(instanceId: number, days = 7, enabled = true) {
  return useQuery<InstanceDailyTrafficResponse>({
    queryKey: ["daily-traffic", instanceId, days],
    queryFn: () => api.getDailyTraffic(instanceId, days),
    staleTime: STALE_MS,
    enabled,
  })
}
