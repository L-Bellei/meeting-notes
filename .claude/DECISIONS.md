# Decisões Arquiteturais

Registro de decisões transversais ao projeto. Decisões específicas de cada feature estão nos planos do Superpowers correspondentes.

---

## [2026-08-30] GPU não-NVIDIA via whisper.cpp/Vulkan como segundo motor; CUDA permanece o caminho NVIDIA

**Contexto:** A v2.9.0 só transcreve em GPU NVIDIA — faster-whisper/ctranslate2 não tem backend AMD no
Windows (ROCm é Linux-only, sem wheel). Pedido do usuário: suporte a placas AMD.

**Alternativas:** (a) um motor só, whisper.cpp/Vulkan para todas as GPUs — instalador cairia de 631 para
~150 MB, mas descarta a homologação CUDA e perde 1,5–3× em NVIDIA; (b) ONNX Runtime + DirectML — sem
pipeline Whisper pronto em Python, semanas de plumbing; (c) ROCm/torch-directml — imaturo no Windows.

**Escolha:** Dois motores lado a lado. `backends/ct2.py` (faster-whisper) segue para CUDA e CPU;
`backends/whispercpp.py` roda o binário `whisper-cli` (Vulkan) via subprocess para AMD/Intel e como
fallback quando CUDA falha. Resolução por chamada em cadeia (`cuda → vulkan → cpu`), sem estado
pegajoso. Backend é escolha interna: o setting `whisper_device` passa a `auto|gpu|cpu` (migration 020
converte `cuda→gpu`); o usuário só vê Auto/GPU/CPU. Modelo GGML quantizado (q5) baixado sob demanda do
HF na primeira transcrição Vulkan (~540 MB no medium), não embarcado. Binário whisper.cpp pinado em
`audio-service/build/whispercpp.version` (b4938), **compilado** por `fetch-whispercpp.ps1` (a release
oficial não publica build Vulkan p/ Windows — exige CMake + VS Build Tools + Vulkan SDK na máquina de
build, instalados em 2026-08-30), embarcado em `_internal/whispercpp/` (+~55 MB no bundle).

**Trade-offs aceitos:** dois formatos de modelo (quem usa Vulkan baixa um segundo modelo); subprocess
em vez de wheel (isolamento de crash de driver vale um processo por transcrição); toolchain C++ vira
pré-requisito da máquina de build; homologação em hardware AMD **em aberto** — validado nesta máquina
forçando Vulkan na RTX 2050 (`WHISPER_FORCE_BACKEND=vulkan`: 169s de áudio em 34,8s no CLI; smoke do
bundle empacotado transcreveu via Vulkan), decisão consciente do usuário sem máquina AMD disponível.

---

## [2026-08-29] Instalador embarca CUDA; device de transcrição é escolha do usuário (reverte 2026-08-21)

**Contexto:** A decisão de 2026-08-21 mantinha o instalador CPU-only para preservar o tamanho das
releases anteriores. As medições daquele item (3,2× de ganho, gravação real de 146s numa RTX 2050
4 GB: `medium` GPU 35,5s vs CPU 112,6s) e o pedido explícito do usuário — que quer escolher o
device, com o app escaneando a máquina — motivaram reabrir a decisão. O experimento de corte do
bundle (Task 8 do plano) validou com transcrição real em CUDA: 240s de fala, 23–24s, mesmos
caracteres, sem fallback.

**Escolha:** Instalador único, sem download sob demanda nem instalador dual. `whisper_device`
(`auto` | `cuda` | `cpu`, default `auto`) viaja por chamada no `POST /transcribe` — não é fixado no
boot do serviço. Fallback GPU→CPU é **por chamada, sem estado pegajoso**: uma falha na GPU cai para
CPU naquela transcrição, mas a chamada seguinte retenta CUDA (ao contrário do singleton antigo, que
travava em CPU até reiniciar o processo). O log do processo filho (stdout/stderr do
`audio-service.exe`, antes descartado no app empacotado) passa a ter destino:
`%LOCALAPPDATA%\meeting-notes\audio-service.log`, com rotação simples. O timeout do `/transcribe`
em `internal/audio/client.go` sobe de 60 min para **4 horas**, para cobrir a pior combinação
medida — tentativa GPU queimada a meio da transcrição seguida de reprocesso inteiro em CPU numa
reunião longa.

