# Changelog de Sessões

---

## [2026-05-02] README profissional

**Contexto:** README existente tinha boa estrutura mas sem profundidade de uso, sem badges, sem fluxo step-by-step.

**Sem plano Superpowers** — documentação pura.

**Fase do workflow Superpowers:** N/A.

**O que foi entregue:**
- Reescrita completa do README: header com logo/badges, instalação rápida, tela de carregamento, todas as funcionalidades em profundidade, fluxo de uso típico, arquitetura, setup de desenvolvimento, API REST completa, histórico de versões
- PR #27 aberto (`docs/rewrite-readme`)

**Decisões transversais registradas em DECISIONS.md:** nenhuma.

---

## [2026-05-02] Fix primeiro boot + v2.2.3

**Contexto:** Bug reportado: ao iniciar o computador e abrir o app pela primeira vez, a loading screen exibia erro "Servidor HTTP — Não foi possível conectar". Segunda abertura funcionava normalmente.

**Sem plano Superpowers** — hotfix direto.

**Causa raiz:** Race condition entre `OnStartup` (Go) e o carregamento do frontend (Wails carrega o WebView2 concorrentemente). No primeiro boot, `OnStartup` é mais lenta; `GetPort()` era chamado antes de `a.port` ser definido, retornando `0`. Frontend fazia poll em `localhost:0` (connection refused imediato) por 15 s até exibir o erro.

**O que foi entregue:**
- Fix `GetPort()` — usa `portReady chan struct{}` para bloquear até a porta estar disponível (PR #26)
- Version bump + release v2.2.3

**Decisões transversais registradas em DECISIONS.md:** sim (race condition Wails/OnStartup).

---

## [2026-05-02] Bug fixes pós-release + v2.2.2

**Contexto:** Três bugs críticos identificados após publicação da v2.2.1.

**Sem plano Superpowers** — hotfixes diretos (nenhuma brainstorming/plan session formal).

**Fase do workflow Superpowers:** N/A.

**O que foi entregue:**
- Fix React Rules of Hooks no `SettingsModal` (PR #21)
- Fix template de nome no `RecordingModal` ao abrir pelo Toolbar (PR #22)
- Fix CUDA `cublas64_12.dll`: DLL dirs carregadas antes dos DLLs + fallback CPU (PR #23)
- Fix CUDA lazy generator: segmentos consumidos dentro do `try` + detecção antecipada em `_resolve_device_compute` (PR #24)
- Version bump + release v2.2.2 com installer (PR #25)

**Decisões transversais registradas em DECISIONS.md:**
- `faster-whisper`: gerador lazy deve ser consumido dentro do bloco `try`
- CUDA: detecção antecipada via `ctypes.CDLL` + fallback reativo como segunda linha de defesa

**Bloqueios encontrados:**
- Fix de CUDA do PR #23 não resolveu o problema: `transcribe()` retorna gerador, erro só aparece ao iterar os segmentos (fora do try original) — corrigido no PR #24
- Testes do `transcriber.py` falharam após mudança em `_resolve_device_compute` (passa a tentar carregar DLLs reais) — corrigido com `_make_transcriber()` helper que mocka o método

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
