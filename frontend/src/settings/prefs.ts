/* =========================================================================
   COSCA DESKTOP — USER PREFERENCES (prefs.ts)
   Contrato §2: as preferências do USER são um estado do DESKTOP, armazenadas em
   `localStorage` do runtime do Desktop (chave `cosca-desktop:prefs`) — NUNCA em
   `.cosca/`, memory, knowledge, learning, family, kernel, blockchain ou audit
   (o Desktop não detém essas áreas). Mecanismo análogo ao `layoutPersistence.ts`
   (§14), porém para preferências de USER PREFERENCE (não layout do projeto).

   Contrato §1: Settings manipula apenas USER PREFERENCE. Nunca
   PROJECT/RUNTIME/KERNEL state, nunca `.cosca`.
   ========================================================================= */

export const PREFS_KEY = 'cosca-desktop:prefs'

export type Theme = 'dark' | 'light'

export type UserPreferences = {
  theme: Theme
  // ↓ apenas o que TEM suporte real hoje — NÃO inventar configuração.
}

/** Defaults honestos: o Desktop abre em dark (estado atual do shell). */
const DEFAULTS: UserPreferences = { theme: 'dark' }

/**
 * Sanitiza um valor vindo de storage (fonte não confiável). Sempre devolve um
 * objeto válido e tipado: ausente/corrompido/campo inválido → default. Nunca
 * inventa campo — só o que o tipo declara.
 */
export function normalizePrefs(raw: any): UserPreferences {
  if (!raw || typeof raw !== 'object') return { ...DEFAULTS }
  return {
    theme: raw.theme === 'light' ? 'light' : 'dark',
  }
}

/** Returns `null` quando ausente OU corrompido (JSON inválido). */
export function loadPrefs(): UserPreferences | null {
  try {
    const raw = localStorage.getItem(PREFS_KEY)
    if (!raw) return null
    return normalizePrefs(JSON.parse(raw))
  } catch {
    return null
  }
}

export function savePrefs(p: UserPreferences): void {
  try {
    localStorage.setItem(PREFS_KEY, JSON.stringify(p))
  } catch {
    /* localStorage indisponível (modo privado etc.) — best-effort */
  }
}

export function clearPrefs(): void {
  try {
    localStorage.removeItem(PREFS_KEY)
  } catch {
    /* noop */
  }
}
