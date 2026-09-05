# COSCA DESKTOP — Architecture RFC

> **Status**: FASE 1 — Architecture RFC (sem código de UI)
> **Autor**: cosca-kernel | **Ordem**: Don + professor (Enterprise Product Mission)
> **Data**: 2026-08-25

---

## 0. Parecer

O **Cosca Desktop** não é "chat + terminal + editor". É o **COSCA ENGINEERING
WORKSPACE**: a superfície através da qual um humano controla, observa e colabora
com o Cosca. O **runtime continua sendo o cérebro operacional**; o Desktop é a
**superfície de interação**; a **segurança pertence ao backend/kernel**; a
**autoridade pertence ao backend**; o **projeto pertence ao seu domínio**; e
**memory/knowledge continuam pertencendo ao Cosca** (o Desktop consome, não duplica).

> **ATUALIZAÇÃO (ADR-0007) — Independência do Binário + Forge.** O cosca-desktop
> é um produto **INDEPENDENTE e distribuível**, que roda em qualquer PC/pasta sem
> depender do cosca root. Ele tem DOIS modos, definidos por DETECÇÃO do root
> (nunca por cópia):
> - **STANDALONE** (fora do root): entra **Agents + Skills**, mas **NÃO entra o
>   Kernel** (a inteligência que opera/orquestra de verdade) **nem a memória/
>   legado da família**. Opera via IA externa/local (preferência do cliente).
> - **FORGE** (detecta o root): entra o **Kernel** e **revela tudo do root**
>   (agents, skills, memória da família, config, arquitetura, family chain, DNA).
> - **Invariante**: o conteúdo do root (kernel + legado + estrutura) **NUNCA é
>   copiado para o binário** — é acessado/revelado a partir do root quando
>   detectado. O binário é sempre o mesmo.

---

## 1. DECISÃO DE TECNOLOGIA

**Escolha: Wails (Go + WebView2).**

Justificativa técnica (não por popularidade — ver `docs/research/LEARNINGS.md`):

| Critério | Electron | Tauri | **Wails (Go+WebView2)** |
|----------|:--------:|:-----:|:----------------------:|
| Linguagem backend | Node | Rust | **Go (mesma do runtime Cosca)** |
| Integração com runtime Cosca | via IPC/fronteira | via IPC/fronteira | **nativa (pacotes Go, mesma engine)** |
| RAM/startup | alto | baixo | **baixo (WebView nativo)** |
| Binário | pesado | leve | **~11MB, único** |
| Integração Windows | ok | ok | **WebView2 nativo** |
| Segurança | ok | melhor | **ok (backend Go, trust no backend)** |

**Consequência chave**: como o backend é Go, o Desktop pode consumir **diretamente**
os pacotes do Cosca (engine, memory, knowledge, execpolicy, audit, provenance) sem
duplicar cliente — ou, se preferir desacoplar, via CLI/daemon. Mantém o Cosca como
fonte de verdade.

---

## 2. BACKEND BOUNDARY (autoridade)

```
UI (React/WebView)
      │  (métodos Wails; a UI NUNCA tem regra crítica)
      ▼
Desktop Application Layer (Go/Wails backend)
      │  autentica, autoriza, orquestra, traduz
      ▼
Cosca Client / API  (execpolicy, session, project)
      ▼
Cosca Runtime  (kernel, agents, skills, workflows)
      ▼
Kernel
```

- **A UI apresenta e solicita.** O backend autoriza e executa.
- **Nenhuma regra crítica na UI.** Não existe "esconder botão" como segurança.

---

## 3. PROJECT ISOLATION (invariante)

```
GLOBAL COSCA
      │
      ├── PROJECT A  →  .cosca/  (state, memory, knowledge, sessions, provenance, audit)
      └── PROJECT B  →  .cosca/  (idem, separado)
```

- O Desktop **nunca cria armazenamento paralelo** que quebre essa separação.
- Memory/knowledge/audit do projeto vivem no `.cosca/` do projeto (já verificado
  por `internal/memory/isolation_test.go` e `internal/project/sandbox_test.go`).
- O Desktop consome esses mecanismos oficiais; **não duplica** memory/knowledge.

---

## 4. DESIGN SYSTEM (tokens)

Antes de qualquer componente, definir tokens (sem valores arbitrários no código):

