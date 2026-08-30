# Backlog

Itens fora do escopo das features já implementadas. Para features com plano ativo do Superpowers, apenas referência — sem duplicação de conteúdo.

---

## Features com plano pronto (não iniciadas)

- _(nenhuma — todos os planos em `docs/superpowers/plans/` já foram implementados)_

---

## Features futuras (não brainstormadas)

- **Notificações de pipeline** — notificação nativa do Windows quando o processamento de uma reunião termina. Acordada, ainda não brainstormada. `git.sr.ht/~jackmordaunt/go-toast/v2` já está no `go.mod` como dependência indireta, sem uso direto.
- **Export** — exportar reunião (ou card do board) em PDF, Markdown ou Notion.

---

## Débitos técnicos

- **Vulkan não homologado em GPU AMD real** — a feature de 2026-08-30 foi validada forçando Vulkan na RTX 2050 (`WHISPER_FORCE_BACKEND=vulkan`). Primeira máquina AMD disponível: instalar, conferir `gpu_vendor: amd` e `gpu_backend: vulkan` no `/health`, transcrever e comparar tempo com CPU. Se falhar, o fallback por chamada cai em CPU — não trava a reunião; verificar também um modelo não-medium (só o GGML do medium foi provado no smoke).
- **Modelo GGML baixado sob demanda na primeira transcrição Vulkan** — sem barra de progresso; ~540 MB (medium q5_0). Se incomodar, pré-baixar ao salvar o seletor em GPU ou expor progresso via `/health`.
- **Drift de tipo nos campos Vulkan do health** — Python emite `gpu_vendor/gpu_backend: null`, o Go decodifica em `string` zero-value e os mirrors reemitem `""`, mas `useAudioHealth.ts` tipa `| null`; sem bug em runtime (comparações estritas contra valores reais), corrigir eventualmente com `*string` no `HealthResponse` ou `"" |` no union TS.

- **Instalador de 631 MB — investigar redução** — a cublas (~700 MB brutos) comprime mal no LZMA e domina o tamanho; a poda de cuDNN já foi feita (1,85→1,07 GB). Direções possíveis: poda adicional dentro da cublas (kernels por arquitetura), compressão sólida/7z como container, ou pacote GPU baixado sob demanda (rejeitado em 2026-08-29 pelo requisito de instalação autossuficiente — reabrir só se o tamanho incomodar na prática).

- **Reuniões reprocessadas guardam resumo velho rotulado como anotação do usuário** — a migration 017 limpa a descrição só onde ela é **idêntica** ao resumo atual. Divergência tem duas causas: o usuário editou, ou o resumo foi regerado depois da criação do card (`useReprocess`, `useRetranscribe`, `useGenerateSummary`). No segundo caso o card guarda um resumo *anterior*, não é limpo, e o modal mostra ~1800 caracteres de texto de IA sob o título "Suas anotações". A review final de 2026-08-22 propôs alargar a migration com `AND updated_at = created_at` (qualquer edição, toggle de task ou salvamento de nota bumpa `updated_at`, então a cláusula não destruiria texto do usuário). **Decisão consciente de não alargar**: a migration roda no banco real do usuário, a heurística é indireta, e a assimetria de risco favorece não mexer — deixar como está é recuperável (o usuário limpa a nota à mão), apagar não é. Reabrir se aparecer na prática.
- **Cards de reunião no board ficaram só com título** — `KanbanCard` renderiza `card.description` como preview de duas linhas. Depois da migration 017 esse campo é vazio em todo card de reunião não anotado, incluindo os criados automaticamente pelo orchestrator. Nem a spec nem o plano previram essa consequência em outra tela; a review final de 2026-08-22 a encontrou. Restaurar o preview exige um campo de excerto de resumo em `BoardCardSummary` — mudança de backend com decisão própria. **Aceito como está** na v2.7.0, verificado na janela nativa pelo usuário.
- **Três hooks de `useMeeting.ts` fora do helper de invalidação** — `useUpdateMeeting`, `useStartRecording` e `useStopRecording` invalidam só `["meeting", id]` e `["meetings"]`, enquanto os outros seis passam por `invalidateMeetingDerivedQueries`. Verificado em 2026-08-22 que **não** é bug ativo: `card.status` não é renderizado em nenhum componente do board, e os dois call sites de `useUpdateMeeting` sempre reenviam título e tema inalterados. Latente — passa a importar se um badge de status entrar no `KanbanCard` ou se surgir UI que edite título/tema por esse endpoint. Rotear os três pelo helper é barato e inócuo.

