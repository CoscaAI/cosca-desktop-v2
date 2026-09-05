# ADR-0001 — Desktop Runtime: Wails (Go + WebView2)

**Status**: ACCEPTED (verificado contra o Cosca real)

## Context
O Cosca Desktop precisa de um runtime desktop. A escolha não deve ser por
popularidade, mas por segurança, performance, integração Windows, RAM, startup
e integração com o runtime Go do Cosca.

## Decision
Usar **Wails v2 (Go + WebView2)**.

**Verificação (GATE 01)**:
- `wails` CLI instalado (`C:\Users\Henrique\go\bin\wails.exe`), build Windows `windows/amd64` OK.
- Build real medido: `cosca-desktop.exe` = **~11MB**; tempo de build ~3–5s.
- WebView2: nativo no Windows (runtime presente no sistema alvo).
- Embedding: `//go:embed all:frontend/dist` + `assetserver.Options.Assets`.
- IPC: bindings gerados (`wailsjs/go/main/App`) expõem métodos tipados — comprovado.
- Lifecycle: `OnStartup/OnShutdown` (Wails) — confirmado no `main.go`.
- Process management / tray / notifications / updater: **não exercitados ainda** — a
  RFC deve marcar como capacidades a validar (não como fato).

## Alternatives
- **Electron**: maduro, ecossistema enorme, mas Chromium+Node → alto RAM/startup.
- **Tauri**: leve, mas backend em Rust (fora da stack Go do Cosca, criaria fronteira).

## Consequences
- Integração direta com pacotes Go do Cosca (engine, execpolicy, memory, knowledge).
- Binário único leve (~11MB, medido).
- WebView2 nativo (Windows), menor RAM que Chromium.

## Security implications
- Segurança no backend Go; a UI (WebView) só apresenta. Trust boundary no backend.

## Performance implications
- Startup/RAM baixos (WebView nativo). A medir no benchmark (FASE 12).

## Rejected alternatives
- Tauri: linguagem não Go (perde integração direta com runtime Cosca).
- Electron: peso (RAM/startup) e superfície maior.

## Non-verified (honestidade)
- Tray, notifications, updater, code signing, auto-update: **não testados** — marcar
  como `NON-VERIFIED` até exercício em FASE 11.
