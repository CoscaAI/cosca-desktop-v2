# ADR-0003 — Windows Sandbox (honestidade)

**Status**: ACCEPTED — limitação real documentada (não vira feature visual)

## Context
No Windows, o Cosca **não possui sandbox de processo** (bwrap/jail). Verificado no
código: `internal/chat/sandbox/gate_other.go` → `"bwrap is not supported on this
platform"`, retornando erro em `execBwrap`. O CLI requere `COSCA_ALLOW_NO_ROOT=1`
(opt-in) e avisa: *"jail unavailable, running WITHOUT sandbox"*.

## Decision
**Não transformar limitação em sandbox fake.** O Sandbox Center mostra o estado REAL:

```
Windows
  Process Sandbox:      NOT AVAILABLE   (honesto)
  Filesystem (tools):   ENFORCED        (Rails.Validate — real)
  Execution Policy:     ACTIVE          (execpolicy Allow/Prompt/Forbidden)
  Approval:             ACTIVE          (cosca approve)
  Project Isolation:    ACTIVE          (testado, .cosca por projeto)
  Process execution:    PARTIAL         (approval-based, sem OS-sandbox)
```

Defesa real no Windows = **approval + execpolicy + projet isolation + filesystem
restrictions (Rails)**. Em Linux (bwrap ativo), o process sandbox fica ACTIVE.

## Alternatives
- Fingir sandbox ativo (rejeitado — viola honestidade arquitetural; é segurança falsa).

## Consequences
- O usuário sabe exatamente o que o agente pode fazer.
- Sandbox Center é um relatório de estado real, não um painel decorativo.

## Security implications
- Sem OS-sandbox no Windows → a contenção de processos depende de execpolicy+approval.
- Filesystem de arquivos é contido (Rails) mesmo sem jail.

## Performance implications
- Relatório de estado real é barato; não adiciona overhead.

## Rejected alternatives
- Sandbox visual "ACTIVE" quando não há enforcement de processo (rejeitado — segurança falsa).

## Status
ACCEPTED
