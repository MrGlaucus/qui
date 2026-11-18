/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { flagClass } from "@/lib/countryFlags"

/** A country option/trigger label: flag icon followed by the country name. */
export function CountryOption({ code, label }: { code: string; label: string }) {
  const flag = flagClass(code)
  return (
    <span className="flex items-center gap-2">
      {flag && <span className={`${flag} rounded-sm text-base leading-none shrink-0`} />}
      <span className="truncate">{label}</span>
    </span>
  )
}
