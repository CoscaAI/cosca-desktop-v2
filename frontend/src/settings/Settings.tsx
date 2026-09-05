/* =========================================================================
   COSCA DESKTOP — CENTRAL DE CONFIGURAÇÕES (Settings.tsx)
   Contrato §4: professional, sidebar + conteúdo. Apenas seções com config REAL
   (6: Appearance / Workspace / Keyboard / Runtime / Diagnostics / About).
   Nada aqui inventa configuração: só USER PREFERENCE (tema) é editável; todo o
   resto é READ-ONLY (layout do projeto, atalhos, runtime, build, about) e lê
   do que o App/backend realmente expõe.

   Governança (§5): tokens `var(--...)`, radius 4/6/8, denso, sem cor hardcoded,
   sem emoji. `Icon` (Lucide) reutilizado. `.btn`, `.badge`, `.panel`, `.input`
   reutilizados; não criamos segundo sistema.
   ========================================================================= */

import React, { useEffect, useMemo, useRef, useState } from 'react'
import { Icon, type IconName } from '../components/Icon'
import { Environment } from '../../wailsjs/runtime/runtime'
import type { LayoutState } from '../layout/layoutModel'

export type SectionId =
  | 'appearance' | 'workspace' | 'keyboard' | 'runtime' | 'diagnostics' | 'about'

type Section = {
  id: SectionId
  label: string
  desc: string
  icon: IconName
  keywords: string
}

// Apenas seções com config real (contrato §3 a §4). NÃO incluir AI/Privacy/
// Performance/Notifications (sem suporte real → NOT AVAILABLE, honesto).
const SECTIONS: Section[] = [
  { id: 'appearance',  label: 'Appearance',  desc: 'Tema claro/escuro · aplicação imediata',   icon: 'sun',      keywords: 'tema theme dark light aparência' },
  { id: 'workspace',   label: 'Workspace',   desc: 'Layout de panes persistido por projeto',    icon: 'project',  keywords: 'layout panes width collapse reset projeto' },
  { id: 'keyboard',    label: 'Keyboard',    desc: 'Atalhos de teclado (read-only)',            icon: 'command',  keywords: 'shortcut atalho teclado keyboard' },
  { id: 'runtime',     label: 'Runtime',     desc: 'Estado do runtime Cosca (read-only)',      icon: 'runtime',  keywords: 'runtime daemon kernel agents skills família' },
  { id: 'diagnostics', label: 'Diagnostics', desc: 'Versão / build / ambiente (read-only)',     icon: 'info',     keywords: 'version build environment diagnóstico ambiente' },
  { id: 'about',       label: 'About',       desc: 'Sobre o COSCA Desktop (read-only)',        icon: 'brand',    keywords: 'sobre about licença versão' },
]

// READ-ONLY: os atalhos REAIS do Desktop. Sem re-mapping (não há suporte de
// re-binding no Desktop → honesto). Cada linha indica o scope honesto.
type Shortcut = { keys: string[]; label: string; scope: string }
const SHORTCUTS: Shortcut[] = [
  { keys: ['Ctrl', 'K'],  label: 'Command palette',              scope: 'Global' },
  { keys: ['Ctrl', ','],  label: 'Abrir Settings',               scope: 'Global' },
  { keys: ['Ctrl', '`'],  label: 'Abrir terminal (projeto)',     scope: 'Global · requer projeto' },
  { keys: ['Ctrl', 'O'],  label: 'Abrir projeto (Home)',         scope: 'Global' },
  { keys: ['Ctrl', 'E'],  label: 'Abrir arquivos (tree)',        scope: 'Global' },
  { keys: ['Ctrl', 'N'],  label: 'Criar novo projeto',           scope: 'Global' },
  { keys: ['Ctrl', 'Enter'], label: 'Executar pedido do agente', scope: 'Workbench do agente' },
  { keys: ['Esc'],        label: 'Fechar prompt / ação atual',   scope: 'Global' },
]

type SettingsProps = {
  dark: boolean
  setTheme: (next: boolean) => void
  layout: LayoutState
  onResetLayout: () => void
  projectName?: string
  projectRoot?: string
  cap: { kernel: boolean; family_memory: boolean; daemon: boolean; agents: number | string; skills: number | string }
  capMode: 'forge' | 'standalone' | null
  rootInfo: Record<string, any> | null
}

