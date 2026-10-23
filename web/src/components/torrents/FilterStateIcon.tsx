/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { cn } from "@/lib/utils"
import type { LucideIcon } from "lucide-react"

/** Time the red copy takes to come back when a hold is released early. */
const RESTORE_MS = 150

interface FilterStateIconProps {
  icon: LucideIcon
  active: boolean
  /** True while a hold is in progress; drives the red fade-out. */
  draining?: boolean
  /** Fade length in ms. Should match the hook's drain window. */
  drainMs?: number
  className?: string
  /** Colour of the base copy, for buttons that tint their own text. */
  baseClassName?: string
}

/**
 * Filter toggle icon that turns red while filters are active.
 *
 * Two copies are stacked: the base one in the button's own colour, and a red
 * one on top. While the button is held the red copy fades out over `drainMs`,
 * so the colour visibly runs out and the user can see the hold completing;
 * releasing early brings it back quickly. Fading is a plain opacity
 * transition rather than a keyframe animation, so it reverses mid-way without
 * jumping.
 */
export function FilterStateIcon({
  icon: Icon,
  active,
  draining = false,
  drainMs = 0,
  className,
  baseClassName,
}: FilterStateIconProps) {
  const fadeMs = draining && drainMs > 0 ? drainMs : RESTORE_MS

  return (
    <span className="relative inline-flex">
      <Icon className={cn(className, baseClassName)} />
      {active && (
        <Icon
          aria-hidden
          className={cn(
            // Reduced motion keeps the red in place until the filters actually
            // clear, instead of fading it out with nothing to replace it.
            "absolute inset-0 text-red-500 motion-reduce:[--filter-drain-target:1]",
            className
          )}
          style={{
            opacity: `var(--filter-drain-target, ${draining ? 0 : 1})`,
            transition: `opacity ${fadeMs}ms linear`,
          }}
        />
      )}
    </span>
  )
}
