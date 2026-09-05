/* =========================================================================
   COSCA DESKTOP — SPLITTER (divisor redimensionável de panes)
   Ferramenta de engenharia, não decoração. Respeita a missão §13/§16/§19:
   - 100% UI: NUNCA chama backend/filesystem/wails durante o drag.
   - Pointer Events nativos (sem lib de drag). `setPointerCapture` no elemento
     RAIZ + React onPointer* → SEM listeners globais que vazam após desmontar.
   - [`requestAnimationFrame`] limita os updates a ~1/frame durante o drag.
   - Keyboard (WCAG/§16): ←/→ (vertical) e ↑/↓ (horizontal) ±8px; Shift ±40px;
     Home → min; End → max.
   - Acessibilidade: role="separator" + aria-orientation/label/valuenow/min/max.

   Assinatura: Splitter({ orientation, onResize:(dx)=>void, label, min, max, value }).
   - orientation 'vertical'   → barra vertical, redimensiona largura (col-resize).
   - orientation 'horizontal' → barra horizontal, redimensiona altura (row-resize).
   O pai aplica clamp via `setWidth`/`setHeight`; aqui apenas reportamos o delta
   incremental desde o último frame (sempre sobre um valor atual fresco).
   ========================================================================= */

import React, { useCallback, useEffect, useRef, useState } from 'react'

type Orientation = 'vertical' | 'horizontal'

export type SplitterProps = {
  orientation: Orientation
  /** delta incremental (px) — o pai soma ao valor atual e aplica clamp. */
  onResize: (dx: number) => void
  label: string
  min: number
  max: number
  value: number
}

export function Splitter({ orientation, onResize, label, min, max, value }: SplitterProps) {
  const vertical = orientation === 'vertical'
  const [active, setActive] = useState(false)
  const el = useRef<HTMLDivElement>(null)
  const drag = useRef<{ id: number; last: number } | null>(null)
  const pending = useRef(0)
  const raf = useRef<number | null>(null)
  const onResizeRef = useRef(onResize)
  onResizeRef.current = onResize

  // Aplica o delta acumulado uma vez por frame (RAF) e limpa o buffer.
  const flush = useCallback(() => {
    raf.current = null
    const dx = pending.current
    pending.current = 0
    if (dx !== 0) onResizeRef.current(dx)
  }, [])

  const schedule = useCallback(() => {
    if (raf.current === null) raf.current = requestAnimationFrame(flush)
  }, [flush])

  // Encerra o drag: descarta o buffer, cancela o RAF, remove estados do body e
  // solta o pointer capture. Idempotente — seguro chamar mais de uma vez.
  const stop = useCallback(() => {
    const d = drag.current
    drag.current = null
    pending.current = 0
    if (raf.current !== null) {
      cancelAnimationFrame(raf.current)
      raf.current = null
    }
    document.body.classList.remove(
      'splitter-dragging',
      vertical ? 'splitter-dragging-col' : 'splitter-dragging-row',
    )
    if (d && el.current) {
      try {
        if (el.current.hasPointerCapture(d.id)) el.current.releasePointerCapture(d.id)
      } catch {
        /* noop — pointer já solto */
      }
    }
  }, [vertical])

  // Se desmontar no meio de um drag, limpa resíduos (sem vazamento).
  useEffect(() => stop, [stop])

  const onPointerDown = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      if (e.button !== 0) return
      e.preventDefault()
      try {
        el.current?.setPointerCapture(e.pointerId)
      } catch {
        /* noop */
      }
      drag.current = { id: e.pointerId, last: vertical ? e.clientX : e.clientY }
      pending.current = 0
      setActive(true)
      document.body.classList.add(
        'splitter-dragging',
        vertical ? 'splitter-dragging-col' : 'splitter-dragging-row',
      )
    },
    [vertical],
  )

  const onPointerMove = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      const d = drag.current
      if (!d) return
      const cur = vertical ? e.clientX : e.clientY
      pending.current += cur - d.last
      d.last = cur
      schedule()
    },
    [vertical, schedule],
  )

  const onPointerUp = useCallback(() => {
    // aplica o delta final pendente ANTES de encerrar (não perde o último frame)
    flush()
    stop()
    setActive(false)
  }, [flush, stop])

  const onPointerCancel = useCallback(() => {
    stop()
    setActive(false)
  }, [stop])

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLDivElement>) => {
      const step = e.shiftKey ? 40 : 8
      let dx = 0
      if (vertical) {
        if (e.key === 'ArrowLeft') dx = -step
        else if (e.key === 'ArrowRight') dx = step
      } else {
        if (e.key === 'ArrowUp') dx = -step
        else if (e.key === 'ArrowDown') dx = step
      }
      if (dx !== 0) {
        e.preventDefault()
        onResizeRef.current(dx)
        return
      }
      if (e.key === 'Home') {
        e.preventDefault()
        onResizeRef.current(min - value)
      } else if (e.key === 'End') {
        e.preventDefault()
        onResizeRef.current(max - value)
      }
      // Enter → noop (o splitter não "aciona" nada)
    },
    [vertical, min, max, value],
  )

  const ariaValue = Math.round(value)

  return (
    <div
      ref={el}
      className={`splitter splitter-${orientation}${active ? ' active' : ''}`}
      data-dragging={active ? 'true' : undefined}
      role="separator"
      aria-orientation={orientation}
      aria-label={label}
      aria-valuenow={ariaValue}
      aria-valuemin={min}
      aria-valuemax={max}
      tabIndex={0}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerCancel}
      onKeyDown={onKeyDown}
    />
  )
}
