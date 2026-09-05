# COSCA DESKTOP — IMPLEMENTATION INVENTORY

> **Tipo**: Auditoria de implementação (fonte de verdade = código + testes + runtime, NÃO resumo de sessão)
> **Autor**: cosca-kernel (orquestrado) · cosca-backend/frontend/uiux/security (execução) · explore (auditoria)
> **Data**: 2026-08-25

---

## Legenda

`VERIFIED` funciona + integrado + testado · `IMPLEMENTED` funciona, integração a fechar · `PARTIAL` parte real, parte falta · `STUB` só UI/sem comportamento real · `NOT_FOUND` inexistente · `UNKNOWN` não provado.

## Tabela de inventário cruzado (RFC §6, 19 áreas)

| # | Área | RFC§6 | Código | UI | Runtime | Testes | Estado | Evidência |
|---|------|-------|--------|----|---------|--------|--------|-----------|
| 1 | Home | §6.1 | `DiscoverProjects` | proj-card grid | — | parcial | **PARTIAL** | app.go:78, App.tsx:411; faltam sessões/estado/atalhos |
| 2 | Projects | §6.2 | create/init/open/discover/validate | fluxo completo | `cosca init` REAL | sim | **IMPLEMENTED** | app.go:78-194; falto import/arquivar/duplicar |
| 3 | Agent Workspace | §6.3 | plan/exec/approve reais | workbench UI | `cosca plan/exec/approve` | sim | **PARTIAL/STUB** | app.go:821-947; fluxo tools/files/diff/tests/result = placeholder |
| 4 | Agent Activity | §6.4 | *só estado client-side* | timeline | **NÃO** (sem eventos) | não | **PARTIAL** | App.tsx:561-573; `setActivity` local, sem stream |
| 5 | Approval Center | §6.5 | state-machine fail-closed | decisão real | authority = runtime | sim | **VERIFIED** | app.go:706-772, testado; falte FILES/NETWORK reais |
| 6 | Sandbox Center | §6.6 | **ausente** | rail "not available" | authority = runtime | não | **NOT_FOUND/STUB** | App.tsx:291; sem estado/método real |
| 7 | Terminal | §6.7 | **ausente** | "not available" | authority = Gate/execpolicy | não | **NOT_FOUND/STUB** | App.tsx:277,603; sem backend |
| 8 | File Explorer | §6.8 | TreeDirs+Read/Write (safeJoin) | tree | Rails principle | sim | **IMPLEMENTED** | app.go:361-427; falte git status/rename/move/delete |
| 9 | Editor | §6.9 | Read/Write | textarea.code | — | parcial | **PARTIAL** | App.tsx:579-583; sem highlight/tabs/split/diff |
| 10 | Diff Experience | §6.10 | **ausente** | "not available" | — | não | **NOT_FOUND/STUB** | App.tsx:554-559 |
| 11 | Command Palette | §6.11 | real, Ctrl+K | 14 cmd | — | parcial | **VERIFIED** (limitado) | App.tsx:332-405; vários `available:false` |
| 12 | Search | §6.12 | **ausente** | "not available" | `knowledge/search`+`session/search` do runtime | não | **NOT_FOUND/STUB** | App.tsx:343 |
| 13 | Session System | §6.13 | **ausente** | só rótulo | runtime: `cosca session`, FTS5 | não | **NOT_FOUND** | App.tsx:543; sem struct/resume/fork |
| 14 | Multi-agent | §6.14 | ausente | ausente | runtime: `agentbridge` | não | **NOT_FOUND** | `AGENT_MODEL='deepseek'` hardcoded |
| 15 | Knowledge | §6.15 | **ausente** | "not available" | runtime: `knowledge/search`+`/v1/knowledge/search` | não | **NOT_FOUND/STUB** | App.tsx:284 |
| 16 | Memory | §6.16 | **ausente** | "not available" | runtime: `memory/search`+`/v1/memory/search` | não | **NOT_FOUND/STUB** | App.tsx:285 |
| 17 | Provenance | §6.17 | **ausente** | "not available" | runtime: `provenance show`+`.cosca/provenance.yaml` | não | **NOT_FOUND/STUB** | App.tsx:286 |
| 18 | Audit Center | §6.18 | **ausente** | "not available" | runtime: `/v1/audit/logs`+`.cosca/audit.db` | não | **NOT_FOUND/STUB** | App.tsx:287; Desktop não lê audit |
| 19 | Runtime Monitor | §6.19 | **ausente** | badges estáticos | runtime: `/v1/status`,`/v1/status/stream`(SSE),`/v1/health` | não | **NOT_FOUND/STUB** | App.tsx:240,470; texto estático |

