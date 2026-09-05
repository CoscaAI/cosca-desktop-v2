// =========================================================================
// COSCA DESKTOP — UI PERCEPTION COLLECTOR (sem GPU). CAMADA 1-2.
// -------------------------------------------------------------------------
// Extraído do App.tsx. Read-only: apenas LÊ o DOM real (getBoundingClientRect +
// computed style) e monta o UISceneModel que o backend (CaptureUI) analisa pela
// UI Perception Engine. NUNCA altera o DOM, nunca inventa campo: o que não é
// observável fica vazio/-1 (honesto). Funções puras (sem hooks) — seguro de
// extrair do componente sem afetar a ordem de hooks (#310).
// =========================================================================
import type { UIColorField, UISceneModel } from './model'

export type ZRgb = { r: number; g: number; b: number; a: number }

export const zText = (el: Element): string =>
  (el.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 120)

export const zLabel = (el: Element): string => {
  const aria = (el.getAttribute('aria-label') || '').trim()
  if (aria) return aria.slice(0, 80)
  const title = (el.getAttribute('title') || '').trim()
  if (title) return title.slice(0, 80)
  return el.textContent ? el.textContent.replace(/\s+/g, ' ').trim().slice(0, 60) : ''
}

export const zState = (el: Element): string => {
  const cls = el.className
  if (typeof cls !== 'string') return ''
  const parts = cls.split(/\s+/).filter(Boolean)
  for (const s of ['active', 'open', 'collapsed', 'disabled', 'selected', 'error', 'done', 'empty']) {
    if (parts.includes(s)) return s
  }
  return ''
}

export const zParseColor = (c: string): ZRgb | null => {
  const m = c.match(/rgba?\(\s*([\d.]+)[,\s]+([\d.]+)[,\s]+([\d.]+)(?:[,\s/]+([\d.]+))?\s*\)/)
  if (!m) return null
  return { r: +m[1], g: +m[2], b: +m[3], a: m[4] == null ? 1 : +m[4] }
}
export const zRelLum = ({ r, g, b }: ZRgb): number => {
  const f = (v: number) => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4) }
  return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b)
}

export const zContrast = (fg: string, bg: string, bgEl: Element): number => {
  const c1 = zParseColor(fg)
  let c2 = zParseColor(bg)
  if (c2 && c2.a < 0.85) {
    c2 = null
    let n: Element | null = bgEl.parentElement
    while (n && n !== document.documentElement) {
      const b = zParseColor(getComputedStyle(n).backgroundColor)
      if (b && b.a >= 0.85) { c2 = b; break }
      n = n.parentElement
    }
  }
  if (!c1 || !c2) return -1
  const l1 = zRelLum(c1); const l2 = zRelLum(c2)
  const hi = Math.max(l1, l2); const lo = Math.min(l1, l2)
  return Math.round(((hi + 0.05) / (lo + 0.05)) * 100) / 100
}

export const zRole = (el: Element): string => {
  if (el.classList.contains('rail-btn')) return 'navitem'
  if (el.classList.contains('splitter')) return 'resizer'
  if (el.classList.contains('panel-head')) return 'heading'
  const tag = el.tagName.toLowerCase()
  if (tag === 'button') return el.getAttribute('role') === 'option' ? 'tab' : 'button'
  if (tag === 'select') return 'select'
  if (tag === 'textarea') return 'editor'
  if (tag === 'input') {
    const t = (el.getAttribute('type') || '').toLowerCase()
    if (t === 'checkbox') return 'checkbox'
    return 'input'
  }
  if (tag === 'svg') return 'icon'
  return 'text'
}

export const zBuild = (el: Element, role: string, id: string): UIColorField => {
  const cs = getComputedStyle(el)
  const r = el.getBoundingClientRect()
  const visible = cs.display !== 'none' && cs.visibility !== 'hidden'
  const disabled = el.getAttribute('aria-disabled') === 'true' || (el as HTMLInputElement).disabled === true
  const text = zText(el)
  const label = zLabel(el)
  const overflow = el.scrollWidth > el.clientWidth + 1 || el.scrollHeight > el.clientHeight + 1
  const tokens: Record<string, string> = {
    'border-radius': cs.borderRadius,
    'font-size': cs.fontSize,
    color: cs.color,
    background: cs.backgroundColor,
    padding: cs.padding,
  }
  return {
    id,
    role,
    label,
    bounds: { x: r.x, y: r.y, width: r.width, height: r.height },
    visible,
    disabled,
    state: zState(el),
    tokens,
    text,
    overflow,
    contrast_ratio: zContrast(cs.color, cs.backgroundColor, el),
  }
}

export const collectUIScene = (): UISceneModel => {
  const shell = document.querySelector<HTMLElement>('.shell')
    || document.getElementById('root')
    || document.body
  const MAX = 100
  let n = 0
  const seen = new Set<Element>()
  const slug = (s: string) => s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 24)
  const buildId = (el: Element, role: string): string => {
    const name = zLabel(el) || zText(el) || el.tagName.toLowerCase()
    return `${role}:${slug(name) || 'el'}-${++n}`
  }

  const root = zBuild(shell, 'workspace', 'workspace:shell-1')

  const CONTAINERS = ['.topbar', '.rail', '.statusbar', '.tree', '.agent-col',
    '.editor-col', '.ctx', '.diff-col', '.intel-col', '.term-col']
  const containerNodes: UIColorField[] = []
  for (const sel of CONTAINERS) {
    const el = document.querySelector<HTMLElement>(sel)
    if (!el || seen.has(el) || n >= MAX) continue
    seen.add(el)
    const role = sel === '.rail' || sel === '.topbar' || sel === '.statusbar' ? 'nav' : 'panel'
    containerNodes.push(zBuild(el, role, buildId(el, role)))
  }
  if (containerNodes.length) root.children = containerNodes

  const LEAF_SEL = ['.rail-btn', '.panel-head', 'button', 'input', 'textarea', 'select', '.splitter']
  const leaves = new Set<Element>()
  for (const sel of LEAF_SEL) {
    document.querySelectorAll<HTMLElement>(sel).forEach(el => {
      if (el.closest('.dialog-back')) return
      if (el.classList.contains('rail') || el.classList.contains('topbar') || el.classList.contains('statusbar')) return
      if (seen.has(el)) return
      leaves.add(el)
    })
  }
  const elements: UIColorField[] = []
  for (const el of leaves) {
    if (n >= MAX) break
    if (seen.has(el)) continue
    seen.add(el)
    const role = zRole(el)
    elements.push(zBuild(el, role, buildId(el, role)))
  }

  return {
    viewport: { x: 0, y: 0, width: window.innerWidth, height: window.innerHeight },
    root,
    elements,
    generated_at: new Date().toISOString(),
  }
}
