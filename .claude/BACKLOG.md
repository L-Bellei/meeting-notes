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
  **Pré-requisito:** corrigir antes o fallback de OOM (item em "Bugs conhecidos") — com 4 GB de VRAM disputada, falta de memória hoje falha a reunião em vez de cair para CPU.
  **Otimização possível (não testada):** cortar `cudnn_engines_precompiled` (562 MB) e `cudnn_adv` (230 MB) levaria o instalador a ~390 MB; a `adv` provavelmente não é exercitada pelo whisper no ctranslate2, e sem a `engines_precompiled` o cuDNN tentaria compilar kernels em runtime. Só um build + teste de `device: cuda` decide.
  **Alternativa sem custo de pacote:** `small` em CPU entrega 68% da velocidade da GPU e já é configurável em Configurações — mas transcreveu 19% menos caracteres que os dois runs de `medium`, o que pede conferência de qualidade antes de considerar equivalente.
- **Instalador 144 MB vs 125,7 MB das releases anteriores** — o `.spec` recriado inclui coisas que o original provavelmente excluía (`av` 67 MB, `onnxruntime` 37 MB, PIL 13 MB). Investigar se são realmente necessárias em runtime.
- **Reparent de temas é só com mouse** — o drag-and-drop usa apenas `PointerSensor`. Adicionar `KeyboardSensor` + um handle de arraste dedicado para tornar a reorganização acessível por teclado.
- **Chunk size warning no build do frontend** — bundle JS de ~518 kB. Considerar code-splitting com `React.lazy` para BoardView e modais pesados.
- **Migrations não são transacionais** — `runMigrations` (`internal/database/database.go`) aplica cada arquivo fora de transação. Uma falha no meio de uma migration não-idempotente deixa o banco parcialmente migrado e não registrado, e o `Open` passa a falhar para sempre — estado ruim de recuperar num app desktop que migra no launch.
- **Silero VAD no PyInstaller** — `vad_filter=True` foi removido por falhar no bundle. Para reativar, os dados do modelo Silero precisam entrar explicitamente no `.spec`.
- **Validação de chave OpenAI é só existência** — `ai.Ping`/`Configured` para `openai` apenas checa se a chave não é vazia. `/api/ai/health` retorna `valid:true` sem validar de fato (TODO em `internal/ai/validate.go`).

---

## Bugs conhecidos (sem plano)

- **Falta de VRAM falha a reunião em vez de cair para CPU** — o fallback em `transcriber.py:124` só trata erros de DLL (`dll`, `cublas`, `cudnn`, `library`, `not found`, `cannot be loaded`). Um erro de out-of-memory da GPU não casa com nenhum e cai no `raise`, marcando a reunião como FAILED. Hoje é latente (produção roda em CPU), mas vira real no dia em que o bundle levar CUDA — e já afeta quem roda em dev. Correção: incluir os padrões de OOM na lista.

- **Mensagem de erro em inglês ao falhar exclusão de tema** — se o `DELETE /api/themes/:id` falhar, `Sidebar.tsx:227` exibe a string crua do backend num UI em pt-BR. Caminho de erro raro; correção de uma linha. (Minor parqueado na review final da v2.6.0.)
- **Hamburger da Toolbar não é gated pela view ativa** — na view Board ele alterna o estado do painel de temas, que não é renderizado ali. O `Ctrl+B` já foi corrigido; o botão não.
- **`DndContext` sem `onDragCancel`** — um arraste cancelado deixa `activeId` obsoleto, mantendo um `droppable` falso até o próximo arraste.
- **Menu `⋯` do tema não reposiciona no scroll** — o popover é `fixed`, ancorado num rect capturado na abertura; rolar a lista deixa ele fora de lugar.
