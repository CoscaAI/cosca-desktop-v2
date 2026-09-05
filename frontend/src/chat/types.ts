// =========================================================================
// COSCA DESKTOP — CHAT (FASE 3): tipos
// -------------------------------------------------------------------------
// A UI NÃO tem lógica cognitiva: ela só consome o provider (daemon
// /v1/run/stream via ChatStream + cosca:chat:event). Tipos apenas nomeiam o
// que o backend realmente devolve (nunca inventamos campo que não veio).
// =========================================================================

export type ChatRole = 'user' | 'assistant'

/** Fase de andamento de uma mensagem do assistente em streaming. */
export type ChatPhase = '' | 'thinking' | 'processing' | 'progress' | 'done' | 'error' | 'cancelled'

/** Uma mensagem da conversa. `streaming`/`pending`/`ended` são estados de UI
 *  de apresentação (o backend não os envia — a UI os deriva dos frames). */
export interface ChatMsg {
  id: string
  role: ChatRole
  content: string
  at?: string
  model?: string
  session_id?: string
  duration_ms?: number
  // Estado de exibição durante/ao fim do streaming:
  streaming?: boolean
  pending?: ChatPhase
  statusDetail?: string // legenda de andamento (progress/stage)
  ended?: 'done' | 'error' | 'cancelled'
  error?: string
}

/** Resposta consolidada de um turno (ChatStream). Shape do backend. */
export interface ChatStreamResult {
  text: string
  session_id: string
  model: string
  provider: string
  agent: string
  duration_ms: number
  ended: 'done' | 'error' | 'cancelled' | string
  error?: string
}

/** Resumo de uma conversa persistida (sidebar, ChatListSessions). */
export interface ChatSessionSummary {
  id: string
  title: string
  model: string
  messages: number
  updated_at: string
}

/** Frame canônico do streaming (shape do daemon re-emitido em cosca:chat:event). */
export interface ChatFrame {
  type: 'thinking' | 'status' | 'progress' | 'response' | 'metadata' | 'done' | 'error' | 'cancelled' | string
  content?: string
  status?: string
  error?: string
  session_id?: string
  model?: string
  provider?: string
  agent?: string
  duration_ms?: number
  token_usage?: Record<string, number>
  metadata?: Record<string, any>
}
