import React, { useState, useEffect, useRef, useMemo } from 'react'
import {
  DiscoverProjects, CreateProject, OpenProject, InitProject, CloseProject,
  HasProject, ProjectInfo, TreeDirs, ReadDir, ReadFile, WriteFile, ValidateLocation,
  AgentApprove, AgentPlan, AgentRunStream, PlanDetails,
  RequestApproval, PendingApproval, ApprovePending, DenyPending,
  CapabilityState, RootMode, RootInfo, ConsumeEvents, SubscribeRuntime,
  ProjectDiff, DiffSnapshot,
  // PROJECT INTELLIGENCE (pane observável por evidência): binding regenerado.
  AnalyzeProject,
  // PROJETO ATIVO: RefreshProject invalida o cache do projeto e re-analisa
  // (mesmo shape de AnalyzeProject: {has_project, profile} | {has_project, error}).
  RefreshProject,
  // FASE 3 (Agente já sabe o contexto — sem perguntar): binding regenerado.
  AgentProjectContext,
  // FASE D (Terminal autorizado): bindings regenerados no App.d.ts.
  RunCommand, RunCommandAuthorized,
  // UI PERCEPTION (sem GPU): binding regenerado — CaptureUI(sceneJSON) analisa a
  // cena visual (bounds/roles/estados/tokens) pela UI Perception Engine.
  CaptureUI,
} from '../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'
import { main } from '../wailsjs/go/models'
// LAYOUT ENGINE (§13/§16/§19) — 100% UI, sem backend; vive em localStorage do Desktop.
import { useLayout } from './layout/layoutStore'
import { Splitter } from './layout/Splitter'
import { BOUNDS, type PaneId } from './layout/layoutModel'
// ICON PRIMITIVE — fonte única de ícones de UI (substitui símbolos Unicode
// improvisados por componentes Lucide stroke-based, minimalistas e consistentes).
import { Icon, type IconName } from './components/Icon'
// FILE EXPLORER + CODE VIEWER (sistema coerente): ícone semântico resolvido por
// is_dir/language/nome (FileIcon) e viewer read-only com highlight.js (CodeViewer).
import { FileIcon } from './components/FileIcon'
import { CodeViewer, type CodeViewerProps } from './components/CodeViewer'
// FORM FIELD primitive — wrapper a11y (label↔control ↔ helper/erro). Não é
// sistema paralelo: reusa `.input` e tokens; associa label por htmlFor↔id.
import { FormField } from './components/FormField'
// CENTRAL DE CONFIGURAÇÕES (§2/§4 do contrato): USER PREFERENCE + seções reais.
import Settings from './settings/Settings'
import { loadPrefs, savePrefs } from './settings/prefs'
// FASE 3 — CHAT (a tela principal de conversa). Consome o provider (daemon
// /v1/run/stream) via ChatStream + cosca:chat:event. Nenhuma lógica cognitiva.
import ChatView from './chat/ChatView'
// HELPER + MODELO PUROS (extraídos do monólito): sem hooks, sem risco de ordem
// de hooks (#310). `./lib/model` = tipos + constantes + formatação/tom;
// `./lib/uiscene` = coletor de cena (DOM read-only) da UI Perception Engine.
import { collectUIScene } from './lib/uiscene'
import {
  AGENT_MODEL, AGENT_STATES, DEFAULT_LOC, RUNTIME_EVENTS,
  asArr, asTech, cmdTone, confGlyph, diffGlyph, diffName, diffTone,
  eventToActivity, evGlyph, intelHas, planCount, planVal, renderPatch,
  riskTone, sevGlyph, sevTone, toDiff, toRuntimeEvent, toneColor,
} from './lib/model'
import type {
  AgentState, Approval, ActivityItem, DiffData, DiffItem, Mode, Project,
  RailId, RiskTone, RuntimeEvent, TermEntry, UiAudit, UiAuditViolation,
  View, AgentPipelineStep, IntelCommand, IntelTech,
} from './lib/model'


