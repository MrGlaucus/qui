import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { ConditionDuration } from "./ConditionDuration"

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }))
vi.mock("@/components/ui/field-help", () => ({ FieldHelp: () => null }))
afterEach(cleanup)

describe("ConditionDuration", () => {
  it("edits and clears a duration without losing the predicate", () => {
    const onChange = vi.fn()
    const condition = { field: "UP_SPEED", operator: "LESS_THAN", value: "204800", holdForSeconds: 30 } as const
    render(<ConditionDuration condition={condition} onChange={onChange} />)
    const input = screen.getByRole("spinbutton")
    fireEvent.change(input, { target: { value: "60" } })
    expect(onChange).toHaveBeenLastCalledWith({ ...condition, holdForSeconds: 60 })
    fireEvent.change(input, { target: { value: "0" } })
    expect(onChange).toHaveBeenLastCalledWith({ ...condition, holdForSeconds: undefined })
    onChange.mockClear()
    fireEvent.change(input, { target: { value: "-1" } })
    expect(onChange).not.toHaveBeenCalled()
  })
})
