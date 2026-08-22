# Changelog de Sessões

---

## [2026-08-22] CardDetailModal — UI/UX, descrição como anotação do usuário — Release pendente

**Plano Superpowers:** `docs/superpowers/plans/2026-08-22-card-detail-modal-ux.md` (9 tasks, Subagent-Driven Development)
**Spec:** `docs/superpowers/specs/2026-08-22-card-detail-modal-ux-design.md`
**Fase do workflow Superpowers:** implementação completa (9/9 tasks, cada uma com review de task) na branch `feat/card-detail-modal-ux`, ainda não integrada — falta abrir o PR e a review final. `master` não muda nesta sessão; ver STATE.md para o que já está lá.

**Entregue — os dez achados de UI/UX investigados nesta mesma data:**
- Um único scroll no corpo do modal; saem as três áreas de scroll aninhadas (corpo + descrição + resumo), que capturavam a roda do mouse.
- `Escape` fecha o modal — era o único do app que não fechava — com `role="dialog"`, `aria-modal` e focus trap.
- Confirmação de exclusão de dois cliques passa a resetar sozinha depois de ~4s (antes nunca resetava) e também ao trocar ou fechar o card.
- Edição de descrição ganha um lápis explícito; clicar no texto não entra mais em edição.
- Optimistic update no checkbox de tasks, com rollback e mensagem de erro por-task.
- Estado vazio de tasks com botão "Gerar tasks", habilitado por `has_transcript`.
- Prioridade e responsável de cada task aparecem na linha.
- Header reescrito: título dominante, barra de 3px na cor do tema, sem badge com fundo colorido.
- Mover de coluna por `<select>` no header, sem precisar fechar o modal e arrastar no board.
- `max-w-[calc(100vw-2rem)]` — o modal encolhe em janela estreita em vez de sangrar.

**O 11º achado, que mudou o desenho mais que os dez juntos:** medido no banco de dev, o card #1
tinha `description` e `summary` byte-a-byte idênticos, **1867 caracteres** cada — a descrição
era a cópia que `BoardCardService.Create` tirava do resumo na criação e nunca ressincronizava.
Editar "a descrição" de um card de reunião era editar uma cópia congelada do resumo, sem
relação nenhuma com o resumo vivo mostrado ao lado. A descrição passa a ser anotação do
usuário, vazia por padrão; a migration `017_card_description_annotations.sql` limpa **apenas**
as descrições ainda idênticas ao resumo, preservando o que foi editado (decisão em
DECISIONS.md).

**Backend:** `has_transcript: boolean` no detalhe do card, via `MeetingRepository.HasTranscript`
(`COUNT(*)`, sem carregar o transcript inteiro), para o botão "Gerar tasks" saber se a operação
é possível antes de o usuário bater num 422.

**Frontend — decomposição em cinco arquivos:** `CardDetailModal.tsx` (~415 linhas) virou casca
mais quatro componentes focados, seguindo o padrão que a aba de temas estabeleceu na v2.6.0:
`CardModalHeader.tsx`, `CardTasksSection.tsx`, `CardNotesSection.tsx` e `ui/ExpandableText.tsx`
(o "ver mais" medido por `scrollHeight` vs `clientHeight`, com `WebkitLineClamp` inline porque o
Tailwind JIT não gera classe de `line-clamp` com valor dinâmico).

**A mesma classe de bug apareceu três vezes neste código:** uma mutation invalidando uma chave
do React Query diferente da que alimenta a view que deveria reagir. Primeiro o checkbox de task
(PR #45), depois o `<select>` de coluna desta branch (`useMoveCard` invalidava só
`["board-cards"]`, mas o select lê `card.column_id` de `["board-card", id]` e revertia
visualmente depois de mover), e depois "Gerar tasks" desta branch (`useGenerateTasks` invalidava
só `["meeting", meetingId]`, mas o estado vazio lê `card.tasks` de `["board-card", id]`, então a
seção continuava mostrando "Nenhuma task" com o servidor já tendo gerado). Os dois casos desta
branch foram corrigidos nos hooks (`useMoveCard` em `useBoard.ts`, `useGenerateTasks` em
`useMeeting.ts`), não nos componentes que os chamam. **Quatro hooks irmãos em `useMeeting.ts`
carregavam o mesmo defeito latente** — `useGenerateSummary`, `useGenerateKeyPoints`,
`useReprocess` e `useRetranscribe` invalidavam só `["meeting", …]`, mas alimentam views que leem
de `["board-card"]`. Fechado na fix wave da review final: extraído o helper único
`invalidateMeetingDerivedQueries(qc, meetingId)` em `useMeeting.ts`, que os seis mutations do
arquivo (os quatro acima, mais `useGenerateTasks` e `useUpdateTask`, que já invalidavam as três
famílias de chave à mão) agora chamam, em vez de deixar um hook correto e quatro errados no mesmo
arquivo.