**Justificativa e trade-offs explícitos:**
- **Tamanho do instalador:** o bundle de CUDA saiu de 1,85 GB para **1,07 GB** depois da poda
  validada na Task 8 (`cudnn_engines_precompiled64_9.dll` e `cudnn_adv64_9.dll` removidos, sem
  regressão de qualidade ou performance na transcrição real).
- **Custo aceito:** máquinas sem GPU NVIDIA carregam DLLs sem uso — estimativa de ~465 MB inúteis
  no instalador, preço do instalador único e autossuficiente (sem download sob demanda).
- **Timeout de 4h é estático** — o client não conhece a duração da gravação; dinâmico seria YAGNI
  neste app single-user.
- Fecha os dois débitos abertos do item "Instalador transcreve em CPU — empacotar GPU" no
  BACKLOG: o downgrade permanente do device (resolvido pelo fallback sem estado pegajoso) e o log
  do fallback sem destino (resolvido pelo `audio-service.log`).

---

## [2026-08-29] Assets-fonte nunca vivem em paths gitignorados

**Contexto:** Terceira perda pelo mesmo padrão: o spec do PyInstaller (perdido, recriado em
2026-08-21), e agora `cmd/desktop/build/appicon.png` + `build/windows/icon.ico` — gitignorados como
"artefatos de build", perdidos na limpeza de ~2026-08-20, e regenerados pelo `wails build` com o
logo default do Wails, que embarcou silenciosamente no exe/instalador das v2.6.0–v2.8.0. O tray
sobreviveu porque `cmd/desktop/assets/tray.ico` era rastreado.

**Escolha:** Todo arquivo que o build **consome** (spec, ícones, templates) é asset-fonte e deve
ser rastreado no git — mesmo quando mora dentro de um diretório de build gitignorado (usar negação
ou remover a entrada, com comentário no `.gitignore` explicando o porquê). Só o que o build
**produz** é artefato.

**Justificativa:** O sintoma da perda é silencioso: builds continuam verdes e o placeholder embarca
em release. O critério "consumido vs produzido" é verificável na revisão de qualquer mudança de
`.gitignore`.

---

## [2026-08-28] Release só sai com smoke test do artefato empacotado

**Contexto:** As v2.6.0 e v2.7.0 foram publicadas com o audio-service **morto no boot** — a
verificação de release era o tamanho do instalador, que não distingue um bundle que sobe de um que
crasha. O defeito só apareceu quando o usuário instalou numa segunda máquina.

**Escolha:** O `build.ps1` sobe o `audio-service.exe` empacotado e exige HTTP 200 em `/health`
(orçamento de 120s reais — o modelo Whisper carrega no lifespan, antes do servidor aceitar
conexões) **antes** do NSIS; sem isso o build falha. Checagens por tamanho continuam como sanidade,
não como gate.

**Justificativa:** O smoke exercita exatamente o modo de falha que escapou duas vezes (ambiente de
build ≠ ambiente pinado). Custa ~40s por build. Corolário: qualquer futuro componente empacotado
ganha o mesmo tratamento — o gate é "o artefato sobe", não "o artefato existe".

---

## [2026-08-29] IA via subscription (Claude Code CLI), provider único — API keys removidas

**Contexto:** O app gerava resumo, pontos-chave e tasks pela Anthropic Messages API com API key, cobrando créditos de API. O usuário tem assinatura Claude (Pro/Max) e quer que esse consumo saia da assinatura em vez de API — mas a Messages API não aceita credencial de assinatura (é explicitamente não-programática). O único caminho oficial para gerar a partir da assinatura é o **Claude Code em modo headless** (`claude -p`), autenticado por token OAuth de longa duração emitido por `claude setup-token`.

