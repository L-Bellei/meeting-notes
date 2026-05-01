# Estado do Projeto — 2026-05-01

## Sessão
- **Data:** 2026-05-01
- **Branch:** `master`
- **Worktree:** nenhum ativo

## Features trabalhadas
**Fixes de gravação + Overlay Widget + Fixes pós-release** — tudo concluído e mergeado em master (PRs #8-#15).

- Spec fixes: `docs/superpowers/specs/2026-05-01-fixes-recording-startup.md`
- Spec overlay: `docs/superpowers/specs/2026-05-01-recording-overlay-widget.md`
- Plano fixes: `docs/superpowers/plans/2026-05-01-fixes-recording-startup.md`
- Plano overlay: `docs/superpowers/plans/2026-05-01-recording-overlay-widget.md`

### O que foi entregue
1. **Fix — Delete orphaned meeting quando /start falha** (`RecordingModal.tsx`): track `createdId`, faz DELETE no catch se criação ocorreu mas start falhou.
2. **Fix — Poll /health antes de marcar app ready** (`App.tsx`): polling 500ms até 15s; erro de startup exibido se servidor não responder; cleanup flag no useEffect.
3. **Feature — Win32 Overlay Widget** (`cmd/desktop/overlay.go`, `tray.go`, `app.go`): pílula always-on-top com dot pulsante, timer MM:SS, botão de stop com confirmação, drag nativo via WM_NCHITTEST, integração com eventos do Orchestrator.
4. **Fix — Overlay Win32 thread affinity** (`overlay.go`): janela criada e message loop no mesmo OS thread via `LockOSThread` + canal `ready` para sincronização.
5. **Fix — `StartRecording` sem notify** (`orchestrator.go`): `o.notify()` não era chamado após atualizar status, overlay nunca aparecia.
6. **Fix — Transcrição GPU/CPU automático** (`audio-service/transcriber.py`, `main.py`): `WHISPER_DEVICE=auto`, DLLs CUDA pré-carregadas via `ctypes.CDLL`, detecção via `ctranslate2.get_cuda_device_count()`.

## Estado do repositório
- Repositório público: `https://github.com/L-Bellei/meeting-notes`
- Branch `master` protegida por ruleset `protect-master` (PRs obrigatórios, sem force push)
- PRs mergeados nesta sessão: #8 (overlay), #9 (docs), #10 (gitignore), #11 (version bump), #12 (overlay+notify fix), #13 (whisper cpu fallback), #14 (whisper auto device), #15 (whisper cuda dll preload)
- Release v2.2.0 publicado com instalador atualizado (inclui todos os fixes)
- Working tree completamente limpa — nada pendente

## Próximo passo imediato
Nenhum. Ver `BACKLOG.md` para próximas features.

## Worktrees paralelos
Nenhum.
