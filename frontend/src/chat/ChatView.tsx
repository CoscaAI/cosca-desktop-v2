// =========================================================================
// COSCA DESKTOP — CHAT VIEW (FASE 3): a tela principal de conversa
// -------------------------------------------------------------------------
// A UI NÃO tem lógica cognitiva. Ela consome o provider (daemon /v1/run/stream
// via ChatStream) e REAPRESENTA os frames canônicos vindos de `cosca:chat:event`
// (thinking/status/progress/response/metadata/done/error/cancelled). A conversa
// é a tela principal (mock do Don); o workbench (AGENT/FILES/… ) continua
// acessível no rail. Sidebar redimensionável via Splitter + design system.
// =========================================================================
import { useCallback, useEffect, useRef, useState } from 'react'
import {
  ChatStream, ChatListSessions, ChatSessionMessages, CancelChatStream,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { Splitter } from '../layout/Splitter'
import { Icon } from '../components/Icon'
import { Markdown } from './Markdown'
import { applyChatFrame, nextMsgID, nextRunID, phaseLabel, toSessionSummary, toStreamResult } from './chatModel'
import type { ChatFrame, ChatMsg, ChatSessionSummary } from './types'

// Modelos/providers configuraveis (o daemon resolve o provider pelo nome). A
// UI so passa a escolha — a autoridade de resolucao e do runtime.
const MODELS = ['deepseek', 'gpt-4o', 'gpt-4o-mini', 'claude-3-5-sonnet', 'claude-3-5-haiku', 'llama-3.1-70b', 'qwen-2.5-72b']

type ChatViewProps = {
  daemonOn?: boolean
  projectName?: string
}

const now = () => new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })

function normalizeFrame(x: any): ChatFrame | null {
  if (!x || typeof x !== 'object') return null
  return x as ChatFrame
}