**Escolha:** `ClaudeCodeClient` (`internal/ai/claude_code_client.go`) spawna `claude -p <prompt> --output-format json [--model <m>]` por chamada (stateless — sem processo quente, sem sidecar Agent SDK), com `CLAUDE_CODE_OAUTH_TOKEN` no ambiente. Este passa a ser o **único** provider de IA: `anthropic_client.go` e `openai_client.go` (e os SDKs correspondentes) saem do `go.mod`; `DynamicAIClient.resolve()` só resolve `claude-code`. Login é iniciado pelo próprio app: o botão "Conectar com Claude" spawna `claude setup-token`, que abre um **console visível** (não headless/oculto) — um spike (Task 1 do plano) provou que o comando exige TTY e recusa rodar com stdout apenas capturado em background. O usuário autoriza no browser, o token aparece no console, e a colagem no campo das Configurações é manual (a captura automática de stdout foi descartada pelo mesmo motivo). `claude_code_model` é campo de **texto livre** no seletor (aliases padrão/haiku/sonnet/opus + campo "Outro…"): não há listagem dinâmica de modelos com credencial de subscription — `claude models list` não existe e `GET /v1/models` recusa token de subscription por ToS. A migration `018_claude_code_provider.sql` marca `ai_provider = 'claude-code'` e apaga as chaves de API antigas; a linha `ai_provider` **permanece no banco** só como registro de estado — fora da whitelist de escrita do `SettingsService`, e o frontend filtra o `PUT` via `pickWritable` para nunca reenviá-la.

**Justificativa e trade-offs explícitos:**
- **Dependência do binário `claude` instalado** na máquina do usuário — sem ele, `ai.Configured` e o health tratam como não configurado.
- **Rate limits da assinatura** (não de billing por token): estourar a cota do plano Pro/Max passa a limitar o app; sem retry automático, para não queimar quota sozinho em cima de um erro transitório.
- **Migration 018 é irreversível** (mesmo padrão da migration 016 de 2026-08-20): downgrade para uma v2.7.x anterior não funciona mais, e as chaves de API apagadas não são recuperáveis — quem migrar precisa recolar credenciais se voltar a usar API key no futuro.
- **Restrição de distribuição:** a Anthropic não permite que produtos de terceiros ofereçam login claude.ai a seus usuários sem aprovação prévia. Este fluxo pressupõe app pessoal, rodando na máquina do próprio assinante — distribuir o app com esse fluxo de login para terceiros exigiria aprovação da Anthropic antes.

---

## [2026-08-22] Descrição de card é anotação do usuário, não cópia do resumo

**Contexto:** `BoardCardService.Create` copiava `summary.Content` para a descrição do card. A
cópia nunca ressincronizava, e o modal renderizava a cópia (seção DESCRIÇÃO) e a fonte viva
(seção RESUMO) uma embaixo da outra — byte-a-byte idênticas no card medido (1867 caracteres nos
dois campos).

**Escolha:** A descrição passa a ser anotação do usuário, vazia por padrão; o Resumo continua
sendo a fonte da IA. A migration `017_card_description_annotations.sql` limpa **apenas** as
descrições ainda idênticas ao resumo, preservando o que foi editado por cima da cópia.

**Justificativa:** Dois campos, dois propósitos — sem essa separação, "editar a descrição" de um
card de reunião era editar uma cópia congelada do resumo, sem nenhuma relação com o resumo vivo
mostrado ao lado. Limpar só as descrições ainda idênticas evita apagar anotações que o usuário já
tenha escrito por cima da cópia antiga.

---

## [2026-08-21] Instalador empacota o audio-service sem CUDA — produção transcreve em CPU

**Contexto:** O `.spec` do PyInstaller vivia em `audio-service/build/pyinstaller/`, que é **gitignored** — nunca foi commitado e foi perdido junto com o diretório `build/`. Ao recriá-lo, um bundle com as DLLs da NVIDIA saiu com **1,9 GB** (`nvidia/cudnn` 993 MB + `nvidia/cublas` 548 MB; todo o resto ~300 MB). Isso expôs um fato que nenhum registro anterior mencionava: os instaladores publicados da v2.4.1, v2.4.2 e v2.5.0 têm **125,7 MB os três**, tamanho compatível apenas com o bundle **sem** as DLLs de CUDA.

**Consequência (não era uma escolha consciente até agora):** no app instalado, `_setup_dll_paths` falha no `importlib.import_module("nvidia.cudnn")`, `ctranslate2.get_cuda_device_count()` retorna 0 e o `transcriber.py` cai para `device="cpu"`, `compute_type="int8"`. Em `wails dev` os pacotes `nvidia.*` estão no site-packages global, então **desenvolvimento roda em CUDA e produção roda em CPU** — a diferença nunca tinha sido notada porque o fallback é silencioso por design (ver decisão de 2026-05-01).

