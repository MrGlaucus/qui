import { afterEach, describe, expect, it, vi } from "vitest"
import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { ScriptTrackerSelect } from "./ScriptTrackerSelect"

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }))
vi.mock("@tanstack/react-query", () => ({ useQuery: () => ({ data: { domains: ["a.example.test"], failed: false } }) }))
vi.mock("@/hooks/useTrackerCustomizations", async importOriginal => {
  const original = await importOriginal<typeof import("@/hooks/useTrackerCustomizations")>()
  return { ...original, useTrackerCustomizations: () => ({ data: [{ id: 1, displayName: "Example", domains: ["a.example.test", "b.example.test"] }] }) }
})
vi.mock("@/components/ui/field-help", () => ({ FieldHelp: () => null }))
vi.mock("@/components/ui/multi-select", () => ({ MultiSelect: ({ options, selected, onChange }: {
  options: { label: string; value: string }[]; selected: string[]; onChange: (values: string[]) => void
}) => <div><output>{JSON.stringify(selected)}</output>{options.map(option => <button key={option.value} onClick={() => onChange([option.value])}>{option.label}</button>)}</div> }))

afterEach(cleanup)

describe("Script tracker selection", () => {
  it("preserves a saved subset and missing domains without expanding them", () => {
    render(<ScriptTrackerSelect instanceIds={[1]} selected={["a.example.test", "old.example.test"]} onChange={vi.fn()} />)
    expect(JSON.parse(screen.getByRole("status").textContent ?? "[]")).toEqual(["a.example.test", "old.example.test"])
    expect(screen.getByText("old.example.test")).not.toBeNull()
  })
  it("expands a selected merged site into individual domains", () => {
    const onChange = vi.fn()
    render(<ScriptTrackerSelect instanceIds={[1]} selected={[]} onChange={onChange} />)
    fireEvent.click(screen.getByText("Example"))
    expect(onChange).toHaveBeenCalledWith(["a.example.test", "b.example.test"])
  })
})
