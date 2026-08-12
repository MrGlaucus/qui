/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useState, useEffect } from "react"

const DEFAULT_ITEMS = ["views", "instances", "status", "categories", "tags", "trackers"]
const VIEWS_SEEDED_KEY = "qui-accordion-views-seeded"
const INSTANCES_SEEDED_KEY = "qui-accordion-instances-seeded"

export function usePersistedAccordion() {
  const [expandedItems, setExpandedItems] = useState<string[]>(() => {
    try {
      const stored = localStorage.getItem("qui-accordion")
      if (!stored) return DEFAULT_ITEMS
      const parsed: unknown = JSON.parse(stored)
      if (!Array.isArray(parsed) || parsed.some((item) => typeof item !== "string")) {
        return DEFAULT_ITEMS
      }
      const items: string[] = parsed

      // Existing users have a stored array predating "views"/"instances", so the
      // new sections would ship collapsed. Expand each once; after that their own
      // toggling wins. The seed markers are written in the effect below, so this
      // stays pure.
      let seeded = items
      if (!localStorage.getItem(VIEWS_SEEDED_KEY) && !seeded.includes("views")) {
        seeded = ["views", ...seeded]
      }
      if (!localStorage.getItem(INSTANCES_SEEDED_KEY) && !seeded.includes("instances")) {
        const viewsIdx = seeded.indexOf("views")
        if (viewsIdx >= 0) {
          seeded = [...seeded.slice(0, viewsIdx + 1), "instances", ...seeded.slice(viewsIdx + 1)]
        } else {
          seeded = ["instances", ...seeded]
        }
      }
      return seeded
    } catch {
      return DEFAULT_ITEMS
    }
  })

  useEffect(() => {
    // A throwing effect unmounts the sidebar to the error boundary; blocked storage must not do that.
    try {
      localStorage.setItem("qui-accordion", JSON.stringify(expandedItems))
      localStorage.setItem(VIEWS_SEEDED_KEY, "1")
      localStorage.setItem(INSTANCES_SEEDED_KEY, "1")
    } catch (error) {
      console.error("Failed to save accordion state to localStorage:", error)
    }
  }, [expandedItems])

  return [expandedItems, setExpandedItems] as const
}
