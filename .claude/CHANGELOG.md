# Changelog de Sessões

---

## [2026-06-06] Release v2.4.0

**Sem plano Superpowers** — sessão de empacotamento/release.

**Fase do workflow Superpowers:** N/A.

**Entregue:**
- Bump `productVersion` 2.3.0 → 2.4.0 (PR #33; `master` protegido exige PR).
- Instalador via `build.ps1`: `dist/meeting-notes-2.4.0-windows-amd64-installer.exe` (125.7 MB, audio-service embutido).
- Tag `v2.4.0` + GitHub Release publicada com o instalador.

**Conteúdo da release:** guard de IA não-configurada, avisos/contramedidas, degradação graciosa do pipeline, resiliência do audio-service e fix do audio-service em dev (entregues na sessão de 2026-06-05, PR #32).

**Correção de doc:** `CLAUDE.md` atualizado — o build canônico é `build.ps1` (não `wails build -nsis` direto, que omite o bundle do audio-service).

**Bloqueios:** push direto a `master` rejeitado por branch protection → bump teve de ir via PR.

---

## [2026-06-05] Fix audio-service em dev + guard de IA não-configurada e resiliência

**Sem plano Superpowers** — conduzido via plan-mode ad-hoc. PR #32 (branch `feat/ai-config-guard-and-resilience`).

**Fase do workflow Superpowers:** finishing (PR aberta, aguardando review/merge).

**Entregue:**
- **Fix audio-service em dev** (`f4aad63`): `startAudioService` usava bundle de release stale; agora pula `-dev.exe` e `findAudioServiceDir` exige `main.py`. Também `taskkill /F /T` no shutdown e `CREATE_NO_WINDOW`.
- **Guard de IA + avisos** (`f1e673b`): desabilita UI dependente de IA (banner/tooltip), degradação graciosa do pipeline (transcrição preservada em vez de FAILED), sentinels de erro (`ai.ErrNotConfigured`/`ErrAIAuthFailed`) corrigindo o caminho 503 morto, e monitor de saúde do audio-service mid-session.

**Verificação:** `go test ./internal/...` ✅, builds ✅, `tsc` ✅; 5 cenários validados manualmente via wails dev (baseline configurada, banner não-configurada + restauração reativa, barra de áudio indisponível + recuperação).

**Decisões transversais registradas em DECISIONS.md:**
- Degradação graciosa do pipeline quando IA não configurada
- Sentinels de erro de IA para mapeamento de status HTTP
- Monitor de saúde do audio-service é desktop-only (eventos Wails)

**Outros:** loading screen (`docs/superpowers/plans/2026-05-02-loading-screen.md`) verificada como completa.

---

## [2026-05-07] AudioPlayer fixes, settings save fix, release v2.3.0

**Sem plano Superpowers** — correções pós-finishing sobre `fix/whisper-hallucination-2.2.5`.

**Fase do workflow Superpowers:** N/A.

**Problemas encontrados e corrigidos:**
- `keep_audio` ausente no `validSettings` — qualquer PUT em `/api/settings` falhava silenciosamente
- AudioContext capturava permanentemente o `<audio>` element; fechar o contexto silenciava o áudio — removido completamente
- `bg-card` invisível (transparente): cor `card` não existia na paleta Tailwind do projeto
- Player ficava atrás de outros elementos apesar de `z-[9999]`: stacking context do componente pai limitava o z-index — corrigido com `createPortal(content, document.body)`
- Drag com `left/top` causava salto na primeira movimentação — substituído por `transform: translate(x, y)`

**Decisões transversais registradas em DECISIONS.md:**
- `createPortal` para widgets flutuantes
- AudioPlayer sem AudioContext (plain `<audio>`)
- Tailwind config com cor `card` explícita

**Entregável:** release v2.3.0 publicada no GitHub (PR #30), installer em `dist/`.

---

## [2026-05-06] Audio Resilience, Player & Transcription Diagnostics (v2.2.5)

**Plano Superpowers:** `docs/superpowers/plans/2026-05-06-audio-resilience-player.md`
**Spec:** `docs/superpowers/specs/2026-05-06-audio-resilience-player-design.md`
**Fase do workflow Superpowers:** finishing (aguardando decisão do usuário pós-teste).

**O que foi entregue (12 tarefas via Subagent-Driven Development):**
- Fix imediato: `vad_filter=True` removido do `transcriber.py` (causava falha completa no PyInstaller bundle)
- Migrations 011 (`audio_path`, `error_message`) e 012 (`keep_audio` setting)
- `Meeting` struct + repository atualizados (AudioPath, ErrorMessage em todas as queries)
- Pre-flight checks de transcrição: `CheckModelLoaded` + `ValidateWAVFile` (TDD)
- Orchestrator revisado: `audio_path` persistido imediatamente após `StopRecording`, erro real salvo em `error_message`, lógica `keep_audio` pós-transcrição
- `RetranscribeRecording` + `RunRetranscribePipeline` no orchestrator
- Handlers: `GET /api/meetings/{id}/audio` + `POST /api/meetings/{id}/retranscribe` (com testes)
- Frontend: tipos `audio_path`/`error_message`, `keep_audio` em Settings, `useRetranscribe` hook
- SettingsModal: toggle "Guardar áudio das reuniões" na aba Transcrição
- MeetingDetail: ícone Volume2, display de `error_message`, botão retry
- `AudioSpectrumVisualizer`: Web Audio API + Canvas animado
- `AudioPlayer`: widget flutuante `fixed bottom-4 right-4`, seek, ±15s centralizados, espectro
- Build v2.2.5 gerado: `dist/meeting-notes-2.2.5-windows-amd64-installer.exe`

**Decisões transversais registradas em DECISIONS.md:**
- WAV permanece no dir do audio-service (Approach A)
- `vad_filter` removido do PyInstaller bundle

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