```text
color.background.base|raised|overlay
color.surface.panel|sidebar|activity|terminal
color.border.subtle|default|strong
color.text.primary|secondary|muted|link
color.accent.primary|secondary|hover
color.status.ok|warn|danger|info|neutral
spacing.1..8          (escala 4px)
radius.sm|md|lg|full
typography.family|size.*|weight.*|lineheight.*
motion.duration.*|easing.*
elevation.shadow.*
iconography.size.*
states: hover|active|focus|disabled|loading|empty|error|success
```

Tema claro/escuro via tokens. Acessibilidade AAA (contraste), keyboard-first.

---

## 5. LAYOUT PRINCIPAL (workspace)

```
┌──────────────────────────────────────────────────────────────┐
│ COSCA │ Projeto │ Command/Search(⌘K) │ Sessão │ Status │ User │
├──────┬────────────────────────────────────────────┬──────────┤
│      │                                            │          │
│ APP  │            WORKSPACE                       │ CONTEXT  │
│ NAV  │   (agent workspace / diff / review)        │ PANEL    │
│      │                                            │          │
├──────┴────────────────────────────────────────────┴──────────┤
│ TERMINAL / OUTPUT / DIFF / LOG / AGENT ACTIVITY (tabs)        │
├──────────────────────────────────────────────────────────────┤
│ Sandbox │ Runtime │ Agent │ Model │ Network │ Git │ Status    │
└──────────────────────────────────────────────────────────────┘
```

- **Top bar**: projeto ativo, Command Palette (⌘K), sessão, status.
- **Rail (APP NAV)**: Home, Projects, Agent, Terminal, Files, Knowledge, Memory,
  Provenance, Audit, Sandbox, System.
- **Workspace**: área principal (contexto da seção — diagrama de fluxo do agente,
  diff, editor, etc.).
- **Context panel**: detalhe contextual (aprovador de risco, origem, propriedades).
- **Bottom status**: sandbox, runtime, agente, modelo, rede, git.

---

## 6. ÁREAS PRINCIPAIS

1. **Home** — projetos recentes, sessões, estado do Cosca, atalhos, criar projeto.
2. **Projects** — criar/abrir/importar/inicializar/arquivar/duplicar; identidade própria.
3. **Agent Workspace** — mostra o FLUXO real do agente:
   `request → plan → actions → tools → commands → files → diff → tests → result`.
   Não é caixa de chat.
4. **Agent Activity** — timeline operacional (rastreável, expansível, filtrável).
5. **Approval Center** — ação, motivo, cwd, arquivos, rede, risco, sandbox, agente,
   sessão, consequência; [Reject] [Approve Once] [Approve Session].
6. **Sandbox Center** — estado REAL (mode, roots writable/read-only, network,
   processes, env, secrets, permissions, policies). **Nunca inventar sandbox.**
7. **Terminal** — tabs, split, processo, kill/restart, copy, search, scrollback, logs.
8. **File Explorer** — tree, busca, git status, rename/move/delete (destrutivo →
   approval), preview, metadata.
9. **Editor** — architecture pronto p/ syntax highlight, tabs, split, search, diff,
   breadcrumbs, symbols (expansível).
10. **Diff Experience** — added/deleted/renamed/staged/unstaged/agent changes;
    Accept/Reject/Revert/Inspect/Compare.
11. **Command Palette** — tudo por teclado (⌘K).
12. **Search** — global, separa naturezas (arquivos/comandos/sessões/knowledge/memory/
    logs/audit/provenance) indicando origem.
13. **Session System** — entidade real (id, project, agent, model, start/end, status,
    actions, tools, files, commands, approvals, result, provenance); resume/fork/
    inspect/archive/export/recover.
14. **Multi-agent** — arquitetura permite Architect/Coder/Reviewer/Tester/Researcher
    (evolução futura).
15. **Knowledge** — query, resultados, score, origem, provenance, projeto, timestamp.
16. **Memory** — origem, escopo, projeto, confiança, provenance, lifecycle. Consome o
    mecanismo oficial; NÃO cria um segundo.
17. **Provenance** — cadeia `user→session→agent→tool→file change→test→commit` navegável.
18. **Audit Center** — filtra por user/agent/project/session/action/command/file/
    timestamp/risk/approval.
19. **Runtime Monitor** — estado REAL; métrica inexistente = `NOT AVAILABLE`.

---

## 7. EVENT ARCHITECTURE

UI reage a eventos estruturados (não polling indiscriminado):

```text
project.created | project.initialized
agent.started | agent.thinking | agent.completed
tool.started | tool.completed
command.requested | approval.required | approval.granted | approval.denied
file.changed | test.started | test.completed
session.completed | runtime.started | runtime.stopped
```

