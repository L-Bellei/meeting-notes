# Estado do Projeto — 2026-08-22

## Sessão
- **Data:** 2026-08-22
- **Branch atual:** `fix/manual-card-draft-description` (1 commit acima de `master`/`bbf9fd1`)
- **Worktree:** nenhum ativo

## Trabalho recente

**PR #45 (`fix/known-bugs`) mergeada em `master` (`bbf9fd1`).** Seis bugs fechados:
- Fallback GPU→CPU amplo no `audio-service` (qualquer exceção de inferência, não só erro de DLL) + 3 testes.
- Quatro correções de frontend: erro de exclusão em pt-BR, cancelamento de drag, menu `⋯` fechando em scroll/resize, hambúrguer na Board view.
- **Checkbox de tasks do `CardDetailModal` nunca gravava** — reportado pelo usuário, fora do plano. `TaskRow` mandava só `{ completed }`, o campo ausente decodificava para `""` e o service devolvia 422. Correção: reusar o tipo `Task` compartilhado, enviar `{ ...task, completed }`, invalidar `board-card`/`board-cards`, e mostrar estado/erro na linha.

**Branch atual:** bug do rascunho de descrição em cards manuais. `toggleTask`/`addTask`/`removeTask` mandavam o `description` do state local; como o `PUT` do card **substitui** a descrição, editar sem salvar e clicar num checkbox persistia o rascunho. Passaram a reenviar `card.description`.

## Fase Superpowers

**N/A** — correção pontual, sem plano. O ciclo da `fix/known-bugs` está completo.

## Próximo passo imediato

Abrir/integrar o PR desta branch. Depois, dois itens no BACKLOG aguardando decisão do usuário:
- **10 itens de UI/UX do `CardDetailModal`** — investigados a pedido do usuário, **sem escopo definido**. Pede `/superpowers:brainstorming`: itens como o `Escape`, o confirm de exclusão e o `status` clicável são decisão de comportamento, não de CSS.
- **Notificações de pipeline** — feature acordada antes, ainda não brainstormada.

Nenhuma release desde a v2.6.0; as correções mergeadas ficam para a próxima versão.

## Worktrees paralelos

Nenhum.

## Estado de release

- **v2.6.0** publicada: https://github.com/L-Bellei/meeting-notes/releases/tag/v2.6.0
- Installer: `dist/meeting-notes-2.6.0-windows-amd64-installer.exe` (144 MB, audio-service embutido)
- Build canônico: `build.ps1` (não `wails build -nsis` direto).
- O `.spec` do PyInstaller é rastreado no git (`audio-service/build/pyinstaller/audio-service.spec`, com negação no `.gitignore`).
- **`master` está à frente da v2.6.0** com seis correções não lançadas.

## Armadilhas de ambiente

Todas no `CLAUDE.md`, seção "Rodando em dev". A que mais custou tempo:

- **O HMR do vite não chega à janela nativa** do `wails dev` — só ao navegador em `localhost:34115`. Reinicie o `wails dev` depois de mexer no frontend, não só no backend. O `hmr update` no log é o vite emitindo, não o webview aplicando: **não** use como prova de que a janela nativa atualizou. Isso fez uma correção correta ser reportada como "não funcionou".
- **O watcher só observa `cmd/desktop`** — mudança em `internal/**` não rebuilda o Go.
- **`SingleInstanceLock`** — um segundo `wails dev` sai com exit 0 em silêncio.
- Dev roda CUDA, produção roda CPU — ver DECISIONS de 2026-08-21.
