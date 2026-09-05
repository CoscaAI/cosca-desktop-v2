# ADR-0007 — Independência do Binário + Escopo do Forge

**Status**: ACCEPTED (decisão do Don, 2026-08-25)

## Context

O cosca-desktop **não é um reflexo do cosca root** — é um produto independente e
distribuível, que deve rodar em qualquer PC ou pasta, sem depender do cosca root.
Porém, quando o binário detecta que está dentro do projeto root do cosca, ele
assume o "modo Forge" e revela a estrutura completa. A confusão anterior era
tratar o Desktop como casca do runtime; o Don definiu a hierarquia exata de o
que entra, o que não entra, e quando.

## Decision

O mesmo binário, dois modos, definidos por DETECÇÃO do root (nunca por cópia):

### BINÁRIO STANDALONE (fora do root, outro PC)
**ENTRA:**
- **Agents** — as capacidades/roles (cosca-backend, cosca-frontend, cosca-uiux, …).
- **Skills** — o arsenal operacional de conhecimento.

**NÃO ENTRA:**
- **O KERNEL** — a inteligência que opera/orquestra de verdade (a "cabeça" que
  coordena e decide). Não está no standalone.
- **A memória / legado da família** — learnings, história, ALMA/loyalty,
  family chain. A alma ancestral não acompanha o binário standalone.

→ Standalone = **mão-de-obra + material** (agents + skills), **sem cabeça
(kernel) e sem alma (legado)**. Opera via IA externa/local (preferência do
cliente), mas não é o Cosca "vivo" — falta quem comanda.

### BINÁRIO NO ROOT (Forge — detecta o projeto root do cosca)
**ENTRA:**
- **O KERNEL** — a inteligência que opera entra aqui.
- **TUDO DO ROOT** — agents, skills, memória da família, configurações,
  arquitetura, family chain, DNA — revelado estruturado.

→ Forge = **cabeça + alma + estrutura**, tudo acessível a partir do root.

## A regra da separação

| | Standalone (outro PC) | Forge (no root) |
|---|---|---|
| Agents | ✅ | ✅ |
| Skills | ✅ | ✅ |
| **Kernel (opera de verdade)** | ❌ NÃO | ✅ SIM |
| **Memória/legado da família** | ❌ NÃO | ✅ SIM (do root) |

**Invariante central:** o conteúdo do root (kernel operacional + memória da
família + estrutura) **NUNCA é copiado para dentro do binário**. Ele é
**acessado/revelado a partir do root** quando o binário o detecta. O binário é
sempre o mesmo; o que muda é se ele detecta o root e passa a expor o que está lá.

## Alternatives
- Tratar o Desktop como casca que sempre consome o runtime (rejeitado — não é
  independente/distribuível).
- Embutir agents + skills + kernel + memória no binário (rejeitado — viola
  "o legado da família não entra"; o binário ficaria carregado com o DNA que
  pertence ao root).
- Criar dois binários distintos (rejeitado — complexidade de distribuição; o
  mesmo binário detecta o root).

## Consequences
- O binário é fino e distribuível; roda em qualquer PC.
- Sem o kernel no standalone, o desktop ali **não orquestra como Cosca vivo** —
  apenas opera (agents + skills) com IA externa/local.
- No root, o kernel entra e o desktop assume a orquestração consciente,
  acessando agents, skills, memória da família e arquitetura do root.
- Detecção do root é o gatilho do modo Forge (a definir exatamente em
  implementação — ex.: presença de `internal/embed/cosca`, `AGENT_DNA.md`,
  family chain / árvore root).

## Security implications
- Nenhum dado da família é distribuído no binário standalone (não vaza legado).
- A memória da família permanece no root; o desktop só acessa quando está lá.
- O kernel (autoridade) só entra no root — não é exposto fora dele.

## Status
ACCEPTED
