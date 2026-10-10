/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { cleanup, render } from "@testing-library/react"
import type { ComponentType } from "react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import type { ZodType } from "zod"
import type {} from "@/router"
import type {} from "@/vite-env"
import type {} from "@/types/launch-queue"
import { ALL_INSTANCES_ID } from "@/lib/instances"
import { Route } from "./instances.index"

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  torrents: vi.fn(() => null),
  layout: { setLayoutRouteState: vi.fn(), resetLayoutRouteState: vi.fn() },
}))
vi.mock("@/pages/Torrents", () => ({ Torrents: mocks.torrents }))
vi.mock("@/contexts/LayoutRouteContext", () => ({ useLayoutRoute: () => mocks.layout }))
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }))

const Component = Route.options.component as ComponentType
const schema = Route.options.validateSearch as ZodType
const filterKey = `qui-filters-${ALL_INSTANCES_ID}`

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.spyOn(Route, "useNavigate").mockReturnValue(mocks.navigate)
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe("unified tracker navigation", () => {
  it("accepts grouped domains and the no-tracker sentinel, ignoring malformed input", () => {
    expect(schema.parse({ trackers: ["one.example.test", "two.example.test"] })).toEqual({ trackers: ["one.example.test", "two.example.test"] })
    expect(schema.parse({ trackers: [""] })).toEqual({ trackers: [""] })
    expect(schema.parse({ trackers: "invalid" })).toEqual({ trackers: undefined })
    expect(schema.parse({ trackers: [] })).toEqual({ trackers: undefined })
  })

  it("replaces stale filters and instance scope before rendering the list", () => {
    localStorage.setItem("qui-filters-global", JSON.stringify({ status: ["downloading"], excludeStatus: ["uploading"] }))
    localStorage.setItem(filterKey, JSON.stringify({ categories: ["old"], excludeTrackers: ["one.example.test"], hashes: ["old-hash"], expr: "ratio < 1" }))
    localStorage.setItem("qui-unified-instance-filter", "1,2")
    localStorage.setItem(`qui-column-filters-${ALL_INSTANCES_ID}`, JSON.stringify([{ columnId: "name", operator: "contains", value: "old" }]))
    const routeSearch = vi.spyOn(Route, "useSearch").mockReturnValue({ trackers: ["two.example.test", "one.example.test", "one.example.test"] })
    const view = render(<Component />)
    expect(mocks.torrents).not.toHaveBeenCalled()
    expect(mocks.layout.setLayoutRouteState).toHaveBeenLastCalledWith({ showInstanceControls: false, instanceId: ALL_INSTANCES_ID })
    expect(JSON.parse(localStorage.getItem("qui-filters-global")!)).toEqual({ status: [], excludeStatus: [] })
    expect(JSON.parse(localStorage.getItem(filterKey)!)).toEqual({
      categories: [], excludeCategories: [], tags: [], excludeTags: [],
      trackers: ["one.example.test", "two.example.test"], excludeTrackers: [], instances: [], excludeInstances: [], expr: "",
    })
    expect(localStorage.getItem("qui-unified-instance-filter")).toBe("")
    expect(localStorage.getItem(`qui-column-filters-${ALL_INSTANCES_ID}`)).toBe("[]")
    const navigation = mocks.navigate.mock.calls[0][0]
    expect(navigation.replace).toBe(true)
    expect(navigation.search({ trackers: ["one.example.test"], modal: "tasks" })).toEqual({ trackers: undefined, modal: "tasks" })
    routeSearch.mockReturnValue({})
    view.rerender(<Component />)
    expect(mocks.torrents).toHaveBeenCalled()
    expect(mocks.layout.setLayoutRouteState).toHaveBeenLastCalledWith({ showInstanceControls: true, instanceId: ALL_INSTANCES_ID })
  })

  it("keeps saved filters on an ordinary visit", () => {
    const stored = JSON.stringify({ trackers: ["saved.example.test"] })
    localStorage.setItem(filterKey, stored)
    localStorage.setItem("qui-unified-instance-filter", "2")
    vi.spyOn(Route, "useSearch").mockReturnValue({})
    render(<Component />)
    expect(localStorage.getItem(filterKey)).toBe(stored)
    expect(localStorage.getItem("qui-unified-instance-filter")).toBe("2")
    expect(mocks.navigate).not.toHaveBeenCalled()
    expect(mocks.torrents).toHaveBeenCalled()
  })
})
