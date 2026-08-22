# Estado do Projeto — 2026-08-22

## Sessão
- **Data:** 2026-08-22
- **Branch atual:** `fix/known-bugs` (6 commits acima de `master`/`bdcfea7`, não pushada)
- **Worktree:** nenhum ativo

## Trabalho recente

**Branch `fix/known-bugs` — implementada e revisada, ainda não integrada.**
- Plano: `docs/superpowers/plans/2026-08-21-known-bug-fixes.md` (2/2 tasks)
- Task 1: fallback GPU→CPU amplo no `audio-service` (+3 testes). Task 2: quatro correções de frontend.
- Review final whole-branch (opus): *Ready to merge **No*** por limpeza omitida do BACKLOG → fix wave de 7 achados → re-review confirmou todos endereçados.
- **Acumulado depois, fora do plano:** correção do checkbox de tasks do `CardDetailModal` (422 "description is required"), reportada pelo usuário e verificada por ele na janela nativa.

## Fase Superpowers

**`finishing` pendente.** O menu de finalização foi apresentado e o usuário escolheu **acumular** a correção nova nesta branch em vez de integrar. `master` é protegido → o caminho é PR.

## Próximo passo imediato

Decidir a integração da `fix/known-bugs` (PR contra `master`). Depois, dois itens já mapeados no BACKLOG e aguardando decisão do usuário:
- Bug do rascunho de descrição em cards manuais (pequeno, isolado).
- Os 10 itens de UI/UX do `CardDetailModal` — investigados a pedido do usuário, **sem escopo definido**; pede `/superpowers:brainstorming` porque vários são decisão de comportamento.

Feature acordada anteriormente e ainda não brainstormada: **notificações de pipeline**.

## Worktrees paralelos

Nenhum.

## Estado de release

- **v2.6.0** publicada: https://github.com/L-Bellei/meeting-notes/releases/tag/v2.6.0
- Installer: `dist/meeting-notes-2.6.0-windows-amd64-installer.exe` (144 MB, audio-service embutido)
- Build canônico: `build.ps1` (não `wails build -nsis` direto).
- O `.spec` do PyInstaller é rastreado no git (`audio-service/build/pyinstaller/audio-service.spec`, com negação no `.gitignore`).
- Nada desta branch foi lançado ainda.

## Armadilhas de ambiente

Todas as três estão no `CLAUDE.md`, seção "Rodando em dev". A que mais custou tempo até agora:

- **O HMR do vite não chega à janela nativa** do `wails dev` — só ao navegador em `localhost:34115`. Reinicie o `wails dev` depois de mexer no frontend, não só no backend. O `hmr update` no log é o vite emitindo, não o webview aplicando: **não** use como prova de que a janela nativa atualizou. Isso fez uma correção correta ser reportada como "não funcionou".
- **O watcher só observa `cmd/desktop`** — mudança em `internal/**` não rebuilda o Go.
- **`SingleInstanceLock`** — um segundo `wails dev` sai com exit 0 em silêncio.
- Dev roda CUDA, produção roda CPU — ver DECISIONS de 2026-08-21.
