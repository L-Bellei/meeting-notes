# Changelog de Sessões

---

## [2026-05-05] Fix segunda instância + Release v2.2.4

**Sem plano Superpowers** — bugfix direto.

**Fase do workflow Superpowers:** N/A.

**Problema:** Ao fechar a janela e reabrir o app pelo atalho do Windows, um novo processo Wails era lançado e travava (colisão de SQLite, tray duplicado, porta HTTP diferente). Reinicialização da máquina era necessária.

**Causa raiz:** Nenhum mecanismo de single-instance existia no projeto.

**O que foi entregue:**
- `options.SingleInstanceLock` adicionado ao `options.App{}` em `cmd/desktop/main.go`
- Método `onSecondInstanceLaunch` em `cmd/desktop/app.go` (unexported para evitar geração de bindings TypeScript desnecessários) — chama `Show` + `WindowUnminimise` na instância existente; segunda instância encerra limpa
- Build v2.2.4 gerado e release publicada no GitHub (PR #28)

**Decisões transversais registradas em DECISIONS.md:** nenhuma.

---

## [2026-05-01] Recording Overlay + Fixes de gravação — v2.2.0

**Features/Fixes:** Overlay Win32, delete de reunião órfã, poll de /health no startup, CUDA auto-detect.

**Planos Superpowers:**
- `docs/superpowers/plans/2026-05-01-fixes-recording-startup.md` — concluído
- `docs/superpowers/plans/2026-05-01-recording-overlay-widget.md` — concluído

**Fase final:** `finishing` — todos os PRs (#8-#15) mergeados, release v2.2.0 publicada no GitHub com installer atualizado.

**O que foi entregue:** Ver STATE.md — lista completa dos 6 entregáveis.

**Decisões transversais registradas em DECISIONS.md:**
- Win32 overlay: `LockOSThread` + canal `ready` para thread affinity
- CUDA audio-service: pré-load de DLLs via `ctypes.CDLL` + detecção via `ctranslate2.get_cuda_device_count()`

**Bloqueios encontrados:**
- Overlay nunca aparecia: `StartRecording` atualizava status no banco mas não chamava `o.notify()` — corrigido
- Overlay Win32 thread affinity: janela criada em goroutine sem `LockOSThread`, eventos nunca chegavam ao loop — corrigido movendo criação para dentro da goroutine fixada
- Transcrição travada em "transcribing": ctranslate2 não encontrava `cublas64_12.dll` porque usa `LoadLibrary` ignorando `os.add_dll_directory` — corrigido com pré-load via `ctypes.CDLL`
- Serviço de áudio com código antigo após reinício do dev: processo `audio-service.exe` persistia entre sessões — matar com `taskkill /F /IM audio-service.exe` antes de `wails dev`

---

## [2026-04-29] Publicação v2.0.0 e infraestrutura do repositório

**Feature:** Nenhuma nova — sessão de publicação e organização.

**Fase do workflow Superpowers:** N/A (pós-finishing).

**O que foi feito:**
- Vinculação ao repositório remoto `https://github.com/L-Bellei/meeting-notes`
- Repositório tornado público com branch protection (`protect-master` ruleset)
- PR #1: `chore/gitignore-e-cleanup` — `.gitignore` expandido para artefatos de build
- PR #2: `docs/update-readme-v2` — README atualizado para v2.0.0
- Release de desenvolvimento `v2.0.0` publicada no GitHub com installer anexado
- Documentação de continuidade criada (`.claude/`, `CLAUDE.md`)

**Decisões transversais:** nenhuma nova (ver `DECISIONS.md`).

**Bloqueios encontrados:**
- Primeiro installer copiado para dist era pré-existente (build anterior sem kanban); corrigido forçando rebuild com `wails build -nsis` e NSIS no PATH

---

## [2026-04-29] Kanban Board — v2.0.0

**Feature:** Global Kanban Board com drag-and-drop, colunas configuráveis, CardDetailModal, filtros e auto-add por tema.

**Plano Superpowers:** `C:\Users\leo_b\.claude\plans\functional-honking-moler.md` (7 tasks, todas concluídas)

**Spec:** `docs/superpowers/specs/2026-04-29-kanban-board-design.md`

**Fase final:** `finishing` — mergeado em `master`, tag `v2.0.0` criada, installer gerado em `dist/meeting-notes-2.0.0-windows-amd64-installer.exe`.

**O que foi entregue:**
- Migration 007: tabelas `board_columns` (seed: Backlog / Em Andamento / Concluído) e `board_cards`
- Repositories: `BoardColumnRepository`, `BoardCardRepository` com rebalanceamento automático de posições
- Services: `BoardColumnService`, `BoardCardService`
- Handler: `BoardHandler` com rotas CRUD de colunas e cards + PATCH `/move`
- Frontend: `BoardView`, `KanbanColumn`, `KanbanCard` (drag-and-drop @dnd-kit), `CardDetailModal`, `BoardFilters`, `ColumnSettingsPanel`
- Hook: `useBoard.ts`, `useBoardColumns.ts`
- Navegação: botão "Board" na Toolbar, `activeView` state em App.tsx
- MeetingDetail: botão "Adicionar ao Board" + badge de card existente
- Theme: campo `auto_add_to_board` + hook no orchestrator para auto-criar card após processamento

**Decisões transversais registradas em DECISIONS.md:**
- Float positions + rebalanceamento
- customPrompt campo único
- Board global, numeração imutável
- Seed de colunas padrão com IDs fixos
- Processo de build do installer (NSIS path)

**Bloqueios encontrados:**
- `makensis` não estava no PATH do bash; resolvido adicionando `/c/Program Files (x86)/NSIS` temporariamente
- Primeiro installer copiado para dist era de build anterior (sem o board); corrigido após identificar a causa
