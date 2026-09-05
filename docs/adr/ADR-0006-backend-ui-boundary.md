# ADR-0006 — Backend / UI Boundary

**Status**: ACCEPTED

## Context
A UI não contém regras de negócio nem autorização. Backend autoriza, UI apresenta.

## Decision
```
UI (React/WebView)
    │  somente apresentar + solicitar (métodos Wails / eventos)
    ▼
Desktop Application Layer (Go/Wails)
    │  autentica · autoriza · traduz · orquestra · valida
    ▼
Cosca Client / API (execpolicy · session · project)
    ▼
Cosca Runtime (kernel · agents · skills · workflows)
```

- A UI pede; o backend decide e executa.
- Erros estruturados (nunca string livre na UI).
- Eventos estruturados (a UI reage, não faz polling de regra).
- Nenhuma capability na UI sem correspondente verificado no backend.

## Alternatives
- Fazer a UI chamar comandos diretos sem camada de autorização (rejeitado).

## Consequences
- Camada de aplicação é a única porta de entrada autorizada.
- Testável: a UI pode ser substituída mantendo o backend.

## Security implications
- Toda ação sensível passa pelo backend (approval/execpolicy).
- A UI não é superfície de autoridade.

## Performance implications
- Uma camada a mais; mantida leve (tradução, não processamento crítico).

## Rejected alternatives
- UI com autoridade (rejeitado — quebra trust boundary e testabilidade).

## Status
ACCEPTED
