# GPU/CPU na transcrição — Design

**Data:** 2026-08-29
**Status:** aprovado em brainstorm (chat), aguardando plano
**Decisões do usuário:** instalador único com CUDA embarcado; default **Auto** (GPU se disponível);
fallback GPU→CPU **por chamada** (retenta GPU na transcrição seguinte).

## Problema

Produção transcreve em CPU (decisão de 2026-08-21 — DECISIONS.md): o instalador exclui as DLLs de
CUDA de propósito. Medições de 2026-08-21 (BACKLOG, gravação real de 146s, RTX 2050 4 GB): `medium`
GPU 35,5s (4,12× tempo real) vs CPU 112,6s (1,30×) — **ganho de 3,2×**. O usuário quer escolher o
device, com o app escaneando a máquina.

## Decisão de arquitetura

**Abordagem A — device por chamada, modelos gerenciados no Python.** O setting `whisper_device`
viaja por transcrição no `POST /transcribe` (como `language` já viaja); o `Transcriber` mantém um
cache de modelos por device e resolve o efetivo a cada chamada. Rejeitadas: device fixado no boot
do serviço com restart ao trocar (derruba gravação em andamento; e o fallback per-call seria
necessário do mesmo jeito) e dois modelos sempre quentes (~2 GB de RAM permanentes).

## Empacotamento

- O `.spec` do PyInstaller passa a coletar `nvidia.cudnn` e `nvidia.cublas` (o
  `_setup_dll_paths()` do `transcriber.py` já os pré-carrega — decisão 2026-05-01, sem mudança de
  código para o load). Instalador estimado: **~610 MB** (LZMA razão 0,27 já medida).
- **Experimento de corte — RESULTADO (2026-08-29): PASSOU.** Bundle podado
  (`cudnn_engines_precompiled64_9.dll` 562 MB + `cudnn_adv64_9.dll` 230 MB removidos) transcreveu
  240s de fala real em `device: cuda` em 23s, mesmos 2876 caracteres da baseline com bundle
  completo (24s), zero warnings de fallback no stderr. Bundle: 1,85 GB → **1,07 GB**. A poda está
  codificada no `.spec` (filtro de `binaries`). Baseline e validação rodadas na RTX 2050 4 GB com
  áudio TTS gerado localmente (nenhuma gravação real disponível na máquina; fala sintética
  exercita os mesmos kernels).
- O smoke test do `build.ps1` não muda (o boot do serviço não depende de CUDA).
- A decisão CPU-only de 2026-08-21 é formalmente revertida por entrada nova no DECISIONS.md
  referenciando este spec.
- **Custo aceito:** máquinas sem GPU NVIDIA carregam ~465 MB de DLLs sem uso — preço do instalador
  autossuficiente.

## Scan e API

- **Scan no audio-service** (quem tem `ctranslate2`): disponibilidade via
  `get_cuda_device_count() > 0`; nome/VRAM via
  `nvidia-smi --query-gpu=name,memory.total --format=csv,noheader` (subprocess best-effort — sem
  `nvidia-smi`, só a disponibilidade). Calculado uma vez no lifespan.
- **`/health` ganha:** `gpu_available: bool`, `gpu_name: string|null`, `gpu_vram_mb: int|null`;
  o campo `device` existente passa a significar "device efetivo da última transcrição".
- **Setting `whisper_device`** (`auto` | `cuda` | `cpu`), default `auto`: migration 019 + whitelist
  (`validateEnum("auto", "cuda", "cpu")`) no `SettingsService`.
- **Go:** o setting é lido por transcrição e enviado no payload do `POST /transcribe` (campo
  `device`). `internal/audio/client.go`: campo novo no request, campos novos no `HealthResponse`,
  e `Transcribe(ctx, path, language)` vira `Transcribe(ctx, path, language, device string)` — call
  sites: orchestrator e retranscribe. `auto` é resolvido no Python.

