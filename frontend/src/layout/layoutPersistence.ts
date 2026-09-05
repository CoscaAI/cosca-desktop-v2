/* =========================================================================
   COSCA DESKTOP — LAYOUT PERSISTENCE (localStorage do Desktop)
   §14 (contrato): o layout é uma preferência do DESKTOP. NUNCA é escrito em
   `.cosca/`, memory, knowledge, learning, family, kernel, blockchain ou audit
   (o Desktop não detém essas áreas). Persiste APENAS em `localStorage` do
   runtime do Desktop, opcionalmente particionado por projeto/workspace via
   `storageKey` (ex.: `cosca-desktop:layout:<root>`).

   Abstração separada do store para permitir migrar para "COSCA Desktop
   settings" no futuro sem tocar em quem consome (layoutStore/App).
   ========================================================================= */

import { LayoutState, normalizeLayout } from './layoutModel'

export const LAYOUT_KEY = 'cosca-desktop:layout'

const keyFor = (storageKey?: string): string =>
  storageKey ? `${LAYOUT_KEY}:${storageKey}` : LAYOUT_KEY

/** Returns null quando ausente OU corrompido (JSON inválido). */
export function loadLayout(storageKey?: string): LayoutState | null {
  const key = keyFor(storageKey)
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return null
    return normalizeLayout(JSON.parse(raw))
  } catch {
    return null
  }
}

export function saveLayout(s: LayoutState, storageKey?: string): void {
  const key = keyFor(storageKey)
  try {
    localStorage.setItem(key, JSON.stringify(s))
  } catch {
    /* localStorage indisponível (modo privado etc.) — layout é best-effort */
  }
}

export function clearLayout(storageKey?: string): void {
  const key = keyFor(storageKey)
  try {
    localStorage.removeItem(key)
  } catch {
    /* noop */
  }
}