export default function App() {
  // Tema inicial = USER PREFERENCE persistida (localStorage `cosca-desktop:prefs`).
  // Padrão HONESTO: dark (estado atual do shell). Senão, aplica o que foi salvo.
  const [dark, setDark] = useState<boolean>(() => {
    const p = loadPrefs()
    return p ? p.theme === 'dark' : true
  })
  const [view, setView] = useState<View>('home')
  // Área ativa do workspace (Agent/Files/Terminal) — o rail destaca a correta.
  // Resolve: clicar em "Files" NÃO fica preso em "Agent" (era hardcoded).
  const [wsArea, setWsArea] = useState<'agent' | 'files' | 'terminal' | 'search'>('agent')
  const [loc, setLoc] = useState(DEFAULT_LOC)
  const [projects, setProjects] = useState<Project[]>([])
  const [name, setName] = useState('')
  const [project, setProject] = useState<Project | null>(null)
  const [busy, setBusy] = useState(false)
  const [steps, setSteps] = useState<{ msg: string; done: boolean }[]>([])
  const [err, setErr] = useState('')
  // FILE EXPLORER (árvore LAZY + Code Viewer).
  // Só a raiz é carregada no mount; cada pasta é carregada sob demanda (ReadDir).
  // `treeRoot` = nós da raiz (main.FileNode), `expanded`/`loading`/`treeErr`
  // = estado por path (expandido / carregando / erro honesto). Separate:
  //   selectedPath  → o item destacado na árvore (estado visual)
  //   activeFile    → o arquivo aberto no Code Viewer (fonte de conteúdo)
  const [treeRoot, setTreeRoot] = useState<main.FileNode[]>([])
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [loading, setLoading] = useState<Record<string, boolean>>({})
  const [treeErr, setTreeErr] = useState<Record<string, string>>({})
  const [selectedPath, setSelectedPath] = useState<string | null>(null)
  const [activeFile, setActiveFile] = useState<main.FileNode | null>(null)
  const [fileContent, setFileContent] = useState('')
  const [fileView, setFileView] = useState<string | null>(null)
  const [fileErr, setFileErr] = useState('')
  const [activity, setActivity] = useState<ActivityItem[]>([])
  const [agentReq, setAgentReq] = useState('')
  const [agentRunning, setAgentRunning] = useState(false)
  const [plan, setPlan] = useState('')
  // FASE B: detalhes reais do plano (PlanDetails) para o Approval Center.
  const [planDetails, setPlanDetails] = useState<Record<string, any> | null>(null)
  // Approval Center (opção B)
  const [approval, setApproval] = useState<Approval | null>(null)
  const [approving, setApproving] = useState(false)
  // Command Palette
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [pq, setPq] = useState('')
  const [pi, setPi] = useState(0)
  // Avisos transitórios (ex.: itens do rail sem tela real)
  const [notice, setNotice] = useState('')
  const noticeTimer = useRef<number | undefined>(undefined)
  // ADR-0007: modo real (forge/standalone) + capabilities reais do backend.
  const [capMode, setCapMode] = useState<Mode | null>(null)
  const [cap, setCap] = useState<{ kernel: boolean; family_memory: boolean; daemon: boolean; agents: number | string; skills: number | string }>({
    kernel: false, family_memory: false, daemon: false, agents: 0, skills: 0,
  })
  const [forgeInfo, setForgeInfo] = useState<Record<string, any> | null>(null)
  const [forgeLoading, setForgeLoading] = useState(false)
  const [forgeErr, setForgeErr] = useState('')
  // FASE C (Diff): working tree — fonte única é ProjectDiff()/DiffSnapshot().
  const [diffData, setDiffData] = useState<DiffData | null>(null)
  const [diffLoading, setDiffLoading] = useState(false)
  const [diffErr, setDiffErr] = useState('')
  const [diffSelected, setDiffSelected] = useState<number | null>(null)
  // FASE D (Terminal autorizado): histórico honesto de comandos + input.
  const [terminalHist, setTerminalHist] = useState<TermEntry[]>([])
  const [termInput, setTermInput] = useState('')
  const [termBusy, setTermBusy] = useState(false)
  const termScrolled = useRef<HTMLDivElement | null>(null)
  const termInputRef = useRef<HTMLInputElement | null>(null)
  // PROJECT INTELLIGENCE: perfil real do projeto (AnalyzeProject), carregado em
  // background ao abrir/trocar o projeto. `intelErr`/`intel`/`intelLoading`
  // refletem o estado HONESTO do backend — a UI nunca fabrica um perfil.
  const [intel, setIntel] = useState<Record<string, any> | null>(null)
  const [intelLoading, setIntelLoading] = useState(false)
  const [intelErr, setIntelErr] = useState('')
  // Invalidação de cache + re-análise do projeto ativo via RefreshProject().
  // `intelRefreshing` cobre a fase de re-analise (RefreshProject) antes de
  // loadIntel/loadAgentCtx ativarem seus próprios flags de loading.
  const [intelRefreshing, setIntelRefreshing] = useState(false)
  // FASE 3 (Agent Project Context): o resumo compacto que o agente JÁ sabe —
  // stack/skills/pipeline/git/container/ci via AgentProjectContext(). Fonte é o
  // backend; a UI só apresenta (nunca inventa). Carregado em background.
  const [agentCtx, setAgentCtx] = useState<Record<string, any> | null>(null)
  const [agentCtxLoading, setAgentCtxLoading] = useState(false)
  const [agentCtxErr, setAgentCtxErr] = useState('')
  // UI PERCEPTION (sem GPU): resultado HONESTO do CaptureUI(sceneJSON) — o resp
  // do Kernel sobre a interface. `uiAudit` é o que o backend devolveu (nunca
  // fabricado); `uiAuditBusy` cobre o ciclo coleta→análise.
  const [uiAudit, setUiAudit] = useState<UiAudit | null>(null)
  const [uiAuditBusy, setUiAuditBusy] = useState(false)
  const seenSeq = useRef<Set<string>>(new Set())

  // LAYOUT ENGINE (§13/§16/§19) — estado de layout, persistido em localStorage do
  // Desktop, particionado por projeto (project?.root) ou 'global' sem projeto.
  const { layout, setWidth, setHeight, toggleCollapse, reset } = useLayout(project?.root || 'global')
  // isCollapsed estabilizado (useCallback) — era função inline recriada a cada
  // render, usada como dep de useCallback/useEffect → re-disparo → loop #310.
  const isCollapsed = React.useCallback(
    (id: PaneId) => !!layout.collapsed[id],
    [layout],
  )
  // DERIVADO (não estado): o botão Intelligence reflete o painel aberto. Fix #310.
  const intelToggled = !layout.collapsed.intelligence

  const flash = (m: string) => {
    setNotice(m)
    window.clearTimeout(noticeTimer.current)
    noticeTimer.current = window.setTimeout(() => setNotice(''), 2600)
  }

  const closePalette = () => { setPaletteOpen(false); setPq(''); setPi(0) }
  const togglePalette = (open?: boolean) => {
    const next = open === undefined ? !paletteOpen : open
    setPaletteOpen(next)
    if (next) { setPq(''); setPi(0) }
  }

  // Alterna o tema E persiste (USER PREFERENCE → cosca-desktop:prefs). É a
  // única escrita de tema: o botão do topo, o palette e a Settings chamam aqui.
  const setTheme = React.useCallback((next: boolean) => {
    setDark(next)
    savePrefs({ theme: next ? 'dark' : 'light' })
  }, [])

  const discover = async (l: string) => { setLoc(l); setProjects(await DiscoverProjects(l)); setView('home') }

  const restoreApproval = async () => {
    try {
      const ap = await PendingApproval()
      if (ap && ap.status) setApproval(ap)
    } catch (_) { /* noop */ }
  }

  React.useEffect(() => {
    discover(DEFAULT_LOC)
    HasProject().then(h => {
      if (h) ProjectInfo().then(i => { if (i.project) open(i.project) })
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // ADR-0007 (FASE A): descobre o modo real (forge/standalone) e capabilities.
  React.useEffect(() => {
    let alive = true
    CapabilityState().then(cs => {
      if (!alive || !cs) return
      if (cs.mode) setCapMode(cs.mode as Mode)
      setCap({
        // Em standalone NÃO há kernel/memória da família — nunca "tracear" verdade.
        kernel: cs.mode === 'forge' && !!cs.kernel,
        family_memory: cs.mode === 'forge' && !!cs.family_memory,
        daemon: !!cs.daemon,
        agents: cs.agents ?? 0,
        skills: cs.skills ?? 0,
      })
    }).catch(() => {})
    RootMode().then(r => {
      if (!alive) return
      setCapMode(prev => prev ?? (r ? 'forge' : 'standalone'))
    }).catch(() => {})
    return () => { alive = false }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Assina os eventos REAIS do runtime (wails EventsOn) — cleanup com EventsOff.
  React.useEffect(() => {
    const push = (raw: any) => {
      const e = toRuntimeEvent(raw)
      const key = `${e.session_id || ''}:${e.seq ?? 'x'}:${e.type}`
      if (e.seq != null) {
        if (seenSeq.current.has(key)) return // dedupe por seq (evita duplicar à la ConsumeEvents)
        seenSeq.current.add(key)
      }
      setActivity(a => [...a, eventToActivity(e)])
    }
    RUNTIME_EVENTS.forEach(([name]) => { EventsOn(name, push) })
    // FASE A fallback: backfill de eventos que já estavam buffered no backend
    // antes da UI assinar (atividade pré-existente do runtime).
    SubscribeRuntime().then(evs => {
      if (Array.isArray(evs)) evs.forEach(e => push(e))
    }).catch(() => {})
    return () => { RUNTIME_EVENTS.forEach(([name]) => { try { EventsOff(name) } catch { /* noop */ } }) }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // FASE A fallback: drena eventos buffered no backend após uma ação.
  const drainEvents = React.useCallback(async () => {
    try {
      const evs = await ConsumeEvents()
      if (Array.isArray(evs)) evs.forEach(e => {
        const key = `${e.session_id || ''}:${e.seq ?? 'x'}:${e.type}`
        if (e.seq != null) {
          if (seenSeq.current.has(key)) return
          seenSeq.current.add(key)
        }
        setActivity(a => [...a, eventToActivity(e)])
      })
    } catch { /* noop */ }
  }, [])

  const loadRoot = React.useCallback(async () => {
    setForgeLoading(true); setForgeErr('')
    try { setForgeInfo(await RootInfo()) } catch (e) { setForgeErr(String(e)) }
    setForgeLoading(false)
  }, [])

  // FASE C (Diff): chama ProjectDiff() — a fonte REAL. Nunca fabrica: se o
  // backend disser `git:false` ou `items:[]`, a UI reflete exatamente isso.
  const loadDiff = React.useCallback(async () => {
    if (!project) { setDiffData(null); return }
    setDiffLoading(true); setDiffErr('')
    try { setDiffData(toDiff(await ProjectDiff())); setDiffSelected(null) }
    catch (e) { setDiffErr(String(e)) }
    setDiffLoading(false)
  }, [project])

  // Abre o pane Diff (expande se recolhido) e carrega o working tree.
  const openDiffPanel = React.useCallback(() => {
    if (isCollapsed('diff')) toggleCollapse('diff')
    loadDiff()
  }, [isCollapsed, toggleCollapse, loadDiff])

  // FASE D: abre o pane Terminal autorizado (expande se recolhido).
  const openTerminalPanel = React.useCallback(() => {
    if (isCollapsed('terminal')) toggleCollapse('terminal')
  }, [isCollapsed, toggleCollapse])

  // PROJECT INTELLIGENCE: carrega o perfil real do projeto (AnalyzeProject).
  // Fonte única — a UI só apresenta `{has_project:true, profile}` quando o
  // backend confirmar; se `has_project:false` ou `error`, reflete isso
  // honestamente. Roda em background (async) e nunca bloqueia o workspace.
  const loadIntel = React.useCallback(async () => {
    if (!project) { setIntel(null); setIntelErr(''); return }
    setIntelLoading(true); setIntelErr('')
    try {
      const r = await AnalyzeProject()
      if (r && r.has_project) {
        if (r.error) { setIntelErr(String(r.error)); setIntel(null) }
        else if (r.profile && typeof r.profile === 'object') setIntel(r.profile)
        else setIntel(null)
      } else {
        setIntel(null)
      }
    } catch (e) { setIntelErr(String(e)); setIntel(null) }
    setIntelLoading(false)
  }, [project])

  // UI PERCEPTION (sem GPU) — fecha o loop "COSCA enxerga a interface":
  // 1) coleta a cena REAL do DOM (collectUIScene — read-only, nunca altera),
  // 2) serializa e envia ao backend (CaptureUI),
  // 3) guarda o resultado honesto (ok + summary + violations | ok:false + error).
  // A UI apenas apresenta o que o Kernel respondeu — nunca fabrica violação.
  const runUIAudit = React.useCallback(async () => {
    setUiAuditBusy(true)
    try {
      const scene = collectUIScene()
      const r = await CaptureUI(JSON.stringify(scene))
      const audit = r && typeof r === 'object' ? (r as Record<string, any>) : null
      if (audit && audit.ok === true && audit.audit && typeof audit.audit === 'object') {
        const a = audit.audit as Record<string, any>
        const violations = asArr(a.violations).map((v: any): UiAuditViolation => ({
          severity: String(v?.severity ?? 'LOW'),
          rule: String(v?.rule ?? ''),
          element_id: String(v?.element_id ?? ''),
          reason: String(v?.reason ?? ''),
          evidence: String(v?.evidence ?? ''),
        }))
        const nested = scene.root?.children?.length ?? 0
        setUiAudit({
          ok: true,
          summary: String(a.summary ?? ''),
          detail: `scene: ${scene.elements.length} folhas · +${nested} rigs`,
          violations,
        })
      } else {
        setUiAudit({
          ok: false,
          summary: '',
          violations: [],
          error: audit && audit.error ? String(audit.error) : 'análise da interface falhou',
        })
      }
    } catch (e) {
      setUiAudit({ ok: false, summary: '', violations: [], error: String(e) })
    }
    setUiAuditBusy(false)
  }, [])

  // Painel/dialog denso do UI Audit (resultado real da percepção do Kernel).
  const uiAuditEl = uiAudit ? (
    <div className="dialog-back" onClick={() => setUiAudit(null)}>
      <div className="panel dialog" style={{ width: 620, maxWidth: '92vw', maxHeight: '80vh', display: 'flex', flexDirection: 'column', overflow: 'hidden' }} onClick={e => e.stopPropagation()}>
        <div className="panel-head" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span>UI Audit · percepção do Kernel (sem GPU)</span>
          <button className="collapse-btn" title="Fechar UI Audit" aria-label="Fechar UI Audit" onClick={() => setUiAudit(null)}><Icon name="close" size={14} /></button>
        </div>
        <div style={{ padding: '8px 12px', borderBottom: '1px solid var(--border)', fontSize: 'var(--fs-sm)', display: 'flex', alignItems: 'center', gap: 8 }}>
          {uiAudit.ok ? <span className={`badge ${uiAudit.violations.length ? 'warn' : 'ok'}`}>{uiAudit.violations.length ? `${uiAudit.violations.length} violações` : 'sem violações'}</span> : <span className="badge danger">erro</span>}
          <span className="mono small" style={{ color: 'var(--text-3)' }}>{uiAudit.summary || uiAudit.error || ''}</span>
        </div>
        <div style={{ flex: 1, minHeight: 0, overflow: 'auto', padding: '4px 12px 12px' }}>
          {uiAudit.ok && uiAudit.violations.length === 0 && (
            <div className="empty" style={{ padding: 'var(--sp-5)' }}><p>Interface percebida sem violações de geometria/acessibilidade.</p></div>
          )}
          {uiAudit.ok && uiAudit.violations.map((v, i) => (
            <div key={i} style={{ display: 'flex', alignItems: 'baseline', gap: 8, padding: '5px 0', borderBottom: '1px solid var(--border)', fontSize: 'var(--fs-xs)' }}>
              <span className="mono" style={{ width: 14, color: v.severity === 'HIGH' ? 'var(--danger)' : v.severity === 'MEDIUM' ? 'var(--warn)' : 'var(--text-3)' }}>{v.severity === 'HIGH' ? '●' : v.severity === 'MEDIUM' ? '◆' : '·'}</span>
              <span className={`badge ${v.severity === 'HIGH' ? 'danger' : v.severity === 'MEDIUM' ? 'warn' : 'neutral'}`} style={{ flex: 'none' }}>{v.severity}</span>
              <span className="mono" style={{ color: 'var(--accent)', flex: 'none' }}>{v.rule}</span>
              <span style={{ flex: 1, minWidth: 0 }}>{v.reason} <span className="mono" style={{ color: 'var(--text-3)' }}>({v.element_id})</span></span>
            </div>
          ))}
          {!uiAudit.ok && <div className="error-box" style={{ margin: 'var(--sp-3)' }}>{uiAudit.error}</div>}
        </div>
      </div>
    </div>
  ) : null
  // Fonte única — se `has_project:false` ou `error`, a UI reflete honestamente
  // (o agente não finge conhecer um projeto que não foi analisado).
  const loadAgentCtx = React.useCallback(async () => {
    if (!project) { setAgentCtx(null); setAgentCtxErr(''); return }
    setAgentCtxLoading(true); setAgentCtxErr('')
    try {
      const r = await AgentProjectContext()
      if (r && r.has_project) {
        if (r.error) { setAgentCtxErr(String(r.error)); setAgentCtx(null) }
        else setAgentCtx(r)
      } else {
        setAgentCtx(null)
      }
    } catch (e) { setAgentCtxErr(String(e)); setAgentCtx(null) }
    setAgentCtxLoading(false)
  }, [project])

  // REFRESH GLOBAL DE INTELIGÊNCIA (painel Intelligence + contexto do agente).
  // `RefreshProject()` invalida o cache do projeto ativo e re-analisa (fresca).
  // Em seguida loadIntel()/loadAgentCtx() re-populam intel/agentCtx lendo do
  // cache incremental já aquecido (sem nova varredura completa — o backend usa
  // cache incremental). Fonte única; a UI nunca fabrica perfil.
  const refreshAllIntel = React.useCallback(async () => {
    if (!project) return
    setIntelRefreshing(true)
    try {
      // Invalida cache + re-analisa. Se falhar, loadIntel/loadAgentCtx reportam
      // o erro HONESTO via intelErr/agentCtxErr (não fabricamos sucesso).
      await RefreshProject()
    } catch { /* noop — os loaders abaixo reportam o estado real */ }
    await Promise.all([loadIntel(), loadAgentCtx()])
    setIntelRefreshing(false)
  }, [project, loadIntel, loadAgentCtx])

  // PROJETO ATIVO → chama AnalyzeProject() + AgentProjectContext() (background).
  // Dispara ao abrir/criar/init (que sempre setam `project`).
  React.useEffect(() => {
    loadIntel()
    loadAgentCtx()
  }, [loadIntel, loadAgentCtx])

  // `intelToggled` é um valor DERIVADO do estado colapsável do pane intelligence
  // (Layout Engine) — NÃO é estado. Derivar evita o loop #310 (era um setState
  // síncrono em useEffect lendo do objeto `layout` trocado ao abrir projeto).

  // Abre o pane Intelligence (expande se recolhido); carregamento é o effect.
  const openIntelPanel = React.useCallback(() => {
    if (isCollapsed('intelligence')) toggleCollapse('intelligence')
  }, [isCollapsed, toggleCollapse])

  // FASE D: auto-scroll do scrollback para o fim a cada novo comando/saída.
  React.useEffect(() => {
    if (termScrolled.current) termScrolled.current.scrollTop = termScrolled.current.scrollHeight
  }, [terminalHist, termBusy])

  // FASE D: foca o prompt quando o pane Terminal é expandido.
  React.useEffect(() => {
    if (!layout.collapsed.terminal) termInputRef.current?.focus()
  }, [layout.collapsed.terminal])

  // Carrega o diff ao abrir o painel (quando o pane Diff está expandido) ou ao
  // trocar de projeto. Fonte real via ProjectDiff(); o painel só está visível
  // no workspace com projeto ativo.
  React.useEffect(() => {
    if (project && !layout.collapsed.diff) loadDiff()
  }, [project, layout.collapsed.diff, loadDiff])

  // Carrega a estrutura do root ao entrar na view FORGE.
  React.useEffect(() => {
    if (view === 'forge') loadRoot()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view])

  const setProgress = (msgs: string[]) => setSteps(msgs.map(m => ({ msg: m, done: false })))
  const markDone = (i: number) => setSteps(s => s.map((x, j) => ({ ...x, done: j <= i })))

  const open = async (root: string) => {
    setBusy(true); setErr(''); setActivity([]); setProgress(['Abrindo projeto…'])
    try {
      const p = await OpenProject(root)
      setProject(p)
      setTreeRoot(await ReadDir(''))
      // FASE 3: a tela principal ao abrir um projeto é a CONVERSA.
      markDone(0); setView('chat'); restoreApproval()
    } catch (e) {
      setErr(String(e))
    }
    setBusy(false)
  }

  const create = async () => {
    setBusy(true); setErr('')
    setProgress(['Validando diretório', 'Criando projeto', 'Executando cosca init', 'Verificando projeto'])
    try {
      await ValidateLocation(loc); markDone(0)
      const p = await CreateProject(name, loc); markDone(1)
      setProject(p); setTreeRoot(await ReadDir('')); markDone(2); markDone(3)
      setSteps(s => [...s.map(x => ({ ...x, done: true })), { msg: 'Abrindo conversa…', done: false }])
      await new Promise(r => setTimeout(r, 300))
      // FASE 3: a tela principal ao criar um projeto é a CONVERSA.
      setView('chat'); restoreApproval()
    } catch (e) { setErr(String(e)) }
    setBusy(false)
  }

  const init = async () => {
    setBusy(true); setErr(''); setProgress(['Executando cosca init', 'Verificando'])
    try { const p = await InitProject(); setProject(p); setTreeRoot(await ReadDir('')); markDone(1); restoreApproval() }
    catch (e) { setErr(String(e)) }
    setBusy(false)
  }

  const closeFn = async () => { await CloseProject(); setProject(null); setApproval(null); setView('home'); discover(loc) }

  // AGENT WORKBENCH ---------------------------------------------------------
  const doPlan = async () => {
    if (!agentReq.trim()) return
    setAgentRunning(true); setErr('')
    setActivity(a => [...a, { at: 'agora', title: 'Gerando plano de execução', body: agentReq }])
    try {
      const out = await AgentPlan(agentReq)
      setPlan(out)
      // FASE B: extrai dados REAIS do plano (files/risk/policy/action/command/cwd/
      // scope/packages/tests/confidence) para o Approval Center decidir com base
      // em fato, não em placeholder. Se falhar/ausente → fica null (honesto).
      try { setPlanDetails(await PlanDetails(out)) } catch { setPlanDetails(null) }
      setActivity(a => [...a, { at: 'agora', title: 'Plano pronto — revisar', body: out.slice(0, 300) }])
    } catch (e) { setErr(String(e)) }
    setAgentRunning(false)
  }

  // Executar (AgentRun) é uma operação de ESCRITA/EXECUÇÃO: o backend é
  // fail-closed e só dispara o runtime com uma aprovação "approved". Por isso o
  // fluxo NÃO vai direto ao AgentRun: primeiro registra a aprovação pendente
  // (RequestApproval) e abre o Approval Center para o usuário DECIDIR.
  const doRun = async () => {
    const req = agentReq.trim()
    if (!req || agentRunning) return
    setErr(''); setAgentRunning(false)
    setActivity(a => [...a, { at: 'agora', title: 'Solicitando aprovação de execução', body: req.slice(0, 200) }])
    try {
      const ap = await RequestApproval(req, AGENT_MODEL, 'exec')
      setApproval(ap)
      if (!ap || ap.status !== 'pending') {
        setErr('Nenhuma aprovação pendente registrada: ' + (ap ? ap.status : 'null'))
        return
      }
    } catch (e) { setErr(String(e)) }
  }
  // Ref do `doRun` (função recriada a cada render) para o handler global de
  // Ctrl+Enter ler SEMPRE o closure mais recente, sem re-assinar o listener.
  const doRunRef = React.useRef(doRun)
  doRunRef.current = doRun

  const approveNow = async (_scope: 'once' | 'session') => {
    if (!approval) return
    setApproving(true); setErr('')
    try {
      const ap = await ApprovePending(approval.id)
      if (!ap || ap.status !== 'approved') {
        setErr('Aprovação falhou (id inválido): ' + (ap ? ap.status : 'null'))
        setApproving(false); return
      }
      setApproval(ap)
      setActivity(a => [...a, { at: 'agora', title: `Aprovação concedida`, body: ap.request.slice(0, 160), kind: 'local' }])
      setAgentRunning(true)
      // FASE B: execução REAL via SSE (emite os eventos do runtime durante o
      // fluxo). O GATE permanece fail-closed: só chegamos aqui porque
      // ApprovePending confirmou status 'approved'; sem aprovação o backend recusa.
      const out = await AgentRunStream(ap.request, AGENT_MODEL)
      await drainEvents() // eventos emitidos durante a execução
      // FASE C (Diff): ao fim da execução, refaz o snapshot do working tree com
      // as mudanças do agente (fonte real DiffSnapshot). Falha silenciosa —
      // mantém o último diff honesto em vez de fabricar um novo.
      try { setDiffData(toDiff(await DiffSnapshot())); setDiffSelected(null) } catch { /* noop */ }
      const offline = /offline|modo exec/i.test(out || '')
      setActivity(a => [...a, {
        at: 'agora',
        // Se o daemon não está de pé, AgentRunStream devolve essa mensagem — a
        // mostramos como o que é (não como um erro fabricado do runtime).
        title: offline ? 'Execução não roteada (daemon offline)' : 'Execução concluída (resultado)',
        body: out, kind: 'local',
      }])
      setApproval(null) // aprovação consumida pelo runtime
    } catch (e) { setErr(String(e)) }
    setAgentRunning(false); setApproving(false)
  }

  const denyNow = async () => {
    if (!approval) return
    setApproving(true); setErr('')
    try {
      const ap = await DenyPending(approval.id)
      setActivity(a => [...a, { at: 'agora', title: 'Aprovação negada', body: (ap && ap.request ? ap.request : '').slice(0, 160) }])
      setApproval(null)
    } catch (e) { setErr(String(e)) }
    setApproving(false)
  }

  const doApprovePlan = async () => {
    if (!plan.trim()) return
    setAgentRunning(true); setErr('')
    setActivity(a => [...a, { at: 'agora', title: 'Aprovando plano (cosca approve)', body: plan.slice(0, 200) }])
    try {
      const out = await AgentApprove(plan)
      setActivity(a => [...a, { at: 'agora', title: 'Aprovação registrada', body: out.slice(0, 300) }])
    } catch (e) { setErr(String(e)) }
    setAgentRunning(false)
  }

  // FILE EXPLORER (lazy) ----------------------------------------------------
  // Fonte de verdade do filesystem é o backend. A UI só pede a pasta expandida.
  const loadDir = async (path: string) => {
    setExpanded(e => ({ ...e, [path]: true }))
    try {
      setLoading(l => ({ ...l, [path]: true }))
      setTreeErr(te => ({ ...te, [path]: '' }))
      const nodes = await ReadDir(path)
      // Merge: substitui os children do nó censurado de `treeRoot` pelo resultado.
      setTreeRoot(prev => mergeChildren(prev, path || '', nodes))
    } catch (e) {
      setTreeErr(te => ({ ...te, [path]: String(e) }))
    } finally {
      setLoading(l => ({ ...l, [path]: false }))
    }
  }

  const toggleDir = (node: main.FileNode) => {
    const p = node.path
    if (!node.is_dir) return // Só pastas abrem/colapsam.
    if (expanded[p]) {
      setExpanded(e => ({ ...e, [p]: false }))
    } else if (!loading[p]) {
      loadDir(p) // Lazy: SÓ esta pasta, nunca re-scan da árvore inteira.
    }
  }

  const openFile = async (node: main.FileNode) => {
    if (node.is_dir) { toggleDir(node); return }
    setSelectedPath(node.path)
    setActiveFile(node)
    setFileErr(''); setFileView(null)
    try {
      const txt = await ReadFile(node.path)
      setFileContent(txt)
      setFileView('code')
    } catch (e) {
      setFileErr(String(e))
    }
  }

  const save = async () => {
    if (activeFile && fileView === 'edit') await WriteFile(activeFile.path, fileContent)
  }
  // livre — a UI chama RunCommand (gate fail-closed + filtro de perigo; read-only
  // roda direto) ou RunCommandAuthorized (sensível/perigoso exige aprovação no
  // gate). A UI apenas apresenta o texto real de volta ao usuário (tone = legenda).
  // Se o backend recusar/exigir aprovação (tone 'warn'), mantém o comando no
  // prompt para o usuário acionar "▶ Autorizado"; caso contrário, limpa o prompt.
  const doRunCmd = async (mode: 'normal' | 'authorized') => {
    const cmd = termInput.trim()
    if (!cmd || !project || termBusy) return
    setTermBusy(true)
    try {
      const out = mode === 'authorized' ? await RunCommandAuthorized(cmd) : await RunCommand(cmd)
      const tone = cmdTone(out)
      setTerminalHist(h => [...h, { cmd, out, tone }])
      if (!(mode === 'normal' && tone === 'warn')) setTermInput('')
    } catch (e) {
      const out = String(e)
      const tone = cmdTone(out)
      setTerminalHist(h => [...h, { cmd, out, tone }])
      if (!(mode === 'normal' && tone === 'warn')) setTermInput('')
    } finally {
      setTermBusy(false)
    }
  }
  const renderNode = (n: main.FileNode, depth: number): React.ReactNode => {
    const isOpen = !!expanded[n.path]
    const isLoading = !!loading[n.path]
    const hasErr = !!treeErr[n.path]
    return (
      <div key={n.path} className="tnode" role="treeitem" aria-expanded={n.is_dir ? isOpen : undefined}
        data-kind={n.is_dir ? 'dir' : 'file'} data-state={isOpen ? 'expanded' : isLoading ? 'loading' : hasErr ? 'error' : 'idle'}>
        <div
          id={'ftree-' + n.path}
          tabIndex={-1}
          className={`item ${selectedPath === n.path ? 'active' : ''}`}
          style={{ paddingLeft: 6 + depth * 14 }}
          onClick={() => n.is_dir ? toggleDir(n) : openFile(n)}
          title={n.path}
        >
          <span className="item-chev" aria-hidden="true">
            {n.is_dir ? <Icon name={isOpen ? 'chevron-down' : 'chevron-right'} size={12} /> : <span className="item-sp" />}
          </span>
          <span className="item-ico"><FileIcon node={n} expanded={isOpen} size={13} /></span>
          <span className="item-name">{n.name}</span>
        </div>
        {n.is_dir && isOpen && (
          isLoading ? (
            <div className="item-loading" style={{ paddingLeft: 20 + depth * 14 }}>carregando…</div>
          ) : hasErr ? (
            <div className="item-error" style={{ paddingLeft: 20 + depth * 14 }}>
              <span>Unable to read directory</span>
              <button className="btn btn--sm" onClick={() => loadDir(n.path)}>Retry</button>
            </div>
          ) : (
            n.children?.map(c => renderNode(c, depth + 1))
          )
        )}
      </div>
    )
  }

  // KEYBOARD NAVIGATION na árvore (item #20): lista plana dos nós visíveis.
  // Só os nós cuja cadeia de pastas está expandida aparecem; mover o foco segue
  // essa ordem. UseMemo com deps [treeRoot, expanded] — estável, sem render loop.
  const flatVisible = useMemo(() => {
    const out: main.FileNode[] = []
    const walk = (nodes: main.FileNode[]) => {
      for (const n of nodes) {
        out.push(n)
        if (n.is_dir && expanded[n.path] && n.children) walk(n.children)
      }
    }
    walk(treeRoot)
    return out
  }, [treeRoot, expanded])

  const treeIdx = flatVisible.findIndex(n => n.path === selectedPath)

  const onTreeKey = (e: React.KeyboardEvent) => {
    if (!flatVisible.length) return
    let next = treeIdx
    switch (e.key) {
      case 'ArrowDown': next = Math.min(treeIdx + 1, flatVisible.length - 1); break
      case 'ArrowUp': next = Math.max(treeIdx - 1, 0); break
      case 'Home': next = 0; break
      case 'End': next = flatVisible.length - 1; break
      case 'ArrowRight': {
        const n = flatVisible[treeIdx]
        if (n && n.is_dir && !expanded[n.path]) { toggleDir(n); return }
        break
      }
      case 'ArrowLeft': {
        const n = flatVisible[treeIdx]
        if (n && n.is_dir && expanded[n.path]) { toggleDir(n); return }
        break
      }
      case 'Enter': {
        const n = flatVisible[treeIdx]
        if (n) { n.is_dir ? toggleDir(n) : openFile(n) }
        return
      }
      default: return
    }
    e.preventDefault()
    if (next >= 0 && flatVisible[next]) {
      setSelectedPath(flatVisible[next].path)
      if (flatVisible[next]) document.getElementById('ftree-' + flatVisible[next].path)?.focus()
    }
  }

  const progressEl = (
    <div className="progress">
      {steps.map((s, i) => (
        <div key={i} className={`p ${s.done ? 'done' : ''}`}>
          <span className="tick">{s.done ? '✓' : '·'}</span>{s.msg}
        </div>
      ))}
    </div>
  )

  // WINDOW CHROME (top bar) — aparência nativa de app, não website ----------
  const chrome = (projectName?: string) => (
    <header className="topbar">
      <button onClick={() => setView('home')} title="Home"
        style={{ display: 'flex', alignItems: 'center', gap: 8, fontWeight: 700, letterSpacing: '.6px', fontSize: 'var(--fs-md)', color: 'var(--text)', background: 'transparent', border: 0, cursor: 'pointer', padding: 0 }}>
        <span className="brand-mark"><Icon name="brand" size={14} /></span><span>COSCA</span>
      </button>
      <span style={{ width: 1, height: 20, background: 'var(--border)', margin: '0 2px' }} />
      <span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 'var(--fs-xs)', color: 'var(--text-2)' }}>
        <span style={{ fontWeight: 700, letterSpacing: '.5px', padding: '1px 7px', borderRadius: 'var(--r-full)', background: 'var(--surface-2)', border: '1px solid var(--border)' }}>PROJECT</span>
        <b style={{ color: 'var(--text)', fontWeight: 600, maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{projectName || '—'}</b>
      </span>
      <span style={{ width: 1, height: 20, background: 'var(--border)', margin: '0 2px' }} />
      <button onClick={() => togglePalette(true)} title="Command palette (Ctrl+K)"
        style={{ flex: 1, maxWidth: 420, minWidth: 160, display: 'flex', alignItems: 'center', gap: 8, padding: '5px 12px', background: 'var(--surface-2)', border: '1px solid var(--border)', borderRadius: 'var(--r-sm)', color: 'var(--text-3)', fontSize: 'var(--fs-sm)', cursor: 'pointer', textAlign: 'left' }}>
        <Icon name="search" size={14} />
        <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>Search or run command</span>
        <span className="kbd-group"><kbd className="kbd">Ctrl</kbd><kbd className="kbd">K</kbd></span>
      </button>
      <span className="spacer" />
      <span className={`badge ${capMode === 'forge' ? 'info' : 'neutral'}`} title={capMode === 'forge' ? 'No root Cosca — kernel presente (ADR-0007)' : 'Fora do root — sem kernel (ADR-0007)'}>
        <span className="dot" /> {capMode === null ? 'MODO · …' : capMode === 'forge' ? 'FORGE · kernel' : 'STANDALONE'}
      </span>
      {capMode === 'forge'
        ? <span className="badge">agents <b>{cap.agents ?? 0}</b> · skills <b>{cap.skills ?? 0}</b></span>
        : <span className="badge">agents+skills <b>—</b></span>}
      <span className={`badge ${cap.daemon ? 'ok' : 'neutral'}`} title="daemon do runtime (estado real)">daemon <b>{cap.daemon ? 'on' : 'off'}</b></span>
      <span className="badge" title="usuário local">user · local</span>
      <button className="iconbtn" onClick={() => setView('settings')} title="Settings (Ctrl+,)" aria-label="Abrir configurações"><Icon name="settings" size={16} /></button>
      <button className="iconbtn" onClick={() => setTheme(!dark)} title="tema" aria-label="Alternar tema"><Icon name={dark ? 'sun' : 'moon'} size={16} /></button>
    </header>
  )

  // STATUS BAR (compacta, profissional) -------------------------------------
  const statusbar = () => (
    <footer className="statusbar">
      <span className="seg"><span className={`badge ${capMode === 'forge' ? 'info' : 'neutral'}`}>mode <b>{capMode === null ? '…' : capMode === 'forge' ? 'forge · kernel' : 'standalone'}</b></span></span>
      <span className="seg"><span className={`badge ${cap.daemon ? 'ok' : 'neutral'}`}><span className="dot" /> runtime <b>{cap.daemon ? 'on' : 'off'}</b></span></span>
      <span className="seg"><span className="badge">project <b>{project?.name || '—'}</b></span></span>
      <span className="seg"><span className={`badge ${agentRunning ? 'warn' : 'ok'}`}>agent <b>{agentRunning ? 'running' : 'idle'}</b></span></span>
      <span className="seg"><span className="badge">model <b>{AGENT_MODEL}</b></span></span>
      <span className="seg"><span className="badge">git <b>—</b></span></span>
      <span className="seg"><span className={`badge ${project?.initialized ? 'ok' : 'warn'}`}>sandbox <b>{project?.initialized ? 'isolated' : 'not init'}</b></span></span>
      <span className="seg"><span className="badge">network <b>—</b></span></span>
      <span className="seg"><span className="badge">tool <b>cosca</b></span></span>
      {notice && <span className="seg" style={{ color: 'var(--warn)' }}>{notice}</span>}
    </footer>
  )

  // linha chave / valor (usada no Approval Center e no Context) -------------
  const kv = (label: string, value: React.ReactNode, tone?: RiskTone) => (
    <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10, padding: '6px 12px', borderBottom: '1px solid var(--border)', fontSize: 'var(--fs-xs)' }}>
      <span style={{ color: 'var(--text-3)', fontWeight: 600, letterSpacing: '.4px', whiteSpace: 'nowrap' }}>{label}</span>
      <b style={{ color: tone ? `var(--${tone})` : 'var(--text)', textAlign: 'right', fontWeight: 600, wordBreak: 'break-all', fontFamily: label === 'WORKING DIR' ? 'var(--font-mono)' : 'inherit' }}>{value}</b>
    </div>
  )

  // NAVIGATION RAIL ---------------------------------------------------------
  const railSections: { items: { id: RailId; label: string; icon: IconName; available: boolean }[] }[] = [
    {
      items: [
        { id: 'home', label: 'HOME', icon: 'home', available: true },
        { id: 'project', label: 'PROJECT', icon: 'project', available: true },
        { id: 'chat', label: 'CHAT', icon: 'chat', available: true },
        { id: 'agent', label: 'AGENT', icon: 'agent', available: true },
        { id: 'files', label: 'FILES', icon: 'files', available: true },
        { id: 'terminal', label: 'TERMINAL', icon: 'terminal', available: !!project },
        { id: 'search', label: 'SEARCH', icon: 'search', available: true },
        { id: 'diff', label: 'DIFF', icon: 'diff', available: !!project },
        { id: 'intelligence', label: 'INTEL', icon: 'intelligence', available: !!project },
      ],
    },
    {
      items: [
        { id: 'forge', label: 'FORGE', icon: 'forge', available: true },
        { id: 'knowledge', label: 'KNOWLEDGE', icon: 'knowledge', available: false },
        { id: 'memory', label: 'MEMORY', icon: 'memory', available: false },
        { id: 'provenance', label: 'PROVENANCE', icon: 'provenance', available: false },
        { id: 'audit', label: 'AUDIT', icon: 'audit', available: false },
      ],
    },
    {
      items: [
        { id: 'sandbox', label: 'SANDBOX', icon: 'sandbox', available: false },
        { id: 'runtime', label: 'RUNTIME', icon: 'runtime', available: false },
      ],
    },
  ]
  const activeRail: RailId = view === 'home' ? 'home' : view === 'create' ? 'project' : view === 'forge' ? 'forge' : view === 'chat' ? 'chat' : (wsArea === 'files' ? 'files' : wsArea === 'terminal' ? 'terminal' : wsArea === 'search' ? 'search' : 'agent')

  const onRail = (it: { id: RailId; label: string; available: boolean }) => {
    if (!it.available) { flash(it.label + ' — not available'); return }
    switch (it.id) {
      case 'home': setView('home'); break
      // PROJECT = abrir projeto existente: vai à Home (lista de projetos). O
      // botão "＋ Criar projeto" (na Home) é o caminho para criar.
      case 'project': setView('home'); break
      case 'chat': setView('chat'); break
      case 'agent':
        setView('workspace'); setWsArea('agent')
        if (isCollapsed('agent')) toggleCollapse('agent')
        if (!isCollapsed('tree')) toggleCollapse('tree') // foco no agente → recolhe a tree
        break
      case 'files':
        setView('workspace'); setWsArea('files')
        if (isCollapsed('tree')) toggleCollapse('tree')
        if (!isCollapsed('agent')) toggleCollapse('agent') // foco nos arquivos → recolhe o agente
        break
      case 'terminal':
        setView('workspace'); setWsArea('terminal'); openTerminalPanel()
        break
      case 'forge': setView('forge'); break
      case 'search': setWsArea('search'); togglePalette(true); break
      case 'diff':
        if (!project) { setView('workspace'); break }
        setView('workspace')
        openDiffPanel()
        break
      case 'intelligence':
        if (!project) { setView('workspace'); break }
        setView('workspace')
        loadIntel()
        openIntelPanel()
        break
      default: flash(it.label + ' — not available')
    }
  }

  const railEl = (
    <aside className="rail" style={{ width: 54 }}>
      {railSections.map((group, gi) => (
        <React.Fragment key={gi}>
          {gi > 0 && <span style={{ width: 26, height: 1, background: 'var(--border)', margin: '4px 0' }} />}
          {group.items.map(it => (
            <button key={it.id}
              className={`rail-btn ${activeRail === it.id ? 'active' : ''} ${it.available ? '' : 'disabled'}`}
              title={it.available ? it.label : it.label + ' (em desenvolvimento — não disponível)'}
              aria-label={it.available ? it.label : it.label + ' (indisponível)'}
              onClick={() => onRail(it)}>
              <Icon name={it.icon} size={16} />
            </button>
          ))}
        </React.Fragment>
      ))}
      <span className="rail-sp" />
      <button className="rail-btn" title="Fechar projeto" aria-label="Fechar projeto" onClick={closeFn}><Icon name="close" size={15} /></button>
    </aside>
  )

  // COMMAND PALETTE ---------------------------------------------------------
  type PaletteCmd = { id: string; label: string; shortcut?: string; context: string; available: boolean; run: () => void }
  const commands: PaletteCmd[] = [
    { id: 'openproj', label: 'Open Project', shortcut: 'Ctrl+O', context: 'Ir para a Home de projetos', available: true, run: () => { setView('home'); closePalette() } },
    { id: 'newproj', label: 'New Project', shortcut: 'Ctrl+N', context: 'Criar novo projeto', available: true, run: () => { setView('create'); closePalette() } },
    { id: 'chat', label: 'Chat', shortcut: '', context: 'Abrir a conversa (tela principal)', available: true, run: () => { setView('chat'); closePalette() } },
    { id: 'openfile', label: 'Open File', shortcut: 'Ctrl+E', context: 'Abrir árvore de arquivos do projeto', available: !!project, run: () => { setView('workspace'); closePalette() } },
    { id: 'run', label: 'Run Command', shortcut: 'Ctrl+Enter', context: 'Executar pedido do agente (pede aprovação)', available: !!project, run: () => { setView('workspace'); closePalette(); doRun() } },
    { id: 'start', label: 'Start Agent', shortcut: '', context: 'Iniciar agente (pede aprovação)', available: !!project, run: () => { setView('workspace'); closePalette(); doRun() } },
    { id: 'plan', label: 'Plan Agent', shortcut: '', context: 'Gerar plano de execução (read-only)', available: !!project, run: () => { setView('workspace'); closePalette(); doPlan() } },
    { id: 'stop', label: 'Stop Agent', shortcut: '', context: 'Cancelar execução em andamento', available: false, run: () => {} },
    { id: 'terminal', label: 'Open Terminal', shortcut: 'Ctrl+`', context: 'Painel de terminal autorizado', available: !!project, run: () => { setView('workspace'); closePalette(); openTerminalPanel() } },
    { id: 'intel', label: 'Project Intelligence', shortcut: '', context: 'Painel do perfil por evidência', available: !!project, run: () => { setView('workspace'); closePalette(); loadIntel(); openIntelPanel() } },
    { id: 'search', label: 'Search', shortcut: 'Ctrl+K', context: 'Busca', available: false, run: () => {} },
    { id: 'approvals', label: 'Approvals', shortcut: '', context: 'Centro de aprovações pendentes', available: !!project && !!approval, run: () => { setView('workspace'); closePalette() } },
    { id: 'sandbox', label: 'Sandbox', shortcut: '', context: 'Isolamento / contenção', available: false, run: () => {} },
    { id: 'runtime', label: 'Runtime', shortcut: '', context: 'Runtime do Cosca', available: false, run: () => {} },
    { id: 'settings', label: 'Settings', shortcut: 'Ctrl+,', context: 'Preferências do usuário', available: true, run: () => { setView('settings'); closePalette() } },
    { id: 'theme', label: 'Toggle Theme', shortcut: '', context: 'Alternar tema claro/escuro', available: true, run: () => { setTheme(!dark); closePalette() } },
    { id: 'resetlayout', label: 'Reset Layout', shortcut: '', context: 'Restaurar layout padrão COSCA', available: true, run: () => { reset(); closePalette() } },
    { id: 'uiaudit', label: 'UI Audit', shortcut: '', context: 'Analisar interface (percepção sem GPU)', available: true, run: () => { closePalette(); runUIAudit() } },
  ]
  const q = pq.trim().toLowerCase()
  const filtered = q ? commands.filter(c => c.label.toLowerCase().includes(q) || c.context.toLowerCase().includes(q)) : commands
  const safePi = Math.min(pi, Math.max(0, filtered.length - 1))

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const mod = e.ctrlKey || e.metaKey
      // ⌘K / Ctrl+K → command palette
      if (mod && (e.key === 'k' || e.key === 'K')) {
        e.preventDefault()
        togglePalette()
        return
      }
      if (paletteOpen) {
        if (e.key === 'Escape') { e.preventDefault(); closePalette() }
        else if (e.key === 'ArrowDown') { e.preventDefault(); setPi(i => Math.min(i + 1, filtered.length - 1)) }
        else if (e.key === 'ArrowUp') { e.preventDefault(); setPi(i => Math.max(i - 1, 0)) }
        else if (e.key === 'Enter') {
          e.preventDefault()
          const c = filtered[safePi]
          if (c && c.available) c.run()
        }
        return
      }
      // Atalhos de app-level (palette fechada). Cada um mapeia a uma ação REAL.
      if (mod && e.key === ',') { e.preventDefault(); setView('settings'); return }
      if (mod && (e.key === 'o' || e.key === 'O')) { e.preventDefault(); setView('home'); return }
      if (mod && (e.key === 'n' || e.key === 'N')) { e.preventDefault(); setView('create'); return }
      if (mod && (e.key === 'e' || e.key === 'E')) { e.preventDefault(); setView('workspace'); setWsArea('files'); return }
      if (mod && e.key === '`') { e.preventDefault(); setView('workspace'); openTerminalPanel(); return }
      if (mod && e.key === 'Enter') {
        // Ctrl+Enter = run do agente. Evita duplo-disparo quando o foco já está
        // no textarea do agente (que trata Ctrl+Enter) ou em qualquer input.
        const t = e.target as HTMLElement | null
        const tag = t ? t.tagName : ''
        if (tag === 'TEXTAREA' || tag === 'INPUT') return
        if (project) { e.preventDefault(); doRunRef.current() }
        return
      }
      if (e.key === 'Escape' && approval) setApproval(null)
      if (e.key === 'Escape' && uiAudit) setUiAudit(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [paletteOpen, filtered, safePi, approval, uiAudit, project, openTerminalPanel])

  const paletteEl = paletteOpen ? (
    <div className="dialog-back" onClick={closePalette}>
      <div className="panel" style={{ width: 580, maxWidth: '92vw', maxHeight: '70vh', display: 'flex', flexDirection: 'column', overflow: 'hidden' }} onClick={e => e.stopPropagation()}>
        <div style={{ padding: '10px 12px', borderBottom: '1px solid var(--border)' }}>
          <input autoFocus className="input" value={pq} onChange={e => { setPq(e.target.value); setPi(0) }} placeholder="Type a command or search…" />
        </div>
        <div style={{ overflow: 'auto', flex: 1 }}>
          {filtered.length === 0 && <div className="empty"><p>Nenhum comando encontrado.</p></div>}
          {filtered.map((c, i) => (
            <div key={c.id}
              onClick={() => { if (c.available) c.run() }}
              style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 12px', cursor: 'pointer', borderBottom: '1px solid var(--border)', background: i === safePi ? 'var(--accent-soft)' : 'transparent', color: i === safePi ? 'var(--text)' : 'var(--text-2)' }}>
              <span style={{ width: 16, textAlign: 'center', color: 'var(--accent)', display: 'inline-flex', justifyContent: 'center' }}>{c.available ? <Icon name="command" size={13} /> : '·'}</span>
              <span style={{ flex: 1, fontWeight: 600 }}>{c.label}</span>
              <span style={{ fontSize: 'var(--fs-xs)', color: 'var(--text-3)' }}>{c.context}</span>
              {c.available && c.shortcut && <span className="kbd">{c.shortcut}</span>}
              {!c.available && <span className="badge warn">not available</span>}
            </div>
          ))}
        </div>
        <div style={{ display: 'flex', gap: 14, padding: '6px 12px', borderTop: '1px solid var(--border)', fontSize: 'var(--fs-xs)', color: 'var(--text-3)' }}>
          <span>↑↓ navegar</span><span>Enter executar</span><span>Esc fechar</span>
        </div>
      </div>
    </div>
  ) : null

  // =========================================================================
  // VIEWS
  // =========================================================================

  if (view === 'home') {
    return (
      <div className={`shell ${dark ? 'dark' : 'light'}`}>
        {chrome()}
        <main className="home">
          <div className="home-head">
            <h1>Projetos <span className="dim">({projects.length})</span></h1>
            <button className="btn primary" onClick={() => setView('create')}>＋ Criar projeto</button>
          </div>
          {projects.length === 0 && <div className="empty"><div className="hero-mark"><Icon name="project" size={18} /></div><p>Nenhum projeto em {loc}</p></div>}
          <div className="project-grid">
            {projects.map(p => (
              <button key={p.root} className="proj-card" onClick={() => open(p.root)}>
                <div className="proj-top"><span className="proj-ico"><Icon name="project" size={15} /></span><span className={`badge ${p.initialized ? 'ok' : 'warn'}`}>{p.initialized ? 'cosca' : 'sem init'}</span></div>
                <div className="proj-name">{p.name}</div>
                <div className="proj-path mono">{p.root}</div>
              </button>
            ))}
          </div>
        </main>
        {statusbar()}
        {paletteEl}
        {uiAuditEl}
      </div>
    )
  }

  if (view === 'create') {
    return (
      <div className={`shell ${dark ? 'dark' : 'light'}`}>
        {chrome()}
        <main className="center">
          <div className="panel dialog create">
            <h2>Criar projeto</h2>
            <FormField label="Nome do projeto" htmlFor="create-name" required>
              <input id="create-name" className="input" value={name} placeholder="ex: Meu Projeto" onChange={e => setName(e.target.value)} />
            </FormField>
            <FormField label="Localização" htmlFor="create-loc">
              <input id="create-loc" className="input mono" value={loc} onChange={e => setLoc(e.target.value)} />
            </FormField>
            {err && <div className="error-box">{err}</div>}
            <div className="row">
              <button className="btn ghost" onClick={() => setView('home')}>Cancelar</button>
              <button className="btn primary" onClick={create} disabled={busy || !name.trim()}>{busy ? 'Criando…' : 'Criar projeto'}</button>
            </div>
            {busy && <div style={{ marginTop: 14 }}>{progressEl}</div>}
          </div>
        </main>
        {statusbar()}
        {paletteEl}
        {uiAuditEl}
      </div>
    )
  }

  if (view === 'forge') {
    const f = forgeInfo
    const isRoot = !!f && !!f.root
    const agentsN = Array.isArray(f?.agents) ? f.agents.length : (f?.agents ?? '—')
    const skillsN = Array.isArray(f?.skills) ? f.skills.length : (f?.skills ?? '—')
    const adrs = Array.isArray(f?.adrs) ? f.adrs : []
    const renderPaths = (obj: any, depth = 0): React.ReactNode => {
      if (!obj || typeof obj !== 'object') return null
      return Object.entries(obj).map(([k, v]) => (
        <React.Fragment key={k}>
          <div style={{ display: 'flex', gap: 8, paddingTop: 2, paddingBottom: 2, paddingLeft: depth * 14, fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)', alignItems: 'baseline' }}>
            <span style={{ color: 'var(--accent)', alignSelf: 'center' }}><Icon name="chevron-right" size={10} /></span>
            <span style={{ color: 'var(--text-2)', fontWeight: 600 }}>{String(k).replace(/_/g, ' ')}</span>
            {typeof v === 'string' && <span style={{ color: 'var(--text-3)', wordBreak: 'break-all' }}>{v}</span>}
            {typeof v === 'number' && <span style={{ color: 'var(--text-3)' }}>× {v}</span>}
          </div>
          {typeof v === 'object' && v !== null && renderPaths(v, depth + 1)}
        </React.Fragment>
      ))
    }

    return (
      <div className={`shell ${dark ? 'dark' : 'light'}`}>
        {chrome(project?.name)}
        <main style={{ flex: 1, overflow: 'auto', padding: 'var(--sp-5) var(--sp-6)', background: 'var(--bg)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 'var(--sp-4)' }}>
            <h1>COSCA FORGE <span className="dim">· estrutura do root</span></h1>
            <span className="spacer" />
            <button className="btn" onClick={loadRoot} disabled={forgeLoading}>{forgeLoading ? 'Revelando…' : 'Atualizar'}</button>
          </div>

          <div className="panel" style={{ padding: 0, overflow: 'hidden' }}>
            <div className="panel-head">{capMode === 'forge' ? 'ROOT · ADR-0007' : 'ROOT · ausente (standalone)'}</div>
            {forgeLoading && <div className="empty" style={{ padding: 'var(--sp-5)' }}><p>Consultando RootInfo()…</p></div>}
            {forgeErr && <div className="error-box" style={{ margin: 'var(--sp-3)' }}>{forgeErr}</div>}
            {!forgeLoading && !forgeErr && isRoot && (
              <div>
                {kv('ROOT DIR', f.root_dir || '—')}
                {kv('MODO', 'FORGE · kernel presente')}
                {kv('ADRs', f.adr_count ?? adrs.length ?? '—')}
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, padding: '8px 12px', borderBottom: '1px solid var(--border)' }}>
                  <span className={`badge ${f.has_embed ? 'ok' : 'neutral'}`}><span className="dot" /> {f.has_embed ? 'embed' : 'embed · ausente'}</span>
                  <span className={`badge ${f.has_dna ? 'ok' : 'neutral'}`}><span className="dot" /> {f.has_dna ? 'dna' : 'dna · ausente'}</span>
                  <span className={`badge ${f.has_opencode_cosca ? 'ok' : 'neutral'}`}><span className="dot" /> {f.has_opencode_cosca ? '.opencode/cosca' : '.opencode/cosca · ausente'}</span>
                  <span className={`badge ${cap.kernel ? 'ok' : 'neutral'}`}><span className="dot" /> kernel {cap.kernel ? 'presente' : 'ausente'}</span>
                  <span className={`badge ${cap.family_memory ? 'ok' : 'neutral'}`}><span className="dot" /> memória da família {cap.family_memory ? 'presente' : 'ausente'}</span>
                </div>
                <div className="panel-head">agents · skills · adrs</div>
                <div style={{ padding: '8px 12px', display: 'flex', flexDirection: 'column', gap: 4, borderBottom: '1px solid var(--border)' }}>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}><span className="mono" style={{ color: 'var(--text-3)', width: 14 }}><Icon name="chevron-right" size={10} /></span><b style={{ color: 'var(--text-2)', fontWeight: 600 }}>agents</b><span style={{ color: 'var(--text-3)' }}>× {agentsN}</span></div>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}><span className="mono" style={{ color: 'var(--text-3)', width: 14 }}><Icon name="chevron-right" size={10} /></span><b style={{ color: 'var(--text-2)', fontWeight: 600 }}>skills</b><span style={{ color: 'var(--text-3)' }}>× {skillsN}</span></div>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}><span className="mono" style={{ color: 'var(--text-3)', width: 14 }}><Icon name="chevron-right" size={10} /></span><b style={{ color: 'var(--text-2)', fontWeight: 600 }}>adrs</b><span style={{ color: 'var(--text-3)' }}>× {adrs.length}</span></div>
                </div>
                {adrs.length > 0 && (
                  <div>
                    <div className="panel-head">ADRs</div>
                    <div style={{ display: 'flex', flexDirection: 'column', padding: '4px 12px' }}>
                      {adrs.map((a, i) => (
                        <div key={i} className="mono small" style={{ padding: '2px 0', color: 'var(--text-2)' }}>{a}</div>
                      ))}
                    </div>
                  </div>
                )}
                {f.paths && (
                  <div>
                    <div className="panel-head">Paths</div>
                    <div style={{ padding: '6px 12px 12px' }}>{renderPaths(f.paths)}</div>
                  </div>
                )}
              </div>
            )}
            {!forgeLoading && !forgeErr && !isRoot && (
              <div className="empty" style={{ padding: 'var(--sp-6) var(--sp-5)' }}>
                <div className="hero-mark"><Icon name="forge" size={18} /></div>
                <p>Sem root — modo STANDALONE (fora do diretório do Cosca).<br />
                  <span className="mono">{'{ root: false }'}</span> · nenhum kernel, memória da família, agents ou skills do root são revelados.</p>
              </div>
            )}
          </div>
        </main>
        {statusbar()}
        {paletteEl}
        {uiAuditEl}
      </div>
    )
  }

  // SETTINGS — CENTRAL DE CONFIGURAÇÕES (USER PREFERENCE + READ-ONLY real)
  if (view === 'settings') {
    return (
      <div className={`shell ${dark ? 'dark' : 'light'}`}>
        {chrome()}
        <Settings
          dark={dark}
          setTheme={setTheme}
          layout={layout}
          onResetLayout={reset}
          projectName={project?.name}
          projectRoot={project?.root}
          cap={cap}
          capMode={capMode}
          rootInfo={forgeInfo}
        />
        {statusbar()}
        {paletteEl}
        {uiAuditEl}
      </div>
    )
  }

  // CHAT — a TELA PRINCIPAL (FASE 3). Conversa = tela principal; o workbench
  // (AGENT/FILES/…) permanece acessível no rail. A UI só consome o provider
  // (ChatStream + cosca:chat:event) — nenhuma lógica cognitiva aqui.
  if (view === 'chat') {
    return (
      <div className={`shell ${dark ? 'dark' : 'light'}`}>
        {chrome(project?.name)}
        <div className="ws">
          {railEl}
          <ChatView daemonOn={cap.daemon} projectName={project?.name} />
        </div>
        {statusbar()}
        {paletteEl}
        {uiAuditEl}
      </div>
    )
  }

  // WORKSPACE — ENGINEERING WORKBENCH
  const lastOut = activity.length ? (activity[activity.length - 1].body || '') : ''
  // FASE B: dados reais do plano (PlanDetails) disponíveis para o Approval Center.
  const pd = planDetails
  const filesArr = Array.isArray(pd?.files) ? pd.files : []
  // Estados que TÊM canal de evento real — o runtime os materializa.
  const runtimeStates = (Object.keys(AGENT_STATES) as AgentState[])
    .filter(k => AGENT_STATES[k].available)
    .map(k => AGENT_STATES[k].label.toLowerCase())

  // FASE C (Diff): working tree — fonte única ProjectDiff()/DiffSnapshot().
  // None disto é inventado: itens/summary/branch vêm do backend, e a UI apenas
  // decide qual estado honesto apresentar (sem projeto / não-git / limpo / mudanças).
  // useMemo: estabiliza as referências (senão `diffItems` é array nova a cada render
  // e hooks com dep `[diffItems]` re-disparam → loop de render, React #310).
  // Correção #310: estes NÃO são hooks (useMemo/useRef/useCallback) porque ficam
  // APÓS os `return` condicionais de home/create/forge — hooks condicionais quebram
  // as Rules of Hooks e causam "Rendered more hooks than previous render" (#310).
  // Valores puros (sem hook) — a estabilidade é irrelevante aqui (não são deps de
  // effect); o `diffData` já é estado e muda só via setDiffData.
  const diffItems = (diffData?.items ?? []) as DiffItem[]
  const diffSummary = diffData?.summary ?? {}
  const diffSel = diffSelected != null ? diffItems[diffSelected] : undefined

  // Navegação por teclado (↑↓ Home/End) na lista de arquivos alterados.
  const onDiffKey = (e: React.KeyboardEvent<HTMLDivElement>) => {
    const n = diffItems.length
    if (n === 0) return
    if (e.key === 'ArrowDown') { e.preventDefault(); setDiffSelected(s => (s == null ? 0 : Math.min(s + 1, n - 1))) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setDiffSelected(s => (s == null ? 0 : Math.max(s - 1, 0))) }
    else if (e.key === 'Home') { e.preventDefault(); setDiffSelected(0) }
    else if (e.key === 'End') { e.preventDefault(); setDiffSelected(n - 1) }
  }

  // Corpo honesto do pane Diff, decidido pelos estados reais do backend.
  const diffBodyEl = (() => {
    if (!project) return <div className="empty"><p>Sem projeto</p></div>
    if (diffErr) return <div className="error-box" style={{ margin: 'var(--sp-3)' }}>{diffErr}</div>
    if (diffLoading && !diffData) return <div className="empty"><p>Consultando ProjectDiff()…</p></div>
    if (!diffData) return <div className="empty"><p>Pressione Refresh para consultar o working tree.</p></div>
    if (diffData.git === false) return (
      <div className="empty">
        <p>não é repositório git</p>
        {diffData.reason && <pre className="mono small" style={{ margin: 0 }}>{diffData.reason}</pre>}
      </div>
    )
    if (!diffItems.length) return <div className="empty"><p>working tree limpo</p></div>
    return (
      <>
        <div className="diff-summary">
          <span><b>{diffSummary.files ?? diffItems.length}</b> files</span>
          <span className="add"><b>{diffSummary.insertions ?? 0}</b> +</span>
          <span className="del"><b>{diffSummary.deletions ?? 0}</b> −</span>
          {diffData.branch && <span className="mono" style={{ marginLeft: 'auto' }}>@{diffData.branch}</span>}
        </div>
        <div className="diff-list" tabIndex={0} role="listbox" aria-label="arquivos alterados no working tree"
          onKeyDown={onDiffKey}>
          {diffItems.map((it, i) => {
            const tone = diffTone(it.status)
            const ad = typeof it.additions === 'number' ? it.additions : 0
            const dd = typeof it.deletions === 'number' ? it.deletions : 0
            return (
              <button key={i} type="button" className={`diff-item${diffSelected === i ? ' active' : ''}`}
                role="option" aria-selected={diffSelected === i}
                aria-label={`${diffName(it.status)}: ${it.path}`}
                onClick={() => setDiffSelected(i)}>
                <span className={`diff-glyph ${tone}`}>{diffGlyph(it.status)}</span>
                <span className="diff-path">{it.path}{it.old_path && it.old_path !== it.path ? <span className="diff-rename"> ← {it.old_path}</span> : null}</span>
                <span className="diff-stat">{ad > 0 || dd > 0 ? <><span className="add">+{ad}</span><span className="del">−{dd}</span></> : null}</span>
              </button>
            )
          })}
        </div>
        {diffSel && (
          <div className="diff-patch">
            <div className="panel-head">{diffGlyph(diffSel.status)} {diffName(diffSel.status)} · <span className="mono">{diffSel.path}</span></div>
            {diffSel.patch ? (
              <pre className="mono small">{renderPatch(diffSel.patch)}</pre>
            ) : (
              <div className="empty" style={{ padding: 'var(--sp-3)' }}><p>(sem patch — arquivo binário/tool-result)</p></div>
            )}
          </div>
        )}
      </>
    )
  })()

  const terminalBodyEl = (() => {
    if (!project) return <div className="empty"><p>Terminal requer projeto ativo</p></div>
    return (
      <>
        <div className="term-scroll" ref={termScrolled} tabIndex={0} role="log" aria-label="saída do terminal">
          {terminalHist.length === 0 && (
            <div className="term-empty">— terminal autorizado · comandos rodam no projeto, com filtro de perigo + aprovação —</div>
          )}
          {terminalHist.map((t, i) => (
            <div key={i} className="term-line">
              <div className="term-prompt">❯ {t.cmd}</div>
              <pre className="term-out" style={{ color: toneColor(t.tone) }}>{t.out}</pre>
            </div>
          ))}
        </div>
        <div className="term-input-row">
          <span className="term-prompt">❯</span>
          <input
            ref={termInputRef}
            className="input mono term-input"
            value={termInput}
            placeholder={termBusy ? 'executando…' : 'digite um comando (roda no projeto)'}
            disabled={termBusy}
            onChange={e => setTermInput(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); doRunCmd('normal') } }}
            aria-label="comando do terminal"
          />
          <button className="btn btn--sm" onClick={() => doRunCmd('normal')} disabled={termBusy || !termInput.trim()}>Run</button>
          <button className="btn btn--sm ghost" onClick={() => doRunCmd('authorized')} disabled={termBusy || !termInput.trim()} title="Exige aprovação (execpolicy) para sensível/perigoso">Autorizado</button>
        </div>
      </>
    )
  })()

  // =========================================================================
  // PROJECT INTELLIGENCE (pane observável — "why detected" por evidência)
  // Perfil REAL de `AnalyzeProject()`. Nada aqui é inventado: cada tecnologia
  // renderiza as evidências `[{kind,value,file}]` que o detector registrou.
  // =========================================================================
  const profile = intel
  // AUTO-CONFIG do workbench, REFLETINDO o contexto mais completo sem duplicar
  // fontes: `agentCtx` (Agente — campos diretos git/container/ci) + perfil PI.
  // Nunca afirma capacidade sem evidência; `branch` continua `—` (só o Diff sabe).
  const ctxGit = agentCtx?.git === true
  const ctxContainer = typeof agentCtx?.container === 'string' && agentCtx.container ? agentCtx.container : ''
  const ctxCi = typeof agentCtx?.ci === 'string' && agentCtx.ci ? agentCtx.ci : ''
  const gitDetected = ctxGit || !!(profile && intelHas(profile, 'repository', 'git'))
  const dockerDetected = !!ctxContainer || !!(profile && (intelHas(profile, 'container', 'docker') || intelHas(profile, 'infrastructure', 'docker')))
  const ciDetected = !!ctxCi || !!(profile && intelHas(profile, 'ci', 'ci'))

  // Uma tecnologia (nome + confiança + evidência). Sem categoria vazia inventada.
  const renderIntelTech = (t: any): React.ReactNode => {
    const tech = asTech(t)
    if (!tech) return null
    const c = confGlyph(tech.confidence || '')
    const evs = asArr(tech.evidence).filter((e: any) => e && (e.value || e.file))
    return (
      <div key={String(tech.name) + '|' + String(tech.confidence || '')} className="tech">
        <div className="tech-row">
          <span className="tech-name">{tech.name}</span>
          <span className={`badge ${c.tone}`} title={`confiança: ${tech.confidence || 'unknown'}`}>
            <span className="conf-glyph" aria-hidden="true">{c.glyph}</span>{tech.confidence || 'unknown'}
          </span>
        </div>
        {evs.length > 0 && (
          <div className="tech-ev" aria-label={`evidência para ${tech.name}`}>
            {evs.map((e: any, i: number) => (
              <div key={i} className="ev">
                <span className="ev-glyph" aria-hidden="true">{evGlyph(e?.kind)}</span>
                <span className="ev-val mono">{String(e?.value ?? '—')}</span>
                <span className="ev-meta">{e?.kind}{e?.file ? ` (${e.file})` : ''}</span>
              </div>
            ))}
          </div>
        )}
        {tech.notes && <div className="tech-note">{tech.notes}</div>}
      </div>
    )
  }

  // Uma categoria plana (panes planares) — só aparece se o backend trouxe itens.
  const renderCategory = (label: string, key: string): React.ReactNode => {
    const items = asArr(profile?.[key]).filter((t: any) => asTech(t))
    if (!items.length) return null
    return (
      <section key={key} className="intel-cat">
        <div className="panel-head">{label} <span className="mono">{items.length}</span></div>
        <div className="cat-body">{items.map(renderIntelTech)}</div>
      </section>
    )
  }

  // Categorias planares (planares, densas) na ordem do contrato do backend.
  const INTEL_CATS: [string, string][] = [
    ['Linguagens', 'languages'],
    ['Frameworks', 'frameworks'],
    ['Package managers', 'package_managers'],
    ['Build', 'build_tools'],
    ['Test', 'test_tools'],
    ['Format', 'formatters'],
    ['Lint', 'linters'],
    ['Typecheck', 'type_checkers'],
    ['Runtime', 'runtime'],
    ['Container', 'container'],
    ['Infra', 'infrastructure'],
    ['Database', 'database'],
    ['Monorepo', 'monorepo'],
    ['CI', 'ci'],
    ['Documentation', 'documentation'],
    ['Repository / Git', 'repository'],
  ]

  // Resumo do topo: "TypeScript · React · Vite · npm" (fontes reais, em ordem).
  const topStack = [
    ...asArr(profile?.languages).map((t: any) => t?.name),
    ...asArr(profile?.frameworks).map((t: any) => t?.name),
    ...asArr(profile?.package_managers).map((t: any) => t?.name),
    ...asArr(profile?.build_tools).map((t: any) => t?.name),
  ].filter((x: any) => !!x).slice(0, 5)

  // Capabilities reais do perfil (ex.: {lint:'eslint', test:'vitest', docker:true}).
  const caps = profile?.capabilities && typeof profile.capabilities === 'object' ? profile.capabilities : {}
  const capEntries = Object.entries(caps).filter(([, v]) => v !== false && v != null)
  const capText = (v: any): string => (v === true ? 'YES' : v === false ? 'NO' : String(v))

  // Comandos detectados: `name → cmd (source)` — densos, mono.
  const intelCmds = asArr(profile?.commands).filter((c: any) => c && (c.name || c.cmd))
  // Config files reais — lista mono.
  const intelCfg = asArr(profile?.config_files)

  const intelBodyEl = (() => {
    if (!project) return <div className="empty"><p>Intelligence requer projeto ativo</p></div>
    if (intelLoading && !intel) return <div className="empty"><p>Analisando projeto…</p></div>
    if (intelErr) return <div className="error-box" style={{ margin: 'var(--sp-3)' }}>{intelErr}</div>
    if (!intel) return <div className="empty"><p>Sem perfil detectado — o PI não reportou o projeto.</p></div>
    const p = intel
    const projTypes = asArr(p.project_types).filter((x: any) => !!x)
    const cats = INTEL_CATS.map(([label, k]) => renderCategory(label, k))
    const hasCats = cats.some((x: any) => !!x)
    return (
      <div className="intel-scroll">
        {/* HERO DO PROJETO */}
        <div className="intel-hero">
          <div className="intel-title">{p.name || project?.name || '—'}</div>
          <div className="intel-meta">
            {projTypes.length > 0 && (
              <span className="badges">
                {projTypes.map((t: any, i: number) => <span key={i} className="badge info">{String(t)}</span>)}
              </span>
            )}
            <span className="badge neutral mono">detected at <b>{p.detected_at || '—'}</b></span>
          </div>
          {topStack.length > 0 && (
            <div className="intel-stack mono" title="stack principal do projeto">{topStack.join(' · ')}</div>
          )}
          <div className="intel-why" role="note">
            <span className="badge neutral mono">WHY DETECTED</span>
            <span className="intel-why-text">cada tecnologia mostra a evidência real do detector.</span>
          </div>
          {capEntries.length > 0 && (
            <div className="intel-caps">
              {capEntries.map(([k, v]) => (
                <span key={k} className={`badge ${v === true ? 'ok' : 'neutral'} mono`}>{k}:{capText(v)}</span>
              ))}
            </div>
          )}
        </div>

        {/* Categorias por evidência */}
        {hasCats && <div className="intel-cats">{cats}</div>}

        {/* Comandos detectados */}
        {intelCmds.length > 0 && (
          <section className="intel-cat">
            <div className="panel-head">Commands <span className="mono">{intelCmds.length}</span></div>
            <div className="cat-body">
              {intelCmds.map((c: any, i: number) => (
                <div key={i} className="cmd">
                  <span className="cmd-name">{c.name || 'cmd'}</span>
                  <span className="cmd-cmd mono" title={c.source || ''}>{c.cmd || '—'}</span>
                  {c.source && <span className="cmd-src">({c.source})</span>}
                </div>
              ))}
            </div>
          </section>
        )}

        {/* Config files */}
        {intelCfg.length > 0 && (
          <section className="intel-cat">
            <div className="panel-head">Config files <span className="mono">{intelCfg.length}</span></div>
            <div className="cat-body">
              {intelCfg.map((f: any, i: number) => (
                <div key={i} className="cfg mono">{typeof f === 'string' ? f : JSON.stringify(f)}</div>
              ))}
            </div>
          </section>
        )}

        {!hasCats && intelCmds.length === 0 && intelCfg.length === 0 && (
          <div className="empty" style={{ padding: 'var(--sp-4)' }}><p>Perfil carregado, mas sem categorias/comandos/config para exibir.</p></div>
        )}
      </div>
    )
  })()

  // =========================================================================
  // AGENT PROJECT CONTEXT (painel do agente — o agente JÁ sabe a stack/pipeline).
  // Fonte única: AgentProjectContext(). A UI só apresenta o que veio; sem
  // skills/pipeline ou has_project:false → mostra honestamente (não inventa).
  // =========================================================================
  const ctx = agentCtx
  const ctxSkills = asArr(ctx?.skills).filter((s: any) => !!s).map((s: any) => String(s))
  const ctxPipeline = asArr(ctx?.pipeline)
    .filter((s: any) => s && (s.step !== undefined || s.tool !== undefined))
    .map((s: any): AgentPipelineStep => ({
      step: String(s?.step ?? ''),
      tool: String(s?.tool ?? ''),
      status: String(s?.status ?? ''),
    }))
  const ctxTypes = asArr(ctx?.type).filter((x: any) => !!x).map(String)
  // Indicador NÃO-cor por status do pipeline — glyph (✓/·/!) + tom semântico.
  const ctxGlyph = (status?: string): { glyph: string; tone: 'ok' | 'neutral' | 'warn' } => {
    switch ((status || '').toLowerCase()) {
      case 'available': return { glyph: '✓', tone: 'ok' }
      case 'not_applicable': return { glyph: '·', tone: 'neutral' }
      case 'unknown': return { glyph: '!', tone: 'warn' }
      default: return { glyph: '·', tone: 'neutral' }
    }
  }

  const ctxBlockEl = (() => {
    const has = !!ctx && ctx.has_project !== false && !agentCtxErr
    return (
      <div className="ctx-block" aria-label="contexto do projeto carregado pelo agente">
        <div className="panel-head" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span>Project context · o agente já sabe</span>
          <button className="btn btn--sm" onClick={refreshAllIntel} disabled={intelRefreshing || agentCtxLoading} title="Invalida o cache e re-analisa o projeto (fresca)">{intelRefreshing ? '…' : 'Refresh'}</button>
        </div>
        {agentCtxLoading && !ctx && <div className="ctx-note">Carregando contexto do projeto…</div>}
        {agentCtxErr && !ctx && <div className="ctx-note">{agentCtxErr}</div>}
        {!agentCtxLoading && !ctx && !agentCtxErr && <div className="ctx-note">Sem contexto — o agente perguntará as convenções.</div>}
        {has && (
          <>
            {ctx.stack
              ? <div className="ctx-stack mono" title="stack auto-detectada">{ctx.stack}</div>
              : <div className="ctx-note">Sem stack detectada.</div>}
            {(ctxTypes.length > 0 || ctx.package_manager || ctxGit || ctxContainer || ctxCi) && (
              <div className="ctx-meta">
                {ctxTypes.map((t: string, i: number) => <span key={i} className="badge info mono">{t}</span>)}
                {ctx.package_manager && <span className="badge neutral mono">pkg <b>{ctx.package_manager}</b></span>}
                {ctxGit && <span className="badge ok mono" title="git detectado">git ✓</span>}
                {ctxContainer && <span className="badge ok mono" title="container detectado">container <b>{ctxContainer}</b></span>}
                {ctxCi && <span className="badge ok mono" title="CI detectado">ci <b>{ctxCi}</b></span>}
              </div>
            )}
            {ctxSkills.length > 0 ? (
              <div className="ctx-skills">
                <div className="ph" aria-hidden="true">Skills match</div>
                <div className="skill-chips">
                  {ctxSkills.map((s: string, i: number) => (
                    <span key={i} className="skill-chip" title={`skill casada: ${s}`}>{s}</span>
                  ))}
                </div>
              </div>
            ) : (
              <div className="ctx-note">Nenhuma skill casada com o perfil.</div>
            )}
            {ctxPipeline.length > 0 ? (
              <div className="pipe">
                <div className="ph" aria-hidden="true">Pipeline auto-descoberto</div>
                {ctxPipeline.map((p: AgentPipelineStep, i: number) => {
                  const g = ctxGlyph(p.status)
                  const na = /not_applicable/i.test(p.status || '') || /^NOT_APPLICABLE$/i.test(p.tool || '')
                  return (
                    <div key={i} className="pipe-step">
                      <span className="pipe-num" aria-hidden="true">{i + 1}</span>
                      <span className={`pipe-glyph ${g.tone}`} aria-hidden="true">{g.glyph}</span>
                      <span className="pipe-name">{p.step || '—'}</span>
                      {na
                        ? <span className="badge neutral mono" title="etapa não aplicável a este projeto">N/A</span>
                        : <span className="pipe-tool mono">{p.tool || '—'}</span>}
                    </div>
                  )
                })}
              </div>
            ) : (
              <div className="ctx-note">Sem pipeline auto-descoberto.</div>
            )}
          </>
        )}
      </div>
    )
  })()

  return (
    <div className={`shell ${dark ? 'dark' : 'light'}`}>
      {chrome(project?.name)}
      {/* inline workspace status (branch / runtime / agent), not cards */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 14px', borderBottom: '1px solid var(--border)', background: 'var(--surface)', fontSize: 'var(--fs-xs)' }}>
        <span className={`badge ${cap.daemon ? 'ok' : 'neutral'}`}><span className="dot" /> runtime <b>{cap.daemon ? 'on' : 'off'}</b></span>
        <span className={`badge ${capMode === 'forge' ? 'info' : 'neutral'}`}>mode <b>{capMode === null ? '…' : capMode === 'forge' ? 'FORGE' : 'STANDALONE'}</b></span>
        <span className="badge">branch <b>—</b></span>
        <span className={`badge ${gitDetected ? 'ok' : 'neutral'}`} title={gitDetected ? 'git detectado (Agente/PI)' : 'git não detectado (Agente/PI)'}>git <b>{gitDetected ? '✓' : '—'}</b></span>
        <span className={`badge ${agentRunning ? 'warn' : 'ok'}`}>agent <b>{agentRunning ? 'running' : 'idle'}</b></span>
        <span className={`badge ${project?.initialized ? 'ok' : 'warn'}`}>sandbox <b>{project?.initialized ? 'isolated' : 'not init'}</b></span>
        {dockerDetected ? <span className="badge ok" title="container detectado (Agente/PI)">container <b>{ctxContainer || 'docker'}</b></span> : null}
        {ciDetected ? <span className="badge ok" title="CI detectado (Agente/PI)">ci <b>{ctxCi || '✓'}</b></span> : null}
        <span className="spacer" />
        <span className="mono" style={{ color: 'var(--text-3)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 420 }}>{project?.root}</span>
      </div>

      <div className="ws">
        {railEl}

        {/* PROJECT TREE */}
        {isCollapsed('tree') ? (
          <div className="collapse-strip vertical" title="Restore files tree">
            <button className="collapse-btn" title="Expand files tree" aria-label="Expand files tree" onClick={() => toggleCollapse('tree')}><Icon name="chevron-right" size={14} /></button>
          </div>
        ) : (
          <>
            <aside className="tree" style={{ width: layout.widths.tree, minWidth: 0 }}>
              <div className="tree-head" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <span>Files</span>
                <button className="collapse-btn" title="Collapse files tree" aria-label="Collapse files tree" onClick={() => toggleCollapse('tree')}><Icon name="chevron-left" size={14} /></button>
              </div>
              <div className="tree-body" role="tree" tabIndex={0} onKeyDown={onTreeKey} aria-label="Arquivos do projeto" style={{ padding: '0 4px' }}>{treeRoot.map(n => renderNode(n, 0))}</div>
            </aside>
            <Splitter
              orientation="vertical"
              onResize={dx => setWidth('tree', layout.widths.tree + dx)}
              value={layout.widths.tree}
              min={BOUNDS.tree.min}
              max={BOUNDS.tree.max}
              label="Resize files tree"
            />
          </>
        )}

        <section className="workspace">
          {/* AGENT WORKBENCH (observable process, not chat bubbles) */}
          {isCollapsed('agent') ? (
            <div className="collapse-strip vertical" title="Restore agent workbench">
              <button className="collapse-btn" title="Expand agent workbench" aria-label="Expand agent workbench" onClick={() => toggleCollapse('agent')}><Icon name="chevron-right" size={14} /></button>
            </div>
          ) : (
            <>
            <div className="agent-col" style={{ width: layout.widths.agent, minWidth: 0 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 12px', borderBottom: '1px solid var(--border)', background: 'var(--surface-2)' }}>
              <span className={`badge ${agentRunning ? 'warn' : 'ok'}`}><span className="dot" /> AGENT {agentRunning ? 'RUNNING' : 'IDLE'}</span>
              <span className="badge">model {AGENT_MODEL}</span>
              {project?.initialized ? <span className="badge ok">sandbox</span> : <span className="badge warn">no sandbox</span>}
            </div>

            {/* REQUEST */}
            <div style={{ borderBottom: '1px solid var(--border)' }}>
              <div className="panel-head">Request</div>
              <div className="agent-input">
                <textarea className="input" value={agentReq} placeholder="Peça ao agente: crie X, edite Y, execute os testes…"
                  onChange={e => setAgentReq(e.target.value)}
                  onKeyDown={e => { if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') { e.preventDefault(); doRun() } }} />
                <div className="row" style={{ marginTop: 0 }}>
                  <button className="btn" onClick={doPlan} disabled={agentRunning || !agentReq.trim()}>Plan</button>
                  <button className="btn primary" onClick={doRun} disabled={agentRunning || !agentReq.trim()}>Executar</button>
                  <span className="hint">plan = revisar · executar = aplicar</span>
                </div>
              </div>
            </div>

            {/* PROJECT CONTEXT (FASE 3) — o agente já sabe a stack/pipeline */}
            {ctxBlockEl}

            {/* PLAN */}
            {plan && (
              <div className="plan-panel">
                <div className="panel-head">Plano (revisar antes de executar)</div>
                <pre className="mono small">{plan.slice(0, 900)}</pre>
                <div className="row">
                  <button className="btn" onClick={() => { setPlan(''); setPlanDetails(null) }}>Descartar</button>
                  <button className="btn primary" onClick={doApprovePlan}>Aprovar e executar</button>
                </div>
              </div>
            )}

            {/* APPROVAL CENTER (decision, not a confirm card) */}
            {approval && (
              <div style={{ borderBottom: '1px solid var(--border)', background: 'color-mix(in srgb, var(--danger) 4%, var(--surface))' }}>
                <div className="panel-head" style={{ color: 'var(--danger)' }}>Approval Center · SECURITY</div>
                <div style={{ padding: '8px 12px', display: 'flex', alignItems: 'center', gap: 8, background: 'color-mix(in srgb, var(--danger) 14%, var(--surface-2))', borderBottom: '1px solid var(--border)' }}>
                  <span className="badge danger" style={{ color: 'var(--danger)' }}><span className="dot" /> {approval.status.toUpperCase()}</span>
                  <span style={{ fontSize: 'var(--fs-xs)', color: 'var(--danger)', fontWeight: 600, letterSpacing: '.4px' }}>UNAPPROVED ACTION AWAITING DECISION</span>
                </div>
                <div style={{ padding: 4 }}>
                  {kv('ACTION', planVal(pd, 'action', 'cosca exec'), 'danger')}
                  {kv('COMMAND', planVal(pd, 'command', approval.request), 'danger')}
                  {kv('PROJECT', project?.name || '—')}
                  {kv('WORKING DIR', planVal(pd, 'cwd', project?.root || '—'))}
                  {kv('SCOPE', planVal(pd, 'scope', '—'))}
                  {kv('FILES', planCount(pd, 'files'))}
                  {kv('POLICY', planVal(pd, 'policy', 'fail-closed · execpolicy'))}
                  {kv('RISK', planVal(pd, 'risk', approval.risk), riskTone(planVal(pd, 'risk', approval.risk)))}
                  {kv('SANDBOX', project?.initialized ? 'isolated (no bwrap)' : 'not init')}
                  {kv('AGENT', approval.model)}
                  {kv('SESSION', 'single-use · consumed on run')}
                </div>
                {/* FASE B: decisão informada — dados reais do plano (quando gerado). */}
                {pd && (
                  <div style={{ padding: '0 12px 8px' }}>
                    <div style={{ fontSize: 'var(--fs-xs)', color: 'var(--text-3)', fontWeight: 600, letterSpacing: '.5px', margin: '2px 0 6px' }}>
                      SECURITY INTELLIGENCE · dados reais do plano
                    </div>
                    <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 6 }}>
                      <span className="badge neutral">packages <b>{planCount(pd, 'packages')}</b></span>
                      <span className="badge neutral">tests expected <b>{planVal(pd, 'tests_expected', '—')}</b></span>
                      <span className={`badge ${Number(planVal(pd, 'confidence_percent', '0')) >= 75 ? 'ok' : 'info'}`}>confidence <b>{planVal(pd, 'confidence_percent', '—')}</b></span>
                    </div>
                    {filesArr.length > 0 && (
                      <div className="mono small" style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', borderRadius: 'var(--r-sm)', padding: '4px 8px', maxHeight: 96, overflow: 'auto' }}>
                        <div style={{ color: 'var(--text-3)', fontWeight: 600, letterSpacing: '.4px' }}>FILES AFFECTED ({filesArr.length})</div>
                        {filesArr.slice(0, 8).map((f, i) => (
                          <div key={i} style={{ padding: '1px 0', color: 'var(--text-2)', wordBreak: 'break-all' }}>{f}</div>
                        ))}
                        {filesArr.length > 8 && <div style={{ padding: '1px 0', color: 'var(--text-3)' }}>+ {filesArr.length - 8} more…</div>}
                      </div>
                    )}
                  </div>
                )}
                <div className="row" style={{ padding: '10px 12px', margin: 0 }}>
                  <button className="btn danger" onClick={denyNow} disabled={approving}>DENY</button>
                  <button className="btn primary" onClick={() => approveNow('once')} disabled={approving}>APPROVE ONCE</button>
                  <button className="btn" onClick={() => approveNow('session')} disabled={approving} style={{ borderColor: 'var(--accent)', color: 'var(--accent)' }}>APPROVE SESSION</button>
                </div>
              </div>
            )}

            {/* OTHER WORKBENCH PANES — honest placeholders */}
            <div style={{ borderBottom: '1px solid var(--border)' }}>
              <div className="panel-head">Tools · Files · Commands · Tests · Result</div>
              <div style={{ padding: '8px 12px', fontSize: 'var(--fs-xs)', color: 'var(--text-3)' }}>
                not available — sem serviço de backend no Desktop
              </div>
            </div>

            {/* ACTIVITY (timeline de eventos REAIS do runtime + ações locais) */}
            <div style={{ flex: 1, overflow: 'auto', minHeight: 0, display: 'flex', flexDirection: 'column' }}>
              <div className="panel-head">Activity · eventos reais</div>
              <div className="activity">
                {activity.length === 0 && <div className="empty"><p>Aguardando eventos do runtime…</p></div>}
                {activity.map((a, i) => {
                  const st = a.state ? AGENT_STATES[a.state] : undefined
                  const isEvent = !!a.kind && a.kind !== 'local'
                  return (
                    <div key={i} className="activity-item">
                      <div className="at">{a.at}</div>
                      <div className="body">
                        <div className="title" style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                          {st && <span className={`status-dot ${st.available ? 'info' : 'neutral'}${isEvent ? ' pulse' : ''}`} />}
                          <span>{a.title}</span>
                          {isEvent && <span className="badge neutral mono">{a.kind}</span>}
                          {a.seq != null && <span className="badge mono">seq #{a.seq}</span>}
                        </div>
                        {a.body && <pre className="mono small">{a.body.slice(0, 240)}</pre>}
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>

            {/* LEGENDA de estados do agente (runtime fornecidos vs NOT AVAILABLE) */}
            <div style={{ borderTop: '1px solid var(--border)' }}>
              <div className="panel-head">Agent states</div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 2, padding: '6px 12px 10px' }}>
                <div style={{ fontSize: 'var(--fs-xs)', color: 'var(--text-3)', marginBottom: 4 }}>
                  Emitidos pelo runtime: <span className="mono">{runtimeStates.join(' · ')}</span>. Demais estados → <b>NOT AVAILABLE</b> (nenhum evento os materializa).
                </div>
                {(Object.keys(AGENT_STATES) as AgentState[]).map(k => {
                  const st = AGENT_STATES[k]
                  return (
                    <div key={k} style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 'var(--fs-xs)' }}>
                      <span className={`status-dot ${st.available ? 'info' : 'neutral'}`} />
                      <span className="mono" style={{ width: 14, color: st.available ? 'var(--text)' : 'var(--text-3)' }}>{st.glyph}</span>
                      <span style={{ flex: 1, color: 'var(--text-2)' }}>{st.label}</span>
                      <span className={`badge ${st.available ? 'ok' : 'neutral'}`}>{st.available ? `via ${st.from}` : 'NOT AVAILABLE'}</span>
                    </div>
                  )
                })}
              </div>
            </div>

            {err && <div className="error-box" style={{ margin: 'var(--sp-3)' }}>{err}</div>}
            </div>
            <Splitter
              orientation="vertical"
              onResize={dx => setWidth('agent', layout.widths.agent + dx)}
              value={layout.widths.agent}
              min={BOUNDS.agent.min}
              max={BOUNDS.agent.max}
              label="Resize agent workbench"
            />
            </>
          )}

          {/* ACTIVE WORKSPACE (editor) */}
          <div className="editor-col">
            <div className="fs-head">
              <span>{activeFile ? `${activeFile.name} — ${activeFile.path}` : 'Editor'}</span>
              {activeFile && fileView === 'edit' && <button className="btn btn--sm" onClick={save}>Salvar</button>}
              {activeFile && fileView === 'code' && <button className="btn btn--sm" onClick={() => setFileView('edit')}>Editar</button>}
              {activeFile && fileView === 'edit' && <button className="btn btn--sm" onClick={() => setFileView('code')}>Visualizar</button>}
              {project && <button className={`btn btn--sm${isCollapsed('diff') ? '' : ' active'}`} onClick={openDiffPanel} title="Abrir painel de diff do working tree" aria-label="Painel de Diff"><Icon name="diff" size={14} /> Diff</button>}
              {project && <button className={`btn btn--sm${intelToggled ? ' active' : ''}`} onClick={() => { setView('workspace'); loadIntel(); openIntelPanel() }} title="Abrir painel de Project Intelligence (perfil por evidência)" aria-label="Painel de Project Intelligence"><Icon name="intelligence" size={14} /> Intelligence</button>}
              {project && <button className={`btn btn--sm${isCollapsed('terminal') ? '' : ' active'}`} onClick={() => { setView('workspace'); openTerminalPanel() }} title="Abrir painel de terminal autorizado" aria-label="Painel de Terminal"><Icon name="terminal" size={14} /> Terminal</button>}
            </div>
            {fileErr ? (
              <div className="empty" data-state="error">
                <div className="hero-mark"><Icon name="file" size={18} /></div>
                <p>Unable to open file</p>
                <pre className="mono small">{fileErr}</pre>
                <div className="row" style={{ gap: 8 }}>
                  {activeFile && <button className="btn btn--sm" onClick={() => openFile(activeFile)}>Retry</button>}
                  {activeFile && <button className="btn btn--sm" onClick={() => { try { navigator.clipboard.writeText(activeFile.path) } catch {} }}>Copy path</button>}
                </div>
              </div>
            ) : activeFile ? (fileView === 'code'
              ? <CodeViewer content={fileContent} language={activeFile.language} fileName={activeFile.name}
                  size={activeFile.size} modifiedAt={activeFile.modified_at}
                  isBinary={isBinaryNode(activeFile)} />
              : <textarea className="code" value={fileContent} onChange={e => setFileContent(e.target.value)} spellCheck={false} />)
              : <div className="empty"><div className="hero-mark"><Icon name="file" size={18} /></div><p>Selecione um arquivo para abrir</p></div>}
          </div>

          {/* CONTEXT */}
          {isCollapsed('context') ? (
            <div className="collapse-strip vertical" title="Restore context panel">
              <button className="collapse-btn" title="Expand context panel" aria-label="Expand context panel" onClick={() => toggleCollapse('context')}><Icon name="chevron-right" size={14} /></button>
            </div>
          ) : (
            <>
            <Splitter
              orientation="vertical"
              onResize={dx => setWidth('context', layout.widths.context - dx)}
              value={layout.widths.context}
              min={BOUNDS.context.min}
              max={BOUNDS.context.max}
              label="Resize context panel"
            />
            <aside className="ctx" style={{ width: layout.widths.context, minWidth: 0 }}>
            <div className="panel-head" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span>Context</span>
              <button className="collapse-btn" title="Collapse context panel" aria-label="Collapse context panel" onClick={() => toggleCollapse('context')}><Icon name="chevron-left" size={14} /></button>
            </div>
            <div className="ctx-row"><span>Projeto</span><b>{project?.name}</b></div>
            <div className="ctx-row"><span>Raiz</span><b className="mono">{project?.root}</b></div>
            <div className="ctx-row"><span>cosca init</span><b>{project?.initialized ? '✓' : '✕'}</b></div>
            <div className="ctx-row"><span>Branch</span><b>—</b></div>
            <div className="ctx-row"><span>Network</span><b>—</b></div>
            {project && !project.initialized && <button className="btn primary" style={{ width: '100%', marginTop: 10 }} onClick={init}>Rodar cosca init</button>}
            {busy && <div style={{ marginTop: 12, padding: '0 12px' }}>{progressEl}</div>}
              </aside>
            </>
          )}

          {/* DIFF (working tree — FASE C) */}
          {isCollapsed('diff') ? (
            <div className="collapse-strip vertical" title="Restore diff panel">
              <button className="collapse-btn" title="Expand diff panel" aria-label="Expand diff panel" onClick={() => openDiffPanel()}><Icon name="chevron-right" size={14} /></button>
            </div>
          ) : (
            <>
            <Splitter
              orientation="vertical"
               onResize={dx => setWidth('diff', layout.widths.diff - dx)}
              value={layout.widths.diff}
              min={BOUNDS.diff.min}
              max={BOUNDS.diff.max}
              label="Resize diff panel"
            />
            <aside className="diff-col" style={{ width: layout.widths.diff, minWidth: 0 }}>
              <div className="diff-head">
                <span className="diff-title">Diff</span>
                <span className="diff-dir mono" title="diretório do working tree">{project?.root || '—'}</span>
                <button className="btn btn--sm" onClick={loadDiff} disabled={diffLoading}>{diffLoading ? '…' : 'Refresh'}</button>
                <button className="collapse-btn" title="Collapse diff panel" aria-label="Collapse diff panel" onClick={() => toggleCollapse('diff')}><Icon name="chevron-left" size={14} /></button>
              </div>
              <div className="diff-body">{diffBodyEl}</div>
            </aside>
            </>
          )}

          {/* INTELLIGENCE (perfil do projeto por evidência — PROJECT INTELLIGENCE) */}
          {isCollapsed('intelligence') ? (
            <div className="collapse-strip vertical" title="Restore intelligence panel">
              <button className="collapse-btn" title="Expand intelligence panel" aria-label="Expand intelligence panel" onClick={() => openIntelPanel()}><Icon name="chevron-right" size={14} /></button>
            </div>
          ) : (
            <>
            <Splitter
              orientation="vertical"
               onResize={dx => setWidth('intelligence', layout.widths.intelligence - dx)}
              value={layout.widths.intelligence}
              min={BOUNDS.intelligence.min}
              max={BOUNDS.intelligence.max}
              label="Resize intelligence panel"
            />
            <aside className="intel-col" style={{ width: layout.widths.intelligence, minWidth: 0 }}>
              <div className="intel-head">
                <span className="intel-title">Intelligence</span>
                {intelLoading ? <span className="badge neutral mono">analisando…</span> : null}
                {intel && <span className="badge ok mono" title="perfil carregado do backend">pi</span>}
                {intelErr ? <span className="badge warn mono">erro</span> : null}
                <span className="spacer" />
                <button className="btn btn--sm" onClick={refreshAllIntel} disabled={intelRefreshing || intelLoading} title="Invalida o cache e re-analisa o projeto (fresca)">{intelRefreshing ? '…' : 'Refresh'}</button>
                <button className="collapse-btn" title="Collapse intelligence panel" aria-label="Collapse intelligence panel" onClick={() => toggleCollapse('intelligence')}><Icon name="chevron-left" size={14} /></button>
              </div>
              <div className="intel-body">{intelBodyEl}</div>
            </aside>
            </>
          )}

          {/* TERMINAL (autorizado — FASE D) */}
          {isCollapsed('terminal') ? (
            <div className="collapse-strip vertical" title="Restore terminal panel">
              <button className="collapse-btn" title="Expand terminal panel" aria-label="Expand terminal panel" onClick={openTerminalPanel}><Icon name="chevron-right" size={14} /></button>
            </div>
          ) : (
            <>
            <Splitter
              orientation="vertical"
              onResize={dx => setWidth('terminal', layout.widths.terminal - dx)}
              value={layout.widths.terminal}
              min={BOUNDS.terminal.min}
              max={BOUNDS.terminal.max}
              label="Resize terminal panel"
            />
            <aside className="term-col" style={{ width: layout.widths.terminal, minWidth: 0 }}>
              <div className="term-head">
                <span className="term-title">Terminal</span>
                <span className="badge neutral mono">autorizado</span>
                <button className="collapse-btn" title="Collapse terminal panel" aria-label="Collapse terminal panel" onClick={() => toggleCollapse('terminal')}><Icon name="chevron-left" size={14} /></button>
              </div>
              {terminalBodyEl}
            </aside>
            </>
          )}
        </section>
      </div>

      {/* bottom strip: OUTPUT / TERMINAL (resize vertical via Splitter horizontal) */}
      {isCollapsed('output') ? (
        <div className="collapse-strip horizontal" title="Restore output panel">
          <button className="collapse-btn" title="Expand output panel" aria-label="Expand output panel" onClick={() => toggleCollapse('output')}><Icon name="chevron-up" size={14} /></button>
        </div>
      ) : (
        <>
        <Splitter
          orientation="horizontal"
          onResize={dx => setHeight(layout.height - dx)}
          value={layout.height}
          min={BOUNDS.output.min}
          max={BOUNDS.output.max}
          label="Resize output panel"
        />
        <div style={{ display: 'flex', borderTop: '1px solid var(--border)', background: 'var(--surface)', minHeight: 0, height: layout.height, flex: 'none', overflow: 'auto', fontSize: 'var(--fs-xs)', color: 'var(--text-2)' }}>
          <button className="collapse-btn" title="Collapse output panel" aria-label="Collapse output panel" onClick={() => toggleCollapse('output')} style={{ height: '100%', padding: '0 6px' }}><Icon name="chevron-down" size={14} /></button>
          <span style={{ padding: '6px 12px', borderRight: '1px solid var(--border)', fontWeight: 600, letterSpacing: '.5px', whiteSpace: 'nowrap' }}>OUTPUT</span>
          <span style={{ padding: '6px 12px', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{err || lastOut || 'Aguardando atividade…'}</span>
          <span style={{ padding: '6px 12px', borderLeft: '1px solid var(--border)', color: 'var(--text-3)', whiteSpace: 'nowrap' }}>TERMINAL — not available</span>
        </div>
        </>
      )}

      {statusbar()}
      {paletteEl}
    </div>
  )
}

// FILE EXPLORER — helpers puros (fora do componente, evitam render loops).
// `mergeChildren` substitui, recursivamente, os `children` do nó cujo `path`
// casa com o `dir`. Raiz (`dir === ''`) substitui o array inteiro de `treeRoot`.
function mergeChildren(nodes: main.FileNode[], dir: string, children: main.FileNode[]): main.FileNode[] {
  if (dir === '') return children
  return nodes.map(n => {
    if (n.path === dir) return main.FileNode.createFrom({ ...n, children, loaded: true })
    if (n.is_dir && n.children) return main.FileNode.createFrom({ ...n, children: mergeChildren(n.children, dir, children) })
    return n
  })
}

// `isBinaryNode` — um arquivo é "binário" (não deve passar pelo code viewer)
// quando o backend o classificou como imagem/vídeo/áudio/arquivo/pdf OU quando
// o conteúdo não é texto legível. O backend já resolve `language`; aqui só
// consolidamos a decisão de exibição.
const BINARY_LANGS = new Set(['image', 'video', 'audio', 'archive', 'pdf'])
function isBinaryNode(node: main.FileNode): boolean {
  return BINARY_LANGS.has(node.language)
}
