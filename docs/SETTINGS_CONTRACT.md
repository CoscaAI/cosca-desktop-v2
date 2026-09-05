# COSCA DESKTOP — SETTINGS CONTRACT (UserPreferences)

> **Objetivo**: contrato único de preferências do usuário. Separa claramente
> USER PREFERENCE / SESSION STATE / PROJECT STATE / RUNTIME STATE / KERNEL STATE.
> Segue a regra: CONHECER → AUDITAR → MAPEAR → CONTRATO → IMPLEMENTAR → VALIDAR.
> **Não inventar configuração sem suporte real. Não duplicar mecanismo existente.**

---

## 1. Separação de estados (NÃO misturar)

| Estado | O que é | Onde vive | Exemplo |
|--------|---------|-----------|---------|
| **USER PREFERENCE** | preferência do usuário, sobrevive a restart | `localStorage` (Desktop) | tema, densidade |
| **SESSION STATE** | estado transitório da sessão atual | memória React | view ativa, projeto aberto |
| **PROJECT STATE** | estado do projeto (isolado) | `<projeto>/.cosca` (via runtime) | `.cosca/` do projeto |
| **RUNTIME STATE** | estado do runtime cosca | runtime/daemon | `CapabilityState` |
| **KERNEL STATE** | memória/conhecimento do COSCA | kernel | **NUNCA** tocar via Settings |

**Regra**: Settings manipula apenas **USER PREFERENCE**. Nunca PROJECT/RUNTIME/KERNEL state, nunca `.cosca`.

## 2. Persistência de preferências (mecanismo JÁ existente)

O Desktop **já persiste** em `localStorage` via `frontend/src/layout/layoutPersistence.ts` (chave `cosca-desktop:layout[:root]`). Este é o mecanismo oficial do Desktop.

**Contrato**: criar/estender um `localStorage` de preferências com chave `cosca-desktop:prefs` (nomeando claramente USER PREFERENCE). **Formato tipado** para o React:
```ts
type UserPreferences = {
  theme: 'dark' | 'light'          // real: `dark` boolean já alterna
  layout: LayoutState              // já persistido (widths/collapsed por projeto)
  // ↓ apenas o que TEM suporte real hoje — NÃO inventar.
  reducedMotion: 'system' | 'always' | 'never'
}
```
Persistência via `loadPrefs()`/`savePrefs()` (padrão do `layoutPersistence`), em `localStorage`, nunca `.cosca`.

## 3. Configurações REAIS (suportadas hoje) vs NÃO implementar

### REAL (implementar)
| Categoria | Config | Suporte real | Efeito |
|-----------|--------|--------------|--------|
| **Appearance** | Theme (Dark/Light) | `dark` boolean já usado globalmente | alterna `.dark`/`.light` no shell, persiste |
| **Workspace** | Layout (widths/collapsed) | `layoutPersistence` já persiste | reutiliza `useLayout` |
| **Keyboard** | Tabela de atalhos | Command Palette (Ctrl+K etc.) | READ-ONLY (mostrar atalhos reais) |
| **Runtime** | Status/info | `CapabilityState`, `RootMode`, `RootInfo` | READ-ONLY (observar, não alterar política) |
| **Diagnostics** | Info versão/build/runtime | bindings + `runtime` | READ-ONLY |
| **About** | Versão/build/licença | Desktop info | READ-ONLY |
| **Interface** | Show labels (rail), reduced-motion | já há `prefers-reduced-motion` CSS | aplicar no shell |

### NÃO implementar (sem suporte real — honesto)
- **AI / Models**: config de provider/model pertence ao **Kernel**, não ao Desktop → exibir como `NOT AVAILABLE` (ler de `CapabilityState`), NÃO criar duplicata.
- **Privacy**: telemetry/analytics **não existem** → NÃO fingir.
- **Performance**: "boost mágico" NÃO → apenas `reduced-motion` real.
- **Notifications**: não há sistema de notificação → NÃO inventar.

## 4. Layout da Central (professional, sidebar + conteúdo)

```
┌──────────────────────────────────────┐
│ Settings                             │
├─────────────┬────────────────────────┤
│ Appearance  │  [conteúdo da seção]   │
│ Workspace   │                        │
│ Keyboard    │                        │
│ Runtime     │                        │
│ Diagnostics │                        │
│ About       │                        │
└─────────────┴────────────────────────┘
```
- Sidebar da settings: compacta, keyboard-navegável, seção ativa destacada.
- **Busca** filtrada por nome/descrição/categoria (glob de sections + configs).
- Apenas seções com config real (6: Appearance/Workspace/Keyboard/Runtime/Diagnostics/About).

## 5. Regras de governança (UI_GOVERNANCE)
- Tokens (`var(--...)`), radius 4/6/8, denso, sem cor hardcoded, sem emoji.
- Estado lógico em `data-*`; físico em `:hover`/`:focus-visible`.
- Reset de seção = reset de USER PREFERENCE (nunca projeto/kernel/memória).
- Alterações simples aplicam imediatamente (theme/density), sem "restart required" quando não necessário.
- Acessibilidade: aria-label em botões de ícone, keyboard, reduced-motion.
- Reutilizar `Icon` (Lucide), `.btn`, `.panel`, `.input`. NÃO criar segundo sistema.
