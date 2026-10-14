/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { PeerIsp } from "./PeerIsp"

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }))
afterEach(cleanup)

describe("PeerIsp", () => {
  it("queries only the clicked peer and does not trigger row actions", () => {
    const lookup = vi.fn(), rowClick = vi.fn()
    render(<div onClick={rowClick}>
      <PeerIsp ip="192.0.2.1" value={undefined} incognitoMode={false} onLookup={lookup} />
      <PeerIsp ip="2001:db8::2" value={undefined} incognitoMode={false} onLookup={lookup} />
    </div>)
    fireEvent.click(screen.getByRole("button", { name: "detailsPanel.queryIsp 2001:db8::2" }))
    expect(lookup.mock.calls).toEqual([["2001:db8::2"]])
    expect(rowClick).not.toHaveBeenCalled()
  })

  it("disables a pending query and allows retry after failure", () => {
    const lookup = vi.fn()
    const view = render(<PeerIsp ip="192.0.2.1" value="loading" incognitoMode={false} onLookup={lookup} />)
    const button = screen.getByRole("button") as HTMLButtonElement
    expect(button.disabled).toBe(true)
    expect(button.getAttribute("aria-busy")).toBe("true")
    fireEvent.click(button)
    expect(lookup).not.toHaveBeenCalled()
    view.rerender(<PeerIsp ip="192.0.2.1" value={null} incognitoMode={false} onLookup={lookup} />)
    fireEvent.click(screen.getByRole("button", { name: "peersTable.retry 192.0.2.1" }))
    expect(lookup).toHaveBeenCalledWith("192.0.2.1")
  })

  it("displays cached results without querying and hides them in incognito mode", () => {
    const lookup = vi.fn()
    const view = render(<PeerIsp ip="192.0.2.1" value="Example Network" incognitoMode={false} onLookup={lookup} />)
    expect(screen.getByText("Example Network").getAttribute("title")).toBe("Example Network")
    expect(screen.queryByRole("button")).toBeNull()
    expect(lookup).not.toHaveBeenCalled()
    view.rerender(<PeerIsp ip="192.0.2.1" value="Example Network" incognitoMode onLookup={lookup} />)
    expect(screen.queryByText("Example Network")).toBeNull()
    expect(screen.queryByRole("button")).toBeNull()
  })

  it("does not offer a lookup for peers without an IP", () => {
    render(<PeerIsp value={undefined} incognitoMode={false} onLookup={vi.fn()} />)
    expect(screen.queryByRole("button")).toBeNull()
  })
})
