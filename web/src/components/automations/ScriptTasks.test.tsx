import { afterEach, describe, expect, it, vi } from "vitest"
import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { ScriptTasks } from "./ScriptTasks"

const mocks = vi.hoisted(() => ({
  overview: { scripts: [{ id: "hdsky", name: "HDSKY", description: "", code: "pass", builtin: true, parameters: { tracker_domains: [] } }], tasks: [], python_available: true },
  instances: [{ id: 1, name: "Test instance" }],
  cache: { invalidateQueries: vi.fn() },
  api: { saveScriptTask: vi.fn(), getScriptTasks: vi.fn(), getScriptTaskRuns: vi.fn() },
}))
vi.mock("@tanstack/react-query", () => ({ useQueryClient: () => mocks.cache, useQuery: ({ queryKey }: { queryKey: string[] }) => ({ data: queryKey[0] === "script-tasks" ? mocks.overview : [] }) }))
vi.mock("@/hooks/useInstances", () => ({ useInstances: () => ({ instances: mocks.instances }) }))
vi.mock("@/lib/api", () => ({ api: mocks.api }))
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key === "scripts.reference.entries" ? [] : key }) }))

afterEach(() => { cleanup(); vi.clearAllMocks() })

describe("Script tasks", () => {
  it("creates HDSKY tasks disabled with an explicit instance scope", () => {
    render(<ScriptTasks />)
    fireEvent.click(screen.getByText("scripts.newTask"))
    expect(screen.getByLabelText("scripts.enabled").getAttribute("aria-checked")).toBe("false")
    expect(screen.getByLabelText("Test instance").getAttribute("aria-checked")).toBe("false")
    expect(screen.queryByText("scripts.allInstances")).toBeNull()
    expect(mocks.api.saveScriptTask).not.toHaveBeenCalled()
    expect(screen.getByText("scripts.hdskyHint")).not.toBeNull()
  })
  it("opens built-in code read-only and exposes copy as a separate action", () => {
    render(<ScriptTasks />)
    fireEvent.click(screen.getByText("scripts.viewCode"))
    expect((screen.getByLabelText("scripts.code") as HTMLTextAreaElement).readOnly).toBe(true)
    expect(screen.queryByText("scripts.save")).toBeNull()
    expect(screen.getByText("scripts.copy")).not.toBeNull()
  })
})