**Escolha:** Manter CPU-only no instalador da 2.6.0, com o `.spec` excluindo os pacotes `nvidia.*` deliberadamente. Motivo: preserva o tamanho e o comportamento das releases anteriores; empacotar GPU levaria o instalador de ~125 MB para a casa de 700 MB–1 GB.

**Justificativa e trade-off explícito:** transcrição com o modelo `medium` em CPU é bem mais lenta que em GPU. A alternativa (empacotar CUDA, possivelmente enxugando o `cudnn_engines_precompiled`) fica no backlog como opção consciente, não como bug. O `.spec` agora é **rastreado no git** (negação no `.gitignore`) para que a receita do bundle não se perca de novo.

---

## [2026-08-20] Volta ao prompt único por tema (revert da estrutura por tipo)

**Contexto:** A v2.5.0 dividiu o prompt do tema em geral + 3 overrides por tipo (resumo / pontos-chave / tarefas), revisitando a decisão de 2026-04-29 que era um YAGNI deliberado. Consulta ao banco de produção mostrou os 3 campos **vazios em todos os temas** — nenhum override foi usado na prática. O custo era um modal com 4 textareas onde 1 basta, mais `PromptFor`/`ThemePrompts` sustentando complexidade que ninguém exercitava.

**Escolha:** Voltar ao `custom_prompt` único. Migration `016` derruba as 3 colunas; `PromptKind`, `Theme.PromptFor` e `models.ThemePrompts` deixam de existir; `ThemeService.Create/Update` recebem `customPrompt string` (um parâmetro, não as 4 strings posicionais que o `ThemePrompts` existia para evitar). O degrau `"" → prompt padrão` continua em `buildInstruction`, então `internal/ai` não muda.

**Justificativa:** A decisão de 2026-04-29 estava certa para este app — usuário único, uso pessoal. Reverter enquanto o custo é zero (campos vazios) é mais barato que manter a estrutura viva à espera de um uso que não veio. **Consequência a registrar:** `DROP COLUMN` é irreversível e migrations rodam ao abrir o banco, então um banco migrado não volta a funcionar numa v2.5.0 — downgrade deixa de ser possível.

---

## [2026-08-20] Estado de UI efêmero em localStorage; preferências de verdade no banco

**Contexto:** O overhaul da aba de temas trouxe dois estados novos de UI: se o painel está fixado, e quais temas estão expandidos. O projeto até então não usava `localStorage` em lugar nenhum — tudo passava pela tabela `settings`.

**Escolha:** Dividir por natureza do dado. **Preferência** (`sidebar_pinned`) vai para `settings`, na whitelist do `SettingsService` — é uma escolha do usuário, controlável também pelo modal de Configurações no futuro. **Estado efêmero de forma variável** (conjunto de temas expandidos) vai para `localStorage["theme_expanded"]`, com poda na leitura contra os temas existentes, para que IDs de temas excluídos não acumulem.

**Justificativa:** Cada clique num chevron viraria um `PUT /api/settings` se a expansão morasse no banco, e um JSON de IDs órfãos acumularia lixo. Já o pin é uma preferência legítima e merece o mesmo tratamento das outras. Corolário: quem lê o pin de dentro de um componente usa uma mutation que invalida só `["settings"]` — `useUpdateSettings` também invalida `["ai-health"]`, o que dispararia um ping real no provider de IA a cada toque no pino.

---

## [2026-08-20] Hierarquia de temas com teto de 2 níveis, validada no service

**Contexto:** `themes.parent_id` permite profundidade arbitrária, e `ThemeService.Update` atribuía o pai **sem validação nenhuma** — auto-referência e ciclos eram alcançáveis pela API. A UI nunca expôs reparent, então nunca importou; o drag-and-drop tornou alcançável.

**Escolha:** Teto de **2 níveis** (tema raiz → subcategoria), validado em `ThemeService` no `Create` e no `Update`: nada é pai de si mesmo; o pai escolhido não pode ter pai; um tema que tem filhos não pode virar filho. Violação → `ValidationError` → HTTP 422. O frontend bloqueia o drop inválido, mas o backend é a autoridade.

**Justificativa:** Ciclos ficam impossíveis por construção, sem precisar subir a cadeia de pais. E o teto torna **correto por construção** o `countForTheme` do frontend, que soma diretos + filhos diretos e ignora netos. Cuidado que o teto não cobre: a contagem exibida na confirmação de exclusão precisa ser a de reuniões **diretas**, porque excluir um tema só desassocia as reuniões dele — as subcategorias sobem para a raiz levando as suas.

