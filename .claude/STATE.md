# Estado do Projeto — 2026-08-21

## Sessão
- **Data:** 2026-08-21
- **Branch atual:** `master` (sincronizado com origin, em `cc2b48a`)
- **Worktree:** nenhum ativo

## Trabalho recente

**Feature "Prompt único por tema + overhaul da aba de temas" — entregue e lançada (v2.6.0, PR #43).**
- Revert dos prompts por tipo (migration 016 derruba as 3 colunas) + reconstrução da aba de temas: painel fixável, chip de filtro visível, linha com barra de cor/badges/menu, exclusão com confirmação escrita, hierarquia de 2 níveis com drag-and-drop, expansão persistida.
- Spec: `docs/superpowers/specs/2026-08-20-themes-single-prompt-and-sidebar-design.md`
- Plano: `docs/superpowers/plans/2026-08-20-themes-single-prompt-and-sidebar.md` (7/7 tasks)

## Fase Superpowers

**N/A** — ciclo completo (brainstorm → spec → plano → execução subagent-driven → review final → finishing → release). Nenhum trabalho em andamento.

## Próximo passo imediato

Nenhum. Próxima feature acordada com o usuário mas **ainda não brainstormada**: **notificações de pipeline** (toast nativo do Windows ao terminar o processamento). Começar por `/superpowers:brainstorming`.

## Worktrees paralelos

Nenhum.

## Estado de release

- **v2.6.0** publicada: https://github.com/L-Bellei/meeting-notes/releases/tag/v2.6.0
- Installer: `dist/meeting-notes-2.6.0-windows-amd64-installer.exe` (144 MB, audio-service embutido)
- Build canônico: `build.ps1` (não `wails build -nsis` direto).
- **O `.spec` do PyInstaller agora é rastreado no git** (`audio-service/build/pyinstaller/audio-service.spec`, com negação no `.gitignore`). Antes vivia num diretório ignorado e foi perdido, bloqueando o build desta release até ser recriado.

## Armadilhas de ambiente descobertas nesta sessão

- **`wails dev` só observa `cmd/desktop`.** Mudanças em `internal/**` não disparam rebuild do Go: o app segue rodando o binário antigo. Depois de mexer no backend, reinicie o `wails dev` — senão dá erro 500 de código velho contra banco já migrado.
- **`SingleInstanceLock`**: subir um segundo `wails dev` com o primeiro rodando faz o novo sair com exit 0 silenciosamente. Mate o processo antes de reiniciar.
- **Dev roda CUDA, produção roda CPU** — ver DECISIONS de 2026-08-21.
