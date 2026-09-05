# COSCA UI INTELLIGENCE — GOVERNANÇA VISUAL E DE INTERAÇÃO

> **Papel**: o kernel usa isto como definição NORMATIVA de "desktop profissional".
> "Melhor desktop" NÃO é opinião do agente — é auditável contra estas regras.
> **Domínio**: UI/UX do COSCA Desktop (Wails+React). É UI Intelligence, distinto de
> Code/Project Intelligence. Segue a epistemologia COSCA: FACT → RULE → DECISION.
> **Regra de ouro**: CONHECER → AVALIAR → DECIDIR → EXECUTAR. Nunca "mexer em tudo".

---

## 1. Epistemologia (significado)

| Tipo | Definição | Exemplo |
|------|-----------|---------|
| **FACT** | O que é verdade hoje (observável, verificável no código). | "O botão de ícone tem 16px." |
| **RULE** | Regra normativa que o sistema deve cumprir (auditável). | "Ações primárias têm alvo mínimo 24px." |
| **DECISION** | Decisão arquitetural/visual já tomada (contexto). | "O COSCA usa densidade compacta no workspace." |

## 2. Critérios de qualidade (o que define "profissional")

Cada critério tem regras auditáveis. Uma violação = EVIDENCE que vira PROPOSTA (gate),
não correção automática por gosto.

### A. VISUAL
- A1. Toda cor/tamanho/raio vem de token (`var(--...)`); **0 hardcoded** fora de `:root`.
- A2. Escalas coerentes: radius (xs/sm/md/lg/xl/2xl), spacing (base 4px), tipografia (`--fs-*` + `--fw-*`).
- A3. Superfícies hierárquicas (bg/surface/surface-2/3) — profundidade por superfície, não só sombra.
- A4. Bordas em escala (`--border-subtle`/`--border`/`--border-strong`).

### B. INTERACTION
- B1. Toda ação interativa tem **feedback** (hover/active/loading/disabled).
- B2. Todo elemento focável tem **`:focus-visible`** (anel consistente, `--ring-*`).
- B3. Resize tem **limites (min/max)** e **estado visual** (`data-dragging`).
- B4. Drag é previsível; **sem jitter/layout jump/loop**.
- B5. Operação assíncrona tem **loading**; erro tem **recuperação** (estado honesto, não "eterno").

### C. ACCESSIBILITY
- C1. Keyboard-first: navegação por teclado, `Enter`/`Space`/`Escape` em controles e overlays.
- C2. `prefers-reduced-motion` respeitado (motion comunica estado, não decora).
- C3. `forced-colors`/high-contrast preserva legibilidade (estado não depende só de cor).
- C4. Contrast AA; aria-labels em botões de ícone isolado.
- C5. Alvos de toque/clique ≥ 24px (desktop).

### D. PERFORMANCE
- D1. Resize é 100% UI (sem backend/filesystem/wails durante drag); `requestAnimationFrame`.
- D2. Sem renderização infinita / loop de hooks (Rules of Hooks: **nenhum hook após `return` condicional**).
- D3. Sem `setTimeout`/`setInterval` mascarando problema de layout.

### E. CONSISTENCY
- E1. Duas ações de mesma importância → mesmo tamanho/peso/tratamento.
- E2. Primitivas reutilizáveis (`Icon`, `.btn`, `.panel`, `Splitter`) — não copiar estilo em cada componente.
- E3. Sem componentes visualmente duplicados; compartilham primitive.

### F. INFORMATION DENSITY
- F1. Densidade controlada: panes, não cards; informação secundária não compete com a principal.
- F2. O editor é o principal consumidor de espaço; painéis colapsáveis não roubam atenção.

### G. DISCOVERABILITY / ERRO
- G1. Todo estado sem dados explica (empty-state); nada fica "vazio mudo".
- G2. Estados de erro/recuperação claros, com ação.

### H. MOTION
- H1. Motion comunica estado (opening/closing/progress/transition), nunca decoração.
- H2. Rápido/preciso (`--dur-fast/normal`, `--ease-out`); reduzido para `prefers-reduced-motion`.

---

## 3. Modelo de estado da interface (para o agente consultar o contrato, não inventar CSS)

### SIDEBAR / NAV
```
collapsed | expanded | hover | focused | active | disabled
```
### PANEL (Diff/Logs/Intelligence/Terminal/Editor)
```
closed | opening | open | resizing | minimized | error | loading
```
### RESIZER / SPLITTER
```
idle | hover | dragging
```