---

## Descobertas-chave da auditoria

### A) Integração atual é request→response de texto (LACUNA ESTRUTURAL)
O Desktop invoca o runtime **somente via `exec.CommandContext` + `cmd.CombinedOutput()`** (bloqueante) para `cosca plan/exec/approve/init`. **NÃO há** modelo de evento/stream (zero `StdoutPipe`/`io.Pipe`/`EventsEmit`/`EventsOn`). Consequência: Agent Activity, Session, Provenance, Runtime Monitor, Diff e o fluxo real **request→plan→actions→tools→files→diff→tests→result** **não são observáveis** — a UI só tem estado fabricado client-side ou placeholder.

### B) O runtime JÁ TEM o protocolo de eventos (não reinventar)
O daemon `cosca serve` expõe:
- **SSE**: `POST /v1/run/stream` (texto incremental: `thinking`→`response`→`done`), `GET /v1/status/stream` (runtime), `GET /v1/agentbridge/sessions/{id}/events/stream` (eventos de agente, com `Last-Event-ID` resume), `POST /v1/knowledge/sync/stream`.
- **WebSocket**: `GET /v1/ws` (Hub, subscribe topic `chat`).
- **Modelo de eventos de agente** (`internal/agentbridge/events.go`): 16 famílias — `stream-start, text-start/delta/end, reasoning-delta, tool-call, tool-approval-request, tool-result, finish-step, finish, file-change, error`.
- `cosca exec` **NÃO** emite eventos (é request→response). O streaming real é o daemon.

### C) Autoridade no runtime (o Desktop NÃO pode duplicar/reimplementar)
`internal/execpolicy` (allow/prompt/forbidden), `internal/gate` (TransitionTable plan→approving→approved→executed, guarda de papel don/admin), `internal/policy`+`dangerous.go`, `internal/chat/sandbox` (Rails + Gate), `internal/level`, `internal/integrity` (loyalty/identidade), `internal/audit`/`trace`/`ledger` (append-only tamper-evident), `internal/provenance`, `internal/memory`/`knowledge`/`embeddings`/`vector`, `internal/orchestration`/`pipeline`/`agentbridge`. **Desktop = superfície; Runtime = autoridade.**

### D) Desalinhamento documentação×código (honestidade)
Os `ADR-0003/0004/0005`, `GATE_REPORT` e parte do `ARCHITECTURE` descrevem capacidades (sandbox, isolamento, memory, audit) que pertencem ao **projeto Cosca pai**, não ao Desktop. O Desktop é uma **casca fina** que executa o binário externo — NÃO importa `internal/...`. Ler esses ADRs como se o Desktop tivesse sandbox/memory reais é ilusão de documentação.

### E) Estado do projeto (Desktop)
- 13 testes, todos PASS (`go test -count=1 ./...` → `ok cosca-desktop`).
- Backend: 100% real (zero placeholder). Frontend: real em Projeto/Filesystem/Agent plan+exec+approve/Approval/CommandPalette/Design System; placeholder honesto em Terminal/Knowledge/Memory/Provenance/Audit/Sandbox/Runtime/Diff/Search/Session/Multi-agent.
- Design System obedecido (panes, radius 4/6/8, accent contido `#3f79c4`, sem gradiente/glass; `App.tsx` 100% `var(--…)`).

---

## O que existe REAL vs STUB (resumo)

**REAL (funciona + testes):** projeto (criar/init-real/abrir/descobrir), filesystem seguro (tree/read/write, safeJoin+nome+symlink+sensível), agent (plan/exec/approve reais + gate fail-closed), approval state-machine, command palette, design system.

**STUB/placeholder ("not available"):** tools/files/commands/diff/tests/result, terminal, rail (knowledge/memory/provenance/audit/sandbox/runtime), search real, badges runtime/git/network/session (texto estático).

**NOT_FOUND:** session system, multi-agent, sandbox center real, diff, runtime monitor real, search backend, audit center.
