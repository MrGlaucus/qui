/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useCallback, useEffect, useRef, useState } from "react"
import type { MouseEvent as ReactMouseEvent, PointerEvent as ReactPointerEvent } from "react"

/** Hold time before the drain animation starts, so a plain click stays a click. */
export const HOLD_GRACE_MS = 200
/** Length of the drain animation. Must match the one the icon renders. */
export const HOLD_DRAIN_MS = 450

const DEFAULT_MOVE_TOLERANCE_PX = 10

interface UseLongPressOptions {
  onLongPress: () => void
  /**
   * Makes the gesture inert: no timers, no drain, no callback. Callers use
   * this when there is nothing to clear or the panel is already open, so a
   * press there stays a plain press.
   */
  disabled?: boolean
  graceMs?: number
  drainMs?: number
  moveTolerancePx?: number
}

interface LongPressHandlers {
  onPointerDown: (event: ReactPointerEvent<HTMLElement>) => void
  onPointerMove: (event: ReactPointerEvent<HTMLElement>) => void
  onPointerUp: () => void
  onPointerLeave: () => void
  onPointerCancel: () => void
  onContextMenu: (event: ReactMouseEvent<HTMLElement>) => void
}

/**
 * Hold-to-confirm for the filter toggles.
 *
 * Pressing and holding first waits out `graceMs` so an ordinary click never
 * starts the animation, then reports `isHolding` while the icon drains for
 * `drainMs`. The callback fires only when the drain finishes, which is also
 * when the icon has gone quiet, so the action and its toast land exactly at
 * the moment the press has visibly completed.
 *
 * Releasing early, or moving far enough to be a scroll, cancels everything.
 * Passing `disabled` turns the whole gesture off, so a button can keep a
 * single hold meaning only in the states where it makes sense.
 *
 * The caller must call `consumeLongPress` from its click handler: a completed
 * hold still emits a click, and that click must not also toggle the sidebar.
 * The flag is one-shot and reset on the next pointerdown, so a long press
 * without a following click cannot swallow an unrelated one later.
 */
export function useLongPress({
  onLongPress,
  disabled = false,
  graceMs = HOLD_GRACE_MS,
  drainMs = HOLD_DRAIN_MS,
  moveTolerancePx = DEFAULT_MOVE_TOLERANCE_PX,
}: UseLongPressOptions): {
  handlers: LongPressHandlers
  isHolding: boolean
  consumeLongPress: () => boolean
} {
  const [isHolding, setIsHolding] = useState(false)
  const graceTimerRef = useRef<number | null>(null)
  const drainTimerRef = useRef<number | null>(null)
  const originRef = useRef<{ x: number; y: number } | null>(null)
  const firedRef = useRef(false)
  const onLongPressRef = useRef(onLongPress)
  onLongPressRef.current = onLongPress

  const cancel = useCallback(() => {
    if (graceTimerRef.current !== null) {
      window.clearTimeout(graceTimerRef.current)
      graceTimerRef.current = null
    }
    if (drainTimerRef.current !== null) {
      window.clearTimeout(drainTimerRef.current)
      drainTimerRef.current = null
    }
    originRef.current = null
    setIsHolding(false)
  }, [])

  useEffect(() => cancel, [cancel])

  // A hold that starts while enabled must not survive the caller disabling it
  // (e.g. clearing the filters mid-press makes them disappear).
  useEffect(() => {
    if (disabled) {
      cancel()
    }
  }, [cancel, disabled])

  const onPointerDown = useCallback((event: ReactPointerEvent<HTMLElement>) => {
    if (event.button !== 0) {
      return
    }

    cancel()
    firedRef.current = false

    if (disabled) {
      return
    }

    originRef.current = { x: event.clientX, y: event.clientY }

    graceTimerRef.current = window.setTimeout(() => {
      graceTimerRef.current = null
      setIsHolding(true)

      drainTimerRef.current = window.setTimeout(() => {
        drainTimerRef.current = null
        setIsHolding(false)
        firedRef.current = true
        navigator.vibrate?.(10)
        onLongPressRef.current()
      }, drainMs)
    }, graceMs)
  }, [cancel, disabled, drainMs, graceMs])

  const onPointerMove = useCallback((event: ReactPointerEvent<HTMLElement>) => {
    const origin = originRef.current
    if (!origin) {
      return
    }

    // A finger drifts more than a mouse over the same hold, so allow it more
    // slack before treating the gesture as a scroll.
    const tolerance = event.pointerType === "touch" ? moveTolerancePx * 1.6 : moveTolerancePx

    if (
      Math.abs(event.clientX - origin.x) > tolerance ||
      Math.abs(event.clientY - origin.y) > tolerance
    ) {
      cancel()
    }
  }, [cancel, moveTolerancePx])

  const consumeLongPress = useCallback(() => {
    if (!firedRef.current) {
      return false
    }

    firedRef.current = false
    return true
  }, [])

  // Touch long press otherwise opens the browser's own context menu mid-hold,
  // which cancels the pointer sequence. Suppress it for as long as a press is
  // in progress; a right click never sets an origin, so it still works.
  const onContextMenu = useCallback((event: ReactMouseEvent<HTMLElement>) => {
    if (originRef.current) {
      event.preventDefault()
    }
  }, [])

  return {
    handlers: {
      onPointerDown,
      onPointerMove,
      onPointerUp: cancel,
      onPointerLeave: cancel,
      onPointerCancel: cancel,
      onContextMenu,
    },
    isHolding,
    consumeLongPress,
  }
}
