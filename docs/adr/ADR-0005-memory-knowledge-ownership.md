# ADR-0005 — Memory & Knowledge Ownership

**Status**: ACCEPTED

## Context
Memory e Knowledge pertencem ao Cosca. O Desktop **consome**, não duplica.

## Decision
```
Cosca owns Memory. Cosca owns Knowledge.

Desktop:
    READ
    QUERY
    DISPLAY
    REQUEST MUTATION THROUGH OFFICIAL API

Desktop MUST NOT:
    create second database
    create second vector store
    create second memory engine
    create second knowledge engine
```

## Verification
- Memória oficial: `internal/memory` (camadas LayerSession/Project/Global + index SQLite FTS5).
- Knowledge oficial: `internal/knowledge` (DB `coscaDir/knowledge.db` + embeddings).
- Desktop backend usa esses mecanismos; não cria engine paralela.

## Consequences
- Fonte de verdade única (Cosca).
- UI é uma janela sobre os mecanismos oficiais.

## Security implications
- Evita duplicação que quebre isolamento/consistência/proveniência.

## Performance implications
- Sem segundo índice → sem custo de manutenção duplicado.

## Rejected alternatives
- Banco/vector store próprio do Desktop (rejeitado — duplica e quebra ownership).

## Status
ACCEPTED
