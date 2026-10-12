import { afterEach, describe, expect, it, vi } from "vitest"
import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import translations from "@/i18n/locales/zh-CN/automations.json"
import { ScriptApiReference } from "./ScriptApiReference"

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => {
  const name = key.replace("scripts.reference.", "") as keyof typeof translations.scripts.reference
  return translations.scripts.reference[name]
} }) }))

afterEach(cleanup)

describe("Script API reference", () => {
  it("opens the Chinese contract and filters methods without hiding field documentation", () => {
    render(<ScriptApiReference />)
    fireEvent.click(screen.getByRole("button"))
    expect(screen.getByText("ctx.instances()")).not.toBeNull()
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "delete_categories" } })
    expect(screen.queryByText("ctx.instances()")).toBeNull()
    expect(screen.getByText("ctx.delete_categories(instance_id, categories)")).not.toBeNull()
    expect(screen.getByText(translations.scripts.reference.fields)).not.toBeNull()
  })
})
