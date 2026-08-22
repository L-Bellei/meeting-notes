# Estado do Projeto — 2026-08-22

## Sessão
- **Data:** 2026-08-22
- **`master` (`5d44982`):** já carrega PR #45 (fallback GPU→CPU amplo no `audio-service`, quatro
  correções de frontend, e o checkbox de tasks do board que nunca gravava) e PR #46 (rascunho de
  descrição persistindo em cards manuais). Nenhuma release desde a v2.6.0 — ver "Estado de
  release". Este resumo descreve `master`; para o que uma branch específica mudou, ver o
  CHANGELOG na data correspondente.
- **Em andamento:** branch `feat/card-detail-modal-ux`, com as 9 tasks do plano
  `docs/superpowers/plans/2026-08-22-card-detail-modal-ux.md` completas (ver CHANGELOG
  [2026-08-22] "CardDetailModal — UI/UX..." para o que ela entrega). Ainda não tem PR aberta.
- **Worktree:** nenhum ativo

## Fase Superpowers

Plano `2026-08-22-card-detail-modal-ux` com as 9 tasks implementadas via Subagent-Driven
Development. Falta a etapa de finishing: abrir o PR e decidir se há uma review final de branch
inteira antes do merge (as reviews até aqui foram por task). Detalhes de execução —
rulings, achados e o que foi parqueado — estão em
`.superpowers/sdd/2026-08-22-card-detail-modal-ux/progress.md`, não duplicados aqui.

## Próximo passo imediato

Decidir o finishing de `feat/card-detail-modal-ux`: abrir o PR (possivelmente com uma review
final whole-branch antes, como as branches anteriores tiveram) e integrar em `master`. A
migration 017 roda no banco do usuário no próximo launch depois do merge — ver "Riscos" na spec.

Depois disso, dois itens no BACKLOG aguardando decisão do usuário:
- **Notificações de pipeline** — feature acordada antes, ainda não brainstormada.
- **Export** — exportar reunião/card em PDF, Markdown ou Notion.

## Worktrees paralelos

Nenhum.

## Estado de release

- **v2.6.0** publicada: https://github.com/L-Bellei/meeting-notes/releases/tag/v2.6.0
- Installer: `dist/meeting-notes-2.6.0-windows-amd64-installer.exe` (144 MB, audio-service embutido)
- Build canônico: `build.ps1` (não `wails build -nsis` direto).
- O `.spec` do PyInstaller é rastreado no git (`audio-service/build/pyinstaller/audio-service.spec`, com negação no `.gitignore`).
- **`master` está à frente da v2.6.0** com as correções de PR #45 e PR #46 ainda não lançadas, e
  `feat/card-detail-modal-ux` soma mais uma rodada de mudanças quando integrar.

## Armadilhas de ambiente

Todas no `CLAUDE.md`, seção "Rodando em dev". A que mais custou tempo:

- **O HMR do vite não chega à janela nativa** do `wails dev` — só ao navegador em `localhost:34115`. Reinicie o `wails dev` depois de mexer no frontend, não só no backend. O `hmr update` no log é o vite emitindo, não o webview aplicando: **não** use como prova de que a janela nativa atualizou. Isso fez uma correção correta ser reportada como "não funcionou".
- **O watcher só observa `cmd/desktop`** — mudança em `internal/**` não rebuilda o Go.
- **`SingleInstanceLock`** — um segundo `wails dev` sai com exit 0 em silêncio.
- Dev roda CUDA, produção roda CPU — ver DECISIONS de 2026-08-21.
