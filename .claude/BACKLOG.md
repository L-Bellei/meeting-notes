# Backlog

Itens fora do escopo das features já implementadas. Para features com plano ativo do Superpowers, apenas referência — sem duplicação de conteúdo.

---

## Features com plano pronto (não iniciadas)

- _(nenhuma — todos os planos em `docs/superpowers/plans/` já foram implementados)_

---

## Features futuras (não brainstormadas)

- **Notificações de pipeline** — notificação nativa do Windows quando o processamento de uma reunião termina. **Próxima feature acordada com o usuário**; começar por `/superpowers:brainstorming`. `git.sr.ht/~jackmordaunt/go-toast/v2` já está no `go.mod` como dependência indireta, sem uso direto.
- **Export** — exportar reunião (ou card do board) em PDF, Markdown ou Notion.

---

## Débitos técnicos

- **Reuniões reprocessadas guardam resumo velho rotulado como anotação do usuário** — a migration 017 limpa a descrição só onde ela é **idêntica** ao resumo atual. Divergência tem duas causas: o usuário editou, ou o resumo foi regerado depois da criação do card (`useReprocess`, `useRetranscribe`, `useGenerateSummary`). No segundo caso o card guarda um resumo *anterior*, não é limpo, e o modal mostra ~1800 caracteres de texto de IA sob o título "Suas anotações". A review final de 2026-08-22 propôs alargar a migration com `AND updated_at = created_at` (qualquer edição, toggle de task ou salvamento de nota bumpa `updated_at`, então a cláusula não destruiria texto do usuário). **Decisão consciente de não alargar**: a migration roda no banco real do usuário, a heurística é indireta, e a assimetria de risco favorece não mexer — deixar como está é recuperável (o usuário limpa a nota à mão), apagar não é. Reabrir se aparecer na prática.
- **Cards de reunião no board ficaram só com título** — `KanbanCard` renderiza `card.description` como preview de duas linhas. Depois da migration 017 esse campo é vazio em todo card de reunião não anotado, incluindo os criados automaticamente pelo orchestrator. Nem a spec nem o plano previram essa consequência em outra tela; a review final de 2026-08-22 a encontrou. Restaurar o preview exige um campo de excerto de resumo em `BoardCardSummary` — mudança de backend com decisão própria. **Aceito como está** na v2.7.0, verificado na janela nativa pelo usuário.
- **Três hooks de `useMeeting.ts` fora do helper de invalidação** — `useUpdateMeeting`, `useStartRecording` e `useStopRecording` invalidam só `["meeting", id]` e `["meetings"]`, enquanto os outros seis passam por `invalidateMeetingDerivedQueries`. Verificado em 2026-08-22 que **não** é bug ativo: `card.status` não é renderizado em nenhum componente do board, e os dois call sites de `useUpdateMeeting` sempre reenviam título e tema inalterados. Latente — passa a importar se um badge de status entrar no `KanbanCard` ou se surgir UI que edite título/tema por esse endpoint. Rotear os três pelo helper é barato e inócuo.