---

## [2026-07-18] Toda janela Win32 em Go: criação + registro de hotkey + message loop na mesma OS thread travada

**Contexto:** Generaliza a decisão de 2026-05-01 (overlay). O tray (`cmd/desktop/tray.go`) criava a janela e registrava o hotkey na goroutine do `Start()`, mas rodava o `GetMessage` loop em **outra** goroutine. Por thread affinity do Win32, mensagens de janela (`WM_LBUTTONUP`/`WM_RBUTTONUP`) e `WM_HOTKEY` são entregues apenas à fila da thread que criou a janela / registrou o hotkey — então cliques no ícone e o hotkey global eram perdidos de forma intermitente.

**Escolha:** Qualquer janela Win32 criada em Go deve fazer **criação da janela, registro de recursos thread-affine (hotkey) e o message loop na mesma goroutine fixada** via `runtime.LockOSThread()`. `Start()` dispara `go run()` e sincroniza a prontidão/erro por um canal `ready chan error`. Corolários:
- **Teardown thread-affine** (`UnregisterHotKey`/`Shell_NotifyIcon(NIM_DELETE)`/`DestroyWindow`) roda na thread do loop: `Stop()` posta `WM_CLOSE` e a window proc executa a limpeza.
- **Dismiss de menu de contexto:** postar `WM_NULL` após `TrackPopupMenu` (KB135788) para o menu fechar corretamente ao clicar fora.

**Justificativa:** Correto por especificação Win32, sem overhead. Elimina toda uma classe de bugs "às vezes o clique/hotkey não funciona". Overlay e tray agora seguem o mesmo padrão.

---

## [2026-06-05] IA não-configurada degrada graciosamente em vez de falhar o pipeline

**Contexto:** Com `auto_generate=true` e sem IA configurada, o pipeline marcava a reunião inteira como `FAILED`, apesar de a transcrição ter sido concluída com sucesso.

**Escolha:** O orchestrator (`maybeGenerate`) faz um pré-check barato e sem rede (`ai.Configured(settings)`) antes de gerar. Sem provider/chave, pula a geração e completa a reunião com a transcrição preservada (log `warn`). O caminho de IA explícito (`RunAIPipeline`/Reprocessar) continua falhando, pois sua única função é gerar.

**Justificativa:** A transcrição é o ativo primário; perdê-la por falta de configuração de IA é regressão. Geração é complementar.

---

## [2026-06-05] Sentinels de erro de IA para mapeamento de status HTTP

**Contexto:** `DynamicAIClient.resolve()` retornava `fmt.Errorf` genérico, então o ramo 503 "AI não configurada" nos handlers era código morto (o `errors.Is` nunca casava) e tudo caía no 502 genérico.

**Escolha:** `ai.ErrNotConfigured` (wrapped em `resolve()`) + `ai.IsAuthError()` (detecta 401/403 do SDK, com fallback por substring). Os services normalizam via `mapAIError` para `services.ErrAINotConfigured` (→ 503) e `services.ErrAIAuthFailed` (→ 502 com mensagem clara). `ai.Configured(map)` é a checagem pura reutilizada por `Ping`, services e orchestrator.

**Justificativa:** Mensagens de erro distintas e acionáveis ("configure a IA" vs "chave inválida") sem acoplar handlers ao pacote `ai`.

---

## [2026-06-05] Monitor de saúde do audio-service é desktop-only (eventos Wails)

**Contexto:** Antes, o estado do audio-service só era checado no startup (loading screen); quedas mid-session não eram sinalizadas.

**Escolha:** `monitorAudioHealth` (goroutine com ticker de 10s em `cmd/desktop/app.go`) emite eventos Wails `audio:down`/`audio:ready` em cada transição; o frontend (`useAudioStatus`) reage desabilitando "Gravar" e exibindo uma barra de aviso. Por usar o canal de eventos Wails, é inerentemente desktop-only — `cmd/api` não recebe; apenas o endpoint `/health` permanece em sincronia entre os dois entry points.

**Justificativa:** `cmd/api` não tem canal de eventos para o frontend; replicar o monitor lá não agregaria valor.

---

## [2026-04-29] Posicionamento de cards com float + rebalanceamento automático

