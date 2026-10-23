/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useCallback, useEffect, useRef } from "react"
import type { MouseEvent as ReactMouseEvent, PointerEvent as ReactPointerEvent } from "react"

const DEFAULT_DELAY_MS = 500
const DEFAULT_MOVE_TOLERANCE_PX = 10
/** A long press only suppresses the click that belongs to it, never a later one. */
const SUPPRESS_WINDOW_MS = 800

interface UseLongPressOptions {
  onLongPress: () => void
  delayMs?: number
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
 * Fires a callback when a button is held down for `delayMs`.
 *
 * Used for the filter toggles, where a short press opens the sidebar and a
 * long press clears the filters. The press is abandoned as soon as the pointer
 * travels more than a few pixels, so scrolling a list that starts on the
 * button does not clear anything by accident.
 *
 * The caller must call `consumeLongPress` from its click handler: after a long
 * press the browser still emits a click, and that click must not also toggle
 * the sidebar.
 */
export function useLongPress({
  onLongPress,
  delayMs = DEFAULT_DELAY_MS,
  moveTolerancePx = DEFAULT_MOVE_TOLERANCE_PX,
}: UseLongPressOptions): { handlers: LongPressHandlers; consumeLongPress: () => boolean } {
  const timerRef = useRef<number | null>(null)
  const originRef = useRef<{ x: number; y: number } | null>(null)
  const firedAtRef = useRef(0)
  const onLongPressRef = useRef(onLongPress)
  onLongPressRef.current = onLongPress

  const cancel = useCallback(() => {
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current)
      timerRef.current = null
    }
    originRef.current = null
  }, [])

  useEffect(() => cancel, [cancel])

  const onPointerDown = useCallback((event: ReactPointerEvent<HTMLElement>) => {
    if (event.button !== 0) {
      return
    }

    cancel()
    originRef.current = { x: event.clientX, y: event.clientY }
    timerRef.current = window.setTimeout(() => {
      timerRef.current = null
      firedAtRef.current = Date.now()
      navigator.vibrate?.(10)
      onLongPressRef.current()
    }, delayMs)
  }, [cancel, delayMs])

  const onPointerMove = useCallback((event: ReactPointerEvent<HTMLElement>) => {
    const origin = originRef.current
    if (!origin) {
      return
    }

    if (
      Math.abs(event.clientX - origin.x) > moveTolerancePx ||
      Math.abs(event.clientY - origin.y) > moveTolerancePx
    ) {
      cancel()
    }
  }, [cancel, moveTolerancePx])

  const consumeLongPress = useCallback(() => {
    if (firedAtRef.current === 0) {
      return false
    }

    const firedRecently = Date.now() - firedAtRef.current < SUPPRESS_WINDOW_MS
    firedAtRef.current = 0
    return firedRecently
  }, [])

  // Touch long press otherwise opens the browser's own context menu.
  const onContextMenu = useCallback((event: ReactMouseEvent<HTMLElement>) => {
    if (firedAtRef.current !== 0) {
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
    consumeLongPress,
  }
}
