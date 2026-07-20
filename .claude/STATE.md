# Estado do Projeto — 2026-07-20

## Sessão
- **Data:** 2026-07-20
- **Branch atual:** `feat/whisper-auto-language` (a partir de `master` em `3c002fb`)
- **Worktree:** nenhum ativo

## Trabalho desta sessão

**1. Release v2.4.1 publicada** (correções de desktop Win32 + empacotamento):
- Fix de entrega de cliques do tray e do hotkey global (thread affinity Win32) + overlay meeting guard — PR #35, commit `45c9e6a`.
- Bump `productVersion` 2.4.0 → 2.4.1 (PR #36 — `master` protegido exige PR).
- Instalador via `build.ps1`: `dist/meeting-notes-2.4.1-windows-amd64-installer.exe` (125.7 MB).
- Tag `v2.4.1` + GitHub Release publicada.

**2. Feature em andamento: Whisper "auto" — detecção real de idioma + exibição.**
- Brainstorm + spec + plano concluídos via Superpowers.
- Spec: `docs/superpowers/specs/2026-07-20-whisper-auto-language-detection-design.md`
- Plano: `docs/superpowers/plans/2026-07-20-whisper-auto-language.md`

## Fase Superpowers

**Executing** — plano escrito e aprovado; iniciando a execução das 4 tasks (TDD).

## Próximo passo imediato

Executar o plano `2026-07-20-whisper-auto-language.md`, task por task:
1. Python — normalizar `auto`/`""`/`None` → detecção real.
2. Migration 014 + `Meeting.Language` + repository.
3. Orchestrator — encaminhar `whisper_language` cru + persistir idioma detectado.
4. Frontend — tipo + `languageLabel` + badge no MeetingDetail.

## Worktrees paralelos

Nenhum.

## Estado de release

- **v2.4.1** publicada: https://github.com/L-Bellei/meeting-notes/releases/tag/v2.4.1
- Installer: `dist/meeting-notes-2.4.1-windows-amd64-installer.exe`
- Build canônico documentado em `CLAUDE.md` → usar `build.ps1`, não `wails build -nsis` direto.