## Transcriber: device por chamada e fallback sem mutação

- Sai o modelo único do `__init__`; entra `self._models: dict[str, WhisperModel]` (criação lazy por
  device). `transcribe(path, language, device="auto")` resolve o efetivo: `cuda` se pedido
  `auto`/`cuda` **e** disponível; senão `cpu`.
- **Boot preservado:** o lifespan carrega eagerly o modelo do device auto-resolvido — `/health` só
  responde após o load (orçamento de 120s do smoke segue válido) e `model_loaded` mantém a
  semântica.
- **Fallback por chamada:** falha com efetivo `cuda` → `logging.warning` com a causa original →
  pega/cria o modelo CPU e refaz, **sem mutar cache nem estado** — a chamada seguinte resolve
  `auto` de novo e retenta CUDA. O modelo CPU do fallback fica no cache (custo aceito: ~1,5 GB de
  RAM após o primeiro fallback). A resposta do `/transcribe` ganha `device` (efetivo usado).
- Se o usuário fixa `cpu` tendo GPU, o modelo CUDA carregado no boot permanece na VRAM até o
  restart do serviço — desperdício menor, aceito e registrado.

## Log com destino (fecha débito do BACKLOG)

O bundle é console app desde a v2.7.1. O Go passa a redirecionar stdout/stderr do processo filho
para `audio-service.log` ao lado do banco, com rotação simples (trunca acima de ~5 MB no boot) —
nos **dois** entry points. O warning do fallback e a causa original da falha de GPU ficam
diagnosticáveis no app empacotado.

## Timeout do Go

`internal/audio/client.go:67`: 60 min → **4 horas** no `/transcribe`. Cobre a pior combinação
medida: tentativa GPU queimada a meio da transcrição + reprocesso inteiro em CPU (1,30× tempo real)
em reuniões longas. Estático de propósito (o client não conhece a duração da gravação — dinâmica é
YAGNI).

## UI (Configurações, seção de transcrição)

- Linha de **scan**: "GPU detectada: <nome>, <VRAM>" ou "Nenhuma GPU NVIDIA — transcrição em CPU".
- **Seletor de device**: Auto (recomendado) / GPU / CPU → `whisper_device`.
- **Device efetivo** da última transcrição (do `/health`).
- Canal até o frontend: o `GET /health` do Go (`cmd/desktop/app.go:143`, e o equivalente no
  `cmd/api`) já espelha `model_loaded` do audio-service — passa a repassar também `gpu_available`,
  `gpu_name`, `gpu_vram_mb` e `device`; o frontend consome por um hook React Query novo
  (`["audio-health"]`) usado só pela seção de transcrição das Configurações.

## Testes

- **Pré-requisito (débito do BACKLOG vira bloqueante):** consertar o foot-gun do harness de
  `audio-service/tests/test_transcriber.py` — o patch de `WhisperModel` deve ficar ativo durante o
  corpo do teste (hoje `_make_transcriber` sai do patch antes de retornar; um mock que lança dentro
  do `try` chamaria o `WhisperModel` real). Primeira task do plano.
- Python: resolução auto/cuda/cpu, cache por device, fallback sem mutação + retentativa de CUDA na
  chamada seguinte, campos novos do `/health` — tudo com `WhisperModel` mockado.
- Go: campo `device` no request do client, campos novos no `HealthResponse`, timeout, whitelist do
  setting.
- Frontend: `tsc --noEmit` + `npm run build` + exercício manual na janela nativa.
- **Validação do usuário:** uma gravação real em GPU (device efetivo + tempo conferidos) e outra
  com CPU forçado; e a transcrição real em CUDA do experimento de corte.

## Fora de escopo (registrado)

- Empacotar modelos Whisper no instalador (download no primeiro uso continua como é).
- Telemetria/histórico de tempos por device.
- Download sob demanda das DLLs e instalador dual (alternativas rejeitadas na decisão de entrega).
