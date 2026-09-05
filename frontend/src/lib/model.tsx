// =========================================================================
// COSCA DESKTOP — MODELO + HELPERS PUROS
// -------------------------------------------------------------------------
// Módulo extraído do App.tsx (monólito → separação de responsabilidade).
// Contém APENAS tipos, constantes e funções PURAS (nenhum hook React) — logo,
// NÃO interfere na ordem de hooks do App (#310). Importável e testável isolado.
// Regra (governança): a UI não inventa campo o que o backend não devolveu;
// helpers só formatam/presentam, nunca simulam. Cores vêm de tokens; a FORMA
// (glyph) carrega significado, cor nunca é a única pista.
// =========================================================================
import type { ReactNode } from 'react'
import { main } from '../../wailsjs/go/models'

export type Project = main.Project
export type TreeEntry = main.TreeEntry
export type Approval = main.Approval
export type RuntimeEvent = main.RuntimeEvent
export type View = 'home' | 'create' | 'workspace' | 'forge' | 'settings' | 'chat'
export type RailId =
  | 'home' | 'project' | 'chat' | 'agent' | 'files' | 'terminal' | 'search' | 'diff'
  | 'intelligence'
  | 'forge'
  | 'knowledge' | 'memory' | 'provenance' | 'audit'
  | 'sandbox' | 'runtime'
export type RiskTone = 'ok' | 'info' | 'warn' | 'danger'
export type Mode = 'forge' | 'standalone'
export type EventKind =
  | 'thinking' | 'response' | 'tool-call' | 'tool-result'
  | 'file-change' | 'finish-step' | 'finish' | 'done'
  | 'writing' | 'executing' | 'completed'
  | 'error' | 'status'
export type AgentState =
  | 'thinking' | 'planning' | 'waiting-approval' | 'executing'
  | 'tool-call' | 'reading' | 'writing' | 'diff'
  | 'completed' | 'failed' | 'cancelled'

export type ActivityItem = {
  at: string
  title: string
  body?: string
  kind?: EventKind | 'local'
  state?: AgentState
  seq?: number
}

export type DiffItem = {
  status: string
  path: string
  additions?: number
  deletions?: number
  patch?: string
  old_path?: string
}
export type DiffData = {
  git?: boolean
  branch?: string
  reason?: string
  items?: DiffItem[]
  summary?: { files?: number; insertions?: number; deletions?: number }
  honesto?: boolean
}
export type DiffTone = 'ok' | 'info' | 'warn' | 'danger' | 'neutral'

export type TermEntry = { cmd: string; out: string; tone: RiskTone }

export type IntelTech = {
  name?: string
  category?: string
  confidence?: string
  evidence?: { kind?: string; value?: string; file?: string }[]
  notes?: string
}
export type IntelCommand = { name?: string; cmd?: string; source?: string }
export type AgentPipelineStep = { step?: string; tool?: string; status?: string }

export type UIColorField = {
  id: string
  role: string
  label: string
  bounds: { x: number; y: number; width: number; height: number }
  visible: boolean
  disabled: boolean
  state: string
  tokens: Record<string, string>
  text: string
  overflow: boolean
  contrast_ratio: number
  children?: UIColorField[]
}
export type UISceneModel = {
  viewport: { x: number; y: number; width: number; height: number }
  root?: UIColorField
  elements: UIColorField[]
  generated_at: string
}
export type UiAuditViolation = {
  severity: string
  rule: string
  element_id: string
  reason: string
  evidence: string
}
export type UiAudit = {
  ok: boolean
  summary: string
  violations: UiAuditViolation[]
  detail?: string
  error?: string
}

export const asArr = (x: any): any[] => (Array.isArray(x) ? x : [])
export const asTech = (x: any): IntelTech | null =>
  x && typeof x === 'object' && (x.name || x.category) ? x : null
export const confGlyph = (c: string): { glyph: string; tone: 'ok' | 'warn' | 'neutral' } => {
  switch ((c || '').toLowerCase()) {
    case 'high': return { glyph: '✓', tone: 'ok' }
    case 'medium': return { glyph: '•', tone: 'warn' }
    case 'low': return { glyph: '!', tone: 'neutral' }
    default: return { glyph: '·', tone: 'neutral' }
  }
}
export const evGlyph = (kind: string): string => {
  switch ((kind || '').toLowerCase()) {
    case 'file': return '▸'
    case 'dependency': return '◆'
    case 'content': return '⌕'
    case 'count': return '#'
    case 'command': return '❯'
    default: return '·'
  }
}
export const intelHas = (profile: any, listKey: string, needle: string): boolean =>
  asArr(profile?.[listKey]).some((t: any) => t && t.name && String(t.name).toLowerCase().includes(needle))

