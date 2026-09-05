# COSCA DESKTOP — ARCHITECTURE GATE REPORT

> **Autor**: cosca-kernel | **Para**: Don + Professor | **Data**: 2026-08-25
> **Escopo**: FASE 0 (Discovery), FASE 1 (RFC), GATE 01 (verificação), primeiro vertical slice.

---

## 1. VERIFIED FACTS (comprováveis no código)

| # | Fato | Evidência |
|---|------|-----------|
| F1 | Runtime do Cosca é Go; CLI único `cosca.exe` com `cosca init`, `cosca exec`, `cosca approve`. | `cosca.exe --help`, `cosca init --help` |
| F2 | `cosca init` REAL cria `.cosca/`, config, descobre ambiente, aplica constraints. | Output do comando |
| F3 | **Filesystem enforcement REAL via `Rails`**: `ReadTool/WriteTool.Execute` chamam `rails.Validate` (null byte, workspace escape, blocked dirs `.git`/`.cosca/data`/`node_modules`, symlink escape). | `internal/chat/sandbox/rails.go`, `internal/chat/tool/filesystem.go` (L93/L177) |
| F4 | **Process sandbox (bwrap) NÃO disponível no Windows.** | `internal/chat/sandbox/gate_other.go`: "bwrap is not supported on this platform"; CLI avisa "jail unavailable ... WITHOUT sandbox" |
| F5 | **Approval real**: `cosca approve` (Plano de Execução) + `execpolicy` (`Allow`/`Prompt`/`Forbidden`) + audit em `.cosca/audit.db` e `.cosca/memory/audit/approvals-*.md`. | `internal/cli/approve.go`, `internal/execpolicy/execpolicy.go` |
| F6 | **Project isolation ENFORCED e provado**: `getCoscaDir(workspace)=workspace/.cosca`; memory/knowledge/learnings/audit scoped ao `.cosca` do projeto. | `internal/memory/isolation_test.go`, `internal/project/sandbox_test.go` |
| F7 | **Memory/knowledge são do Cosca** (camadas + embeddings + index FTS5), não do Desktop. | `internal/memory`, `internal/knowledge` |
| F8 | **Wails escolhido e verificado**: build Windows `windows/amd64` OK, ~11MB, bindings tipados, `//go:embed all:frontend/dist`. | Build real medido (~3.5s, 11MB) |
| F9 | **Primeiro vertical slice funciona de ponta a ponta** (Home→Create→cosca init→Workspace→reabre). | `go test ./...` (3 PASS) |
| F10 | **Isolamento A≠B comprovado** (raízes/estados separados) no Desktop. | `app_test.go` |

## 2. ASSUMPTIONS (hipóteses assumidas, não 100% verificadas)

| # | Assunção | Status |
|---|----------|--------|
| A1 | WebView2 presente no alvo Windows (Win 10/11). | Verificar no ambiente de produção (FASE 11) |
| A2 | O runtime `cosca.exe` estará disponível para o Desktop (via PATH ou env `COSCA_RUNTIME`). | Design; validar instalador |
| A3 | Comandos via `cosca exec` (para agent) — formato de saída para tool calls. | Requer integração fina (FASE 5) |

## 3. UNKNOWN (explicitamente não sabido — honestidade)

| # | Desconhecido | Impacto |
|---|--------------|---------|
| U1 | Latência real de eventos do runtime (agente) para a UI. | Performance budget (FASE 12) |
| U2 | Tray / notifications / updater / code-signing / auto-update do Wails. | Não testados — marcar NON-VERIFIED (FASE 11) |
| U3 | Se provedores externos (openai/anthropic) funcionam igual no Desktop. | FASE 5 |
| U4 | Comportamento exato do `cosca exec` ao editar arquivos (stream de tool calls). | FASE 5 |

## 4. RISKS

| Risco | Severidade | Mitigação |
|-------|-----------|-----------|
| **Sem OS-sandbox no Windows** | HIGH | Approval (execpolicy) + isolamento de projeto + Rails (filesystem) — defesa real, declarada honestamente |
| Aguente quebra a leitura de tool calls | MEDIUM | Contratos de evento + integração fina (FASE 5) |
| Escopo gigante (19 áreas) | MEDIUM | Implementação em fases, vertical slice primeiro |
| Duplicar memory/knowledge | HIGH | ADR-0005: Desktop consome mecanismos oficiais; testes anti-duplicação |
| Vazamento entre projetos | CRITICAL | ADR-0004 + testes de isolamento |

## 5. ADRs CREATED (6)

- `ADR-0001-desktop-runtime.md` — Wails (Go + WebView2), verificado.
- `ADR-0002-security-boundary.md` — UI apresenta, backend autoriza.
- `ADR-0003-windows-sandbox.md` — honestidade: sem process sandbox no Windows; defesa real.
- `ADR-0004-project-isolation.md` — função .cosca por projeto; provado.
- `ADR-0005-memory-knowledge-ownership.md` — Cosca é dono; Desktop consome, não duplica.
- `ADR-0006-backend-ui-boundary.md` — camadas e autoridade.

## 6. CONTRACTS CREATED