**Todo defeito relevante achado nas reviews das tasks 4, 5, 6 e 8 era defeito do plano, não do
trabalho dos implementers** — eles transcreveram o plano fielmente; era o plano que estava
errado (Task 5: `useMoveCard` sem a invalidação certa; Task 8: o `onError` do plano restaurava o
snapshot de **todas** as tasks em vez de só a que falhou, então marcar duas tasks em sequência e
uma falhar desmarcava a outra, que tinha tido sucesso). Duas dessas ocorrências foram introduzidas
pela própria correção do controlador para um achado anterior, não pelo plano original: a remoção
do `notesTextareaRef` (para não violar `noUnusedLocals` na Task 4) deixaria a Task 7 sem compilar
se não fosse pega no preflight; e o Shift+Tab escapando do focus trap na Task 4 foi consequência
direta de focar o painel do modal em vez do primeiro elemento focável — a própria correção do
achado anterior daquela mesma task.

**Sem teste de render no frontend, a verificação continua sendo `tsc --noEmit` + `npm run build`
mais roteiro manual na janela nativa.** Os dois bugs corrigidos nos PRs #45/#46 e os achados desta
branch (foco inicial caindo no botão de excluir em vez do painel, select de coluna revertendo,
"Gerar tasks" não atualizando a view) são exatamente o tipo de regressão que um único teste de
montagem pegaria — reforça o débito já registrado no BACKLOG.

**Processo/Qualidade:** brainstorm → spec → plano → execução via Subagent-Driven Development (9
tasks, implementer + review por task) → 15 rulings do controlador registrados no ledger de
execução (`.superpowers/sdd/2026-08-22-card-detail-modal-ux/progress.md`). Tasks 1, 2, 3 e 7
fecharam com review limpa; tasks 5, 6 e 8 precisaram de uma rodada de fix cada. **Task 4
precisou de duas:** a primeira corrigiu o achado original (foco caindo no botão de excluir a
cada re-render), mas a própria correção — focar o painel do modal em vez do primeiro elemento
focável — abriu um novo Important (`Shift+Tab` escapando do focus trap, porque o painel focado
tem `tabIndex={-1}` e ficava fora do seletor de `focusable`); a segunda rodada fechou esse achado
generalizando o handler de `Tab` para tratar qualquer foco fora de `focusable` como o caso de
wrap. Todas resolvidas antes de seguir para a próxima task.

**Parqueado no BACKLOG:** o primitivo `Modal` compartilhado para os outros cinco modais do app
(nenhum tem `role="dialog"` nem focus trap) — ver decisão 4 da spec.

---

## [2026-08-22] Correção dos bugs conhecidos + checkbox de tasks do board — Release pendente

**Plano Superpowers:** `docs/superpowers/plans/2026-08-21-known-bug-fixes.md` (2 tasks, Subagent-Driven Development)
**Fase do workflow Superpowers:** ciclo completo — PR #45 mergeada em `master` (`bbf9fd1`). Sem release: as correções ficam para a próxima versão.

**Entregue:**
- **Fallback GPU→CPU amplo** no `audio-service`: qualquer exceção de inferência na GPU (OOM, driver, DLL) recarrega o modelo em CPU e refaz a transcrição, em vez de só erros de DLL. 3 testes novos, incluindo asserção do log.
- **Quatro correções de frontend**: mensagem de erro de exclusão em pt-BR, cancelamento de drag, fechamento do menu `⋯` em scroll/resize, hambúrguer na Board view.
- **Checkbox de tasks do CardDetailModal nunca gravava** (reportado pelo usuário, fora do plano): `TaskRow` mandava só `{ completed }`, o `updateTaskRequest.Description` é `string` simples, o campo ausente decodificava para `""` e o `TaskService.Update` devolvia **422 "description is required"**. O checkbox controlado revertia e o erro era engolido. Correção em três pontos — reusar o tipo `Task` compartilhado em `BoardCardDetail` (o tipo inline estreito escondia `due_date`, que um spread teria apagado), enviar `{ ...task, completed }` como o `MeetingDetail` já fazia, e invalidar `board-card`/`board-cards` no `useUpdateTask`. Mais `salvando...`/mensagem de erro na linha, para a falha parar de ser silenciosa.