// Linha chave/valor READ-ONLY densa (reuso do padrão `kv` do App).
const kv = (label: string, value: React.ReactNode, tone?: 'ok' | 'warn' | 'danger' | 'info' | 'neutral', mono = false) => (
  <div className="settings-kv">
    <span className="settings-kv-label">{label}</span>
    <b className={`settings-kv-value${tone ? ` ${tone}` : ''}${mono ? ' mono' : ''}`}>{value}</b>
  </div>
)

export default function Settings(props: SettingsProps) {
  const { dark, setTheme, layout, onResetLayout, projectName, projectRoot, cap, capMode, rootInfo } = props
  const [active, setActive] = useState<SectionId>('appearance')
  const [q, setQ] = useState('')
  const [env, setEnv] = useState<{ buildType: string; platform: string; arch: string } | null>(null)
  const navRef = useRef<HTMLElement | null>(null)

  // Build/ambiente REAL via Environment() (wails runtime) — honesto, nunca inventa.
  useEffect(() => {
    let alive = true
    Environment().then(e => { if (alive && e) setEnv({ buildType: e.buildType, platform: e.platform, arch: e.arch }) })
      .catch(() => { if (alive) setEnv(null) })
    return () => { alive = false }
  }, [])

  const ql = q.trim().toLowerCase()
  const matches = useMemo(
    () => (ql ? SECTIONS.filter(s => `${s.label} ${s.desc} ${s.keywords}`.toLowerCase().includes(ql)) : []),
    [ql],
  )

  const jump = (id: SectionId) => { setActive(id); setQ('') }

  // Navegação por teclado da sidebar (roving tabindex: Arrow/Home/End).
  const onNavKey = (e: React.KeyboardEvent) => {
    const idx = SECTIONS.findIndex(s => s.id === active)
    let next = idx
    if (e.key === 'ArrowDown') next = Math.min(idx + 1, SECTIONS.length - 1)
    else if (e.key === 'ArrowUp') next = Math.max(idx - 1, 0)
    else if (e.key === 'Home') next = 0
    else if (e.key === 'End') next = SECTIONS.length - 1
    else return
    if (next !== idx) {
      e.preventDefault()
      const id = SECTIONS[next].id
      setActive(id)
      const el = navRef.current?.querySelector<HTMLButtonElement>(`[data-section="${id}"]`)
      el?.focus()
      el?.scrollIntoView({ block: 'nearest' })
    }
  }

  const onSearchKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && matches.length > 0) { e.preventDefault(); jump(matches[0].id) }
    if (e.key === 'Escape') { e.preventDefault(); setQ('') }
  }

  // ---------------------------------------------------------------------
  // SEÇÕES
  // ---------------------------------------------------------------------

  const themeToggle = (next: boolean) => (
    <div className="settings-seq" role="radiogroup" aria-label="Tema do Desktop">
      <button role="radio" aria-checked={!dark} className={`settings-seq-btn${!dark ? ' active' : ''}`} onClick={() => setTheme(false)}>
        <Icon name="sun" size={13} /> Light
      </button>
      <button role="radio" aria-checked={dark} className={`settings-seq-btn${dark ? ' active' : ''}`} onClick={() => setTheme(true)}>
        <Icon name="moon" size={13} /> Dark
      </button>
    </div>
  )

  const appearanceEl = (
    <div className="settings-section">
      <div className="settings-section-head">Appearance</div>
      <p className="settings-desc">Preferência de aparência do Desktop. Aplicação imediata, sem reiniciar.</p>
      <div className="settings-card">
        <div className="settings-card-head">Theme</div>
        <div className="settings-row">
          <div className="settings-row-body">
            <span className="settings-row-label">Modo do tema</span>
            <span className="settings-row-desc">Dark (default) ou Light. Persistido em localStorage.</span>
          </div>
          <div className="settings-row-value">{themeToggle(dark)}</div>
        </div>
      </div>
      <div className="settings-reset-row">
        <button className="btn ghost" onClick={() => setTheme(true)}>Reset (Dark)</button>
      </div>
    </div>
  )

  const widths: [string, number, boolean][] = [
    ['Tree', layout.widths.tree, layout.collapsed.tree],
    ['Agent', layout.widths.agent, layout.collapsed.agent],
    ['Context', layout.widths.context, layout.collapsed.context],
    ['Terminal', layout.widths.terminal, layout.collapsed.terminal],
    ['Diff', layout.widths.diff, layout.collapsed.diff],
    ['Intelligence', layout.widths.intelligence, layout.collapsed.intelligence],
  ]

  const workspaceEl = (
    <div className="settings-section">
      <div className="settings-section-head">Workspace</div>
      <p className="settings-desc">
        Layout de panes persistido em localStorage, isolado por projeto:{' '}
        <span className="mono">{projectRoot || 'global'}</span>. Leitura do estado real (read-only).
      </p>
      <div className="settings-card">
        <div className="settings-card-head">Layout · {projectName || '—'}</div>
        {widths.map(([label, width, collapsed]) => (
          <div className="settings-row" key={label}>
            <div className="settings-row-body"><span className="settings-row-label">{label}</span></div>
            <div className="settings-row-value">
              <span className="badge neutral mono">{width}px</span>
              <span className={`badge ${collapsed ? 'warn' : 'ok'}`}>{collapsed ? 'collapsed' : 'open'}</span>
            </div>
          </div>
        ))}
        <div className="settings-row">
          <div className="settings-row-body"><span className="settings-row-label">Bottom strip (OUTPUT)</span></div>
          <div className="settings-row-value">
            <span className="badge neutral mono">{layout.height}px</span>
            <span className={`badge ${layout.collapsed.output ? 'warn' : 'ok'}`}>{layout.collapsed.output ? 'collapsed' : 'open'}</span>
          </div>
        </div>
      </div>
      <div className="settings-note">
        Redimensionar/colapsar é feito no próprio workspace (splitters). Aqui é leitura + reset do
        layout persistido — o engine de layout (<span className="mono">useLayout</span>) não é duplicado.
      </div>
      <div className="settings-reset-row">
        <button className="btn ghost" onClick={onResetLayout}>Reset Layout</button>
      </div>
    </div>
  )

  const keyboardEl = (
    <div className="settings-section">
      <div className="settings-section-head">Keyboard</div>
      <p className="settings-desc">Atalhos reais do Desktop. Read-only — não há suporte de re-mapping (honesto).</p>
      <div className="settings-card">
        <div className="settings-card-head">Shortcuts</div>
        <div className="settings-shortcuts">
          {SHORTCUTS.map((s, i) => (
            <div className="settings-shortcut" key={i}>
              <div className="settings-shortcut-body">
                <div className="settings-shortcut-label">{s.label}</div>
                <div className="settings-shortcut-scope">{s.scope}</div>
              </div>
              <div className="settings-shortcut-keys kbd-group" aria-label={`atalho: ${s.keys.join(' + ')}`}>
                {s.keys.map((k, j) => <kbd key={j} className="kbd">{k}</kbd>)}
              </div>
            </div>
          ))}
        </div>
      </div>
      <div className="settings-note">Sem edição de atalhos: o runtime não expõe re-binding de teclado.</div>
    </div>
  )

  const runtimeEl = (
    <div className="settings-section">
      <div className="settings-section-head">Runtime</div>
      <p className="settings-desc">
        Estado real do runtime Cosca via <span className="mono">CapabilityState()</span> /{' '}
        <span className="mono">RootMode()</span>. Somente leitura — não altera política do kernel.
      </p>
      <div className="settings-card">
        <div className="settings-card-head">CapabilityState</div>
        {kv('MODE', capMode ? capMode.toUpperCase() : '…')}
        {kv('KERNEL', cap.kernel ? 'presente' : 'ausente', cap.kernel ? 'ok' : 'neutral')}
        {kv('FAMILY MEMORY', cap.family_memory ? 'presente' : 'ausente', cap.family_memory ? 'ok' : 'neutral')}
        {kv('DAEMON', cap.daemon ? 'on' : 'off', cap.daemon ? 'ok' : 'warn')}
        {kv('AGENTS', String(cap.agents ?? 0))}
        {kv('SKILLS', String(cap.skills ?? 0))}
      </div>
      <div className="settings-note">
        Kernel/memória da família são lidos apenas no modo FORGE (root, ADR-0007). O Desktop não
        expõe escrita de política — observa, não altera.
      </div>
    </div>
  )

  const diagnosticsEl = (
    <div className="settings-section">
      <div className="settings-section-head">Diagnostics</div>
      <p className="settings-desc">Informações de build/ambiente do Desktop. Read-only. Nunca expõe segredos.</p>
      <div className="settings-card">
        <div className="settings-card-head">Build / Environment</div>
        {kv('BUILD', env?.buildType || '—')}
        {kv('PLATFORM', env?.platform || '—', undefined, true)}
        {kv('ARCH', env?.arch || '—', undefined, true)}
        {kv('MODE', capMode ?? '—')}
        {kv('DAEMON', cap.daemon ? 'on' : 'off', cap.daemon ? 'ok' : 'warn')}
        {kv('ROOT', rootInfo?.root ? 'FORGE (root detectado)' : 'STANDALONE', rootInfo?.root ? 'info' : 'neutral')}
        {kv('ACTIVE PROJECT', projectName || '—')}
      </div>
      <div className="settings-note">Sem variáveis de ambiente / credenciais expostas (fail-closed).</div>
    </div>
  )

  const aboutEl = (
    <div className="settings-section">
      <div className="settings-section-head">About</div>
      <div className="settings-card">
        <div className="settings-about-hero">
          <Icon name="brand" size={22} />
          <div>
            <div className="settings-about-title">COSCA Desktop</div>
            <div className="settings-about-meta">Engineering Workbench · observabilidade &amp; controle</div>
          </div>
        </div>
        {kv('BUILD', env?.buildType || '—')}
        {kv('PLATFORM', env?.platform || '—', undefined, true)}
        {kv('ARCH', env?.arch || '—', undefined, true)}
        {kv('MODE', capMode ?? '—')}
        {kv('KERNEL', cap.kernel ? 'presente' : 'ausente', cap.kernel ? 'ok' : 'neutral')}
        {kv('FAMILY MEMORY', cap.family_memory ? 'presente' : 'ausente', cap.family_memory ? 'ok' : 'neutral')}
        {kv('LICENSE', '—')}
      </div>
      <div className="settings-note">
        Sem versão numérica exposta pelo binding do Desktop: build/plataforma via{' '}
        <span className="mono">Environment()</span> (honesto). Licença não exposta pelo runtime.
      </div>
    </div>
  )

  const renderSection = (id: SectionId): React.ReactNode => {
    switch (id) {
      case 'appearance': return appearanceEl
      case 'workspace': return workspaceEl
      case 'keyboard': return keyboardEl
      case 'runtime': return runtimeEl
      case 'diagnostics': return diagnosticsEl
      case 'about': return aboutEl
      default: return null
    }
  }

  return (
    <main className="settings" aria-label="Central de Configurações">
      <aside className="settings-nav" ref={navRef} role="tablist" aria-label="Seções de configurações" onKeyDown={onNavKey}>
        <div className="settings-navlist">
          {SECTIONS.map(s => (
            <button key={s.id}
              type="button"
              data-section={s.id}
              role="tab"
              aria-selected={active === s.id}
              tabIndex={active === s.id ? 0 : -1}
              className={`settings-item${active === s.id ? ' active' : ''}`}
              onClick={() => jump(s.id)}>
              <span className="settings-item-icon"><Icon name={s.icon} size={15} /></span>
              <span className="settings-item-body">
                <span className="settings-item-label">{s.label}</span>
                <span className="settings-item-desc">{s.desc}</span>
              </span>
            </button>
          ))}
        </div>
      </aside>

      <section className="settings-content" role="tabpanel" aria-label={SECTIONS.find(s => s.id === active)?.label}>
        <div className="settings-searchwrap">
          <label className="settings-search">
            <Icon name="search" size={14} />
            <input
              className="input input-search settings-search-input"
              value={q}
              onChange={e => setQ(e.target.value)}
              onKeyDown={onSearchKey}
              placeholder="Search settings…"
              aria-label="Buscar configurações"
            />
            {!q && <span className="kbd">⌘</span>}
          </label>
        </div>
        <div className="settings-scroll">
          {ql ? (
            <div className="settings-results">
              <div className="settings-section-head">Search results <span className="badge neutral mono">{matches.length}</span></div>
              {matches.length === 0 && <div className="settings-note">Nenhuma configuração encontrada para “{q}”.</div>}
              {matches.map(s => (
                <button key={s.id} type="button" className="settings-result" onClick={() => jump(s.id)}>
                  <span className="settings-result-icon"><Icon name={s.icon} size={14} /></span>
                  <span className="settings-result-body">
                    <span className="settings-result-name">{s.label}</span>
                    <span className="settings-result-desc">{s.desc}</span>
                  </span>
                </button>
              ))}
              {matches.length > 0 && <div className="settings-note">Pressione Enter ou clique para abrir a seção.</div>}
            </div>
          ) : (
            renderSection(active)
          )}
        </div>
      </section>
    </main>
  )
}
