# GPU AMD (e outras não-NVIDIA) na transcrição via whisper.cpp/Vulkan — Design

**Data:** 2026-08-30
**Base:** v2.9.0 (`docs/superpowers/specs/2026-08-29-gpu-cpu-transcription-design.md`)
**Branch:** `feat/vulkan-transcription`

## Problema

A v2.9.0 entregou transcrição em GPU, mas só para NVIDIA: faster-whisper roda sobre ctranslate2,
que no Windows conhece apenas CUDA (ROCm existe só em Linux, sem wheel oficial). Quem tem placa
AMD — ou Intel — cai em CPU sem opção. Não há flag a acrescentar no motor atual: suporte a AMD
exige um segundo motor de inferência.

## Decisão de arquitetura

**Dois motores lado a lado.** faster-whisper/CUDA continua sendo o caminho NVIDIA, intocado.
whisper.cpp com backend **Vulkan** entra como motor para qualquer GPU não-NVIDIA (AMD, Intel) e
como fallback de GPU quando CUDA falha. Alternativas rejeitadas:

- **Um motor só (whisper.cpp/Vulkan para tudo, remover CUDA):** reduziria o instalador de 631 MB
  para ~150 MB, mas descarta a feature homologada na v2.9.0 e Vulkan em NVIDIA é tipicamente
  1,5–3× mais lento que CUDA/cuDNN — perda real para o usuário atual.
- **ONNX Runtime + DirectML:** API nativa Windows, mas não há pipeline Whisper pronto e estável em
  Python sobre DirectML; exigiria reimplementar decodificação/timestamps.
- **ROCm / torch-directml:** suporte Windows imaturo ou abandonado.

Integração com whisper.cpp por **binário `whisper-cli` empacotado, invocado via subprocess** — não
por wheel Python. Motivos: não há wheel com Vulkan confiável; e isolamento — um crash no driver da
GPU mata o processo filho, não o uvicorn.

Modelo GGML em **q5_0** (medium ≈ 540 MB) em vez de f16 (≈ 1,5 GB): qualidade praticamente igual,
metade da VRAM — relevante em placas AMD de entrada.

## UX