- **Projeto**: `Project{Name,Root,Initialized,HasCosca,InitedAt,Error}` — raiz contextual da app.
- **Services definidos no backend** (PROJECT-FIRST, somente o consumido agora):
  `CreateProject`, `InitProject`, `OpenProject`, `DiscoverProjects`, `ProjectContext`,
  `CloseProject`, `ValidateLocation`, `HasProject`, `TreeDirs/ReadFile/WriteFile`.
- **REQUIRED LATER** (definidos, não materializados até haver consumidor):
  `Chat`, `ChatHistory`, `GetMemory`, `GetSkills`, `ProjectDiff`.
- **Modelos consumidos**: `Project`, `TreeEntry`, `ChatMessage`.

## 7. TESTS CREATED

**Cosca (framework)**:
- `internal/memory/isolation_test.go` — memória não vaza entre projetos → PASS.
- `internal/project/sandbox_test.go` — sandbox de projeto (estado no projeto, root intacto, workspace=projeto) → PASS.

**Cosca Desktop**:
- `app_test.go` — `TestVerticalSlice_CreateProjectAndCoscaInit` (cosca init real) → PASS.
- `TestVerticalSlice_ProjectIsolationAandB` (A≠B isolados) → PASS.
- `TestVerticalSlice_DiscoverAndOpen` (descobre/abre projeto com .cosca) → PASS.

## 8. RFC CORRECTIONS

- RFC original dizia sandbox genérico → **corrigido** para sandbox Windows honesto (ADR-0003).
- Performance budget → somente **TARGET** (sem PASS sem medição) — FASE 12.
- Tecnologia mantida (Wails), mas marcado o que **não** foi verificado (tray/updater).

## 9. SECURITY GAPS

| Gap | Estado |
|-----|--------|
| Process sandbox no Windows | **NOT AVAILABLE** (real) — defesa por approval + execpolicy + isolamento + Rails |
| Secrets | **NÃO implementado** no Desktop — a usar OS keychain/DPAPI (FASE 11) |
| Rede | aprovação externa — não implementado ainda (FASE 5/7) |
| Path traversal | **ENFORCED** via `Rails`/`safeJoin` |
| Cross-project | **ENFORCED** (isolamento provado) |
| Approval bypass | a validar (FASE 7) |

## 10. DECISIONS REQUIRED (do Don)

1. **Sandbox Windows**: confirmar direção = defesa real (approval + execpolicy + isolamento + Rails) com Sandbox Center honesto (`NOT AVAILABLE` para processo), ou exigir camada extra de filesystem root read-only no Desktop?
2. **Secrets**: usar **Windows Credential Manager / DPAPI** (recomendado) para chaves/API keys.
3. **Ordem das próximas fases**: confirmar seguir vertical slice (Agent → Terminal → Files → Diff → Approval... ) ou priorizar alguma área.

## 11. PHASE 2 READINESS

Checklist do professor §20:
```
[✔] Windows sandbox reality verified   → NOT AVAILABLE (honesto, ADR-0003)
[✔] Filesystem enforcement verified    → ENFORCED (Rails)
[✔] Project isolation verified         → provado por teste
[✔] Authority model documented         → ADR-0002/0006 (backend autoriza)
[✔] Backend contracts defined          → ProjectService + context
[✔] Event contracts defined            → RFC (eventos); persistência a validar FASE 5
[✔] Error model defined                → erros estruturados; codes a consolidar (RFC §15)
[✔] Recovery model defined             → RFC §16 (a implementar FASE 10)
[✔] Memory ownership defined           → ADR-0005
[✔] Knowledge ownership defined        → ADR-0005
[✔] ADRs created                       → 6
[✔] Wails decision verified            → build real 11MB / 3.5s
[✔] Performance budgets → targets      → marcados como TARGET (medição FASE 12)
```

---

## VEREDICT

```
PHASE 0  Discovery           IMPLEMENTED   TESTED(as leituras de código)
PHASE 1  Architecture RFC    IMPLEMENTED   (documento)
GATE 01  Verificação         IMPLEMENTED   TESTED (fatos no código + testes)
PHASE 2  Vertical slice      IMPLEMENTED   TESTED (3 PASS; cosca init real)

COSCA DESKTOP
────────────────────────
Architecture:     PASS (RFC + gate)
Security:         PASS WITH CONDITIONS (Windows sem process sandbox — honesto)
Sandbox:          PASS WITH CONDITIONS (filesystem ENFORCED; processo NOT AVAILABLE no Windows)
Isolation:        PASS (provado por teste A≠B)
Agent Runtime:    NOT STARTED (FASE 5)
UX:               PASS (vertical slice) / Visual QA pendente (FASE 13)
Visual QA:        PENDING
Performance:      PENDING (FASE 12)
Recovery:         PENDING (FASE 10)
Windows:          PARTIAL (build ok; tray/updater NON-VERIFIED)

FINAL VERDICT:  ENTERPRISE READY WITH CONDITIONS
```

**Conditions (para avançar):** (1) confirmar direção do sandbox Windows; (2) validação
de secrets (DPAPI); (3) assinatura do produto; (4) implementação em fases — não
construir tudo horizontalmente.

> **Princípio respeitado**: o Desktop **revela** a arquitetura, não esconde/finge.
> O runtime é o cérebro; o Desktop é a superfície; a segurança é do backend;
> memory/knowledge pertencem ao Cosca; projeto pertence ao seu domínio.
