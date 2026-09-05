# ADR-0002 — Security Boundary

**Status**: ACCEPTED

## Context
O Desktop não pode ter regra crítica nem segurança na UI. A segurança pertence ao
backend/kernel. Autoridade não pode depender de esconder botão.

## Decision
Boundary explícito:

```
UI (React/WebView)            → apresenta, solicita
Desktop Application Layer (Go) → autentica, autoriza, traduz, orquestra
Cosca Client/API               → execpolicy, session, project
Cosca Runtime                  → kernel, agents, skills, workflows
Kernel                         → autoridade última
```

- A UI **nunca** decide autorização.
- Operações sensíveis exigem verificação no backend (execpolicy + approval).
- Secrets: OS keychain/DPAPI, nunca na UI/config.

## Alternatives
- Colocar regras na UI (rejeitado — viola trust boundary).

## Consequences
- Backend Go é o guardião; UI é fina.
- Desacoplamento: UI pode ser trocada sem tocar a segurança.

## Security implications
- Trust boundary no backend → reduz superfície de ataque do WebView.
- Aprovações e política de execução aplicadas onde a autoridade está.

## Performance implications
- Aprovação/verificação no backend adiciona latência mínima (aceitável).

## Rejected alternatives
- Segurança na UI (rejeitado: não é enforcement, é ocultação).

## Status
ACCEPTED
