import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { TooltipProvider } from "@/components/ui/tooltip"
import type { Automation, InstanceResponse } from "@/types"
import { WorkflowSyncDialog } from "./WorkflowSyncDialog"

const mocks = vi.hoisted(() => ({
  getAutomationSync: vi.fn(), listAutomations: vi.fn(), configureAutomationSync: vi.fn(), detachAutomationSync: vi.fn(),
  success: vi.fn(), error: vi.fn(),
}))
vi.mock("@/lib/api", () => ({ api: mocks }))
vi.mock("sonner", () => ({ toast: mocks }))
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }))

const rule: Automation = {
  id: 11, instanceId: 1, name: "Source rule", trackerPattern: "*", enabled: true, dryRun: false, notify: false, sortOrder: 1,
  conditions: { schemaVersion: "1", pause: { enabled: true } }, syncKey: "test-sync",
}
const instances = [{ id: 1, name: "Source" }, { id: 2, name: "Target" }] as InstanceResponse[]
const key = (name: string) => `preferences.workflowSync.${name}`

afterEach(cleanup)
beforeEach(() => {
  vi.resetAllMocks()
  mocks.getAutomationSync.mockResolvedValue([])
  mocks.listAutomations.mockResolvedValue([])
  mocks.configureAutomationSync.mockResolvedValue({ created: 1, updated: 0 })
  mocks.detachAutomationSync.mockResolvedValue(undefined)
})

function mount(value = rule) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const close = vi.fn()
  const invalidate = vi.spyOn(client, "invalidateQueries")
  render(<QueryClientProvider client={client}><TooltipProvider><WorkflowSyncDialog rule={value} instances={instances} onClose={close} /></TooltipProvider></QueryClientProvider>)
  return { close, invalidate }
}

describe("WorkflowSyncDialog", () => {
  it("selects automatic sync with local enabled state preserved and invalidates all instances", async () => {
    const { close, invalidate } = mount()
    fireEvent.click(await screen.findByRole("checkbox", { name: "Target" }))
    expect(screen.getByRole("checkbox", { name: new RegExp(key("automatic")) }).getAttribute("data-state")).toBe("checked")
    fireEvent.click(screen.getByRole("button", { name: key("save") }))
    await waitFor(() => expect(close).toHaveBeenCalled())
    expect(mocks.configureAutomationSync).toHaveBeenCalledWith(1, 11, { automatic: true, targets: [{ instanceId: 2, ruleId: undefined, preserveEnabled: true }] })
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["automations"] })
  })

  it("restores linked targets and sends an empty set when removing the follower", async () => {
    const follower = { ...rule, id: 22, instanceId: 2, syncSourceId: 11, syncPreserveEnabled: false }
    mocks.getAutomationSync.mockResolvedValue([follower])
    mocks.listAutomations.mockResolvedValue([follower])
    mount()
    const target = await screen.findByRole("checkbox", { name: "Target" })
    expect(target.getAttribute("data-state")).toBe("checked")
    expect(screen.getByRole("combobox").getAttribute("data-disabled")).not.toBeNull()
    fireEvent.click(target)
    fireEvent.click(screen.getByRole("button", { name: key("save") }))
    await waitFor(() => expect(mocks.configureAutomationSync).toHaveBeenCalledWith(1, 11, { automatic: true, targets: [] }))
  })

  it("offers followers detachment instead of editable sync settings", async () => {
    const { close } = mount({ ...rule, id: 22, instanceId: 2, syncSourceId: 11, syncSourceInstanceId: 1 })
    expect(screen.queryByRole("checkbox")).toBeNull()
    expect(mocks.getAutomationSync).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole("button", { name: key("detach") }))
    await waitFor(() => expect(close).toHaveBeenCalled())
    expect(mocks.detachAutomationSync).toHaveBeenCalledWith(2, 22)
  })

  it("keeps settings open when a target conflict rejects the save", async () => {
    mocks.configureAutomationSync.mockRejectedValue(new Error("conflict"))
    const { close } = mount()
    fireEvent.click(await screen.findByRole("checkbox", { name: "Target" }))
    fireEvent.click(screen.getByRole("button", { name: key("save") }))
    await waitFor(() => expect(mocks.error).toHaveBeenCalledWith(key("failed")))
    expect(close).not.toHaveBeenCalled()
  })
})