- **Primitivo `Modal` compartilhado** — `CardDetailModal` ganhou `Escape`, `role="dialog"`, `aria-modal` e focus trap em 2026-08-22, mas os outros cinco modais (`SearchModal`, `SettingsModal`, `RecordingModal`, `CreateManualCardModal`, `ThemeEditModal`) seguem cada um com seu `Escape` e **nenhum** com `role="dialog"` nem focus trap. Extrair um componente `Modal` e migrar os seis foi deixado fora de escopo por ser risco desproporcional sem teste de render.
- ~~**Quatro hooks irmãos em `useMeeting.ts` com o mesmo defeito de invalidação de cache**~~ — `useGenerateSummary`, `useGenerateKeyPoints`, `useReprocess` e `useRetranscribe` invalidavam só `["meeting", meetingId]`, mas alimentam telas do board que leem `summary`/`key_points`/`tasks` de `["board-card", id]`. Era a mesma forma do bug corrigido no checkbox de task (PR #45) e, na branch `feat/card-detail-modal-ux`, no `useMoveCard` e no `useGenerateTasks` (ver CHANGELOG [2026-08-22]). Resolvido na fix wave da review final: os quatro hooks e os dois já corretos passaram a chamar o helper único `invalidateMeetingDerivedQueries` (`useMeeting.ts`).
- **Sem infra de teste no frontend** — não há vitest nem testing-library, então toda verificação de UI é `tsc --noEmit` + `npm run build` + exercício manual. A review final da v2.6.0 apontou isso como o argumento mais forte do backlog: os dois bloqueadores daquela branch (um droppable inerte e uma contagem errada em texto de confirmação) seriam pegos por um único teste de render.
- **Instalador 144 MB vs 125,7 MB das releases anteriores** — o `.spec` recriado inclui coisas que o original provavelmente excluía (`av` 67 MB, `onnxruntime` 37 MB, PIL 13 MB). Investigar se são realmente necessárias em runtime.
- **Reparent de temas é só com mouse** — o drag-and-drop usa apenas `PointerSensor`. Adicionar `KeyboardSensor` + um handle de arraste dedicado para tornar a reorganização acessível por teclado.
- **Chunk size warning no build do frontend** — bundle JS de ~518 kB. Considerar code-splitting com `React.lazy` para BoardView e modais pesados.
- **Migrations não são transacionais** — `runMigrations` (`internal/database/database.go`) aplica cada arquivo fora de transação. Uma falha no meio de uma migration não-idempotente deixa o banco parcialmente migrado e não registrado, e o `Open` passa a falhar para sempre — estado ruim de recuperar num app desktop que migra no launch.
- **Silero VAD no PyInstaller** — `vad_filter=True` foi removido por falhar no bundle. Para reativar, os dados do modelo Silero precisam entrar explicitamente no `.spec`.
- **Health do claude-code valida binário, não o token** — validação real só no botão Testar conexão; `/api/ai/health` responde `valid:true` com token expirado.
- **`claude -p` headless herda configuração user-level da máquina** — skills, MCP servers e `CLAUDE.md` global do Claude Code instalado na máquina são herdados pelo processo headless usado nas gerações, o que pode adicionar latência de boot ou comportamento inesperado. O modo `--bare` não aceita token de subscription, então não é saída viável. Investigar flags de isolamento de config quando isso incomodar na prática.
- **Instalador NSIS não detecta o app aberto** — instalar por cima com o Meeting Notes rodando falha ao sobrescrever `Meeting Notes.exe` e aborta no meio: os arquivos do `audio-service/` e o `uninstall.exe` ficam novos, o exe principal e o registro (DisplayVersion) ficam velhos — instalação híbrida silenciosa (observado em 2026-08-28 ao instalar a v2.7.0 sobre a v2.6.0). O template NSIS do Wails precisa de um check de processo em execução (fechar ou abortar antes de extrair).

---

## Bugs conhecidos (sem plano)

- _(nenhum)_
