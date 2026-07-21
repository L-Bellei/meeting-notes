# Estado do Projeto — 2026-07-21

## Sessão
- **Data:** 2026-07-21
- **Branch atual:** `master` (sincronizado com origin, em `1b2b706`)
- **Worktree:** nenhum ativo

## Trabalho recente

**Feature "Prompts por tipo de geração" — entregue e lançada (v2.5.0, PR #40).**
- Tema ganha prompt geral + 3 overrides por tipo (resumo / pontos-chave / tarefas). Precedência: específico → geral (`custom_prompt`) → default embutido.
- `Theme.PromptFor(kind)` resolve; `models.ThemePrompts` threadado por service/handler; migration 015 (+3 colunas). Frontend: 4 textareas no `ThemeEditModal`.
- Conduzida via Superpowers: brainstorm → spec → plano → execução TDD (4 tasks, subagent-driven) → review final (Ready to merge: Yes) → validada ao vivo.
- Spec: `docs/superpowers/specs/2026-07-20-theme-type-prompts-design.md`
- Plano: `docs/superpowers/plans/2026-07-20-theme-type-prompts.md`

**Releases desta sequência de sessões:**
- **v2.4.1** — fix tray/hotkey (thread affinity Win32) + overlay meeting guard.
- **v2.4.2** — Whisper "auto" detecção real de idioma + badge de idioma no MeetingDetail.
- **v2.5.0** — prompts por tipo de geração.

## Fase Superpowers

**N/A** — feature de prompts por tipo finalizada e lançada. Nenhum trabalho em andamento.

## Próximo passo imediato

Nenhum. Backlog em `.claude/BACKLOG.md` para a próxima feature (brainstorm → plan via Superpowers).

## Worktrees paralelos

Nenhum.

## Estado de release

- **v2.5.0** publicada: https://github.com/L-Bellei/meeting-notes/releases/tag/v2.5.0
- Installer: `dist/meeting-notes-2.5.0-windows-amd64-installer.exe`
- Build canônico: `build.ps1` (não `wails build -nsis` direto).