O backend emite; a UI assina e renderiza.

---

## 8. SECURITY MODEL (honesto — no Windows)

- **Trust boundaries**: UI → Desktop App Layer → Cosca Client → Runtime → Kernel.
  A autoridade e a segurança ficam no backend/kernel.
- **Capability model**: operações sensíveis exigem capability concedida pelo backend.
- **Sandbox real**: o runtime Cosca no **Windows não tem jail/bwrap** (verificado:
  "jail unavailable ... run WITHOUT sandbox"). Portanto:
  - Defesa real no Windows = **approval (execpolicy Allow/Prompt/Forbidden) +
    read-only roots + isolamento de projeto (.cosca por projeto)**.
  - O Sandbox Center mostra o estado REAL (Windows → "process sandbox not available;
    enforced by approval + project isolation"). **Nunca fingir sandbox.**
  - Em Linux (bwrap) → sandbox de processo ativo (mostrar status).
- **Secrets**: OS keychain/DPAPI (Windows Credential Manager), nunca na UI/config.
- **Rede**: aprovação explícita para acesso externo.
- **Filesystem**: escrever só em roots permitidos (workspace do projeto).
- **Processos**: comandos com risco exigem approval.
- **Sessões**: isoladas por projeto.

---

## 9. PERFORMANCE BUDGET (meta — medir na FASE 12)

```text
startup                  < 1.5s
idle RAM                 < 90MB
idle CPU                 < 1%
project open             < 0.5s
agent event latency      < 100ms
terminal startup         < 0.3s
editor responsiveness    < 16ms/frame
```

---

## 10. RECOVERY / DEGRADED

- **Recovery**: se o Desktop fechar durante tarefa → restart → detectar sessão
  incompleta → show recovery → resume/inspect/discard. Nunca perder contexto.
- **Degraded**: provider/runtime/network/knowledge indisponível → UI explica o
  estado (nunca "carregando…" eterno), e não fabrica métrica.

---

## 11. IMPLEMENTAÇÃO EM FASES

```
FASE 0  Discovery                                       ✔ done
FASE 1  Architecture RFC                                ✔ done (este doc)
FASE 2  Design system (tokens)                          ◻
FASE 3  Desktop shell (layout + nav + top/bottom)       ◻
FASE 4  Project workspace (projects + open/init)        ◻
FASE 5  Agent runtime integration (activity + flow)     ◻
FASE 6  Terminal + filesystem + editor (arquitetura)    ◻
FASE 7  Approval + sandbox (reais, honestos)            ◻
FASE 8  Diff + provenance + audit                       ◻
FASE 9  Knowledge + memory (consumir, não duplicar)     ◻
FASE 10 Recovery + observability                        ◻
FASE 11 Installer + updater + Windows integration       ◻
FASE 12 Performance + security hardening                ◻
FASE 13 Visual QA                                       ◻
```

---

## 12. TEST MATRIX (núcleo)

- **Functional**: project create/init, agent exec, command, file edit, terminal,
  diff, approval, sandbox, sessions, recovery.
- **Security**: unauthorized fs, network, path traversal, permission escalation,
  cross-project access, secret leakage, approval bypass.
- **Isolation**: Projeto A NUNCA acessa Projeto B (já provado por testes memory/project).
- **Recovery**: kill durante operação → restart → recover, sem corrupção/duplicação.

---

## 13. RISCOS / DECISÕES ABERTAS

| Risco | Decisão |
|-------|---------|
| Sandbox de processo indisponível no Windows | Defesa por approval + execpolicy + isolamento de projeto; estado real no Sandbox Center |
| Duplicar memory/knowledge | NUNCA — consome mecanismos oficiais do Cosca |
| Escopo gigante | Implementação em fases (FASE 2→13), cada fase validada antes de avançar |
| Acoplamento UI↔backend | Backend boundary; UI apresenta, backend autoriza |

---

## 14. RESULTADO ESPERADO (definição de pronto)

Arquitetura documentada · backend boundary definido · isolamento verificado ·
sandbox honesto · approval model implementado · diff implementado · provenance ·
audit · sessões · recovery · terminal · file explorer · editor-ready · command
palette · keyboard-first · a11y · dark/light · Windows quality bar · performance
medida · segurança testada · Visual QA · sem estado fake · sem duplicar memory/
knowledge · sem vazamento entre projetos.

> **Princípio**: o Desktop **revela** a arquitetura do Cosca — não a esconde,
> não a finge, e o runtime continua sendo o cérebro; a segurança é do backend.
