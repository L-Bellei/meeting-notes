# Estado do Projeto — 2026-08-22

## Sessão
- **Data:** 2026-08-22
- **`master` (`75baa22`) = v2.7.0 publicada.** Carrega o overhaul do `CardDetailModal` (PR #47) mais
  as correções que já estavam acumuladas: PR #45 (fallback GPU→CPU amplo no `audio-service`, quatro
  correções de frontend, e o checkbox de tasks do board que nunca gravava) e PR #46 (rascunho de
  descrição persistindo em cards manuais). Nada pendente de lançamento. Este resumo descreve
  `master`; para o que cada branch mudou, ver o CHANGELOG na data correspondente.
- **Worktree:** nenhum ativo

## Fase Superpowers

**N/A** — ciclo completo do plano `2026-08-22-card-detail-modal-ux`: brainstorm → spec → plano → 9
tasks via Subagent-Driven Development → review final whole-branch (opus) → fix wave → finishing →
release v2.7.0. O workspace de execução foi apagado; o registro é o git e o CHANGELOG.

## Próximo passo imediato

Nenhum em andamento. Duas features no BACKLOG aguardando sua decisão:
- **Notificações de pipeline** — acordada há tempo, ainda não brainstormada. Começar por
  `/superpowers:brainstorming`.
- **Export** — exportar reunião/card em PDF, Markdown ou Notion.

E três débitos que a v2.7.0 deixou explicitamente registrados em `BACKLOG.md` (Débitos técnicos),
todos com o raciocínio de por que ficaram fora: preview dos cards de reunião no board, reuniões
reprocessadas guardando resumo velho sob "Suas anotações", e três hooks de `useMeeting.ts` fora do
helper de invalidação (verificados latentes, não defeituosos).

O argumento mais forte do backlog continua sendo **`vitest` no frontend**: quatro dos seis defeitos
reais encontrados durante a execução da v2.7.0 seriam pegos por um único teste de montagem —
o foco inicial caindo no botão de excluir, o `Shift+Tab` escapando do trap, o select de coluna
revertendo, e o estado vazio não atualizando depois de gerar tasks.

## Worktrees paralelos

Nenhum.

## Estado de release

- **v2.7.0** publicada: https://github.com/L-Bellei/meeting-notes/releases/tag/v2.7.0
- Installer: `dist/meeting-notes-2.7.0-windows-amd64-installer.exe` (143,3 MB, audio-service
  embutido, transcrição em CPU). Bundle de 344 MB + binário de 31 MB conferidos em
  `cmd/desktop/build/bin` antes do NSIS — a checagem que evita publicar instalador sem o
  audio-service.
- Build canônico: `build.ps1` (não `wails build -nsis` direto). Mate o `wails dev` antes: ele
  trava arquivos em `build/bin`.
- O `.spec` do PyInstaller é rastreado no git (`audio-service/build/pyinstaller/audio-service.spec`, com negação no `.gitignore`).
- **`master` está em paridade com a última release.** Nada mergeado sem lançar.
- A migration 017 roda no banco do usuário no primeiro launch da v2.7.0 e limpa a descrição dos
  cards de reunião onde ela ainda é idêntica ao resumo.

## Armadilhas de ambiente

Todas no `CLAUDE.md`, seção "Rodando em dev". A que mais custou tempo:

- **O HMR do vite não chega à janela nativa** do `wails dev` — só ao navegador em `localhost:34115`. Reinicie o `wails dev` depois de mexer no frontend, não só no backend. O `hmr update` no log é o vite emitindo, não o webview aplicando: **não** use como prova de que a janela nativa atualizou. Isso fez uma correção correta ser reportada como "não funcionou".
- **O watcher só observa `cmd/desktop`** — mudança em `internal/**` não rebuilda o Go.
- **`SingleInstanceLock`** — um segundo `wails dev` sai com exit 0 em silêncio.
- Dev roda CUDA, produção roda CPU — ver DECISIONS de 2026-08-21.