export const DEFAULT_LOC = 'C:/Users/Henrique/Documents/projects'
export const AGENT_MODEL = 'deepseek'

export const RUNTIME_EVENTS: [string, EventKind][] = [
  ['cosca:agent:thinking', 'thinking'],
  ['cosca:agent:response', 'response'],
  ['cosca:agent:tool-call', 'tool-call'],
  ['cosca:agent:writing', 'writing'],
  ['cosca:agent:executing', 'executing'],
  ['cosca:agent:completed', 'completed'],
  ['cosca:agent:done', 'done'],
  ['cosca:agent:error', 'error'],
  ['cosca:status', 'status'],
]

export const EVENT_LABEL: Record<EventKind, string> = {
  thinking: 'pensando',
  response: 'respondendo',
  'tool-call': 'ferramenta',
  'tool-result': 'ferramenta',
  'file-change': 'escrevendo arquivo',
  'finish-step': 'passo concluído',
  finish: 'concluído',
  done: 'concluído',
  writing: 'escrevendo',
  executing: 'executando',
  completed: 'concluído',
  error: 'erro',
  status: 'status do runtime',
}

export const AGENT_STATES: Record<AgentState, { label: string; glyph: string; available: boolean; from?: string }> = {
  'thinking':         { label: 'Pensando',             glyph: '◌', available: true,  from: 'cosca:agent:thinking' },
  'planning':         { label: 'Planejando',           glyph: '…', available: false },
  'waiting-approval': { label: 'Aguardando aprovação', glyph: '⏸', available: false },
  'executing':        { label: 'Executando',           glyph: '▶', available: true,  from: 'cosca:agent:executing' },
  'tool-call':        { label: 'Ferramenta',           glyph: '⌘', available: true,  from: 'cosca:agent:tool-call' },
  'reading':          { label: 'Lendo',                glyph: '▤', available: false },
  'writing':          { label: 'Escrevendo',           glyph: '▥', available: true,  from: 'cosca:agent:writing' },
  'diff':             { label: 'Diff',                 glyph: '±', available: false },
  'completed':        { label: 'Concluído',            glyph: '✓', available: true,  from: 'cosca:agent:completed' },
  'failed':           { label: 'Falhou',               glyph: '✕', available: true,  from: 'cosca:agent:error' },
  'cancelled':        { label: 'Cancelado',            glyph: '⊘', available: false },
}

export const stateFromEvent = (t: string): AgentState | undefined => {
  switch (t) {
    case 'thinking': return 'thinking'
    case 'tool-call':
    case 'tool-result': return 'tool-call'
    case 'file-change': return 'writing'
    case 'finish-step': return 'executing'
    case 'done':
    case 'finish': return 'completed'
    case 'error': return 'failed'
    default: return undefined
  }
}

export const eventToActivity = (e: RuntimeEvent): ActivityItem => ({
  at: e.at || 'agora',
  title: EVENT_LABEL[e.type as EventKind] || e.type || 'evento do runtime',
  body: e.content,
  kind: (e.type as EventKind) || 'status',
  state: stateFromEvent(e.type),
  seq: e.seq,
})

export const toRuntimeEvent = (x: any): RuntimeEvent => {
  if (!x) return main.RuntimeEvent.createFrom({ type: 'status', content: '', seq: undefined })
  if (typeof x === 'string') {
    try { return main.RuntimeEvent.createFrom(JSON.parse(x)) }
    catch { return main.RuntimeEvent.createFrom({ type: 'status', content: x, seq: undefined }) }
  }
  return main.RuntimeEvent.createFrom(x)
}

export const riskTone = (r: string): RiskTone => {
  const x = (r || '').toLowerCase()
  if (x.includes('exec')) return 'danger'
  if (x.includes('write') || x.includes('network')) return 'warn'
  if (x.includes('read')) return 'info'
  return 'warn'
}

