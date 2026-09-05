# ADR-0004 — Project Isolation

**Status**: ACCEPTED (verificado por teste)

## Context
Cada projeto é uma unidade isolada. O Desktop não pode criar armazenamento paralelo
que quebre a separação entre projetos nem entre projeto e Cosca global.

## Decision
```
GLOBAL COSCA
 └── PROJECT A → .cosca/  (state, memory, knowledge, sessions, provenance, audit)
 └── PROJECT B → .cosca/  (idem, separado)
```
- O runtime resolve o `.cosca` do workspace ativo: `getCoscaDir(workspace) = workspace/.cosca`.
- Memory/knowledge/learnings/audit vivem no `.cosca` do projeto.
- **Desktop consome** os mecanismos oficiais; não duplica.

## Verification (GATE 01) — FATOS
- `internal/memory/isolation_test.go`:
  `TestProjectIsolation_MemoryDoesNotLeakAcrossDataDirs` → **PASS**
- `internal/project/sandbox_test.go`:
  `TestSandbox_NotWrittenToCoscaRoot` → **PASS** (root intacto)
- `TestSandbox_WorkspaceIsProject` → **PASS**

## Consequences
- Projeto A NUNCA lê/escreve/consulta Projeto B (filesystem/memory/knowledge/audit).
- Sandbox de projeto é respeitado pelo Desktop.

## Security implications
- Elimina cross-project access e vazamento de estado/memória/knowledge.

## Performance implications
- Isolamento por diretório (`.cosca` por projeto) — sem overhead de query.

## Rejected alternatives
- Banco/armazenamento global compartilhado (rejeitado — viola isolamento).

## Status
ACCEPTED