**Regra**: estado lógico → atributo `data-*` (`data-state`, `data-active`, `data-dragging`);
estado físico (ponteiro/teclado) → pseudo-classe (`:hover`, `:focus-visible`).

---

## 4. Separação intenção × implementação

```
Intent:  abrir painel de logs
Context: workspace=project, panel=logs, width=320
Constraints: min=240, max=600, keyboard=true

DECISION (kernel/comportamento): closed → opening → open; allow resize; loading se sem dados.
UI (apresentação): render LogsPanel, aplicar tokens/estados.
```
- **Kernel decide comportamento** (contrato do estado).
- **UI decide apresentação** (tokens/componentes).

---

## 5. Arquivos de conhecimento normativo (domínio UI)

Em `internal/embed/cosca/` (ou `docs/` do Desktop):
```
UI_ARCHITECTURE.md    → estrutura/layers (tokens → primitivas → componentes → estados)
UI_DESIGN_SYSTEM.md   → tokens, escalas, superfícies, tipografia
UI_INTERACTION.md     → drag/resize/estados/keyboard
UI_ACCESSIBILITY.md   → focus-visible, reduced-motion, forced-colors, aria
UI_PERFORMANCE.md     → resize 100% UI, sem loop, Rules of Hooks
UI_GOVERNANCE.md      → este documento (critérios + epistemologia)
```

---

## 6. Fluxo de auditoria (UI Analyzer → Rule Engine)

```
UI SOURCE → UI ANALYZER → RULE ENGINE → PASS | VIOLATION
                                            ↓
                                       EVIDENCE
                                            ↓
                                       PROPOSAL
                                            ↓
                                        GATE (approval/nível)
                                            ↓
                                          FIX
                                            ↓
                                       RE-AUDIT
```

### Níveis de operação (quem pode modificar)
- **LEVEL 1**: observa/analisa (audit, evidência).
- **LEVEL 2**: modifica UI/código dentro das políticas (tokens, estados, consistência).
- **LEVEL 3**: altera arquitetura crítica → **exige consentimento** (Don/professor).

---

## 7. Modelo de saída (COSCA UI AUDIT)

```
UI AUDIT — cosca-desktop
Architecture      97/100   Consistency  94/100   Accessibility 91/100
Interaction       96/100   Performance  98/100   Visual hierarchy 93/100
Violations: 7
[HIGH] Sidebar icon sem estado de interação.
[MED] 3 componentes burlam tokens semânticos.
[MED] Resize sem equivalente por teclado.
[LOW] 2 valores de spacing arbitrários.
```

**A regra acima de tudo**: não adicionar complexidade só porque é possível.
O melhor desktop é aquele em que "era óbvio que deveria funcionar assim."

---

## 8. Registro de violações conhecidas (triage)

> Violações aceitas/adiadas propositalmente (com regra e justificativa). Não são
> correções automáticas por gosto; são dívidas registradas para evitar risco de
> regressão em código de alto acoplamento (mesmo quando tecnicamente corrigível).

### [MED] C4 — Botões de ícone isolado sem `aria-label` (icon-only)
- **Regra normativa (C4)**: "aria-labels em botões de ícone isolado." O `title`
  não substitui o `aria-label` — `title` depende de hover (não é acessível por
  teclado/leitor de tela de forma confiável).
- **Estado atual (FACT)**: em `frontend/src/App.tsx`, os botões de ícone do rail
  (`.rail-btn`, railSections) e o `.iconbtn` de tema possuem `title`, mas não
  `aria-label`. Os botões do `diff-list`/`term-input`/`tech-ev` já usam
  `aria-label` (parcela auditável).
- **DECISION**: adiar (triage) — NÃO editar `App.tsx` nesta rodada. Os `rail-btn`
  têm `title` + alta densidade e o componente é renderizado no meio de lógica de
  estado (`activeRail`, `onRail`, `closeFn`); mexer aqui tem risco de regressão de
  layout/estado sem ganho proporcional. Registrado como violação **MEDIUM**.
- **Ação futura sugerida (gated)**: adicionar `aria-label={it.label}` nos `rail-btn`
  e `aria-label="alternar tema"` no `iconbtn`, validando build e estados de foco.
