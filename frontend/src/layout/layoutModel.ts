/* =========================================================================
   COSCA DESKTOP — LAYOUT MODEL (pure, No-React)
   Define os tokens/constantes do layout do workspace (o tipo `wombo` de
   panes que o Desktop renderiza): árvore de arquivos, workbench do agente,
   painel de contexto, bottom strip (OUTPUT) e as colunas DIFF/TERMINAL.

   §14 (contrato): o layout pertence ao DESKTOP. NUNCA escreve em `.cosca/`,
   memory, knowledge, learning, family, kernel, blockchain ou audit. Vive em
   `localStorage` do Desktop (ver layoutPersistence.ts). Nada daqui importa
   React — é 100% puro para ser testável e migrável.
   ========================================================================= */

// FASE C (Diff): pane de diff do working tree — colapsável e redimensionável,
// como os demais. Abre via rail/header; `ProjectDiff` é a única fonte de dado.
// FASE D (Terminal): pane de terminal autorizado — colapsável/redimensionável
// como uma coluna à direita; `RunCommand`/`RunCommandAuthorized` são a fonte.
export type PaneId = 'tree' | 'agent' | 'context' | 'output' | 'terminal' | 'diff' | 'intelligence'

// As colunas redimensionáveis horizontalmente (largura). O editor é
// flex:1 e consome o espaço restante (§9); por isso NÃO está neste record.
export type WidthPaneId = 'tree' | 'agent' | 'context' | 'terminal' | 'diff' | 'intelligence'

export type LayoutState = {
  widths: Record<WidthPaneId, number>
  /** altura do bottom strip (output/terminal), em px */
  height: number
  collapsed: Record<PaneId, boolean>
}

export const DEFAULT_LAYOUT: LayoutState = {
  widths: { tree: 232, agent: 360, context: 250, terminal: 400, diff: 340, intelligence: 340 },
  height: 36,
  // terminal/intelligence começam RECOLHIDOS (não ocupam o workspace até abrir).
  collapsed: { tree: false, agent: false, context: false, output: false, terminal: true, diff: true, intelligence: true },
}

// Limites rígidos por pane (min/max em px). O editor (flex:1) absorve o resto.
export const BOUNDS: Record<PaneId, { min: number; max: number }> = {
  tree:    { min: 160, max: 420 },
  agent:   { min: 280, max: 600 },
  context: { min: 180, max: 420 },
  output:  { min: 36,  max: 400 },
  terminal:{ min: 260, max: 720 },
  diff:    { min: 300, max: 760 },
  intelligence: { min: 280, max: 600 },
}

const clampNum = (v: number, min: number, max: number, fallback: number): number => {
  if (typeof v !== 'number' || Number.isNaN(v)) return fallback
  return Math.min(Math.max(v, min), max)
}

/** Clampa uma largura de coluna aos limites do painel. */
export function clampWidth(id: WidthPaneId, px: number): number {
  const b = BOUNDS[id]
  return clampNum(px, b.min, b.max, b.min)
}

/** Clampa a altura do bottom strip aos limites do OUTPUT. */
export function clampHeight(px: number): number {
  const b = BOUNDS.output
  return clampNum(px, b.min, b.max, b.min)
}

/** Retorna uma cópia profunda (nested widths/collapsed) do layout. */
export function cloneLayout(s: LayoutState): LayoutState {
  return { widths: { ...s.widths }, height: s.height, collapsed: { ...s.collapsed } }
}

/**
 * Sanitiza um valor vindo de storage/fonte não confiável.
 * - Não-objeto → DEFAULT_LAYOUT.
 * - Campo ausente / inválido / NaN → valor default AQUELA campo.
 * - Fora dos bounds → clampa a min/max.
 * Garante que todos os campos existem. Usada na recuperação de layout corrompido.
 */
export function normalizeLayout(raw: any): LayoutState {
  const base = cloneLayout(DEFAULT_LAYOUT)
  if (!raw || typeof raw !== 'object') return base

  const rw = raw.widths && typeof raw.widths === 'object' ? raw.widths : {}
  const rc = raw.collapsed && typeof raw.collapsed === 'object' ? raw.collapsed : {}

  const width = (id: WidthPaneId, fallback: number) =>
    clampWidth(id, typeof rw[id] === 'number' ? rw[id] : fallback)

  return {
    widths: {
      tree:    width('tree',    base.widths.tree),
      agent:   width('agent',   base.widths.agent),
      context: width('context', base.widths.context),
      terminal: width('terminal', base.widths.terminal),
      diff:    width('diff',    base.widths.diff),
      intelligence: width('intelligence', base.widths.intelligence),
    },
    height: clampHeight(typeof raw.height === 'number' ? raw.height : base.height),
    collapsed: {
      tree:    !!rc.tree,
      agent:   !!rc.agent,
      context: !!rc.context,
      output:  !!rc.output,
      // terminal começa RECOLHIDO por padrão (layouts antigos sem a chave
      // hidratam como recolhido — não ocupa o workspace até ser aberto).
      terminal: rc.terminal == null ? base.collapsed.terminal : !!rc.terminal,
      // diff começa RECOLHIDO por padrão (não ocupa o workspace em layouts
      // persistidos sem essa chave); só abre quando o usuário pede.
      diff:    rc.diff == null ? base.collapsed.diff : !!rc.diff,
      // intelligence começa RECOLHIDO por padrão — só abre quando o usuário pede.
      intelligence: rc.intelligence == null ? base.collapsed.intelligence : !!rc.intelligence,
    },
  }
}