**Armadilha de ambiente descoberta (a mais cara desta sessão):** o **HMR do vite não chega à janela nativa** do `wails dev`. A correção do checkbox foi reportada como "não funcionou" com o código já correto: o navegador em `localhost:34115` aplicava a atualização, a janela do WebView2 seguia com o código do boot. Eu tratei o `hmr update` no log do `wails dev` como prova de que o webview recebeu — é o vite *emitindo*, não o cliente *aplicando*. Registrado no `CLAUDE.md` ao lado da armadilha do watcher só observar `cmd/desktop`.

**Método que resolveu:** o diagnóstico saiu de reproduzir o payload exato contra o app rodando (`{completed:true}` → 422; `{...task,completed:true}` → 200) e de conferir o **banco** depois de cada teste, o que separou "não gravou" de "gravou e a tela não atualizou". A prova final foi o usuário testar na web: uma task passou a `completed` no banco, o que descartou o código e apontou para a janela.

**Parqueado no BACKLOG:** downgrade permanente para CPU até reiniciar o app, log do fallback sem destino no app empacotado, foot-gun na fixture de testes do `transcriber`, e os 10 itens de UI/UX do `CardDetailModal` (investigados, sem decisão de escopo).

**Sequência (PR seguinte, `fix/manual-card-draft-description`):** o bug do rascunho de descrição em cards manuais — `toggleTask`/`addTask`/`removeTask` mandavam o `description` do state local, e como o `PUT /api/board/cards/{id}` **substitui** a descrição, editar sem salvar e clicar num checkbox persistia o rascunho. Passaram a reenviar `card.description` (o valor persistido). Mecanismo confirmado contra a API antes da correção: marcar uma task gravou `'RASCUNHO nao salvo'` como descrição de um card temporário.

---

## [2026-08-21] Prompt único por tema + overhaul da aba de temas — Release v2.6.0

