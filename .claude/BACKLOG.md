# Backlog

Itens fora do escopo das features já implementadas. Para features com plano ativo do Superpowers, apenas referência — sem duplicação de conteúdo.

---

## Features com plano pronto (não iniciadas)

- _(nenhuma — todos os planos em `docs/superpowers/plans/` já foram implementados)_

---

## Features futuras (não brainstormadas)

- **UI/UX do CardDetailModal** — investigado em 2026-08-22 a pedido do usuário, ainda **sem decisão de escopo**. Dez itens, do que mais dói para o que menos: (1) três áreas de scroll aninhadas (corpo + descrição `max-h-56` + resumo `max-h-40`), que capturam a roda do mouse e deixam a descrição parada no meio de uma frase; (2) é o único modal do app que **não fecha com `Escape`** — `SearchModal`, `SettingsModal`, `RecordingModal`, `Sidebar` e `ThemeRowMenu` todos tratam; falta também `role="dialog"` e focus trap; (3) o confirm de dois cliques do excluir **nunca reseta** `confirmDelete`, então um clique acidental arma a exclusão para qualquer clique posterior; (4) clicar na descrição troca a leitura formatada por um `textarea` com o texto cru (JSON na cara, quando estruturado) e não há nenhuma pista de que é clicável; (5) checkboxes sem feedback — **parcialmente feito**: `salvando...` e mensagem de erro entraram junto com a correção do checkbox; falta o optimistic update; (6) `card.tasks.length > 0` esconde a seção inteira, sem estado vazio nem acesso ao "Gerar tasks" que já existe em `useGenerateTasks`; (7) `priority` e `assignee` vêm na resposta e o modal ignora; (8) hierarquia invertida no header — o título da reunião é `text-sm`, o menor elemento da tela, e o badge do tema herda a cor do tema (vermelho lê como erro); (9) `status` é texto morto, mover de coluna exige fechar o modal e arrastar; (10) `w-[640px]` sem `max-w`, sangra fora da viewport em janela estreita. Vários são decisão de comportamento, não de CSS — pede `/superpowers:brainstorming` antes de plano.

- **Notificações de pipeline** — notificação nativa do Windows quando o processamento de uma reunião termina. **Próxima feature acordada com o usuário**; começar por `/superpowers:brainstorming`. `git.sr.ht/~jackmordaunt/go-toast/v2` já está no `go.mod` como dependência indireta, sem uso direto.
- **Export** — exportar reunião (ou card do board) em PDF, Markdown ou Notion.

---

## Débitos técnicos

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
- **Validação de chave OpenAI é só existência** — `ai.Ping`/`Configured` para `openai` apenas checa se a chave não é vazia. `/api/ai/health` retorna `valid:true` sem validar de fato (TODO em `internal/ai/validate.go`).
- **Foot-gun no test harness de `audio-service/tests/test_transcriber.py`** — o helper `_make_transcriber` sai do `patch("transcriber.WhisperModel", ...)` antes de retornar, e a fixture `transcriber` usa `device="cuda"` por padrão. Com o `except` mais amplo em `transcribe()`, um futuro teste baseado nessa fixture cujo mock lance dentro do `try` vai acabar chamando o `WhisperModel` **real** — um download do HuggingFace ou um load de vários GB em vez de uma falha rápida e legível. Nada dispara isso hoje. Direção: manter o patch ativo durante o corpo do teste, ou um patch de módulo com `autouse`.

---

## Bugs conhecidos (sem plano)

- _(nenhum)_