export function ChatView({ daemonOn, projectName }: ChatViewProps) {
  const [sessions, setSessions] = useState<ChatSessionSummary[]>([])
  const [activeId, setActiveId] = useState<string>('') // '' = no-stata conversa nova
  const [messages, setMessages] = useState<ChatMsg[]>([])
  const [input, setInput] = useState('')
  const [model, setModel] = useState('deepseek')
  const [streaming, setStreaming] = useState(false)
  const [loadingSession, setLoadingSession] = useState(false)
  const [err, setErr] = useState('')
  // Sidebar redimensionavel (Splitter) — largura local (nao persiste em disco).
  const [sidebarW, setSidebarW] = useState(260)
  const scrollRef = useRef<HTMLDivElement | null>(null)
  const offRef = useRef<(() => void) | null>(null)
  const runRef = useRef<string | null>(null)

  const activeSummary = sessions.find(s => s.id === activeId) || null

  const refreshSessions = useCallback(async () => {
    try {
      const list = await ChatListSessions()
      setSessions(list.map(toSessionSummary))
    } catch {
      /* mantém a lista atual (honesto) */
    }
  }, [])

  // Carrega a lista de conversas ao montar.
  useEffect(() => {
    refreshSessions()
  }, [refreshSessions])

  // Auto-scroll para a última mensagem ao mudar.
  useEffect(() => {
    if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight
  }, [messages, streaming])

  // Desregistra o listener de eventos ao desmontar (evita vazamento).
  useEffect(() => {
    return () => {
      if (offRef.current) { try { offRef.current() } catch { /* noop */ } }
    }
  }, [])

  const selectSession = useCallback(async (id: string) => {
    if (streaming || id === activeId) return
    setActiveId(id)
    setLoadingSession(true); setErr('')
    try {
      const msgs = await ChatSessionMessages(id)
      setMessages(msgs.map((m: any): ChatMsg => ({
        id: nextMsgID(),
        role: (m?.role === 'user' ? 'user' : 'assistant') as ChatMsg['role'],
        content: String(m?.content ?? ''),
        at: m?.at ?? '',
      })))
    } catch (e) {
      setErr(String(e)); setMessages([])
    }
    setLoadingSession(false)
  }, [streaming, activeId])

  const newChat = useCallback(() => {
    if (streaming) return
    setActiveId('')
    setMessages([])
    setErr('')
  }, [streaming])

  const send = useCallback(async (raw?: string) => {
    const text = (raw !== undefined ? raw : input).trim()
    if (!text || streaming) return
    setInput(''); setErr('')

    const sessionId = activeId || ''
    const userMsg: ChatMsg = { id: nextMsgID(), role: 'user', content: text, at: now() }
    const asstId = nextMsgID()
    setMessages(prev => [...prev, userMsg, { id: asstId, role: 'assistant', content: '', streaming: true, pending: 'thinking' }])
    setStreaming(true)

    const runID = nextRunID()
    runRef.current = runID

    // Assina o canal do backend: cada frame canonico atualiza a ultima
    // mensagem do assistente (token a token, sem lógica cognitiva no front).
    const off = EventsOn('cosca:chat:event', (raw: any) => {
      const frame = normalizeFrame(raw)
      if (!frame || !frame.type) return
      setMessages(prev => prev.map(m => (m.id === asstId ? applyChatFrame(m, frame) : m)))
    })
    offRef.current = off

    try {
      const result = toStreamResult(await ChatStream(runID, text, sessionId, model))
      setMessages(prev => prev.map(m => {
        if (m.id !== asstId) return m
        const next: ChatMsg = { ...m, streaming: false }
        if (result.ended) next.ended = result.ended as ChatMsg['ended']
        if (result.session_id) next.session_id = result.session_id
        if (result.model) next.model = result.model
        if (result.error) { next.error = result.error; next.ended = 'error' }
        if (result.text && !next.content) next.content = result.text
        return next
      }))
      if (result.session_id) setActiveId(result.session_id)
      await refreshSessions()
    } catch (e) {
      setMessages(prev => prev.map(m => m.id === asstId ? { ...m, streaming: false, ended: 'error', error: String(e) } : m))
      setErr(String(e))
    } finally {
      runRef.current = null
      try { off() } catch { /* noop */ }
      offRef.current = null
      setStreaming(false)
    }
  }, [input, streaming, activeId, model, refreshSessions])

  const cancel = useCallback(async () => {
    if (!runRef.current) return
    try { await CancelChatStream(runRef.current) } catch { /* noop */ }
  }, [])

  const regenerate = useCallback(() => {
    const lastUser = [...messages].reverse().find((m: ChatMsg) => m.role === 'user')
    if (lastUser) void send(lastUser.content)
  }, [messages, send])

  const copyText = useCallback((t: string) => {
    try { void navigator.clipboard?.writeText(t) } catch { /* noop */ }
  }, [])

  // Determina o título do painel ativo.
  const panelTitle = activeSummary?.title || (messages.length ? 'Conversa' : 'Nova conversa')

  const renderAssistantMeta = (m: ChatMsg) => (
    <div className="chat-msg-meta">
      {m.model && <span className="badge neutral mono">model {m.model}</span>}
      {m.ended === 'done' && <span className="badge ok">done</span>}
      {m.duration_ms != null && m.duration_ms > 0 && <span className="badge neutral mono">{m.duration_ms} ms</span>}
      {m.ended === 'error' && <span className="badge danger">erro</span>}
      {m.ended === 'cancelled' && <span className="badge warn">cancelado</span>}
      {m.at && <span className="chat-msg-time">{m.at}</span>}
    </div>
  )

  return (
    <div className="chat-view">
      {/* SIDEBAR — lista de conversas + nova conversa */}
      <aside className="chat-side" style={{ width: sidebarW, minWidth: 0 }}>
        <div className="chat-side-head">
          <span className="chat-side-title">Conversas</span>
          <button className="btn btn--sm primary" onClick={newChat} disabled={streaming} title="Nova conversa" aria-label="Nova conversa">
            <Icon name="command" size={13} /> Nova
          </button>
        </div>
        <div className="chat-side-list">
          {sessions.length === 0 && (
            <div className="chat-side-empty">Nenhuma conversa persistida.<br />Inicie abaixo.</div>
          )}
          {sessions.map(s => (
            <button key={s.id} type="button"
              className={`chat-side-item${s.id === activeId ? ' active' : ''}`}
              onClick={() => selectSession(s.id)}
              disabled={streaming}
              title={s.id}>
              <span className="chat-side-item-title">{s.title}</span>
              <span className="chat-side-item-meta">
                {s.model && <span className="mono">{s.model}</span>}
                <span>{s.messages} msgs</span>
              </span>
            </button>
          ))}
        </div>
      </aside>
      <Splitter
        orientation="vertical"
        onResize={dx => setSidebarW(w => Math.min(Math.max(w + dx, 200), 420))}
        value={sidebarW}
        min={200}
        max={420}
        label="Resize chat sidebar"
      />

      {/* PAINEL DE CONVERSA */}
      <section className="chat-main">
        <div className="chat-head">
          <span className="chat-head-title" title={activeSummary?.id || ''}>{panelTitle}</span>
          {activeSummary?.id && <span className="badge neutral mono" title="session_id (resume)">session {activeSummary.id.slice(0, 8)}…</span>}
          {activeId && !activeSummary?.id && <span className="badge neutral mono">session {activeId.slice(0, 8)}…</span>}
          <span className="spacer" />
          <span className={`badge ${daemonOn ? 'ok' : 'neutral'}`}><span className="dot" /> daemon {daemonOn ? 'on' : 'off'}</span>
          <label className="chat-model">
            <span className="chat-model-label">modelo</span>
            <select className="input input-sm" value={model} onChange={e => setModel(e.target.value)} disabled={streaming} aria-label="modelo">
              {MODELS.map(m => <option key={m} value={m}>{m}</option>)}
            </select>
          </label>
        </div>

        {err && <div className="error-box" style={{ margin: 'var(--sp-2) var(--sp-3)' }}>{err}</div>}

        <div className="chat-scroll" ref={scrollRef}>
          {loadingSession && <div className="empty" style={{ padding: 'var(--sp-5)' }}><p>Carregando conversa…</p></div>}
          {!loadingSession && messages.length === 0 && (
            <div className="chat-empty">
              <div className="hero-mark"><Icon name="agent" size={18} /></div>
              <p className="chat-empty-title">Inicie uma conversa</p>
              <p className="chat-empty-sub">O kernel delibera (ADR-032) e escalava ao provider. Mensagens usam markdown + código destacado.</p>
            </div>
          )}
          {messages.map(m => m.role === 'user' ? (
            <div key={m.id} className="chat-row user">
              <div className="chat-bubble user">
                <div className="chat-bubble-text">{m.content}</div>
                {m.at && <span className="chat-bubble-time">{m.at}</span>}
              </div>
            </div>
          ) : (
            <div key={m.id} className="chat-row assistant">
              <div className="chat-avatar"><Icon name="agent" size={14} /></div>
              <div className="chat-msg">
                {m.streaming && !m.content ? (
                  <div className="chat-pending">
                    <span className="status-dot info pulse" />
                    <span>{phaseLabel(m.pending ?? 'thinking')}</span>
                    {m.statusDetail ? <span className="mono chat-pending-detail">{m.statusDetail}</span> : null}
                  </div>
                ) : (
                  <>
                    {m.content ? <Markdown content={m.content} /> : <span className="chat-md-dim">(sem texto — {m.ended || '…'})</span>}
                    {m.error && <div className="approval-banner" style={{ marginTop: 'var(--sp-2)' }}>{m.error}</div>}
                    {!m.streaming && (
                      <div className="chat-msg-actions">
                        <button className="btn btn--sm ghost" onClick={() => copyText(m.content)} title="Copiar resposta" aria-label="Copiar resposta">
                          <Icon name="command" size={12} /> Copiar
                        </button>
                        <button className="btn btn--sm ghost" onClick={regenerate} disabled={streaming} title="Regenerar (reenvia a última pergunta)" aria-label="Regenerar">
                          <Icon name="command" size={12} /> Regenerar
                        </button>
                      </div>
                    )}
                  </>
                )}
                {renderAssistantMeta(m)}
              </div>
            </div>
          ))}
          {streaming && (
            <div className="chat-row assistant">
              <div className="chat-avatar"><Icon name="agent" size={14} /></div>
              <div className="chat-pending">
                <span className="status-dot info pulse" />
                <span>conversando…</span>
              </div>
            </div>
          )}
        </div>

        {/* COMPOSER */}
        <div className="chat-composer">
          <textarea
            className="input chat-input"
            value={input}
            placeholder={streaming ? 'Aguardando resposta…' : 'Pergunte ao COSCA… (Shift+Enter = nova linha, Enter = enviar)'}
            disabled={streaming}
            onChange={e => setInput(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                if (!streaming) void send()
              }
            }}
            aria-label="mensagem"
          />
          <div className="chat-composer-row">
            <span className="hint">COSCA · Kernel-First · /v1/run</span>
            <span className="spacer" />
            {streaming ? (
              <button className="btn danger" onClick={cancel}>Cancelar</button>
            ) : (
              <button className="btn primary" onClick={() => send()} disabled={!input.trim()}>
                <Icon name="command" size={13} /> Enviar
              </button>
            )}
          </div>
        </div>
      </section>
    </div>
  )
}

export default ChatView
