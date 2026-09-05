# COSCA DESKTOP — IMPLEMENTATION GAP PLAN

> **Derivado de**: IMPLEMENTATION_INVENTORY.md · RFC §6 · ADRs · auditoria do runtime Cosca
> **Autor**: cosca-kernel · **Data**: 2026-08-25

---

## DECISÃO ARQUITETURAL PRINCIPAL (confirma/refina a RFC)

**Integração por DAEMON (REST/SSE/WS) como canal primário** — não request→response de texto.

- O Desktop hoje executa `cosca plan/exec/approve/init` via `CombinedOutput()` (bloqueante, sem eventos).
- O runtime já tem `cosca serve` (REST + gRPC + SSE + WebSocket) e o modelo de eventos de agente (`agentbridge`: `tool-call`, `tool-approval-request`, `tool-result`, `finish-step`, `file-change`, `reasoning-delta`, `text-delta`, `finish`...).
- **Mudança**: o Desktop passa a rodar/consumir `cosca serve --host 127.0.0.1 --port 14120` (por projeto, com `--data-dir <projeto>/.cosca`) e a assinar eventos por SSE/WS.
- **Autoridade NUNCA no Desktop**: plan/approve/execpolicy/Rails/gate/memory/knowledge/audit/provenance/recovery continuam 100% no runtime. O Desktop **apresenta/observa/delega**.

Isso desbloqueia simultaneamente: Agent Activity observável, Session, Provenance, Runtime Monitor, Diff (via `file-change`/`tool-result`), Search (knowledge/session), Audit Center — todos consumindo o mecanismo oficial, **sem duplicar nada**.

---

## Priorização

| Prioridade | Critério | Itens |
|-----------|----------|-------|
| **P0** | Bloqueia Desktop funcional | Canal de eventos (daemon SSE/WS) · Agent Workbench observável (activity/fluxo real) · Approval com dados reais do plano |
| **P1** | Necessário para substituir OpenCode | Diff · Terminal · Sandbox Center · Runtime Monitor · Session · Memory/Knowledge (consumir) · Search |
| **P2** | Enterprise hardening | Provenance · Audit Center · Command Palette completo (sem placeholder falso) · Multi-agent · Recovery |
| **P3** | Polish/futuro | Windows integration (installer, updater, code-signing, WebView2 check, secrets DPAPI, tray) · Performance (métricas reais) · Visual QA |

---

## DIVISÃO EM FASES (vertical slice, fechando backend→binding→UI→runtime→teste)

### FASE A — Canal de eventos (P0) [raiz de tudo]
- Subir `cosca serve` por projeto (daemon REST/SSE/WS) + auth (JWT/API key).
- Cliente SSE/WS no Desktop (chat, agent eventos, status stream).
- **VERIFICADO no runtime**: `POST /v1/run/stream` (SSE), `GET /v1/agentbridge/sessions/{id}/events/stream`, `GET /v1/status/stream`, `GET /v1/ws`.
- **Não duplicar**: `agentbridge`/event model são do runtime; o Desktop é só cliente.

### FASE B — Agent Workbench observável (P0)
- UI representa estados reais: `thinking / planning / waiting-approval / executing / tool-call / reading / writing / terminal / diff / completed / failed / cancelled`.
- Activity timeline alimentada por **eventos reais** (não `setActivity` client-side).
- Approval Center com dados reais do plano (files, scope, risk, policy, execpolicy).

### FASE C — Diff (P1)
- Consumir `tool-result`/`file-change` do runtime para calcular added/modified/deleted; before/after; review; accept/reject (se suportado pelo runtime).
- Preservar isolamento do projeto.

### FASE D — Terminal (P1)
- **Se ausente** (confirmado: sem engine de processo no Desktop). Implementar via **execpolicy/Gate do runtime** — NUNCA executor paralelo que contorne Rails/execpolicy/approval.
- Se o runtime expuser processo via REST/SSE, consumir; senão minimal via `cosca exec` com política.

### FASE E — Sandbox Center + Runtime Monitor (P1/P2)
- **Sandbox Center**: mostrar estado REAL. Windows → `NOT AVAILABLE (bwrap é Linux-only)` + defesa por approval/execpolicy/isolamento (honesto). No Linux → bwrap ativo.
- **Runtime Monitor**: consumir `GET /v1/status`,`/v1/status/stream`,`/v1/health`,`/metrics`. Métrica inexistente = `NOT AVAILABLE` (não fabricar).

### FASE F — Memory / Knowledge / Session / Search (P1)
- **Consumir, NUNCA duplicar**: `/v1/memory/search`, `/v1/knowledge/search`, `cosca session search`, `.cosca/*.db` (leitura read-only).
- Apenas camada de apresentação/consumo. Zero DesktopMemory/DesktopKnowledge.

### FASE G — Provenance / Audit / Recovery (P2)
- Provenance: `cosca provenance show`/`/v1/…` + `.cosca/provenance.yaml`.
- Audit Center: `/v1/audit/logs` + `.cosca/audit.db` (leitura).
- Recovery: consumir protocolo do runtime (`pipeline/recovery`, `durable RecoverStale`, `agentbridge` resume). Não inventar state machine de recovery.

### FASE H — Windows integration (P3)
- WebView2 check, installer, updater, code-signing, secrets (Credential Manager/DPAPI), tray. **Só após core fechado.**

### FASE I — Performance (P3)
- Medir de verdade: startup, first paint, project open, cosca init, agent event latency, memory, CPU. **TARGET ≠ PASS** — sem medição, não marcar PASS.

---

## NÃO DUPLICAR (anti-duplication invariants)
- DesktopMemory / DesktopKnowledge / DesktopAudit / DesktopProvenance / DesktopSecurity — **PROIBIDO** se o mecanismo oficial existe no Runtime.
- NUNCA reimplementar execpolicy/approval/Rails/gate/level/integrity.
- Desktop = superfície; Runtime = autoridade; Projeto = território; UI = observabilidade+controle.

---

## CONFLITO implementação×arquitetura (documentado, §do protocolo)
- **A implementação atual** (request→response via `CombinedOutput`) **NÃO segue** a RFC §7 (event architecture). 
- **Resolução pela autoridade**: RFC §7 + runtime `agentbridge`/`api/stream` mandam integrar por daemon/eventos. **PAREI a implementação da FASE B/A até o Don validar** o redirecionamento para `cosca serve` (mudança de base, decisão arquitetural do Don/professor).

---

## RESULTADO DEFINITIVO DA AUDITORIA (classificação por área)
**VERIFIED**: Approval Center, Command Palette, Design System. **IMPLEMENTED**: Projects, File Explorer. **PARTIAL**: Home, Agent Workspace, Agent Activity (sem eventos), Editor. **STUB**: Tool panes, Terminal, Sandbox Center, Knowledge, Memory, Provenance, Audit, Runtime Monitor, Search, Diff. **NOT_FOUND**: Session System, Multi-agent, Audit Center real, Diff real, Runtime Monitor real.

---
*A re-implementação deve sempre: verificar mecanismo existente → definir contrato → backend → Runtime → bindings → UI → Design System → testar → executar → verificar → documentar.*
