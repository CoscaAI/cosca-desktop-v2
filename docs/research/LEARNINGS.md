# Research / Aprendizados Externos — Cosca Desktop

Registro de conhecimento externo usado na arquitetura (obrigatório pela missão:
não esconder o que precisou ser aprendido). Sempre que possível, usar a fonte
primária (documentação oficial).

---

## D0 — Design principles (UI/UX workbench) — cosca-desktop

- **WHY REQUIRED**: o Desktop não é um "site numa janela" nem um dashboard de
  olhar. É uma ferramenta que um engenheiro abre 8h/dia — e a UI precisa impor
  precisão, controle, observabilidade, densidade e autoridade.
- **IDENTIDADE (nunca pra copiar, pra princípio)**: VS Code Workbench, JetBrains,
  Codex, Visual Studio, Figma desktop, GitHub/Linear desktop, Windows Terminal.
  Pane ─ tree ─ activity ─ editor ─ context ─ statusbar; keyboard-first; AA/AAA.
- **DECISÕES DE DESIGN**:
  - **Panes planares, não cards**: superfície + borda sutil + hierarquia por
    camada. Elevação (`--elev-*`) é privilégio de superfícies flutuantes
    (dialog/palette/dropdown) — nunca em "card".
  - **Radius reduzido/controlado**: `--radius-sm 4 · md 6 · lg 8`. Pills
    (`--radius-full`) só para status/tags. Fim do excesso de canto arredondado.
  - **Densidade**: controles em `--ctrl-h 30px`/`24px`, listas compactas,
    metadata inline, breadcrumbs, tabs, split panes. Nada de um-card-por-info.
  - **Cor**: neutro workspace com tinta azulada (nunca cinza puro), accent
    contido (`#3f79c4`, sem purple/neon/gradiente/glassmorphism/blob/hero).
    Estado sempre com indicador **não-cor** (forma/texto), não só cor.
  - **Tipografia compacta**: `--fs-md 13px` base; monospace (`--font-mono`) só
    para comandos/caminhos/hashes/IDs/logs/código.
  - **Motion comunica estado** (fast `100ms` / `160ms`), nunca decoração;
    `prefers-reduced-motion` respeitado.
  - **A11y AAA**: contraste, `:focus-visible` 2px accent, high-contrast e
    `forced-colors` (o estado não depende de cor).
- **HOW IT AFFECTS COSCA**: define o contrato de tokens (`:root` em
  `frontend/src/design.css`) que o frontend consome via `var()` e a gramática de
  classes (`.pane`, `.tabbar/.tab`, `.palette`, `.approval-*`, `.term`,
  `.status-dot`, `.divider-*`). Editor de código mantém fundo escuro nos 2 temas
  via    `--code-bg` (padrão IDE).

## D0.1 — DESIGN DRIFT GATE

Qualquer alteração de UI/UX MUST:

1. consumir tokens existentes antes de criar novos;
2. reutilizar componentes/classes existentes antes de criar variantes;
3. preservar a gramática de workbench;
4. não introduzir padrões SaaS/dashboard sem justificativa arquitetural;
5. não introduzir cor, radius, spacing, shadow ou motion arbitrários;
6. validar dark + light + forced-colors quando afetar componentes compartilhados;
7. registrar novo token/componente quando a necessidade não puder ser
   expressa pelo design system existente.

### REJECTION CONDITIONS

A mudança deve ser considerada DESIGN DRIFT se introduzir:

- gradiente decorativo;
- glassmorphism;
- neon/purple "AI aesthetic";
- cards ornamentais;
- radius excessivo;
- shadow em superfícies planares;
- espaçamento arbitrário;
- informação duplicada em widgets;
- estado comunicado exclusivamente por cor;
- animação sem função operacional.

### PRIORITY

Design System > componente local > preferência estética do implementador.

---

## T1 — Codex: approval workflow e sandbox
- **WHY REQUIRED**: o Desktop precisa de um modelo de aprovação e sandbox coerente
  com a indústria, mas nativo do Cosca.
