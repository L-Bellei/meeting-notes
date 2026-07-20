# Estado do Projeto — 2026-07-20

## Sessão
- **Data:** 2026-07-20
- **Branch atual:** `master` (sincronizado com origin, em `d0032d1`)
- **Worktree:** nenhum ativo

## Trabalho desta sessão

**1. Release v2.4.1** — fix de cliques do tray/hotkey (thread affinity Win32) + overlay meeting guard (PRs #35/#36).

**2. Feature "Whisper auto" — detecção real de idioma + exibição (entregue, PR #37).**
- `"auto"`/`""`/`None` → detecção real no faster-whisper; idioma detectado persistido (`Meeting.Language`, migration 014) e exibido como badge no MeetingDetail.
- Conduzida via Superpowers: brainstorm → spec → plano → execução TDD (4 tasks, subagent-driven) → review final (Ready to merge: Yes) → PR.
- Spec: `docs/superpowers/specs/2026-07-20-whisper-auto-language-detection-design.md`
- Plano: `docs/superpowers/plans/2026-07-20-whisper-auto-language.md`
- Validada ao vivo (migration 014 aplica, app sobe, badge de idioma correto).

**3. Release v2.4.2** — empacotamento da feature acima:
- Bump `productVersion` 2.4.1 → 2.4.2 (PR #38 — `master` protegido).
- Instalador via `build.ps1`: `dist/meeting-notes-2.4.2-windows-amd64-installer.exe` (125.7 MB).
- Tag `v2.4.2` + GitHub Release publicada.

## Fase Superpowers

**N/A** — feature whisper-auto finalizada (finishing concluído) e lançada. Nenhum trabalho em andamento.

## Próximo passo imediato

Nenhum. Backlog em `.claude/BACKLOG.md` para a próxima feature (brainstorm → plan via Superpowers).

## Worktrees paralelos

Nenhum.

## Estado de release

- **v2.4.2** publicada: https://github.com/L-Bellei/meeting-notes/releases/tag/v2.4.2
- Installer: `dist/meeting-notes-2.4.2-windows-amd64-installer.exe`
- Build canônico: `build.ps1` (não `wails build -nsis` direto).
