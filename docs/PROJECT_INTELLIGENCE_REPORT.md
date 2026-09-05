# COSCA DESKTOP — PROJECT INTELLIGENCE (implementação e estado)

> **Autor**: cosca-kernel (orquestração) · cosca-backend + cosca-frontend + cosca-uiux (execução)
> **Data**: 2026-08-25
> **Missão**: "abrir um projeto → o COSCA entende tudo → o usuário vê POR QUÊ" — Professional Project-Aware Engineering Workbench.

---

## O que foi implementado (consumindo o mecanismo oficial, nunca duplicando)

### Núcleo — `internal/projectintel/` (read-only, por evidência)
- **`types.go`** — `ProjectProfile` extensível (sem enum gigante): `Confidence` (high/medium/low), `Signal` (evidência), `Tech` (categoria livre), `DetectedCommand`, 16 categorias + `ProjectTypes` + `Capabilities` + `Evidence` + `DetectedAt` + `Repository`.
- **Detecção por MÚLTIPLOS sinais, nunca por nome de pasta**: linguagens (11), frameworks (frontend+backend), package managers (lockfiles), build/test/format/lint/typecheck, runtime, container, infra, database (por config, nunca conecta), monorepo, CI, docs, Git, comandos.
- **Anti-falso-positivo**: `NeverByFolderName` (pasta `react/` sem evidência → não declara React). **Anti-inflação**: ignora `node_modules/.git/vendor/.cosca/...`.
- **README-only, nunca executa comandos**; limites de tamanho/profundidade.

### Modularização (feedback do professor)
- `detect.go` (1332 linhas) **eliminado** → **~16 arquivos por categoria** (`detector.go` 632, `language.go`, `framework.go`, `package_manager.go`, `build.go`, `tooling.go`, `container.go`, `database.go`, `monorepo.go`, `ci.go`, `docs.go`, `commands.go`, `runtime.go`, `repository.go`, `intelligence.go`, `cache.go`). API pública intacta; extensível sem recompilar.

### Camada de valor (consumo, não duplicação)
- **`AnalyzeProject()`** — binding que expõe o `ProjectProfile` à UI.
- **`AgentProjectContext()`** — injeta o resumo no Agente (stack, package_manager, build/test/lint/format, skills, pipeline, commands) — **o agente não precisa perguntar**.
- **`AgentSkillMatch`** — mapeia tech → **só as skills aplicáveis** (nunca todas).
- **`AgentPipeline`** — pipeline auto-descoberto (format → lint → typecheck → test → build), etapas sem ferramenta → `NOT_APPLICABLE` (não erro).
- **`Cache` (incremental)** — `GetOrAnalyze` por fingerprint de arquivos relevantes; `Invalidate`; **em memória, nunca em `.cosca`/memory/knowledge/family**.

### UI (observável, engineering, não SaaS)
- **Painel Intelligence** (pane colapsável no Layout Engine) — "why detected": nome + confiança (`✓ high / • medium / ! low`) + evidência (lista mono com `◆ dep / ▸ file / ⌕ content / # count / ❯ command`).
- **Bloco "Project context · o agente já sabe"** no painel do Agente — stack + skills match + pipeline auto-descoberto (passos com `✓/·/!` + badge N/A).
- **Botão Refresh** (invalida cache + reanalisa) no Intelligence e no contexto do agente.
- **Auto-configuração honesta**: badge git/container/ci só quando detectado; senão `—`.

---

## Testes

| Pacote | Resultado |
|--------|-----------|
| `internal/projectintel` (10 fixtures + Git + Skills/Pipeline + Cache) | **18 PASS** |
| `cosca-desktop` (app_test.go: project, fs, approval, diff, terminal, root, context, stream) | **~40 PASS** (1 SKIP pré-existente: `os.Symlink` no Windows) |
| **Total** | **~58 PASS · 0 FAIL · 1 SKIP** |

**Fixtures testados**: React+Vite+TS, Next+TS+pnpm, Go, Python, Rust (mock), Docker+Monorepo, Empty, NeverByFolderName, IgnoresNodeModules, Analyze_DSL, Git (repository), Skills, Pipeline (Go/TS), Cache (hit/invalidate/relevant-files/roots).

**Regressões preservadas** (prova de que nada quebrou): Create/Open Project, isolamento A≠B, safeJoin, approval fail-closed, Diff, Terminal autorizado, root detection, FASE A/B/C/D.

---

## Honestidade (VERIFIED vs NON-VERIFIED vs NOT AVAILABLE)

- **VERIFIED** (compile + teste): núcleo de detecção por evidência, modularização, `AnalyzeProject`, `AgentProjectContext`, `AgentSkillMatch`, `AgentPipeline`, cache incremental, painel observável (pane + why-detected), contexto do agente (skills + pipeline), botão Refresh, build frontend OK, `go test` OK, **EXE compilado e aberto**.
- **NON-VERIFIED** (testar no app real, visual): a experiência real de abrir um projeto e ver a detecção na tela (o EXE está aberto — validar o painel `◉ Intelligence` e o bloco "agente já sabe"); a resolução de confiança/evidência em projetos reais complexos.
- **NOT AVAILABLE** (honesto, por design): o PI **não** detecta "em runtime" (não executa build/lint/test para validar — separa `STATIC DISCOVERY` de `RUNTIME VALIDATION`); não conecta a bancos; não executa deployment; não carrega kernel/memória institucional (ADR-0007); a UI mostra `N/A` para etapas sem ferramenta.
- **Regras respeitadas**: zero duplicação de kernel/memory/skills do COSCA (só nomeia skills); `Project Intelligence` ≠ kernel/memória; cache em memória (nunca `.cosca`); read-only, não-destrutivo.

---

## Definção de pronto (cruzada com a missão)

| Item | Status |
|------|--------|
| ProjectProfile implementado | ✅ |
| evidence/confidence implementados | ✅ |
| linguagem/framework/PM/formatter/lint/typecheck/test/build detectados | ✅ |
| Docker/monorepo/Git/CI/documentação detectados | ✅ |
| editor config + file icons + minimap | ⚠️ (config parcial; icons/minimap pendentes — próxima fase) |
| commands descobertos | ✅ |
| Skills auto-matched | ✅ |
| pipeline auto-descoberto | ✅ |
| discovery incremental + cache | ✅ |
| UI não bloqueada (background) | ✅ |
| segurança/isolamento preservados | ✅ |
| nenhuma memória institucional duplicada | ✅ |
| nenhuma dependência obrigatória do Kernel | ✅ |
| testes de fixtures implementados | ✅ |
| testes existentes continuam PASS | ✅ |
| build Desktop continua PASS | ✅ |

---

## Próximo passo recomendado
1. **Editor intelligence + file icons + minimap** (missão §16-18) — preencher o que ficou pendente.
2. **Validar visualmente** o painel Intelligence e o contexto do agente no EXE aberto (NON-VERIFIED → VERIFIED).
3. Opcional: **RUNTIME VALIDATION** (executar build/lint/test detectados, com aprovação) — separado do static discovery, respeitando a autoridade do runtime.