**Contexto:** Ordem manual de cards dentro de colunas do kanban precisa ser persistida no SQLite sem renumeração constante.

**Alternativas:**
- Integer sequencial com renumeração ao mover (simples, mas O(n) writes)
- Float com inserção no meio sem renumeração (O(1) na maioria dos casos)

**Escolha:** Float. Threshold de rebalanceamento: gap < 1e-9 dentro de uma coluna dispara renumeração completa da coluna (1000, 2000, 3000...).

**Justificativa:** Operação de drag-and-drop é frequente; renumeração raramente ocorre na prática.

---

## [2026-04-29] customPrompt de tema substitui completamente o prompt padrão

**Contexto:** `Theme.CustomPrompt` é enviado para summary, key_points e tasks via o mesmo campo. Não há prompts separados por tipo de geração.

**Alternativas:**
- Campo único substituindo tudo (implementado)
- Campos separados por tipo (`custom_summary_prompt`, `custom_key_points_prompt`, `custom_tasks_prompt`)

**Escolha:** Campo único por ora (YAGNI).

**Justificativa:** Usuário único, uso pessoal. Se necessário, separar futuramente — schema suporta adição de colunas sem breaking change.

---

## [2026-04-29] Board é global, colunas são globais, numeração de cards é sequencial e imutável

**Contexto:** Design do kanban board.

**Escolha:** Um board global, colunas globais (não por tema), contador global de cards sequencial nunca reutilizado (estilo issue tracker).

**Justificativa:** App single-user, complexidade extra de boards por tema não agrega valor no uso atual.

---

## [2026-04-29] Seed de 3 colunas padrão na migration 007

**Contexto:** Board precisa de colunas para funcionar. Usuário pode não configurar nada logo após instalar.

**Escolha:** Migration 007 insere Backlog, Em Andamento, Concluído com IDs fixos (`col-backlog`, `col-wip`, `col-done`).

**Justificativa:** IDs fixos permitem idempotência se a migration rodar em banco existente.

---

## [2026-05-01] Win32 overlay: criação de janela e message loop no mesmo OS thread

**Contexto:** Ao criar uma janela Win32 em Go via goroutine, os eventos WM_* nunca chegam ao `GetMessage` se o loop roda em thread diferente da criação (Win32 thread affinity).

**Alternativas:**
- Criar janela na goroutine principal (inviável em Wails — bloquearia o runtime)
- Usar `PostThreadMessage` para despachar para outra thread (complexo)
- Criar janela e rodar o message loop na mesma goroutine fixada ao OS thread

**Escolha:** Goroutine dedicada com `runtime.LockOSThread()` / `defer runtime.UnlockOSThread()`. Canal `ready chan struct{}` sinaliza o HWND de volta ao chamador após `CreateWindowEx`.

**Justificativa:** Pattern simples, correto por especificação Win32, sem overhead extra. Qualquer janela Win32 criada em Go deve seguir este padrão.

---

## [2026-05-01] CUDA no audio-service: pré-load de DLLs + detecção via ctranslate2

**Contexto:** ctranslate2 usa `LoadLibrary` internamente e ignora `os.add_dll_directory`. Em Windows, `cublas64_12.dll` e DLLs do cudnn não são encontradas sem pré-carregamento explícito.

**Alternativas:**
- Adicionar DLLs ao PATH do sistema (requer configuração manual por máquina)
- Detectar CUDA via `torch.cuda.is_available()` (torch não está no venv do audio-service)
- Pré-carregar via `ctypes.CDLL` + detectar via `ctranslate2.get_cuda_device_count()`

**Escolha:** `_setup_dll_paths()` carrega todos os `.dll` de `nvidia.cudnn` e `nvidia.cublas` via `ctypes.CDLL` antes de instanciar `WhisperModel`. Detecção de GPU: `ctranslate2.get_cuda_device_count() > 0`.

**Justificativa:** Sem dependência de torch. Funciona em qualquer Windows com ou sem GPU NVIDIA. Em máquinas sem CUDA, os pacotes nvidia.* não estão instalados e o bloco é ignorado silenciosamente.

---

## [2026-04-29] Processo de build do installer

**Contexto:** `wails build` não encontra `makensis` no PATH por padrão.

**Escolha:** Comando de build completo para Windows:
```bash
cd cmd/desktop
PATH="$PATH:/c/Program Files (x86)/NSIS" wails build -nsis
cp "build/bin/Meeting Notes-amd64-installer.exe" "../../dist/meeting-notes-X.Y.Z-windows-amd64-installer.exe"
```