- **Primitivo `Modal` compartilhado** — `CardDetailModal` ganhou `Escape`, `role="dialog"`, `aria-modal` e focus trap em 2026-08-22, mas os outros cinco modais (`SearchModal`, `SettingsModal`, `RecordingModal`, `CreateManualCardModal`, `ThemeEditModal`) seguem cada um com seu `Escape` e **nenhum** com `role="dialog"` nem focus trap. Extrair um componente `Modal` e migrar os seis foi deixado fora de escopo por ser risco desproporcional sem teste de render.
- ~~**Quatro hooks irmãos em `useMeeting.ts` com o mesmo defeito de invalidação de cache**~~ — `useGenerateSummary`, `useGenerateKeyPoints`, `useReprocess` e `useRetranscribe` invalidavam só `["meeting", meetingId]`, mas alimentam telas do board que leem `summary`/`key_points`/`tasks` de `["board-card", id]`. Era a mesma forma do bug corrigido no checkbox de task (PR #45) e, na branch `feat/card-detail-modal-ux`, no `useMoveCard` e no `useGenerateTasks` (ver CHANGELOG [2026-08-22]). Resolvido na fix wave da review final: os quatro hooks e os dois já corretos passaram a chamar o helper único `invalidateMeetingDerivedQueries` (`useMeeting.ts`).
- **Sem infra de teste no frontend** — não há vitest nem testing-library, então toda verificação de UI é `tsc --noEmit` + `npm run build` + exercício manual. A review final da v2.6.0 apontou isso como o argumento mais forte do backlog: os dois bloqueadores daquela branch (um droppable inerte e uma contagem errada em texto de confirmação) seriam pegos por um único teste de render.
- **Instalador transcreve em CPU — empacotar GPU (analisado e medido em 2026-08-21)**
  O bundle exclui as DLLs de CUDA de propósito (ver DECISIONS 2026-08-21). Medições nesta máquina (RTX 2050 4 GB, 12 threads), gravação real de 146s, via a própria classe `Transcriber`:

  | Config | Transcrição | × tempo real | Chars |
  |---|---|---|---|
  | `medium` GPU (int8_float16) | 35,5s | 4,12× | 2218 |
  | `medium` CPU (int8) — produção hoje | 112,6s | 1,30× | 2171 |
  | `small` CPU (int8) | 53,0s | 2,76× | 1758 |

  **Ganho:** 3,2×. Numa gravação de 85 min (a maior no banco): ~21 min na GPU vs ~65 min na CPU.
  **Custo:** as 14 DLLs somam 1,68 GB brutos; compressão LZMA medida (razão 0,27 em amostra de 72 MB) projeta **+465 MB** no instalador → de 144 MB para **~610 MB**.
  **Sem mudança de código:** o `transcriber.py` já detecta e usa CUDA; faltam só as DLLs no pacote.
  **Pré-requisito (resolvido em 2026-08-21):** o fallback de GPU→CPU já existe no `except` de `transcribe()` em `audio-service/transcriber.py` — qualquer falha na GPU (DLL ausente, OOM, driver) recarrega o modelo em CPU e tenta de novo em vez de falhar a reunião.
  **Pré-requisito (ainda aberto):** o timeout de 60 min em `internal/audio/client.go:67`. Com `medium` em CPU medindo 1,30× tempo real, ~46 min de áudio já consomem o orçamento inteiro numa única passagem. Com o fallback de GPU, uma falha a meio da transcrição (driver/kernel, diferente de OOM que falha logo no início) queima tempo de parede na GPU antes de reiniciar a transcrição inteira em CPU — o Go pode expirar enquanto o Python ainda processa, e o orchestrator marca a reunião como FAILED, perdendo exatamente a reunião que o fallback deveria salvar. É preciso aumentar ou remover esse timeout antes de embarcar as DLLs de CUDA. Hoje é latente (produção é CPU-only; `device == "cuda"` só ocorre em dev).
  **Observações abertas sobre o fallback de GPU (não bloqueiam este item, mas não devem se perder):**
  - *Downgrade para CPU é permanente até reiniciar o app* — `Transcriber` é um singleton criado uma vez no lifespan do FastAPI; após um fallback, `self.device = "cpu"` fixa **todas as reuniões seguintes** em CPU (≈3,2× mais lento) até o processo reiniciar. Fazia sentido quando só condições permanentes disparavam o fallback; agora um aperto momentâneo de VRAM tem o mesmo efeito permanente. Direções possíveis: tentar em CPU sem mutar a instância, ou reprovar CUDA na próxima chamada. Ao menos é observável — `/health` reporta o device efetivo.
  - *No app empacotado, o log de warning do fallback não tem destino* — o `audio-service.exe` empacotado é iniciado sem stdout/stderr conectado e com `CREATE_NO_WINDOW`, então `logging.warning` é descartado; e o erro HTTP só expõe a exceção *final*, perdendo a causa original na GPU. Precisa de stderr do processo empacotado indo para um arquivo de log, ou do erro original incluído na resposta.
  **Otimização possível (não testada):** cortar `cudnn_engines_precompiled` (562 MB) e `cudnn_adv` (230 MB) levaria o instalador a ~390 MB; a `adv` provavelmente não é exercitada pelo whisper no ctranslate2, e sem a `engines_precompiled` o cuDNN tentaria compilar kernels em runtime. Só um build + teste de `device: cuda` decide.
  **Alternativa sem custo de pacote:** `small` em CPU entrega 68% da velocidade da GPU e já é configurável em Configurações — mas transcreveu 19% menos caracteres que os dois runs de `medium`, o que pede conferência de qualidade antes de considerar equivalente.
- **Instalador 144 MB vs 125,7 MB das releases anteriores** — o `.spec` recriado inclui coisas que o original provavelmente excluía (`av` 67 MB, `onnxruntime` 37 MB, PIL 13 MB). Investigar se são realmente necessárias em runtime.
- **Reparent de temas é só com mouse** — o drag-and-drop usa apenas `PointerSensor`. Adicionar `KeyboardSensor` + um handle de arraste dedicado para tornar a reorganização acessível por teclado.
- **Chunk size warning no build do frontend** — bundle JS de ~518 kB. Considerar code-splitting com `React.lazy` para BoardView e modais pesados.
- **Migrations não são transacionais** — `runMigrations` (`internal/database/database.go`) aplica cada arquivo fora de transação. Uma falha no meio de uma migration não-idempotente deixa o banco parcialmente migrado e não registrado, e o `Open` passa a falhar para sempre — estado ruim de recuperar num app desktop que migra no launch.
- **Silero VAD no PyInstaller** — `vad_filter=True` foi removido por falhar no bundle. Para reativar, os dados do modelo Silero precisam entrar explicitamente no `.spec`.
- **Health do claude-code valida binário, não o token** — validação real só no botão Testar conexão; `/api/ai/health` responde `valid:true` com token expirado.
- **Foot-gun no test harness de `audio-service/tests/test_transcriber.py`** — o helper `_make_transcriber` sai do `patch("transcriber.WhisperModel", ...)` antes de retornar, e a fixture `transcriber` usa `device="cuda"` por padrão. Com o `except` mais amplo em `transcribe()`, um futuro teste baseado nessa fixture cujo mock lance dentro do `try` vai acabar chamando o `WhisperModel` **real** — um download do HuggingFace ou um load de vários GB em vez de uma falha rápida e legível. Nada dispara isso hoje. Direção: manter o patch ativo durante o corpo do teste, ou um patch de módulo com `autouse`.
- **`claude -p` headless herda configuração user-level da máquina** — skills, MCP servers e `CLAUDE.md` global do Claude Code instalado na máquina são herdados pelo processo headless usado nas gerações, o que pode adicionar latência de boot ou comportamento inesperado. O modo `--bare` não aceita token de subscription, então não é saída viável. Investigar flags de isolamento de config quando isso incomodar na prática.
- **Instalador NSIS não detecta o app aberto** — instalar por cima com o Meeting Notes rodando falha ao sobrescrever `Meeting Notes.exe` e aborta no meio: os arquivos do `audio-service/` e o `uninstall.exe` ficam novos, o exe principal e o registro (DisplayVersion) ficam velhos — instalação híbrida silenciosa (observado em 2026-08-28 ao instalar a v2.7.0 sobre a v2.6.0). O template NSIS do Wails precisa de um check de processo em execução (fechar ou abortar antes de extrair).

---

## Bugs conhecidos (sem plano)

- _(nenhum)_
