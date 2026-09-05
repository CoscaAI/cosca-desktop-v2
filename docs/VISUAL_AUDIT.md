# COSCA DESKTOP — Visual Audit (Enterprise Workbench)

> **Data**: 2026-08-25
> **Autor**: cosca-kernel (orquestração) · cosca-uiux + cosca-frontend (execução)
> **Missão**: elevar o visual de "SaaS dashboard bonito" para "Enterprise Engineering Workbench / Professional Developer Tool".

---

## 1. Antes → Depois (transformação)

| Dimensão | ANTES (SaaS) | DEPOIS (Workbench) |
|----------|--------------|--------------------|
| Estética | cards gigantes, dashboards | panes planares (`pane ├ context ├ activity ├ editor └ output`) |
| Radius | excessivo (12+, rounded-xl) | controlado/small (`--radius-sm 4 / md 6 / lg 8`); pill só p/ status/tags |
| Sombra | em todo card | sutil, só em `.dialog`/`.palette` (`--elev-1/2`) |
| Cor | acento blue `#2563eb` (SaaS) | acento contido `#3f79c4`; superfícies de tinta azulada (nunca cinza puro) |
| Tipografia | Nunito (órfã) + headings grandes | Segoe UI Variable + escala compacta (`--fs-md 13`); monospace SÓ técnico |
| Densidade | espaçamento amplo | `--ctrl-h 30/24`, paddings reduzidos, metadata inline |
| Janela | "website numa janela" | chrome nativo: COSCA · PROJECT · ⌕Search⌘K · session · runtime · user |
| Navegação | 3 botões-ícone | rail compacto 54px, 3 grupos com separadores, tooltips, active indicator |
| Agent | chatbot/bolhas | Agent Workbench: REQUEST · PLAN · ACTIONS · STATUS · APPROVALS · ACTIVITY |
| Approval | (inexistente) | Approval Center · SECURITY (decisão, não card de confirmação) |
| Status | decorativo | status bar profissional: runtime · project · agent · model · git · sandbox · network |

---

## 2. Design System (tokens entregues)

- **Surfaces** (tinta azulada): `--bg #070a12` · `--surface #0d121d` · `--surface-2 #161e2d` · `--surface-3 #1d2839` · `--overlay rgb(4 7 12/.62)`
- **Borders**: `--border #1f2a3c` · `--border-strong #31415c`
- **Text**: `--text #e9eef7` · `--text-2 #9aa8c0` · `--text-3 #75829e`
- **Accent** (contido): `--accent #3f79c4` · `--accent-strong #2c5da0` · `--accent-soft` (color-mix 14%) · `--on-accent #fff`
- **Status** (reconhecível sem só cor): `--ok #42b26a` · `--warn #dfa03c` · `--danger #e25a5a` · `--info #4f9fd0` · `--neutral #8190aa`
- **Code** (editor sempre escuro): `--code-bg #040709` · `--code-fg #d3e0ff`
- **Spacing** (escala 4px): `--sp-1..7` = 4/8/12/16/24/32/48
- **Radius reduzido**: sm 4 · md 6 · lg 8 · full 999
- **Typography**: `--font-family "Segoe UI Variable Text"` · `--font-mono "Cascadia Code"` · `--fs-xs 11/sm 12/md 13/lg 14/xl 17` · `--fw 400/500/600/700` · `--lh-tight 1.25 / --lh-normal 1.5`
- **Motion** (estado, rápido): `--dur-fast 100ms` · `--dur-normal 160ms` · `--ease-out cubic-bezier(.2,.7,.2,1)`
- **Elevation sutil**: `--elev-1` · `--elev-2` (só dialog/palette)
- **Controles**: `--ctrl-h 30px` · `--ctrl-h-sm 24px` (densidade)

---

## 3. Approval Center (opção B — integração real)

Fluxo consumindo os métodos do backend (bindings regenerados):
`RequestApproval(req, model, 'exec')` → painel "Approval Center · SECURITY" mostra:
- **ACTION**: cosca exec · **COMMAND** (request, tom `--danger`) · **PROJECT** · **WORKING DIR** (mono) · **FILES / NETWORK** (`—` sem backend + honesto) · **POLICY**: fail-closed · execpolicy · **RISK** (cor via `riskTone()`: exec→danger, write/network→warn, read→info) · **SANDBOX** · **AGENT** (model) · **SESSION** (single-use)
- Botões: **DENY** (`DenyPending`) · **APPROVE ONCE / APPROVE SESSION** (`ApprovePending` → `AgentRun`)
- Backend é **fail-closed**: sem `approved` registrado, `AgentRun` NÃO executa.

---

## 4. Command Palette

- Trigger `Ctrl+K`/`Cmd+K` global; `Esc` fecha; `↑/↓` navegam; `Enter` executa; busca substring.
- 14 comandos (Open Project, New Project, Open File, Run Command, Start/Stop Agent, Plan Agent, Open Terminal, Search, Approvals, Sandbox, Runtime, Settings, Toggle Theme).
- Comandos sem tela real → marcados `ⓘ not available` (não executam — honestidade, sem estado fake).

---

## 5. Verificação (medida)

| Item | Resultado |
|------|-----------|
| `npm run build` (tsc + vite) | ✅ OK — `✓ built in 665ms` (31 módulos, CSS 21.30kB, JS 217.39kB) |
| `go build ./...` | ✅ OK |
| `go vet ./...` | ✅ OK |
| `go test -count=1 ./...` | ✅ `ok cosca-desktop` (build/test fresh) |
| Cores hardcoded no `App.tsx` | ✅ **0** (100% tokens via `var(--…)`; 33 usos) |
| Bindings de aprovação | ✅ `RequestApproval / PendingApproval / ApprovePending / DenyPending` presentes |
| Funcionalidade anterior | ✅ intacta (Discover/Create/Init/Open/Close/Tree/Read/Write/Plan/Run/Approve) |

---

## 6. Remaining Issues / NOT AVAILABLE (honestidade)

- **Network, git(branch/tag), SESSION scope, FILES target do approval**: sem backend no Desktop → exibidos `—`/`not available`. Nunca valores fake.
- **TOOLS · FILES · COMMANDS · DIFF · TESTS · RESULT** (workbench): `not available — sem serviço de backend no Desktop`.
- **Rail**: TERMINAL, KNOWLEDGE, MEMORY, PROVENANCE, AUDIT, SANDBOX, RUNTIME → visuais, respondem "not available" (sem tela real ainda).
- **APPROVE SESSION**: usa a mesma `ApprovePending` do single-use (o runtime Cosca não persiste escopo de sessão no Desktop) — sinalizado no painel.

---

## 7. Veredito

```
Visual QA (Enterprise Workbench):      PASS
Functional tests (backend):            PASS
Enterprise Workbench assessment:       PASS (panes, densidade, identidade própria, não-cópia)
Remaining SaaS signals:                MÍNIMOS (só áreas ainda sem backend, expostas como NOT AVAILABLE)
```

**Princípio respeitado**: o Desktop revela a arquitetura e o estado real — nunca finge. O runtime resta o cérebro; a UI agora parece o que é: uma ferramenta de engenharia que um profissional usa 8h/dia.