**Justificativa:** NSIS está instalado em `C:\Program Files (x86)\NSIS` mas não está no PATH padrão do bash.

---

## [2026-05-06] WAV permanece no dir do audio-service (Approach A)

**Contexto:** Pipeline de transcrição precisava de resiliência — nunca perder o áudio gravado e permitir retry enquanto o WAV existir.

**Alternativas:**
- Approach A: WAV fica em `recordings/` do audio-service; Go guarda o path absoluto no banco e serve via HTTP diretamente.
- Approach B: Copiar o WAV para um diretório controlado pelo Go backend após StopRecording.

**Escolha:** Approach A — sem cópia ou movimentação de arquivos.

**Justificativa:** Retry chama `/transcribe` no audio-service passando o path já existente. Sem overhead de cópia, sem gerência de segundo diretório. Path é persistido imediatamente após `StopRecording`, antes de qualquer falha possível.

**Política de delete:**
- Falha na transcrição: WAV nunca deletado (independente de `keep_audio`), para que retry seja sempre possível.
- Sucesso na transcrição: deletar somente se `keep_audio = false`. Se `keep_audio = true`, manter indefinidamente.

---

## [2026-05-07] Widgets flutuantes usam createPortal(content, document.body)

**Contexto:** AudioPlayer renderizado dentro da árvore React ficava limitado pelo stacking context do componente pai, mesmo com `z-[9999]`.

**Alternativas:**
- Aumentar z-index indefinidamente (não resolve stacking context de pai com `transform`, `filter`, `will-change`)
- Mover o componente para mais alto na árvore (acopla desnecessariamente)
- `createPortal(content, document.body)` — renderiza fora de qualquer stacking context

**Escolha:** `createPortal` para qualquer widget flutuante (modais, players, tooltips que precisam sobrepor tudo).

**Justificativa:** Solução canônica do React. Mantém o estado e event handlers dentro da árvore React mas insere o DOM diretamente no `body`.

---

## [2026-05-07] AudioPlayer usa plain `<audio>` sem AudioContext

**Contexto:** `AudioContext.createMediaElementSource()` captura o elemento `<audio>` permanentemente. Ao fechar o AudioContext no cleanup do useEffect, o áudio fica mudo (roteado para contexto fechado). React StrictMode agrava com double-mount.

**Alternativas:**
- Gerenciar AudioContext sem fechar no cleanup (leak de recursos)
- Nunca reconectar após o primeiro mount (frágil)
- Usar plain `<audio>` sem AudioContext

**Escolha:** Remover AudioContext do AudioPlayer inteiramente. O visualizador de espectro usa animação canvas aleatória (fake) em vez de Web Audio API real.

**Justificativa:** Playback correto tem prioridade sobre visualizador preciso. Animação fake é indistinguível para o usuário. Elimina toda a complexidade de captura/cleanup.

---

## [2026-05-07] Tailwind config com cor card explícita (não CSS variable)

**Contexto:** Este projeto usa Tailwind com paleta de cores custom hardcoded (não o sistema de CSS variables do shadcn/ui). Classes como `bg-card` resolvem para `transparent` se `card` não estiver na paleta.

**Escolha:** Adicionar todas as cores necessárias diretamente em `tailwind.config.js` como valores hex. Não migrar para CSS variables.

**Justificativa:** O projeto já usa este padrão desde o início. Migrar para CSS variables seria refactor sem benefício para app single-user sem theming dinâmico.

---

## [2026-05-06] vad_filter removido do transcriber.py (não compatível com PyInstaller)

**Contexto:** `vad_filter=True` foi adicionado para suprimir loops de alucinação do Whisper, mas causa falha completa de transcrição no bundle PyInstaller — os arquivos do modelo Silero VAD não estão incluídos no `.spec`.

**Escolha:** Remover `vad_filter`. Manter os demais parâmetros anti-alucinação: `condition_on_previous_text=False`, `compression_ratio_threshold=1.8`, `repetition_penalty=1.1`.

**Justificativa:** Silero VAD requer dados do modelo que precisariam ser explicitamente adicionados ao `.spec` (testado e não incluído). Os outros três parâmetros resolvem o loop de alucinação sem dependência externa.