- **SOURCE**: documentação e código do OpenAI Codex (app-server, execpolicy, sandbox).
  `https://github.com/openai/codex`
- **WHAT WAS LEARNED**: o Codex separa (a) `execpolicy` = regras allow/prompt/forbidden
  por comando; (b) sandbox de processo (bwrap/landlock) para conter execução;
  (c) aprovação explícita antes de comandos arriscados; (d) apresentação de diff
  e (e) sessões como entidades com provenance. Comandos arriscados = prompt de
  aprovação; a decisão é registrada (audit).
- **HOW IT AFFECTS COSCA**: reforça o modelo já existente no Cosca — `cosca approve`
  (Plano de Execução) + `internal/execpolicy` (Allow/Prompt/Forbidden) + audit.
  O Desktop deve expor essa aprovação de forma clara (Approval Center), não esconder.

## T2 — IDE / workspace model
- **WHY REQUIRED**: o Desktop é um "engineering workspace", então precisa entender
  o modelo de ideais modernos (workbench, editor groups, command palette, terminal).
- **SOURCE**: VS Code API / workbench architecture.
  `https://code.visualstudio.com/api`
- **WHAT WAS LEARNED**: workspace = conjunto de contextos (pastas, editor, terminal,
  painéis laterais) coordenados por uma barra de comandos central; a UI é "densa"
  mas organizada em grupos; tudo acionável por teclado; acessibilidade como requisito.
- **HOW IT AFFECTS COSCA**: inspira o layout (top bar + rail + workspace + context
  + bottom status) e a necessidade de Command Palette + keyboard-first + acessibilidade.

## T3 — Desktop runtime escolha
- **WHY REQUIRED**: precisa escolher a tecnologia de desktop (não por popularidade,
  mas por segurança/performance/integração Windows/consumo/startup/extensibilidade).
- **SOURCE**: documentação oficial de Tauri, Electron, WebView2, Windows App SDK, Wails.
  `https://v2.wails.app/` · `https://v2.tauri.app/` · `https://www.electronjs.org/`
  `https://learn.microsoft.com/windows/apps/` 
- **WHAT WAS LEARNED**:
  - **Electron**: maduro, grande ecossistema, mas pesado (Chromium+Node, alto RAM/startup).
  - **Tauri**: leve (WebView nativo + Rust backend), baita security/performance, mas
    backend em Rust (fora da stack do Cosca — Go).
  - **Wails (Go + WebView2)**: WebView nativo Windows (leve), **backend em Go** (mesma
    linguagem do runtime Cosca → integração direta de pacotes, binário único, menos
    fronteira de comunicação), build simples, ~11MB, startup rápido.
  - **WebView2 / Windows App SDK**: runtime nativo Windows para renderização.
- **HOW IT AFFECTS COSCA**: decisão = **Wails (Go + WebView2)**. Motivo: integração
  nativa com o runtime Cosca em Go (sem duplicar cliente), binário único leve,
  WebView2 nativo do Windows, RAM/startup baixos. É a escolha com o melhor equilíbrio
  para um produto Cosca (não por popularidade).

## T4 — Segurança em desktop (approval / sandbox / secrets)
- **WHY REQUIRED**: nenhuma operação sensível pode depender de esconder botão na UI.
- **SOURCE**: modelos de sandbox/approval de Codex, VS Code (workspace trust),
  e boas práticas de desktop (keychain/credential storage, clipboard, path traversal).
- **WHAT WAS LEARNED**: trust boundary deve estar no backend/kernel, nunca na UI.
  Secrets em OS keychain/DPAPI, não em arquivo de config da UI. O sandbox é real
  se o runtime oferecer; caso contrário, declarar estado real (nunca "faker" bonito).
- **HOW IT AFFECTS COSCA**: Sandbox Center deve mostrar o estado REAL (no Windows o
  jail/bwrap não está disponível → defendido por approval + execpolicy + read-only
  roots + isolamento de projeto). Nunca inventar um sandbox visual que não existe.
