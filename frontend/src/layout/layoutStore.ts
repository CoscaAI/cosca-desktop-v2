/* =========================================================================
   COSCA DESKTOP — LAYOUT STORE (React hook `useLayout`)
   Centraliza o estado de layout do workspace. Fonte da verdade: `LayoutState`
   (layoutModel.ts). Persistência via layoutPersistence.ts (localStorage do
   Desktop, §14). O store NÃO toca em backend/filesystem — é 100% UI.

   O `storageKey` opcional particiona o layout por workspace/projeto:
   `useLayout(project?.root || 'global')`. Quando não há projeto, usa 'global' —
   quando há, isolado por raiz. Sempre em localStorage, NUNCA no .cosca.
   ========================================================================= */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  LayoutState,
  PaneId,
  WidthPaneId,
  DEFAULT_LAYOUT,
  clampWidth,
  clampHeight,
  cloneLayout,
} from './layoutModel'
import { loadLayout, saveLayout } from './layoutPersistence'

export function useLayout(storageKey?: string) {
  const [layout, setLayout] = useState<LayoutState>(
    () => loadLayout(storageKey) ?? cloneLayout(DEFAULT_LAYOUT),
  )
  const keyRef = useRef(storageKey)
  const layoutRef = useRef(layout)
  layoutRef.current = layout

  // Mudou o projeto/workspace (storageKey) → recarrega o layout daquele escopo.
  useEffect(() => {
    if (keyRef.current !== storageKey) {
      keyRef.current = storageKey
      const next = loadLayout(storageKey) ?? cloneLayout(DEFAULT_LAYOUT)
      layoutRef.current = next
      setLayout(next)
    }
  }, [storageKey])

  // Aplica um novo layout e persiste (os setters clampam ANTES de aplicar).
  const apply = useCallback((next: LayoutState) => {
    layoutRef.current = next
    setLayout(next)
    saveLayout(next, storageKey)
  }, [storageKey])

  const setWidth = useCallback((id: WidthPaneId, px: number) => {
    const cur = layoutRef.current
    apply({ ...cur, widths: { ...cur.widths, [id]: clampWidth(id, px) } })
  }, [apply])

  const setHeight = useCallback((px: number) => {
    const cur = layoutRef.current
    apply({ ...cur, height: clampHeight(px) })
  }, [apply])

  const toggleCollapse = useCallback((id: PaneId) => {
    const cur = layoutRef.current
    apply({ ...cur, collapsed: { ...cur.collapsed, [id]: !cur.collapsed[id] } })
  }, [apply])

  const reset = useCallback(() => apply(cloneLayout(DEFAULT_LAYOUT)), [apply])

  return useMemo(
    () => ({ layout, setWidth, setHeight, toggleCollapse, reset }),
    [layout, setWidth, setHeight, toggleCollapse, reset],
  )
}