O seletor das Configurações permanece **Auto / GPU / CPU**. O backend (CUDA ou Vulkan) é escolha
interna do app; o status mostra placa, VRAM e backend detectados ("GPU detectada: AMD Radeon RX
7600 (8 GB) · Vulkan"). Se o backend for Vulkan e o modelo GGML ainda não estiver em cache, uma
nota informa que ele será baixado na primeira transcrição (~540 MB para `medium`).

## audio-service

### Fachada e backends

`transcriber.py` vira fachada: mantém a API consumida por `main.py` (`transcribe(path, language,
device)`, `gpu_available`, `gpu_name`, `gpu_vram_mb`, `device`) e delega a um de dois backends com
interface mínima `transcribe(path, lang) -> TranscribeResult`:

- `backends/ct2.py` — código atual (faster-whisper CUDA + CPU int8), movido sem mudança de
  comportamento. Cache de `WhisperModel` por device permanece.
- `backends/whispercpp.py` — novo. Monta a linha de comando do `whisper-cli`
  (`-m <ggml> -f <wav> -l <lang|auto> -oj`, saída JSON), roda via `subprocess.run` com
  `CREATE_NO_WINDOW`, parseia o JSON em `TranscribeResult` (texto, idioma detectado, duração).
  Exit code não-zero ou JSON inválido viram exceção.

### Scan de GPU (no boot, uma vez)

- **CUDA:** como hoje (`ctranslate2.get_cuda_device_count()` + carga da cublas).
- **Vulkan:** disponível se `vulkan-1.dll` (loader do driver) carrega via `ctypes` **e** há um
  adaptador DXGI discreto que não seja "Microsoft Basic Render Driver". Enumeração de adaptadores
  via DXGI (`dxgi.dll`, presente em todo Windows) fornece vendor (por VendorId PCI: 0x10DE NVIDIA,
  0x1002 AMD, 0x8086 Intel), nome e VRAM dedicada. `nvidia-smi` deixa de ser necessário para
  nome/VRAM, mas permanece como fonte preferencial quando CUDA está ativo.
- Sem o binário `whisper-cli` empacotado (caso típico do `wails dev`), Vulkan é reportado
  indisponível e o log avisa uma vez.

### Resolução de device (por chamada)

Entrada aceita: `auto` | `gpu` | `cuda` | `cpu` (`cuda` mantido por compatibilidade e depuração).

1. `cpu` → ct2/cpu.
2. `cuda` → ct2/cuda se disponível, senão cpu.
3. `auto` / `gpu` → CUDA disponível → `cuda`; senão Vulkan disponível → `vulkan`; senão `cpu`.

Variável de ambiente `WHISPER_FORCE_BACKEND=vulkan` (dev/homologação apenas) força Vulkan mesmo
com CUDA disponível — é o que permite validar o caminho na RTX 2050.

### Fallback por chamada, sem estado pegajoso

Mesma regra da v2.9.0: falha em `cuda` → tenta `vulkan` se disponível → `cpu`. Falha em `vulkan`
→ `cpu`. Cada chamada re-resolve do zero; nada fica travado em CPU. `TranscribeResult.device`
reporta o efetivo (`cuda` | `vulkan` | `cpu`) e `self.device` é atualizado antes de cada tentativa
para que `/health` reflita a última tentativa mesmo em falha.

### Modelo GGML

Baixado sob demanda do Hugging Face (`ggerganov/whisper.cpp`, arquivo `ggml-<model>-q5_0.bin`)
via `huggingface_hub` para o mesmo diretório de cache do faster-whisper, na primeira transcrição
Vulkan. Mapeamento `whisper_model` → GGML é 1:1 (tiny, base, small, medium, large-v3). `/health`
expõe `vulkan_model_ready`.

## Contrato da API

`/health` — campos novos, atuais preservados:

```json
{
  "device": "vulkan",
  "gpu_available": true,
  "gpu_name": "AMD Radeon RX 7600",
  "gpu_vram_mb": 8192,
  "gpu_vendor": "amd",
  "gpu_backend": "vulkan",
  "vulkan_model_ready": false
}
```

- `gpu_available`: qualquer backend de GPU utilizável (CUDA ou Vulkan).
- `gpu_vendor`: `nvidia` | `amd` | `intel` | `other` | `null`.
- `gpu_backend`: `cuda` | `vulkan` | `null` — o que `auto` escolheria agora.
- `vulkan_model_ready`: GGML do `whisper_model` atual já em cache.

`/transcribe` — `device` aceita `auto|gpu|cuda|cpu`; resposta traz `device` efetivo.

## Go

- `internal/audio/client.go`: `HealthResponse` ganha `GPUVendor`, `GPUBackend`,
  `VulkanModelReady`. `Transcribe` não muda de assinatura.
- Health mirrors em `cmd/desktop/app.go` e `cmd/api/main.go` espelham os três campos (os dois
  entry points em sincronia).
- Setting `whisper_device`: enum passa de `auto|cuda|cpu` para **`auto|gpu|cpu`**. Migration
  `020_whisper_device_gpu.sql`: `UPDATE settings SET value='gpu' WHERE key='whisper_device' AND
  value='cuda';` — sem perda, reversível.
- Orchestrator não muda: repassa o valor do setting.
- Timeout do `/transcribe` permanece 4h.

## Frontend

- `SettingsModal`: opção `cuda` → `gpu` (rótulo "GPU"); status com placa, VRAM e backend; nota do
  download do modelo quando `gpu_backend === "vulkan" && !vulkan_model_ready`.
- `useAudioHealth.ts`: tipos dos três campos novos.

## Empacotamento

- Versão do whisper.cpp pinada em `audio-service/build/whispercpp.version`.
- `audio-service/build/fetch-whispercpp.ps1` obtém o binário para `audio-service/vendor/whispercpp/`
  (gitignorado). Fonte preferencial: asset de release do GitHub com Vulkan para Windows x64. **Se a
  release oficial não publicar build Vulkan**, o script compila via CMake (`-DGGML_VULKAN=1`, exige
  Vulkan SDK na máquina de build). A Task 1 do plano é um spike que decide qual dos dois.
- `.spec` do PyInstaller inclui `vendor/whispercpp/*` em `_internal/whispercpp/`.
- `build.ps1` falha cedo se `vendor/whispercpp/whisper-cli.exe` não existir (mesmo padrão do
  smoke test). Smoke continua só `/health`.
- Runtime Vulkan (`vulkan-1.dll`) vem do driver da placa — não é embarcado.
- Impacto no instalador: +20–40 MB (binário + DLLs ggml). O modelo GGML **não** vai no instalador.

## Testes

- **Python** (suíte segue ~2s, nenhum teste toca GPU, binário ou rede):
  - `backends/whispercpp.py`: `subprocess.run` mockado — linha de comando montada, parse do JSON,
    exit code não-zero e JSON inválido viram exceção.
  - Fachada com backends fake: matriz de resolução (`auto|gpu|cuda|cpu` × CUDA/Vulkan
    disponível/indisponível), as duas cadeias de fallback, `device` efetivo no resultado e no
    `/health`, ausência de estado pegajoso.
  - Scan: DXGI e `ctypes.CDLL` mockados; vendor por VendorId; binário ausente → Vulkan indisponível.
  - `main.py`: `/health` com campos novos; `/transcribe` aceita `gpu`.
- **Go**: whitelist aceita `gpu`, recusa `cuda`; migration 020 converte `cuda`→`gpu`; client
  decodifica os campos novos; health mirrors.
- **Frontend**: `tsc --noEmit` + `npm run build`.
- **Homologação manual** (nesta máquina, RTX 2050): `WHISPER_FORCE_BACKEND=vulkan`, transcrever um
  wav real, conferir `device: "vulkan"` e tempo razoável; depois sem a variável, conferir que Auto
  volta a CUDA. Prova em hardware AMD fica em aberto — decisão consciente do usuário (sem máquina
  AMD disponível).

## Fora de escopo (registrado)

ROCm; DirectML; Linux/macOS; benchmark Vulkan×CUDA; VAD; expor o backend no seletor da UI;
embarcar o modelo GGML no instalador; múltiplas GPUs.

## Decisão transversal (para `.claude/DECISIONS.md` ao fechar)

GPU não-NVIDIA via whisper.cpp/Vulkan como segundo motor de inferência; CUDA permanece o caminho
NVIDIA. Backend é escolha interna; o usuário só vê Auto/GPU/CPU.
