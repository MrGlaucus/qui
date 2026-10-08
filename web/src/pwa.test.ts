/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

const { update, register } = vi.hoisted(() => ({
  update: vi.fn().mockResolvedValue(undefined),
  register: vi.fn().mockResolvedValue(undefined),
}))

vi.mock("workbox-window", () => ({
  Workbox: class {
    update = update
    register = register
    addEventListener = vi.fn()
  },
}))

vi.mock("./lib/base-url", () => ({
  getBaseUrl: () => "/",
  withBasePath: (path: string) => `/${path}`,
}))

vi.mock("sonner", () => ({ toast: Object.assign(vi.fn(), { dismiss: vi.fn() }) }))

const reload = vi.fn()
let page: EventTarget & { visibilityState: string }
let browser: EventTarget

beforeEach(() => {
  vi.resetModules()
  vi.clearAllMocks()
  page = Object.assign(new EventTarget(), { visibilityState: "visible" })
  browser = Object.assign(new EventTarget(), { location: { reload } })
  vi.stubGlobal("document", page)
  vi.stubGlobal("window", browser)
  vi.stubGlobal("navigator", { onLine: true, serviceWorker: new EventTarget() })
})

afterEach(() => vi.unstubAllGlobals())

describe("PWA update checks", () => {
  it("checks for a new build on returning to the foreground, without duplicate listeners", async () => {
    const { setupPWAAutoUpdate } = await import("./pwa")
    setupPWAAutoUpdate()
    setupPWAAutoUpdate()
    await vi.dynamicImportSettled()

    expect(register).toHaveBeenCalledTimes(1)
    page.visibilityState = "hidden"
    page.dispatchEvent(new Event("visibilitychange"))
    expect(update).not.toHaveBeenCalled()
    page.visibilityState = "visible"
    page.dispatchEvent(new Event("visibilitychange"))
    expect(update).toHaveBeenCalledTimes(1)
  })

  it.each([true, false])("reloads a stale chunk only when online=%s", async (online) => {
    // Plain HTTP can lack service workers but still needs stale-chunk recovery.
    vi.stubGlobal("navigator", { onLine: online })
    const { setupPWAAutoUpdate } = await import("./pwa")
    setupPWAAutoUpdate()
    browser.dispatchEvent(new Event("vite:preloadError"))
    expect(reload).toHaveBeenCalledTimes(online ? 1 : 0)
    expect(register).not.toHaveBeenCalled()
  })
})
