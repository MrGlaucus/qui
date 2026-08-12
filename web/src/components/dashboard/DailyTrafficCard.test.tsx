import { describe, expect, it } from "vitest"
import { dailyTrafficSourceLabel } from "@/components/dashboard/DailyTrafficCard"
import type { DailyTrafficDataSource } from "@/types"

describe("dailyTrafficSourceLabel", () => {
  it("maps each data source to its i18n key", () => {
    expect(dailyTrafficSourceLabel("session")).toBe("dailyTraffic.sourceSession")
    expect(dailyTrafficSourceLabel("alltime")).toBe("dailyTraffic.sourceAlltime")
    expect(dailyTrafficSourceLabel("restart")).toBe("dailyTraffic.sourceRestart")
    expect(dailyTrafficSourceLabel("reinstall")).toBe("dailyTraffic.sourceReinstall")
  })

  it("falls back to session for unknown sources", () => {
    expect(dailyTrafficSourceLabel("unknown" as DailyTrafficDataSource)).toBe("dailyTraffic.sourceSession")
  })
})