export const cmdTone = (out: string): RiskTone => {
  const x = out || ''
  if (/recus|negad|aprova|approv|perigoso|denied|deneg|execpolicy|fail-closed|sens[íi]vel|requires approv|not authorized|uac|politic/i.test(x)) return 'warn'
  if (/erro|error|not found|notfound|failed|falhou|exit code|permiss|no such|unrecognized|invalid|exception/i.test(x)) return 'danger'
  return 'ok'
}
export const toneColor = (t: RiskTone): string =>
  t === 'danger' ? 'var(--danger)' : t === 'warn' ? 'var(--warn)' : t === 'info' ? 'var(--info)' : 'var(--code-fg)'

export const planVal = (pd: Record<string, any> | null, k: string, fallback: string) => {
  const v = pd && pd[k]
  if (v == null || v === '') return fallback
  return String(v)
}
export const planCount = (pd: Record<string, any> | null, k: string): string => {
  const v = pd && pd[k]
  if (Array.isArray(v)) return v.length ? `${v.length} item${v.length > 1 ? 's' : ''}` : '—'
  if (v == null || v === '') return '—'
  return String(v)
}

export const diffGlyph = (s: string): string => {
  switch (s) {
    case 'A': return '+'
    case 'M': return '±'
    case 'D': return '−'
    case 'R': return '→'
    case '??': return '?'
    default: return '·'
  }
}
export const diffTone = (s: string): DiffTone => {
  switch (s) {
    case 'A': return 'ok'
    case 'M': return 'warn'
    case 'D': return 'danger'
    case 'R':
    case '??': return 'info'
    default: return 'neutral'
  }
}
export const diffName = (s: string): string => {
  switch (s) {
    case 'A': return 'Adicionado'
    case 'M': return 'Modificado'
    case 'D': return 'Removido'
    case 'R': return 'Renomeado'
    case '??': return 'Nao rastreado'
    default: return 'Alterado'
  }
}
export const toDiff = (x: any): DiffData => {
  if (!x || typeof x !== 'object') {
    return { git: false, reason: 'resposta do backend inválida', items: [], honesto: true }
  }
  const items = Array.isArray(x.items) ? x.items.map((i: any): DiffItem => ({
    status: String(i?.status ?? ''),
    path: String(i?.path ?? ''),
    additions: typeof i?.additions === 'number' ? i.additions : undefined,
    deletions: typeof i?.deletions === 'number' ? i.deletions : undefined,
    patch: typeof i?.patch === 'string' ? i.patch : undefined,
    old_path: typeof i?.old_path === 'string' ? i.old_path : undefined,
  })) : []
  const s = x.summary && typeof x.summary === 'object' ? x.summary : {}
  return {
    git: !!x.git,
    branch: typeof x.branch === 'string' ? x.branch : undefined,
    reason: typeof x.reason === 'string' ? x.reason : undefined,
    items,
    summary: {
      files: typeof s.files === 'number' ? s.files : undefined,
      insertions: typeof s.insertions === 'number' ? s.insertions : undefined,
      deletions: typeof s.deletions === 'number' ? s.deletions : undefined,
    },
    honesto: !!x.honesto,
  }
}
export const renderPatch = (patch: string): ReactNode => {
  const lines = patch.split('\n')
  return (
    <div>
      {lines.map((ln, i) => {
        let cls = 'diff-ctx'
        if (ln.startsWith('+++') || ln.startsWith('---')) cls = 'diff-meta'
        else if (ln.startsWith('@@')) cls = 'diff-hunk'
        else if (ln.startsWith('+')) cls = 'diff-add'
        else if (ln.startsWith('-')) cls = 'diff-del'
        else if (ln.startsWith('\\')) cls = 'diff-meta'
        return (
          <span key={i} className={`diff-line ${cls}`}>{ln || '\u00a0'}</span>
        )
      })}
    </div>
  )
}

export const sevGlyph = (s: string): string => {
  switch ((s || '').toUpperCase()) {
    case 'HIGH': return '!'
    case 'MEDIUM': return '·'
    case 'LOW': return '—'
    default: return '·'
  }
}
export const sevTone = (s: string): RiskTone => {
  switch ((s || '').toUpperCase()) {
    case 'HIGH': return 'danger'
    case 'MEDIUM': return 'warn'
    case 'LOW': return 'info'
    default: return 'info'
  }
}