**Plano Superpowers:** `docs/superpowers/plans/2026-08-20-themes-single-prompt-and-sidebar.md`
**Spec:** `docs/superpowers/specs/2026-08-20-themes-single-prompt-and-sidebar-design.md`
**Fase do workflow Superpowers:** finishing concluído (PR #43 mergeada) + release.

**Entregue (7 tasks via Subagent-Driven Development):**
- **Revert dos prompts por tipo** (migration `016` com 3 `DROP COLUMN`): saem `PromptKind`, `Theme.PromptFor`, `models.ThemePrompts`; `ThemeService.Create/Update` recebem `customPrompt string`. `internal/ai` intocado. Motivo: os 3 campos estavam **vazios em todos os temas** do banco de produção.
- **Painel de temas fixável** (`sidebar_pinned` em settings), sem auto-close ao selecionar, `Ctrl+B`, recolhe no Board e retoma ao voltar.
- **Chip de filtro ativo** no header de Reuniões — antes nada indicava filtro com a gaveta fechada.
- **Linha reescrita**: barra de cor de 3px, badges de prompt/auto-board, menu `⋯` sempre visível, três **botões irmãos** (corrige interativo aninhado).
- **Exclusão com confirmação escrita** usando a contagem de reuniões **diretas** e o efeito real das FKs `ON DELETE SET NULL`.
- **Hierarquia de 2 níveis** validada no service (auto-referência / pai-com-pai / tema-com-filhos → 422) + drag-and-drop com faixa de raiz; expansão em `localStorage`, podada contra temas existentes.
- **Modal unificado** de criar/editar com cor, descrição e prompt.

**Bugs achados testando o app rodando (não pelo review estático):**
- **Faixa "mover para a raiz" era UI morta** — `useDroppable` registrava no contexto default do dnd-kit porque o hook rodava no componente que renderiza o `DndContext`. Rebaixar tema era porta de mão única. (Critical na review final.)
- **Contagem errada na confirmação de exclusão** — usava a soma que inclui reuniões dos filhos.
- **`{...drag.attributes}` reaninhava interativos** — dnd-kit emite `role="button" tabIndex=0` mesmo desabilitado, desfazendo a correção de acessibilidade da própria branch.
- **Corrida no boot da query de settings** — `useSettings` disparava antes de `initApi(port)`, URL relativa retornava HTML, query morria em erro; o pino então parecia morto (clique mandava o valor que já estava no banco). Corrigido com gate de readiness via `useSyncExternalStore`. **Reportado pelo usuário**, não pelos reviews.

**Processo/Qualidade:** brainstorm → spec → plano → execução TDD (7 tasks, implementer + task-reviewer por task) → review final whole-branch (opus): *Ready to merge **No*** com 1 Critical + 2 Important → fix wave de 10 achados → re-review (9/10) → 1 minor parqueado. Duas decisões de "plano vs review" foram levadas ao usuário.

**Build/empacotamento:** o `.spec` do PyInstaller **estava perdido** (vivia em diretório gitignored, nunca commitado). Recriado, validado com `device: cuda` (1,9 GB) e depois reduzido a CPU-only (344 MB) por decisão do usuário. Agora **rastreado no git**. Instalador de 144 MB validado ponta a ponta: o binário de produção sobe o **próprio** bundle e o `/health` responde `model_loaded: true`.

**Decisões transversais registradas:** prompt único (revisita 2026-04-29 e 2026-07-21), localStorage para estado efêmero de UI vs settings no banco, teto de 2 níveis na hierarquia, e **instalador CPU-only** (descoberta de que produção nunca teve CUDA).

**Bloqueios:** o build da release ficou bloqueado até o `.spec` ser recriado do zero — não havia cópia do bundle nem do spec em nenhum lugar da máquina.

---

## [2026-07-21] Prompts personalizados por tipo de geração — Release v2.5.0

**Plano Superpowers:** `docs/superpowers/plans/2026-07-20-theme-type-prompts.md`
**Spec:** `docs/superpowers/specs/2026-07-20-theme-type-prompts-design.md`
**Fase do workflow Superpowers:** finishing concluído (PR #40 mergeada) + release.

**Entregue (4 tasks via Subagent-Driven Development):**
- **Migration 015** + `Theme.CustomSummaryPrompt/CustomKeyPointsPrompt/CustomTasksPrompt` + `Theme.PromptFor(kind)` (precedência específico → geral → `""`) + repository.
- **`models.ThemePrompts`** (General/Summary/KeyPoints/Tasks) threadado por `ThemeService.Create/Update` + request structs do handler (evita 4 strings posicionais).
- Geração (3 handlers + orchestrator `runAIGeneration`) resolve por tipo via `PromptFor`. **Assinaturas de AI client e generation services inalteradas** (o degrau `"" → default` já é feito por `buildInstruction`).
- **Frontend**: `ThemeEditModal` com "Prompt geral" + 3 textareas (resumo/pontos/tarefas) + hint de precedência; tipo `Theme` + payload de update.

**Estrutura escolhida:** geral + 3 overrides (revisita a decisão de 2026-04-29 "customPrompt campo único", que era YAGNI deliberado). Zero perda de dados: temas existentes caem no geral via fallback.

**Processo/Qualidade:** brainstorm → spec → plano → execução TDD (implementer haiku + task-reviewer sonnet por task) → review final whole-branch (opus): *Ready to merge: Yes*, 0 Critical/Important. 3 Minors deferidos (dedup cosmético no handler; gofmt align — moot por CRLF do repo; create sem prompts — escopo planejado). Validado ao vivo (migration 015 aplica; resumo segue prompt específico, pontos/tarefas caem no geral).

**Release:** bump 2.4.2 → 2.5.0 (PR #41, minor), instalador via `build.ps1` (`dist/meeting-notes-2.5.0-windows-amd64-installer.exe`, 125.7 MB), tag `v2.5.0` + GitHub Release.

**Bloqueios:** merges de PR (#40, #41) exigiram aprovação explícita do usuário (classificador de auto-mode).

---

## [2026-07-20] Whisper "auto" — detecção real de idioma + exibição — Release v2.4.2

**Plano Superpowers:** `docs/superpowers/plans/2026-07-20-whisper-auto-language.md`
**Spec:** `docs/superpowers/specs/2026-07-20-whisper-auto-language-detection-design.md`
**Fase do workflow Superpowers:** finishing concluído (PR #37 mergeada) + release.

**Entregue (4 tasks via Subagent-Driven Development):**
- **Python** (`audio-service/transcriber.py`): `"auto"`/`""`/`None` → `language=None` no faster-whisper (detecção real); removido o fallback silencioso `or self.default_language` e, no cleanup do review final, o próprio estado morto `default_language`/`WHISPER_LANGUAGE`.
- **Migration 014** + `Meeting.Language *string` + repository (SELECT/UPDATE/scan; round-trip testado, inclusive NULL).
- **Orchestrator**: encaminha `whisper_language` cru (inclui `"auto"`) e persiste `trResp.Language` nos **dois** caminhos (captura e retranscribe).
- **Frontend**: tipo `Meeting.language`, helper `languageLabel` (nomes PT + fallback pro código), badge no MeetingDetail (só quando há idioma). IA já é agnóstica de idioma → sem mudança de IA.

**Processo/Qualidade:** brainstorm → spec → plano → execução TDD (implementer haiku + task-reviewer sonnet por task) → review final whole-branch (opus): *Ready to merge: Yes*, 0 Critical/Important. 3 Minors triados: 2 corrigidos (dead `default_language`, hardening de teste), 1 deferido (teste dedicado do retranscribe — bloco byte-idêntico ao caminho de captura já coberto).

**Verificação:** `go vet`/`go test ./...` verde; Python `pytest` (transcriber) verde; `tsc`/`build` ok. Validado ao vivo via `wails dev` (migration 014 aplica, boot limpo, audio-service CUDA saudável, badge de idioma).

**Release:** bump 2.4.1 → 2.4.2 (PR #38), instalador via `build.ps1` (`dist/meeting-notes-2.4.2-windows-amd64-installer.exe`, 125.7 MB), tag `v2.4.2` + GitHub Release.

**Bloqueios:** merges de PR (#37, #38) exigiram aprovação explícita do usuário (classificador de auto-mode do Claude Code).

---

## [2026-07-18] Fix tray clicks/hotkey + overlay meeting guard — Release v2.4.1

**Sem plano Superpowers** — bugfix ad-hoc + release (mesmo padrão de #32/#33-34).

**Fase do workflow Superpowers:** N/A (implement → review inline → finishing).

**Entregue:**
- **Tray clicks + hotkey global** (`45c9e6a`, PR #35): criação da janela, registro do hotkey e message loop movidos para uma única OS thread travada (`run()` com `LockOSThread`), corrigindo thread affinity do Win32 que fazia cliques do ícone e o hotkey serem perdidos. Teardown via `WM_CLOSE` na thread do loop; `WM_NULL` pós-`TrackPopupMenu` (KB135788) para o menu dispensar corretamente.
- **Overlay meeting guard** (mesmo PR): `HideIfMeeting(meetingID)` impede que um status terminal (transcribing/processing/completed/failed) de uma reunião anterior esconda o overlay de uma gravação mais nova. Coberto por `cmd/desktop/overlay_test.go` (2 casos).
- Bump `productVersion` 2.4.0 → 2.4.1 (PR #36; `master` protegido).
- Instalador via `build.ps1`: `dist/meeting-notes-2.4.1-windows-amd64-installer.exe` (125.7 MB, audio-service embutido).
- Tag `v2.4.1` + GitHub Release publicada com o instalador.

**Verificação:** `go vet ./...` limpo, `go test ./...` verde (inclui os testes do guard), ambos entry points compilam; app validado via `wails dev` (audio-service saudável em CUDA, monitor de saúde recuperando após o load do modelo `medium`).

**Decisão transversal registrada em DECISIONS.md:**
- Toda janela Win32 em Go: criação + registro de hotkey + message loop na mesma OS thread travada (generaliza a decisão de 2026-05-01 do overlay para o tray).

**Bloqueios:** merges de PR (`#35`, `#36`) bloqueados pelo classificador de auto-mode do Claude Code → exigiram aprovação explícita do usuário.

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
