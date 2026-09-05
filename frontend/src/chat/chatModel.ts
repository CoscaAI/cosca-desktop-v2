// =========================================================================
// COSCA DESKTOP — CHAT (FASE 3): helpers puros (sem React)
// -------------------------------------------------------------------------
// Extraem o "shape" gerado pelo App monólito: sem hooks, sem estado, apenas
// transformações determinísticas de apresentação. A UI nunca fabrica conteúdo
// que o backend não enviou.
// =========================================================================
import type { ChatFrame, ChatMsg, ChatPhase, ChatSessionSummary, ChatStreamResult } from './types'

// Um id de run (para CancelChatStream). Sem dependência externa → seguro.
let runSeq = 0
export function nextRunID(): string {
  // crypto.randomUUID() quando disponível (WebView moderno); fallback do App.
  const base = (globalThis as any)?.crypto?.randomUUID ? (globalThis as any).crypto.randomUUID() : `run-${Date.now()}-${++runSeq}`
  return String(base)
}

// Id de mensagem (local, não persiste). Simples e único o suficiente.
let msgSeq = 0
export function nextMsgID(): string {
  return `m-${Date.now()}-${++msgSeq}`
}

/** Aplica um frame canônico do daemon a uma mensagem do assistente (retorna o
 *  próximo estado SEM mutar o original — imutável para o React). */
export function applyChatFrame(prev: ChatMsg, frame: ChatFrame): ChatMsg {
  switch (frame.type) {
    case 'thinking':
      return { ...prev, pending: 'thinking', statusDetail: frame.content || '' }
    case 'status':
      return { ...prev, pending: (frame.status === 'processing' ? 'processing' : (frame.status as ChatPhase)) || 'processing' }
    case 'progress':
      return { ...prev, pending: 'progress', statusDetail: frame.content || prev.statusDetail }
    case 'response':
      return { ...prev, content: (prev.content || '') + (frame.content || ''), pending: 'processing' }
    case 'metadata':
      return {
        ...prev,
        model: frame.model || prev.model,
        session_id: frame.session_id || prev.session_id,
      }
    case 'done':
      return { ...prev, streaming: false, ended: 'done', pending: 'done', session_id: frame.session_id || prev.session_id }
    case 'error':
      return { ...prev, streaming: false, ended: 'error', error: frame.error || frame.content || 'erro do runtime', pending: 'error' }
    case 'cancelled':
      return { ...prev, streaming: false, ended: 'cancelled', pending: 'cancelled' }
    default:
      return prev
  }
}

/** Normaliza um resumo de sessão vindo do backend (campos ausentes → honesto). */
export function toSessionSummary(x: any): ChatSessionSummary {
  return {
    id: String(x?.id ?? ''),
    title: String(x?.title ?? 'Nova conversa'),
    model: String(x?.model ?? ''),
    messages: typeof x?.messages === 'number' ? x.messages : 0,
    updated_at: String(x?.updated_at ?? ''),
  }
}

/** Normaliza o resultado consolidado do ChatStream (campos honestos). */
export function toStreamResult(x: any): ChatStreamResult {
  return {
    text: String(x?.text ?? ''),
    session_id: String(x?.session_id ?? ''),
    model: String(x?.model ?? ''),
    provider: String(x?.provider ?? ''),
    agent: String(x?.agent ?? ''),
    duration_ms: typeof x?.duration_ms === 'number' ? x.duration_ms : 0,
    ended: String(x?.ended ?? 'error'),
    error: x?.error ? String(x.error) : undefined,
  }
}

/** Título curto e honesto de um resumo de sessão vazio. */
export function titleForSession(id: string, fallback?: string): string {
  if (fallback && fallback.trim() !== '') return fallback
  return id ? `Conversa ${id.slice(0, 8)}` : 'Nova conversa'
}

/** Legenda compacta de fase para o indicador do assistente. */
export function phaseLabel(p: ChatPhase): string {
  switch (p) {
    case 'thinking': return 'pensando…'
    case 'processing': return 'processando…'
    case 'progress': return 'avançando…'
    default: return ''
  }
}
