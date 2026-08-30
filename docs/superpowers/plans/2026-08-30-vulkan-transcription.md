# GPU AMD via whisper.cpp/Vulkan — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transcrever em GPU AMD/Intel (qualquer placa com driver Vulkan) mantendo intacto o caminho NVIDIA/CUDA da v2.9.0, com o usuário vendo só Auto/GPU/CPU.

**Architecture:** `transcriber.py` vira fachada sobre dois backends com a mesma interface — `backends/ct2.py` (faster-whisper, código atual movido) e `backends/whispercpp.py` (binário `whisper-cli` com Vulkan, via subprocess). Um módulo `gpuscan.py` detecta CUDA, loader Vulkan e adaptador DXGI (vendor/nome/VRAM). A resolução de device gera uma **cadeia de tentativas** por chamada (`cuda → vulkan → cpu`), sem estado pegajoso. Go e frontend só espelham três campos novos do `/health` e trocam o valor `cuda` do setting por `gpu`.

**Tech Stack:** Python 3 (FastAPI, faster-whisper, huggingface_hub, soundfile, ctypes/DXGI), whisper.cpp (binário Windows x64 com Vulkan), PyInstaller, Go 1.22+ (chi, modernc/sqlite), React 19 + TypeScript.

**Spec:** `docs/superpowers/specs/2026-08-30-amd-gpu-vulkan-transcription-design.md`

## Global Constraints

- Windows x64 apenas; runtime Vulkan (`vulkan-1.dll`) vem do driver da placa — **não** é embarcado.
- Nenhum teste toca GPU, binário real, rede ou `WhisperModel` real. Suíte Python deve seguir em ~2s; Go em `go test ./...`.
- Sem comentários no código, salvo WHY não-óbvio (CLAUDE.md).
- Os dois entry points Go (`cmd/api/main.go`, `cmd/desktop/app.go`) permanecem em sincronia.
- Migrations são embed em `internal/database/migrations/` e aplicadas em `Open`. Testes de repositório/DB usam SQLite real em `t.TempDir()`.
- `whisper_device` aceito pelo Go passa a ser exatamente `auto | gpu | cpu`. O audio-service aceita `auto | gpu | cuda | cpu`.
- `TranscribeResult.device` e `/health.device` valem `cuda | vulkan | cpu`.
- Modelo GGML sempre quantizado: `tiny/base/small → q5_1`, `medium/large → q5_0` (nomes exatos dos arquivos no repo HF `ggerganov/whisper.cpp`); `large` do setting mapeia para `large-v3`.
- Bundle PyInstaller sempre gerado com `audio-service\.venv\Scripts\python.exe`.
- Commits pequenos, mensagem em pt-BR no padrão do repo (`feat:`, `fix:`, `test:`, `docs:`, `chore:`), com trailer `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.
- Todos os comandos abaixo assumem cwd = raiz do repo, exceto quando indicado.

## File Structure

| Arquivo | Responsabilidade |
|---|---|
| `audio-service/gpuscan.py` (novo) | Detecção: `probe_cuda`, `nvidia_smi_info`, `dxgi_adapters`, `vulkan_loader_present`, `scan()` → `GPUInfo`. Sem estado. |
| `audio-service/backends/__init__.py` (novo) | Pacote vazio. |
| `audio-service/backends/ct2.py` (novo) | `CT2Backend`: DLL paths NVIDIA, cache de `WhisperModel` por device, `transcribe(path, lang, device)`. Código atual movido. |
| `audio-service/backends/whispercpp.py` (novo) | `WhisperCppBackend`: localização do `whisper-cli.exe`, download/cache do GGML, subprocess + parse do JSON. |
| `audio-service/transcriber.py` (reescrito) | Fachada: API que `main.py` consome; resolução da cadeia de devices; fallback por chamada; atributos de health. |
| `audio-service/main.py` | `/health` com 3 campos novos; `/transcribe` aceita `gpu`. |
| `audio-service/tests/test_gpuscan.py` (novo), `tests/test_ct2_backend.py` (novo), `tests/test_whispercpp_backend.py` (novo), `tests/test_transcriber.py` (reescrito), `tests/test_main.py` | Testes. |
| `audio-service/build/whispercpp.version` (novo), `audio-service/build/fetch-whispercpp.ps1` (novo) | Pin e obtenção do binário. |
| `audio-service/build/pyinstaller/audio-service.spec` | Inclui `vendor/whispercpp/*` em `_internal/whispercpp/`. |
| `.gitignore` | `/audio-service/vendor/`; negação para `build/whispercpp.version` e `build/fetch-whispercpp.ps1`. |
| `build.ps1` | Falha cedo sem `whisper-cli.exe`; confere presença no bundle copiado. |
| `internal/audio/client.go` + `client_test.go` | 3 campos novos em `HealthResponse`. |
| `internal/services/settings_service.go` + `_test.go` | Enum `auto/gpu/cpu`. |
| `internal/database/migrations/020_whisper_device_gpu.sql` (novo) + `migration_020_test.go` (novo) | `cuda → gpu`. |
| `cmd/desktop/app.go`, `cmd/api/main.go` | Health mirrors. |
| `frontend/src/hooks/useAudioHealth.ts`, `frontend/src/components/settings/SettingsModal.tsx` | Tipos e UI. |
| `.claude/DECISIONS.md`, `.claude/BACKLOG.md`, `CLAUDE.md` | Registro. |

---

### Task 1: Spike — obter o `whisper-cli` com Vulkan e provar o contrato (INTERATIVO, sessão principal)

Este task é exploratório: exige rede, a GPU desta máquina e julgamento. Não é delegável a subagent. Saída: um binário local funcionando, o script que o reproduz, e o formato real do JSON registrado no spec.

**Files:**
- Create: `audio-service/build/whispercpp.version`
- Create: `audio-service/build/fetch-whispercpp.ps1`
- Modify: `.gitignore`
- Modify: `docs/superpowers/specs/2026-08-30-amd-gpu-vulkan-transcription-design.md` (seção "Empacotamento" — resultado do spike)

**Interfaces:**
- Produces: `audio-service/vendor/whispercpp/whisper-cli.exe` (+ DLLs `ggml*.dll`, `whisper.dll`) — o caminho que Task 4 e Task 9 assumem. Flags do CLI confirmadas: `-m`, `-f`, `-l`, `-oj`, `-of`, `-np`.

- [ ] **Step 1: Descobrir se a release oficial publica build Vulkan para Windows.**

```powershell
gh release view --repo ggerganov/whisper.cpp --json tagName,assets --jq '.tagName, (.assets[].name)'
```

Procurar um asset com `vulkan` e `x64`/`win` no nome (ex.: `whisper-vulkan-bin-x64.zip`). Anotar a tag (ex.: `v1.7.6`).

- [ ] **Step 2a (existe asset Vulkan): escrever o pin e o script de download.**

`audio-service/build/whispercpp.version`:
```
v1.7.6
whisper-vulkan-bin-x64.zip
```
(linha 1 = tag; linha 2 = nome exato do asset — substituir pelos valores reais do Step 1.)

`audio-service/build/fetch-whispercpp.ps1`:
```powershell
#Requires -Version 7
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Root      = Split-Path $PSScriptRoot -Parent
$VendorDir = Join-Path $Root "vendor\whispercpp"
$Pin       = Get-Content (Join-Path $PSScriptRoot "whispercpp.version")
$Tag       = $Pin[0].Trim()
$Asset     = $Pin[1].Trim()
$Url       = "https://github.com/ggerganov/whisper.cpp/releases/download/$Tag/$Asset"
$Zip       = Join-Path $env:TEMP $Asset

if (Test-Path (Join-Path $VendorDir "whisper-cli.exe")) {
    Write-Host "whisper-cli já presente em $VendorDir ($Tag). Apague a pasta para rebaixar."
    exit 0
}

Write-Host "Baixando $Url"
Invoke-WebRequest -Uri $Url -OutFile $Zip -UseBasicParsing
New-Item -ItemType Directory -Force $VendorDir | Out-Null
Expand-Archive -Path $Zip -DestinationPath $VendorDir -Force
Remove-Item $Zip

# O zip pode vir com um subdiretório (Release/, bin/): achatar para vendor/whispercpp/.
$exe = Get-ChildItem $VendorDir -Recurse -Filter "whisper-cli.exe" | Select-Object -First 1
if (-not $exe) { Write-Error "whisper-cli.exe não encontrado no asset $Asset"; exit 1 }
if ($exe.DirectoryName -ne $VendorDir) {
    Get-ChildItem $exe.DirectoryName -File | Move-Item -Destination $VendorDir -Force
    Get-ChildItem $VendorDir -Directory | Remove-Item -Recurse -Force
}
Write-Host "OK: $(Join-Path $VendorDir 'whisper-cli.exe') ($Tag)"
```

- [ ] **Step 2b (NÃO existe asset Vulkan): compilar via CMake.**

Pré-requisitos na máquina de build: Vulkan SDK (`winget install KhronosGroup.VulkanSDK`), CMake, Visual Studio Build Tools (MSVC). Nesse caso `whispercpp.version` tem só a tag e o script é:

```powershell
#Requires -Version 7
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Root      = Split-Path $PSScriptRoot -Parent
$VendorDir = Join-Path $Root "vendor\whispercpp"
$Tag       = (Get-Content (Join-Path $PSScriptRoot "whispercpp.version"))[0].Trim()
$Src       = Join-Path $env:TEMP "whisper.cpp-$Tag"

if (Test-Path (Join-Path $VendorDir "whisper-cli.exe")) {
    Write-Host "whisper-cli já presente em $VendorDir ($Tag)."; exit 0
}
if (-not $env:VULKAN_SDK) { Write-Error "VULKAN_SDK não definido — instale o Vulkan SDK."; exit 1 }

if (-not (Test-Path $Src)) {
    git clone --depth 1 --branch $Tag https://github.com/ggerganov/whisper.cpp $Src
}
cmake -S $Src -B "$Src\build" -DGGML_VULKAN=1 -DBUILD_SHARED_LIBS=ON -DCMAKE_BUILD_TYPE=Release
cmake --build "$Src\build" --config Release --target whisper-cli
New-Item -ItemType Directory -Force $VendorDir | Out-Null
Copy-Item "$Src\build\bin\Release\*.exe" $VendorDir -Force
Copy-Item "$Src\build\bin\Release\*.dll" $VendorDir -Force
Write-Host "OK: $(Join-Path $VendorDir 'whisper-cli.exe') ($Tag)"
```

- [ ] **Step 3: `.gitignore`** — acrescentar, depois do bloco do PyInstaller:

```
# Binário do whisper.cpp (Vulkan) obtido por build/fetch-whispercpp.ps1 — não rastreado,
# como o bundle. O pin e o script que o reproduzem SÃO rastreados.
/audio-service/vendor/
!/audio-service/build/whispercpp.version
!/audio-service/build/fetch-whispercpp.ps1
```

Verificar que a negação funciona (o bloco `/audio-service/build/*` acima ignora tudo em `build/`):
```powershell
git check-ignore -v audio-service/build/fetch-whispercpp.ps1; git status --short audio-service/build
```
Expected: `check-ignore` não lista o arquivo (exit 1) e `git status` mostra os dois arquivos como untracked.

- [ ] **Step 4: Rodar o script e provar o binário.**

```powershell
.\audio-service\build\fetch-whispercpp.ps1
.\audio-service\vendor\whispercpp\whisper-cli.exe --help 2>&1 | Select-String -Pattern "-oj|-of|-np|-l "
```
Expected: as quatro flags aparecem na ajuda.

- [ ] **Step 5: Transcrição real via Vulkan na RTX 2050 e captura do JSON.**

Usar um `.wav` real de gravação (achar em `audio-service/tmp/` ou no `RECORDINGS_DIR` do app: `%LOCALAPPDATA%\meeting-notes\recordings` — conferir no `cmd/desktop/app.go:326-330` qual é). Baixar o modelo pequeno para o spike:

```powershell
$m = "$env:TEMP\ggml-base-q5_1.bin"
Invoke-WebRequest "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base-q5_1.bin" -OutFile $m -UseBasicParsing
$out = "$env:TEMP\spike-out"
.\audio-service\vendor\whispercpp\whisper-cli.exe -m $m -f "<caminho do wav>" -l auto -oj -of $out -np
Get-Content "$out.json" -TotalCount 60
```

Verificar e anotar: (a) o stderr do CLI mostra `ggml_vulkan: Found 1 Vulkan devices` com o nome da placa; (b) o `.json` tem `result.language` e `transcription[].text` / `transcription[].offsets.to`; (c) o wav com sample rate de gravação (não 16 kHz) é aceito — se o CLI reclamar de sample rate, a tag pinada é antiga demais (precisa ≥ versão com miniaudio/resample); subir a tag e repetir.

- [ ] **Step 6: Registrar o resultado no spec.** Na seção "Empacotamento" do spec, substituir o parágrafo "Fonte preferencial: ... A Task 1 do plano é um spike que decide qual dos dois." por um parágrafo com: tag pinada, origem (asset de release X ou CMake), tamanho da pasta `vendor/whispercpp`, e o trecho do JSON real (chaves usadas).

- [ ] **Step 7: Commit**

```powershell
git add .gitignore audio-service/build/whispercpp.version audio-service/build/fetch-whispercpp.ps1 docs/superpowers/specs/2026-08-30-amd-gpu-vulkan-transcription-design.md
git commit -m "chore: pin e script de obtenção do whisper-cli (Vulkan) para o audio-service"
```

---

### Task 2: `gpuscan.py` — detecção de CUDA, Vulkan e adaptador DXGI

**Files:**
- Create: `audio-service/gpuscan.py`
- Test: `audio-service/tests/test_gpuscan.py`

**Interfaces:**
- Produces:
  ```python
  @dataclass
  class Adapter: name: str; vendor_id: int; vram_mb: int
  @dataclass
  class GPUInfo:
      cuda_available: bool = False
      vulkan_available: bool = False
      vendor: Optional[str] = None      # "nvidia" | "amd" | "intel" | "other" | None
      name: Optional[str] = None
      vram_mb: Optional[int] = None
      @property available -> bool       # cuda_available or vulkan_available
      @property backend -> Optional[str] # "cuda" | "vulkan" | None
  def vendor_from_id(vendor_id: int) -> str
  def probe_cuda() -> bool
  def nvidia_smi_info() -> tuple[Optional[str], Optional[int]]
  def dxgi_adapters() -> list[Adapter]
  def vulkan_loader_present() -> bool
  def scan(whispercli_present: bool, *, probe_cuda=probe_cuda, nvidia_smi_info=nvidia_smi_info,
           dxgi_adapters=dxgi_adapters, vulkan_loader_present=vulkan_loader_present) -> GPUInfo
  ```
  As sondas reais são injetáveis por kwarg — é assim que os testes as substituem.

- [ ] **Step 1: Escrever os testes**

`audio-service/tests/test_gpuscan.py`:
```python
from gpuscan import Adapter, GPUInfo, scan, vendor_from_id


def _scan(**overrides):
    probes = dict(
        whispercli_present=True,
        probe_cuda=lambda: False,
        nvidia_smi_info=lambda: (None, None),
        dxgi_adapters=lambda: [],
        vulkan_loader_present=lambda: False,
    )
    probes.update(overrides)
    return scan(**probes)


def test_vendor_from_id():
    assert vendor_from_id(0x10DE) == "nvidia"
    assert vendor_from_id(0x1002) == "amd"
    assert vendor_from_id(0x8086) == "intel"
    assert vendor_from_id(0x1414) == "other"


def test_nothing_detected():
    info = _scan()
    assert info == GPUInfo()
    assert info.available is False
    assert info.backend is None


def test_cuda_wins_and_uses_nvidia_smi_for_name():
    info = _scan(
        probe_cuda=lambda: True,
        nvidia_smi_info=lambda: ("NVIDIA GeForce RTX 2050", 4096),
        dxgi_adapters=lambda: [Adapter("NVIDIA GeForce RTX 2050", 0x10DE, 3900)],
        vulkan_loader_present=lambda: True,
    )
    assert info.cuda_available is True
    assert info.vulkan_available is True
    assert info.backend == "cuda"
    assert info.vendor == "nvidia"
    assert info.name == "NVIDIA GeForce RTX 2050"
    assert info.vram_mb == 4096


def test_cuda_without_nvidia_smi_falls_back_to_dxgi_name():
    info = _scan(
        probe_cuda=lambda: True,
        dxgi_adapters=lambda: [Adapter("NVIDIA GeForce RTX 2050", 0x10DE, 3900)],
    )
    assert info.name == "NVIDIA GeForce RTX 2050"
    assert info.vram_mb == 3900


def test_amd_via_vulkan():
    info = _scan(
        dxgi_adapters=lambda: [Adapter("AMD Radeon RX 7600", 0x1002, 8192)],
        vulkan_loader_present=lambda: True,
    )
    assert info.cuda_available is False
    assert info.vulkan_available is True
    assert info.backend == "vulkan"
    assert info.vendor == "amd"
    assert info.name == "AMD Radeon RX 7600"
    assert info.vram_mb == 8192


def test_vulkan_requires_whispercli_binary():
    info = _scan(
        whispercli_present=False,
        dxgi_adapters=lambda: [Adapter("AMD Radeon RX 7600", 0x1002, 8192)],
        vulkan_loader_present=lambda: True,
    )
    assert info.vulkan_available is False
    assert info.backend is None
    assert info.vendor == "amd"


def test_vulkan_requires_loader():
    info = _scan(dxgi_adapters=lambda: [Adapter("AMD Radeon RX 7600", 0x1002, 8192)])
    assert info.vulkan_available is False


def test_vulkan_requires_an_adapter():
    info = _scan(vulkan_loader_present=lambda: True)
    assert info.vulkan_available is False


def test_picks_adapter_with_most_dedicated_vram():
    info = _scan(
        dxgi_adapters=lambda: [
            Adapter("Intel(R) UHD Graphics", 0x8086, 128),
            Adapter("AMD Radeon RX 6700", 0x1002, 12288),
        ],
        vulkan_loader_present=lambda: True,
    )
    assert info.name == "AMD Radeon RX 6700"
    assert info.vendor == "amd"


def test_probe_exceptions_are_swallowed():
    def boom():
        raise OSError("dxgi.dll missing")
    info = _scan(dxgi_adapters=boom, vulkan_loader_present=lambda: True)
    assert info == GPUInfo()
```

- [ ] **Step 2: Rodar e ver falhar**

```powershell
cd audio-service; .venv\Scripts\python.exe -m pytest tests/test_gpuscan.py -q
```
Expected: FAIL — `ModuleNotFoundError: No module named 'gpuscan'`.

- [ ] **Step 3: Implementar `audio-service/gpuscan.py`**

```python
import ctypes
import subprocess
import sys
from ctypes import POINTER, Structure, byref, c_long, c_size_t, c_uint, c_void_p, wintypes
from dataclasses import dataclass
from typing import Callable, Optional

VENDOR_IDS = {0x10DE: "nvidia", 0x1002: "amd", 0x8086: "intel"}
DXGI_ADAPTER_FLAG_SOFTWARE = 2


@dataclass
class Adapter:
    name: str
    vendor_id: int
    vram_mb: int


@dataclass
class GPUInfo:
    cuda_available: bool = False
    vulkan_available: bool = False
    vendor: Optional[str] = None
    name: Optional[str] = None
    vram_mb: Optional[int] = None

    @property
    def available(self) -> bool:
        return self.cuda_available or self.vulkan_available

    @property
    def backend(self) -> Optional[str]:
        if self.cuda_available:
            return "cuda"
        if self.vulkan_available:
            return "vulkan"
        return None


def vendor_from_id(vendor_id: int) -> str:
    return VENDOR_IDS.get(vendor_id, "other")


def probe_cuda() -> bool:
    try:
        import ctranslate2
        if ctranslate2.get_cuda_device_count() <= 0:
            return False
        for dll in ("cublas64_12.dll", "cublas64_11.dll"):
            try:
                ctypes.CDLL(dll)
                return True
            except OSError:
                continue
    except Exception:
        pass
    return False


def nvidia_smi_info() -> tuple[Optional[str], Optional[int]]:
    try:
        flags = 0x08000000 if sys.platform == "win32" else 0
        out = subprocess.run(
            ["nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits"],
            capture_output=True, text=True, timeout=5, creationflags=flags,
        )
        first = out.stdout.strip().splitlines()[0]
        raw_name, raw_mem = first.rsplit(",", 1)
        return raw_name.strip(), int(float(raw_mem.strip()))
    except Exception:
        return None, None


def vulkan_loader_present() -> bool:
    if sys.platform != "win32":
        return False
    try:
        ctypes.CDLL("vulkan-1.dll")
        return True
    except OSError:
        return False


class _GUID(Structure):
    _fields_ = [("Data1", wintypes.DWORD), ("Data2", wintypes.WORD), ("Data3", wintypes.WORD),
                ("Data4", ctypes.c_ubyte * 8)]


class _LUID(Structure):
    _fields_ = [("LowPart", wintypes.DWORD), ("HighPart", wintypes.LONG)]


class _DXGI_ADAPTER_DESC1(Structure):
    _fields_ = [
        ("Description", wintypes.WCHAR * 128),
        ("VendorId", c_uint), ("DeviceId", c_uint), ("SubSysId", c_uint), ("Revision", c_uint),
        ("DedicatedVideoMemory", c_size_t), ("DedicatedSystemMemory", c_size_t),
        ("SharedSystemMemory", c_size_t),
        ("AdapterLuid", _LUID), ("Flags", c_uint),
    ]


_IID_IDXGIFactory1 = _GUID(0x770AAE78, 0xF26F, 0x4DBA,
                           (ctypes.c_ubyte * 8)(0xA8, 0x29, 0x25, 0x3C, 0x83, 0xD1, 0xB3, 0x87))

# Índices na vtable COM: IUnknown(0-2) + IDXGIObject(3-6) + IDXGIFactory(7-11) + IDXGIFactory1(12-13);
# IUnknown(0-2) + IDXGIObject(3-6) + IDXGIAdapter(7-9) + IDXGIAdapter1::GetDesc1(10).
_VT_RELEASE = 2
_VT_FACTORY1_ENUM_ADAPTERS1 = 12
_VT_ADAPTER1_GET_DESC1 = 10


def _method(obj: c_void_p, index: int, restype, *argtypes):
    vtable = ctypes.cast(obj, POINTER(POINTER(c_void_p)))[0]
    return ctypes.WINFUNCTYPE(restype, c_void_p, *argtypes)(vtable[index])


def dxgi_adapters() -> list[Adapter]:
    if sys.platform != "win32":
        return []
    dxgi = ctypes.WinDLL("dxgi")
    dxgi.CreateDXGIFactory1.argtypes = [POINTER(_GUID), POINTER(c_void_p)]
    dxgi.CreateDXGIFactory1.restype = c_long
    factory = c_void_p()
    if dxgi.CreateDXGIFactory1(byref(_IID_IDXGIFactory1), byref(factory)) != 0 or not factory:
        return []
    enum_adapters = _method(factory, _VT_FACTORY1_ENUM_ADAPTERS1, c_long, c_uint, POINTER(c_void_p))
    out: list[Adapter] = []
    try:
        index = 0
        while True:
            adapter = c_void_p()
            if enum_adapters(factory, index, byref(adapter)) != 0:
                break
            try:
                desc = _DXGI_ADAPTER_DESC1()
                get_desc = _method(adapter, _VT_ADAPTER1_GET_DESC1, c_long, POINTER(_DXGI_ADAPTER_DESC1))
                if get_desc(adapter, byref(desc)) == 0 and not (desc.Flags & DXGI_ADAPTER_FLAG_SOFTWARE):
                    out.append(Adapter(desc.Description, desc.VendorId,
                                       int(desc.DedicatedVideoMemory // (1024 * 1024))))
            finally:
                _method(adapter, _VT_RELEASE, c_uint)(adapter)
            index += 1
    finally:
        _method(factory, _VT_RELEASE, c_uint)(factory)
    return out


def _safe(fn: Callable, default):
    try:
        return fn()
    except Exception:
        return default


def scan(
    whispercli_present: bool,
    *,
    probe_cuda: Callable[[], bool] = probe_cuda,
    nvidia_smi_info: Callable[[], tuple[Optional[str], Optional[int]]] = nvidia_smi_info,
    dxgi_adapters: Callable[[], list[Adapter]] = dxgi_adapters,
    vulkan_loader_present: Callable[[], bool] = vulkan_loader_present,
) -> GPUInfo:
    cuda = _safe(probe_cuda, False)
    adapters = _safe(dxgi_adapters, [])
    loader = _safe(vulkan_loader_present, False)
    primary = max(adapters, key=lambda a: a.vram_mb, default=None)

    info = GPUInfo(cuda_available=cuda)
    if primary is not None:
        info.vendor = vendor_from_id(primary.vendor_id)
        info.name = primary.name
        info.vram_mb = primary.vram_mb
    if cuda:
        smi_name, smi_vram = _safe(nvidia_smi_info, (None, None))
        info.vendor = "nvidia"
        info.name = smi_name or info.name
        info.vram_mb = smi_vram or info.vram_mb
    info.vulkan_available = bool(loader and primary is not None and whispercli_present)
    return info
```

Nota para o implementador: o `test_probe_exceptions_are_swallowed` espera `GPUInfo()` puro — ele passa `vulkan_loader_present=True` mas sem adaptador (a sonda DXGI lançou), então `vulkan_available` fica `False` e `vendor/name/vram` ficam `None`. O código acima satisfaz isso.

- [ ] **Step 4: Rodar e ver passar**

```powershell
cd audio-service; .venv\Scripts\python.exe -m pytest tests/test_gpuscan.py -q
```
Expected: 10 passed.

- [ ] **Step 5: Verificação manual da sonda DXGI nesta máquina (não é teste automatizado)**

```powershell
cd audio-service; .venv\Scripts\python.exe -c "import gpuscan; print(gpuscan.dxgi_adapters()); print(gpuscan.vulkan_loader_present())"
```
Expected: lista com `Adapter(name='NVIDIA GeForce RTX 2050', vendor_id=4318, vram_mb=~4096)` (e possivelmente a integrada) e `True`. Se lançar, corrigir a vtable/estrutura antes de seguir — é o único código desta task que não é coberto pela suíte.

- [ ] **Step 6: Commit**

```powershell
git add audio-service/gpuscan.py audio-service/tests/test_gpuscan.py
git commit -m "feat: gpuscan com sondas de CUDA, loader Vulkan e adaptador DXGI"
```

---

### Task 3: `backends/ct2.py` e fachada com cadeia de devices (só CUDA/CPU ainda)

Move o faster-whisper para um backend e reescreve `transcriber.py` como fachada com cadeia de tentativas. O comportamento externo é o da v2.9.0 — os testes existentes são adaptados apenas nos alvos de patch e na forma do scan, não na intenção.

**Files:**
- Create: `audio-service/backends/__init__.py` (vazio)
- Create: `audio-service/backends/ct2.py`
- Rewrite: `audio-service/transcriber.py`
- Create: `audio-service/tests/test_ct2_backend.py`
- Rewrite: `audio-service/tests/test_transcriber.py`

**Interfaces:**
- Consumes: `gpuscan.scan`, `gpuscan.GPUInfo` (Task 2).
- Produces:
  ```python
  # backends/ct2.py
  class CT2Backend:
      def __init__(self, model_name: str, compute_type: str)
      def setup_dll_paths(self) -> None
      def compute_for(self, device: str) -> str
      def get_model(self, device: str)            # cache por device
      def transcribe(self, path: Path, lang: Optional[str], device: str) -> tuple[str, str, float]  # (text, language, duration)
  # transcriber.py
  @dataclass TranscribeResult(transcript, language, duration_seconds, model, device="cpu")
  class Transcriber:
      def __init__(self, model_name, device, compute_type, recordings_dir, *, ct2=None, vulkan=None)
      model_name, default_device, compute_type, recordings_dir, model_loaded, device
      gpu: GPUInfo
      gpu_available, gpu_name, gpu_vram_mb, gpu_vendor, gpu_backend   # propriedades sobre self.gpu
      vulkan_model_ready -> bool                                       # False nesta task
      def _setup_dll_paths(self)                                       # delega ao ct2 (mantido para os patches dos testes)
      def _scan(self) -> GPUInfo                                       # chama transcriber.scan_gpu(...)
      def _chain(self, requested: str) -> list[str]
      def transcribe(self, path, language=None, device="auto") -> TranscribeResult
  ```
  `vulkan` é um objeto opcional com `.available: bool`, `.model_ready: bool` e `.transcribe(path, lang) -> (text, language, duration)`; nesta task ele é `None` e a cadeia nunca inclui `"vulkan"`. Task 5 injeta o backend real.

- [ ] **Step 1: Testes do backend ct2** — `audio-service/tests/test_ct2_backend.py`:

```python
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from backends.ct2 import CT2Backend


@pytest.fixture
def backend():
    with patch("backends.ct2.WhisperModel") as cls:
        b = CT2Backend("medium", "int8_float16")
        b._cls = cls
        yield b


def test_compute_for_cpu_is_int8_even_with_gpu_compute_type(backend):
    assert backend.compute_for("cpu") == "int8"
    assert backend.compute_for("cuda") == "int8_float16"


def test_compute_for_auto_maps_to_int8_float16_on_cuda():
    with patch("backends.ct2.WhisperModel"):
        b = CT2Backend("medium", "auto")
    assert b.compute_for("cuda") == "int8_float16"
    assert b.compute_for("cpu") == "int8"


def test_get_model_caches_per_device(backend):
    backend.get_model("cuda"); backend.get_model("cuda"); backend.get_model("cpu")
    assert backend._cls.call_count == 2
    backend._cls.assert_any_call("medium", device="cuda", compute_type="int8_float16")
    backend._cls.assert_any_call("medium", device="cpu", compute_type="int8")


def test_transcribe_concatenates_segments_and_returns_info(backend, tmp_path):
    seg1 = MagicMock(); seg1.text = " oi "
    seg2 = MagicMock(); seg2.text = "mundo"
    info = MagicMock(); info.language = "pt"; info.duration = 10.5
    backend._cls.return_value.transcribe.return_value = (iter([seg1, seg2]), info)

    text, language, duration = backend.transcribe(tmp_path / "rec.wav", None, "cpu")

    assert (text, language, duration) == ("oi mundo", "pt", 10.5)
    args, kwargs = backend._cls.return_value.transcribe.call_args
    assert args == (str(tmp_path / "rec.wav"),)
    assert kwargs["language"] is None
    assert kwargs["condition_on_previous_text"] is False
    assert kwargs["compression_ratio_threshold"] == 1.8
    assert kwargs["repetition_penalty"] == 1.1


def test_transcribe_passes_language(backend, tmp_path):
    info = MagicMock(); info.language = "en"; info.duration = 1.0
    backend._cls.return_value.transcribe.return_value = (iter([]), info)
    backend.transcribe(tmp_path / "rec.wav", "en", "cpu")
    assert backend._cls.return_value.transcribe.call_args.kwargs["language"] == "en"


def test_transcribe_surfaces_lazy_generator_errors(backend, tmp_path):
    def bad():
        raise RuntimeError("Library cublas64_12.dll is not found")
        yield
    backend._cls.return_value.transcribe.return_value = (bad(), MagicMock())
    with pytest.raises(RuntimeError, match="cublas64_12"):
        backend.transcribe(tmp_path / "rec.wav", None, "cuda")


def test_setup_dll_paths_noop_on_non_windows(monkeypatch):
    monkeypatch.setattr("backends.ct2.sys.platform", "linux")
    fake = MagicMock()
    if hasattr(__import__("os"), "add_dll_directory"):
        monkeypatch.setattr("os.add_dll_directory", fake)
    with patch("backends.ct2.WhisperModel"):
        CT2Backend("medium", "int8").setup_dll_paths()
    fake.assert_not_called()
```

- [ ] **Step 2: Rodar e ver falhar**

```powershell
cd audio-service; .venv\Scripts\python.exe -m pytest tests/test_ct2_backend.py -q
```
Expected: FAIL — `ModuleNotFoundError: No module named 'backends'`.

- [ ] **Step 3: Implementar `backends/__init__.py` (vazio) e `backends/ct2.py`**

```python
import os
import sys
from pathlib import Path
from typing import Optional

from faster_whisper import WhisperModel

TRANSCRIBE_KWARGS = dict(
    # Evita realimentação de alucinação: cada trecho de 30s é decodificado
    # isoladamente, um segmento ruim não contamina os seguintes.
    condition_on_previous_text=False,
    compression_ratio_threshold=1.8,
    repetition_penalty=1.1,
)


class CT2Backend:
    def __init__(self, model_name: str, compute_type: str):
        self.model_name = model_name
        self.compute_type = compute_type
        self._models: dict[str, "WhisperModel"] = {}
        self._dll_handles = []

    def compute_for(self, device: str) -> str:
        if device == "cpu":
            # CPU sempre int8: um compute_type de GPU (int8_float16) herdado quebraria o modelo.
            return "int8"
        return self.compute_type if self.compute_type != "auto" else "int8_float16"

    def get_model(self, device: str):
        if device not in self._models:
            self._models[device] = WhisperModel(
                self.model_name, device=device, compute_type=self.compute_for(device)
            )
        return self._models[device]

    def transcribe(self, path: Path, lang: Optional[str], device: str) -> tuple[str, str, float]:
        model = self.get_model(device)
        segments, info = model.transcribe(str(path), language=lang, **TRANSCRIBE_KWARGS)
        text = " ".join(seg.text.strip() for seg in segments).strip()
        return text, info.language, info.duration

    def setup_dll_paths(self) -> None:
        if sys.platform != "win32":
            return
        import ctypes
        import importlib
        nvidia_dirs: list[Path] = []
        for pkg in ("nvidia.cuda_runtime", "nvidia.cudnn", "nvidia.cublas", "nvidia.cufft"):
            try:
                module = importlib.import_module(pkg)
            except ImportError:
                continue
            if module.__file__ is None:
                continue
            base = Path(module.__file__).parent
            for candidate in (base / "bin", base / "lib"):
                if candidate.exists():
                    nvidia_dirs.append(candidate)
        # Todos os diretórios entram no search path antes de qualquer carga:
        # cublas depende de cudart e a resolução precisa achar os dois.
        for d in nvidia_dirs:
            try:
                self._dll_handles.append(os.add_dll_directory(str(d)))
            except Exception:
                pass
        for d in nvidia_dirs:
            for dll in d.glob("*.dll"):
                try:
                    ctypes.CDLL(str(dll))
                except Exception:
                    pass
```

- [ ] **Step 4: Rodar e ver passar**

```powershell
cd audio-service; .venv\Scripts\python.exe -m pytest tests/test_ct2_backend.py -q
```
Expected: 7 passed.

- [ ] **Step 5: Reescrever `tests/test_transcriber.py`** — mesma intenção dos testes atuais, com três mudanças mecânicas: `patch("transcriber.WhisperModel")` → `patch("backends.ct2.WhisperModel")`; `patch.object(Transcriber, "_scan_gpu", return_value=(avail, name, vram))` → `patch("transcriber.scan_gpu", return_value=GPUInfo(cuda_available=avail, vendor="nvidia" if avail else None, name=name, vram_mb=vram))`; o teste `test_setup_dll_paths_noop_on_non_windows` sai daqui (agora vive em `test_ct2_backend.py`). Conteúdo completo do arquivo:

```python
import logging
from contextlib import contextmanager
from unittest.mock import MagicMock, patch

import pytest

from gpuscan import GPUInfo
from transcriber import Transcriber, TranscribeResult


def _gpu(avail=True, name=None, vram=None):
    return GPUInfo(cuda_available=avail, vendor="nvidia" if avail else None, name=name, vram_mb=vram)


@contextmanager
def _make_transcriber(tmp_path, device="cuda", compute_type="int8_float16"):
    """Patches ficam ativos durante o corpo do teste: um mock que lança dentro
    de transcribe() jamais pode alcançar o WhisperModel real (download de GB)."""
    fake_model = MagicMock()
    with patch("backends.ct2.WhisperModel", return_value=fake_model) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(device == "cuda")):
        t = Transcriber(model_name="medium", device=device, compute_type=compute_type, recordings_dir=tmp_path)
        t._fake_model = fake_model
        t._mock_cls = mock_cls
        yield t


@pytest.fixture
def transcriber(tmp_path):
    with _make_transcriber(tmp_path) as t:
        yield t


def _info(lang="pt", dur=1.0):
    info = MagicMock(); info.language = lang; info.duration = dur
    return info


def test_fixture_patch_active_during_test_body(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
    transcriber._fake_model.transcribe.side_effect = RuntimeError("boom cuda")
    transcriber._mock_cls.side_effect = RuntimeError("segundo load também falha")
    with pytest.raises(RuntimeError, match="segundo load também falha"):
        transcriber.transcribe(wav)
    assert transcriber._mock_cls.call_count == 2


def test_init_loads_model_and_sets_attributes(tmp_path):
    with patch("backends.ct2.WhisperModel", return_value=MagicMock()) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths") as mock_setup, \
         patch("transcriber.scan_gpu", return_value=_gpu(True, "NVIDIA GeForce RTX 2050", 4096)):
        t = Transcriber("medium", "cuda", "int8_float16", tmp_path)
    mock_cls.assert_called_once_with("medium", device="cuda", compute_type="int8_float16")
    mock_setup.assert_called_once()
    assert t.model_loaded is True
    assert t.model_name == "medium"
    assert t.device == "cuda"


def test_scan_exposed_on_attributes(tmp_path):
    with patch("backends.ct2.WhisperModel", return_value=MagicMock()), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(True, "NVIDIA GeForce RTX 2050", 4096)):
        t = Transcriber("medium", "auto", "auto", tmp_path)
    assert t.gpu_available is True
    assert t.gpu_name == "NVIDIA GeForce RTX 2050"
    assert t.gpu_vram_mb == 4096
    assert t.gpu_vendor == "nvidia"
    assert t.gpu_backend == "cuda"
    assert t.vulkan_model_ready is False
    assert t.device == "cuda"


def test_transcribe_path_outside_recordings_dir_raises(transcriber, tmp_path):
    with pytest.raises(ValueError, match="outside recordings dir"):
        transcriber.transcribe(tmp_path.parent / "elsewhere.wav")


def test_transcribe_path_does_not_exist_raises(transcriber, tmp_path):
    with pytest.raises(ValueError, match="does not exist"):
        transcriber.transcribe(tmp_path / "missing.wav")


def test_transcribe_returns_result(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
    seg = MagicMock(); seg.text = " oi mundo "
    transcriber._fake_model.transcribe.return_value = (iter([seg]), _info("pt", 10.5))

    result = transcriber.transcribe(wav)

    assert result == TranscribeResult("oi mundo", "pt", 10.5, "medium", "cuda")


@pytest.mark.parametrize("language,expected", [("auto", None), ("", None), (None, None), ("en", "en")])
def test_transcribe_language_normalisation(transcriber, tmp_path, language, expected):
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
    transcriber._fake_model.transcribe.return_value = (iter([]), _info("en"))
    transcriber.transcribe(wav, language=language)
    assert transcriber._fake_model.transcribe.call_args.kwargs["language"] == expected


def test_transcribe_uses_cpu_when_device_cpu_requested(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
    transcriber._fake_model.transcribe.return_value = (iter([]), _info())
    result = transcriber.transcribe(wav, device="cpu")
    assert result.device == "cpu"
    assert transcriber._mock_cls.call_count == 2
    transcriber._mock_cls.assert_called_with("medium", device="cpu", compute_type="int8")


def test_transcribe_gpu_alias_resolves_to_cuda(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
    transcriber._fake_model.transcribe.return_value = (iter([]), _info())
    assert transcriber.transcribe(wav, device="gpu").device == "cuda"


def test_transcribe_model_cache_reuses_per_device(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
    transcriber._fake_model.transcribe.return_value = (iter([]), _info())
    transcriber.transcribe(wav); transcriber.transcribe(wav)
    assert transcriber._mock_cls.call_count == 1


def _factory(cuda_side_effect, cpu_side_effect):
    def model_factory(name, device, compute_type):
        m = MagicMock()
        m.transcribe.side_effect = cuda_side_effect if device == "cuda" else cpu_side_effect
        return m
    return model_factory


def test_transcribe_fallback_does_not_stick(tmp_path):
    calls = []
    def flaky(*a, **k):
        calls.append("cuda")
        if calls.count("cuda") == 1:
            raise RuntimeError("CUDA failed with error out of memory")
        return (iter([]), _info())
    def ok(*a, **k):
        calls.append("cpu"); return (iter([]), _info())

    with patch("backends.ct2.WhisperModel", side_effect=_factory(flaky, ok)), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(True, "RTX", 4096)):
        t = Transcriber("medium", "auto", "auto", tmp_path)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        r1 = t.transcribe(wav)
        r2 = t.transcribe(wav)

    assert (r1.device, r2.device) == ("cpu", "cuda")
    assert calls == ["cuda", "cpu", "cuda"]


def test_transcribe_cuda_error_falls_back_to_cpu_and_logs_once(tmp_path, caplog):
    seg = MagicMock(); seg.text = "fallback"
    with patch("backends.ct2.WhisperModel",
               side_effect=_factory(RuntimeError("CUDA failed with error out of memory"),
                                    lambda *a, **k: (iter([seg]), _info("pt", 3.0)))) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(True, "RTX", 4096)):
        t = Transcriber("medium", "cuda", "int8_float16", tmp_path)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        with caplog.at_level(logging.WARNING):
            result = t.transcribe(wav)

    assert result.transcript == "fallback"
    assert result.device == "cpu"
    assert t.device == "cpu"
    mock_cls.assert_called_with("medium", device="cpu", compute_type="int8")
    warnings = [r for r in caplog.records if r.levelno == logging.WARNING]
    assert len(warnings) == 1
    assert "out of memory" in warnings[0].getMessage()


def test_transcribe_error_on_cpu_propagates(tmp_path):
    cpu_model = MagicMock(); cpu_model.transcribe.side_effect = ValueError("invalid audio format")
    with patch("backends.ct2.WhisperModel", return_value=cpu_model) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(False)):
        t = Transcriber("medium", "cpu", "int8", tmp_path)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        with pytest.raises(ValueError, match="invalid audio format"):
            t.transcribe(wav)
    assert t.device == "cpu"
    assert mock_cls.call_count == 1


def test_transcribe_cpu_retry_failure_propagates(tmp_path):
    with patch("backends.ct2.WhisperModel",
               side_effect=_factory(RuntimeError("CUDA failed with error out of memory"),
                                    ValueError("corrupt wav"))), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(True, "RTX", 4096)):
        t = Transcriber("medium", "cuda", "int8_float16", tmp_path)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        with pytest.raises(ValueError, match="corrupt wav"):
            t.transcribe(wav)
    assert t.device == "cpu"


def test_chain_without_gpu_is_cpu_only(tmp_path):
    with patch("backends.ct2.WhisperModel", return_value=MagicMock()), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(False)):
        t = Transcriber("medium", "auto", "auto", tmp_path)
    assert t._chain("auto") == ["cpu"]
    assert t._chain("gpu") == ["cpu"]
    assert t._chain("cuda") == ["cpu"]
    assert t._chain("cpu") == ["cpu"]
    assert t.gpu_backend is None


def test_chain_with_cuda(tmp_path):
    with patch("backends.ct2.WhisperModel", return_value=MagicMock()), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(True)):
        t = Transcriber("medium", "auto", "auto", tmp_path)
    assert t._chain("auto") == ["cuda", "cpu"]
    assert t._chain("gpu") == ["cuda", "cpu"]
    assert t._chain("cuda") == ["cuda", "cpu"]
    assert t._chain("cpu") == ["cpu"]
```

- [ ] **Step 6: Rodar e ver falhar**

```powershell
cd audio-service; .venv\Scripts\python.exe -m pytest tests/test_transcriber.py -q
```
Expected: FAIL — `ImportError: cannot import name 'scan_gpu'` (ou `AttributeError` em `transcriber.scan_gpu`).

- [ ] **Step 7: Reescrever `audio-service/transcriber.py`**

```python
import logging
import os
from dataclasses import dataclass
from pathlib import Path
from typing import Optional, Protocol

from backends.ct2 import CT2Backend
from gpuscan import GPUInfo
from gpuscan import scan as scan_gpu

GPU_ALIASES = ("auto", "gpu", None, "")


@dataclass
class TranscribeResult:
    transcript: str
    language: str
    duration_seconds: float
    model: str
    device: str = "cpu"


class VulkanBackend(Protocol):
    available: bool
    model_ready: bool

    def transcribe(self, path: Path, lang: Optional[str]) -> tuple[str, str, float]: ...


class Transcriber:
    def __init__(
        self,
        model_name: str,
        device: str,
        compute_type: str,
        recordings_dir: Path,
        *,
        ct2: Optional[CT2Backend] = None,
        vulkan: Optional[VulkanBackend] = None,
    ):
        self.model_name = model_name
        self.default_device = device
        self.compute_type = compute_type
        self.recordings_dir = Path(recordings_dir).resolve()
        self._ct2 = ct2 or CT2Backend(model_name, compute_type)
        self._vulkan = vulkan
        self._setup_dll_paths()
        self.gpu: GPUInfo = self._scan()
        first = self._chain(device)[0]
        if first in ("cuda", "cpu"):
            self._ct2.get_model(first)
        self.device = first
        self.model_loaded = True

    def _setup_dll_paths(self) -> None:
        self._ct2.setup_dll_paths()

    def _scan(self) -> GPUInfo:
        return scan_gpu(whispercli_present=bool(self._vulkan and self._vulkan.available))

    @property
    def gpu_available(self) -> bool:
        return self.gpu.available

    @property
    def gpu_name(self) -> Optional[str]:
        return self.gpu.name

    @property
    def gpu_vram_mb(self) -> Optional[int]:
        return self.gpu.vram_mb

    @property
    def gpu_vendor(self) -> Optional[str]:
        return self.gpu.vendor

    @property
    def gpu_backend(self) -> Optional[str]:
        return self.gpu.backend

    @property
    def vulkan_model_ready(self) -> bool:
        return bool(self._vulkan and self._vulkan.model_ready)

    def _chain(self, requested: Optional[str]) -> list[str]:
        if requested == "cpu":
            return ["cpu"]
        chain: list[str] = []
        if requested == "cuda":
            if self.gpu.cuda_available:
                chain.append("cuda")
                if self.gpu.vulkan_available:
                    chain.append("vulkan")
            return chain + ["cpu"]
        if os.getenv("WHISPER_FORCE_BACKEND") == "vulkan" and self.gpu.vulkan_available:
            return ["vulkan", "cpu"]
        if self.gpu.cuda_available:
            chain.append("cuda")
        if self.gpu.vulkan_available:
            chain.append("vulkan")
        return chain + ["cpu"]

    def _run(self, device: str, path: Path, lang: Optional[str]) -> tuple[str, str, float]:
        if device == "vulkan":
            return self._vulkan.transcribe(path, lang)
        return self._ct2.transcribe(path, lang, device)

    def transcribe(self, path: Path, language: Optional[str] = None, device: str = "auto") -> TranscribeResult:
        resolved = Path(path).resolve()
        try:
            resolved.relative_to(self.recordings_dir)
        except ValueError:
            raise ValueError(f"path outside recordings dir: {path}")
        if not resolved.exists():
            raise ValueError(f"path does not exist: {path}")

        lang = None if language in (None, "", "auto") else language
        chain = self._chain(device)
        for index, attempt in enumerate(chain):
            # Atribuído antes da tentativa: se a última também falhar, /health reporta onde parou.
            self.device = attempt
            try:
                text, detected, duration = self._run(attempt, resolved, lang)
            except Exception as e:
                if index == len(chain) - 1:
                    raise
                # Transcrição é o ativo primário: qualquer falha na GPU vale a próxima
                # tentativa da cadeia NESTA chamada; a próxima chamada re-resolve do zero.
                logging.warning("%s inference failed (%s), retrying this call on %s", attempt, e, chain[index + 1])
                continue
            return TranscribeResult(
                transcript=text,
                language=detected,
                duration_seconds=duration,
                model=self.model_name,
                device=attempt,
            )
        raise RuntimeError("unreachable: empty device chain")
```

- [ ] **Step 8: Rodar a suíte inteira**

```powershell
cd audio-service; .venv\Scripts\python.exe -m pytest -q
```
Expected: tudo verde (test_main.py continua passando — a assinatura pública não mudou). Contar: 49 anteriores − 15 do test_transcriber antigo + 20 novos do test_transcriber + 7 do ct2 + 10 do gpuscan ≈ 71 passed. Se `test_main.py::test_health_*` falhar por `gpu_vendor`, é porque a Task 6 ainda não rodou — não deve falhar, pois `/health` ainda não lê os campos novos.

- [ ] **Step 9: Commit**

```powershell
git add audio-service/backends audio-service/transcriber.py audio-service/tests/test_ct2_backend.py audio-service/tests/test_transcriber.py
git commit -m "refactor: faster-whisper vira backend ct2; Transcriber vira fachada com cadeia de devices"
```

---

### Task 4: `backends/whispercpp.py` — binário, modelo GGML e parse do JSON

**Files:**
- Create: `audio-service/backends/whispercpp.py`
- Test: `audio-service/tests/test_whispercpp_backend.py`

**Interfaces:**
- Consumes: JSON do `whisper-cli -oj` (formato confirmado na Task 1: `result.language`, `transcription[].text`, `transcription[].offsets.to` em ms).
- Produces:
  ```python
  GGML_FILES = {"tiny": "ggml-tiny-q5_1.bin", "base": "ggml-base-q5_1.bin", "small": "ggml-small-q5_1.bin",
                "medium": "ggml-medium-q5_0.bin", "large": "ggml-large-v3-q5_0.bin", "large-v3": "ggml-large-v3-q5_0.bin"}
  HF_REPO = "ggerganov/whisper.cpp"
  def find_whispercli() -> Optional[Path]
  class WhisperCppBackend:
      def __init__(self, model_name: str, exe: Optional[Path] = None, *, runner=subprocess.run,
                   downloader=hf_hub_download, cache_lookup=try_to_load_from_cache)
      available: bool          # exe existe
      model_ready: bool        # GGML já em cache (propriedade, consulta cache_lookup)
      def model_path(self) -> Path  # baixa se preciso
      def build_command(self, model: Path, wav: Path, lang: Optional[str], out_prefix: Path) -> list[str]
      def transcribe(self, path: Path, lang: Optional[str]) -> tuple[str, str, float]
  ```

- [ ] **Step 1: Escrever os testes** — `audio-service/tests/test_whispercpp_backend.py`:

```python
import json
import subprocess
from pathlib import Path
from unittest.mock import MagicMock

import pytest

from backends.whispercpp import GGML_FILES, HF_REPO, WhisperCppBackend, find_whispercli


def _json(text_parts, language="pt", last_to_ms=12500):
    return {
        "result": {"language": language},
        "transcription": [
            {"text": t, "offsets": {"from": i * 1000, "to": last_to_ms if i == len(text_parts) - 1 else (i + 1) * 1000}}
            for i, t in enumerate(text_parts)
        ],
    }


@pytest.fixture
def exe(tmp_path):
    p = tmp_path / "vendor" / "whisper-cli.exe"
    p.parent.mkdir(parents=True)
    p.write_bytes(b"MZ")
    return p


@pytest.fixture
def model_file(tmp_path):
    m = tmp_path / "hf" / "ggml-medium-q5_0.bin"
    m.parent.mkdir(parents=True)
    m.write_bytes(b"ggml")
    return m


def _runner_writing(json_obj, returncode=0, stderr=""):
    def run(cmd, **kwargs):
        of = Path(cmd[cmd.index("-of") + 1])
        if json_obj is not None:
            of.with_suffix(".json").write_text(json.dumps(json_obj), encoding="utf-8")
        return subprocess.CompletedProcess(cmd, returncode, stdout="", stderr=stderr)
    return MagicMock(side_effect=run)


def test_ggml_map_is_quantized_and_large_maps_to_v3():
    assert GGML_FILES["medium"] == "ggml-medium-q5_0.bin"
    assert GGML_FILES["small"] == "ggml-small-q5_1.bin"
    assert GGML_FILES["large"] == "ggml-large-v3-q5_0.bin"
    assert HF_REPO == "ggerganov/whisper.cpp"


def test_available_reflects_exe_presence(exe, tmp_path):
    assert WhisperCppBackend("medium", exe).available is True
    assert WhisperCppBackend("medium", tmp_path / "nope.exe").available is False
    assert WhisperCppBackend("medium", None).available is False


def test_find_whispercli_prefers_env_override(monkeypatch, exe):
    monkeypatch.setenv("WHISPER_CPP_BIN", str(exe))
    assert find_whispercli() == exe


def test_find_whispercli_returns_none_when_absent(monkeypatch, tmp_path):
    monkeypatch.delenv("WHISPER_CPP_BIN", raising=False)
    monkeypatch.setattr("backends.whispercpp._search_roots", lambda: [tmp_path])
    assert find_whispercli() is None


def test_find_whispercli_in_search_root(monkeypatch, tmp_path):
    monkeypatch.delenv("WHISPER_CPP_BIN", raising=False)
    p = tmp_path / "whispercpp" / "whisper-cli.exe"
    p.parent.mkdir(); p.write_bytes(b"MZ")
    monkeypatch.setattr("backends.whispercpp._search_roots", lambda: [tmp_path])
    assert find_whispercli() == p


def test_model_ready_uses_cache_lookup(exe, model_file):
    b = WhisperCppBackend("medium", exe, cache_lookup=lambda repo, fn: str(model_file))
    assert b.model_ready is True
    b2 = WhisperCppBackend("medium", exe, cache_lookup=lambda repo, fn: None)
    assert b2.model_ready is False


def test_model_path_downloads_once(exe, model_file):
    downloader = MagicMock(return_value=str(model_file))
    b = WhisperCppBackend("medium", exe, downloader=downloader, cache_lookup=lambda r, f: None)
    assert b.model_path() == model_file
    assert b.model_path() == model_file
    downloader.assert_called_once_with(repo_id=HF_REPO, filename="ggml-medium-q5_0.bin")


def test_build_command(exe, model_file, tmp_path):
    b = WhisperCppBackend("medium", exe)
    cmd = b.build_command(model_file, tmp_path / "rec.wav", "pt", tmp_path / "out")
    assert cmd[0] == str(exe)
    assert cmd[cmd.index("-m") + 1] == str(model_file)
    assert cmd[cmd.index("-f") + 1] == str(tmp_path / "rec.wav")
    assert cmd[cmd.index("-l") + 1] == "pt"
    assert cmd[cmd.index("-of") + 1] == str(tmp_path / "out")
    assert "-oj" in cmd and "-np" in cmd


def test_build_command_language_none_is_auto(exe, model_file, tmp_path):
    cmd = WhisperCppBackend("medium", exe).build_command(model_file, tmp_path / "r.wav", None, tmp_path / "o")
    assert cmd[cmd.index("-l") + 1] == "auto"


def test_transcribe_parses_json(exe, model_file, tmp_path):
    runner = _runner_writing(_json([" Olá ", " mundo "], "pt", 12500))
    b = WhisperCppBackend("medium", exe, runner=runner, downloader=lambda **k: str(model_file),
                          cache_lookup=lambda r, f: None)
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"RIFF")

    text, language, duration = b.transcribe(wav, None)

    assert (text, language, duration) == ("Olá mundo", "pt", 12.5)
    kwargs = runner.call_args.kwargs
    assert kwargs["capture_output"] is True
    assert kwargs["text"] is True
    assert kwargs.get("creationflags", 0) == 0x08000000


def test_transcribe_raises_on_nonzero_exit_with_stderr(exe, model_file, tmp_path):
    runner = _runner_writing(None, returncode=1, stderr="ggml_vulkan: device lost")
    b = WhisperCppBackend("medium", exe, runner=runner, downloader=lambda **k: str(model_file),
                          cache_lookup=lambda r, f: None)
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"RIFF")
    with pytest.raises(RuntimeError, match="device lost"):
        b.transcribe(wav, None)


def test_transcribe_raises_when_json_missing(exe, model_file, tmp_path):
    runner = _runner_writing(None, returncode=0)
    b = WhisperCppBackend("medium", exe, runner=runner, downloader=lambda **k: str(model_file),
                          cache_lookup=lambda r, f: None)
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"RIFF")
    with pytest.raises(RuntimeError, match="no JSON output"):
        b.transcribe(wav, None)


def test_transcribe_raises_when_unavailable(tmp_path):
    b = WhisperCppBackend("medium", None)
    with pytest.raises(RuntimeError, match="whisper-cli not available"):
        b.transcribe(tmp_path / "rec.wav", None)


def test_transcribe_cleans_temp_output(exe, model_file, tmp_path, monkeypatch):
    created = []
    runner = _runner_writing(_json(["x"]))
    b = WhisperCppBackend("medium", exe, runner=runner, downloader=lambda **k: str(model_file),
                          cache_lookup=lambda r, f: None)
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"RIFF")
    b.transcribe(wav, None)
    of = Path(runner.call_args.args[0][runner.call_args.args[0].index("-of") + 1])
    assert not of.parent.exists()
```

- [ ] **Step 2: Rodar e ver falhar**

```powershell
cd audio-service; .venv\Scripts\python.exe -m pytest tests/test_whispercpp_backend.py -q
```
Expected: FAIL — `ModuleNotFoundError: No module named 'backends.whispercpp'`.

- [ ] **Step 3: Implementar `audio-service/backends/whispercpp.py`**

```python
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Callable, Optional

from huggingface_hub import hf_hub_download, try_to_load_from_cache

HF_REPO = "ggerganov/whisper.cpp"
GGML_FILES = {
    "tiny": "ggml-tiny-q5_1.bin",
    "base": "ggml-base-q5_1.bin",
    "small": "ggml-small-q5_1.bin",
    "medium": "ggml-medium-q5_0.bin",
    "large": "ggml-large-v3-q5_0.bin",
    "large-v3": "ggml-large-v3-q5_0.bin",
}
EXE_NAME = "whisper-cli.exe"
CREATE_NO_WINDOW = 0x08000000 if sys.platform == "win32" else 0


def _search_roots() -> list[Path]:
    roots = []
    frozen = getattr(sys, "_MEIPASS", None)
    if frozen:
        roots.append(Path(frozen))
    roots.append(Path(__file__).resolve().parent.parent / "vendor")
    return roots


def find_whispercli() -> Optional[Path]:
    override = os.getenv("WHISPER_CPP_BIN")
    if override:
        p = Path(override)
        return p if p.exists() else None
    for root in _search_roots():
        candidate = root / "whispercpp" / EXE_NAME
        if candidate.exists():
            return candidate
    return None


class WhisperCppBackend:
    def __init__(
        self,
        model_name: str,
        exe: Optional[Path] = None,
        *,
        runner: Callable = subprocess.run,
        downloader: Callable = hf_hub_download,
        cache_lookup: Callable = try_to_load_from_cache,
    ):
        self.model_name = model_name
        self.exe = Path(exe) if exe else None
        self._runner = runner
        self._downloader = downloader
        self._cache_lookup = cache_lookup
        self._model_path: Optional[Path] = None

    @property
    def available(self) -> bool:
        return bool(self.exe and self.exe.exists())

    @property
    def ggml_filename(self) -> str:
        return GGML_FILES.get(self.model_name, GGML_FILES["medium"])

    @property
    def model_ready(self) -> bool:
        if self._model_path is not None:
            return True
        cached = self._cache_lookup(HF_REPO, self.ggml_filename)
        return isinstance(cached, str) and Path(cached).exists()

    def model_path(self) -> Path:
        if self._model_path is None:
            self._model_path = Path(self._downloader(repo_id=HF_REPO, filename=self.ggml_filename))
        return self._model_path

    def build_command(self, model: Path, wav: Path, lang: Optional[str], out_prefix: Path) -> list[str]:
        return [
            str(self.exe),
            "-m", str(model),
            "-f", str(wav),
            "-l", lang or "auto",
            "-oj",
            "-of", str(out_prefix),
            "-np",
        ]

    def transcribe(self, path: Path, lang: Optional[str]) -> tuple[str, str, float]:
        if not self.available:
            raise RuntimeError("whisper-cli not available")
        model = self.model_path()
        with tempfile.TemporaryDirectory(prefix="whispercpp-") as tmp:
            out_prefix = Path(tmp) / "out"
            proc = self._runner(
                self.build_command(model, path, lang, out_prefix),
                capture_output=True, text=True, creationflags=CREATE_NO_WINDOW,
            )
            if proc.returncode != 0:
                raise RuntimeError(f"whisper-cli exited {proc.returncode}: {proc.stderr.strip()[-2000:]}")
            json_path = out_prefix.with_suffix(".json")
            if not json_path.exists():
                raise RuntimeError("whisper-cli produced no JSON output")
            data = json.loads(json_path.read_text(encoding="utf-8"))
        segments = data.get("transcription", [])
        text = " ".join(seg.get("text", "").strip() for seg in segments).strip()
        language = data.get("result", {}).get("language", lang or "")
        last_ms = max((seg.get("offsets", {}).get("to", 0) for seg in segments), default=0)
        return text, language, last_ms / 1000.0
```

Se o JSON real capturado na Task 1 divergir (chaves diferentes), ajustar **primeiro o teste** (`_json` helper) e depois o parse — o spec já registrou o formato real.

- [ ] **Step 4: Rodar e ver passar**

```powershell
cd audio-service; .venv\Scripts\python.exe -m pytest tests/test_whispercpp_backend.py -q
```
Expected: 15 passed.

- [ ] **Step 5: Commit**

```powershell
git add audio-service/backends/whispercpp.py audio-service/tests/test_whispercpp_backend.py
git commit -m "feat: backend whisper.cpp (Vulkan) via subprocess com download do GGML sob demanda"
```

---

### Task 5: Ligar o Vulkan na fachada — cadeia, força por env e campos de health

**Files:**
- Modify: `audio-service/transcriber.py` (construtor: backend Vulkan padrão)
- Modify: `audio-service/tests/test_transcriber.py` (novos testes)

**Interfaces:**
- Consumes: `WhisperCppBackend`, `find_whispercli` (Task 4); `_chain`, `_run` (Task 3).
- Produces: `Transcriber(...)` sem `vulkan=` constrói `WhisperCppBackend(model_name, find_whispercli())` sozinho; `gpu_backend == "vulkan"` quando aplicável; `TranscribeResult.device == "vulkan"`.

- [ ] **Step 1: Acrescentar testes ao fim de `tests/test_transcriber.py`**

```python
class FakeVulkan:
    def __init__(self, available=True, model_ready=False, side_effect=None, result=("vk text", "pt", 4.0)):
        self.available = available
        self.model_ready = model_ready
        self.side_effect = side_effect
        self.result = result
        self.calls = []

    def transcribe(self, path, lang):
        self.calls.append((path, lang))
        if self.side_effect:
            raise self.side_effect
        return self.result


def _amd():
    return GPUInfo(cuda_available=False, vulkan_available=True, vendor="amd", name="AMD Radeon RX 7600", vram_mb=8192)


def _nvidia_both():
    return GPUInfo(cuda_available=True, vulkan_available=True, vendor="nvidia", name="RTX", vram_mb=4096)


def _make_with_vulkan(tmp_path, gpu, vulkan, device="auto", cpu_side_effect=None):
    cpu_model = MagicMock()
    cpu_model.transcribe.side_effect = cpu_side_effect or (lambda *a, **k: (iter([]), _info("pt", 1.0)))
    patches = (
        patch("backends.ct2.WhisperModel", return_value=cpu_model),
        patch.object(Transcriber, "_setup_dll_paths"),
        patch("transcriber.scan_gpu", return_value=gpu),
    )
    return patches, cpu_model


def test_default_vulkan_backend_is_built_from_find_whispercli(tmp_path):
    with patch("backends.ct2.WhisperModel", return_value=MagicMock()), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(False)) as scan, \
         patch("transcriber.find_whispercli", return_value=None):
        t = Transcriber("medium", "auto", "auto", tmp_path)
    assert t._vulkan is not None
    assert t._vulkan.available is False
    assert scan.call_args.kwargs["whispercli_present"] is False


def test_scan_receives_whispercli_presence(tmp_path):
    with patch("backends.ct2.WhisperModel", return_value=MagicMock()), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_amd()) as scan:
        Transcriber("medium", "auto", "auto", tmp_path, vulkan=FakeVulkan(available=True))
    assert scan.call_args.kwargs["whispercli_present"] is True


def test_amd_auto_uses_vulkan_and_reports_it(tmp_path):
    vk = FakeVulkan(model_ready=True)
    patches, cpu_model = _make_with_vulkan(tmp_path, _amd(), vk)
    with patches[0], patches[1], patches[2]:
        t = Transcriber("medium", "auto", "auto", tmp_path, vulkan=vk)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        result = t.transcribe(wav, language="pt")

    assert t.device == "vulkan"
    assert t.gpu_backend == "vulkan"
    assert t.gpu_vendor == "amd"
    assert t.vulkan_model_ready is True
    assert result == TranscribeResult("vk text", "pt", 4.0, "medium", "vulkan")
    assert vk.calls == [(wav.resolve(), "pt")]
    cpu_model.transcribe.assert_not_called()


def test_amd_boot_does_not_preload_ct2_model(tmp_path):
    with patch("backends.ct2.WhisperModel") as cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_amd()):
        t = Transcriber("medium", "auto", "auto", tmp_path, vulkan=FakeVulkan())
    cls.assert_not_called()
    assert t.model_loaded is True
    assert t.device == "vulkan"


def test_amd_chains():
    class _T(Transcriber):
        def __init__(self): self.gpu = _amd()
    t = _T()
    assert t._chain("auto") == ["vulkan", "cpu"]
    assert t._chain("gpu") == ["vulkan", "cpu"]
    assert t._chain("cuda") == ["cpu"]
    assert t._chain("cpu") == ["cpu"]


def test_nvidia_with_both_chains_cuda_then_vulkan(monkeypatch):
    monkeypatch.delenv("WHISPER_FORCE_BACKEND", raising=False)
    class _T(Transcriber):
        def __init__(self): self.gpu = _nvidia_both()
    t = _T()
    assert t._chain("auto") == ["cuda", "vulkan", "cpu"]
    assert t._chain("cuda") == ["cuda", "vulkan", "cpu"]


def test_force_backend_vulkan_env(monkeypatch):
    monkeypatch.setenv("WHISPER_FORCE_BACKEND", "vulkan")
    class _T(Transcriber):
        def __init__(self): self.gpu = _nvidia_both()
    t = _T()
    assert t._chain("auto") == ["vulkan", "cpu"]
    assert t._chain("gpu") == ["vulkan", "cpu"]
    assert t._chain("cuda") == ["cuda", "vulkan", "cpu"]
    assert t._chain("cpu") == ["cpu"]


def test_force_backend_ignored_when_vulkan_unavailable(monkeypatch):
    monkeypatch.setenv("WHISPER_FORCE_BACKEND", "vulkan")
    class _T(Transcriber):
        def __init__(self): self.gpu = _gpu(True)
    assert _T()._chain("auto") == ["cuda", "cpu"]


def test_vulkan_failure_falls_back_to_cpu_in_same_call(tmp_path, caplog):
    vk = FakeVulkan(side_effect=RuntimeError("whisper-cli exited 1: device lost"))
    patches, cpu_model = _make_with_vulkan(tmp_path, _amd(), vk)
    with patches[0], patches[1], patches[2]:
        t = Transcriber("medium", "auto", "auto", tmp_path, vulkan=vk)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        with caplog.at_level(logging.WARNING):
            result = t.transcribe(wav)

    assert result.device == "cpu"
    assert t.device == "cpu"
    cpu_model.transcribe.assert_called_once()
    assert any("vulkan inference failed" in r.getMessage() for r in caplog.records)


def test_vulkan_failure_does_not_stick(tmp_path):
    vk = FakeVulkan()
    vk.side_effect = RuntimeError("first fails")
    patches, cpu_model = _make_with_vulkan(tmp_path, _amd(), vk)
    with patches[0], patches[1], patches[2]:
        t = Transcriber("medium", "auto", "auto", tmp_path, vulkan=vk)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        r1 = t.transcribe(wav)
        vk.side_effect = None
        r2 = t.transcribe(wav)
    assert (r1.device, r2.device) == ("cpu", "vulkan")
    assert len(vk.calls) == 2


def test_cuda_failure_tries_vulkan_before_cpu(tmp_path):
    vk = FakeVulkan()
    cuda_model = MagicMock(); cuda_model.transcribe.side_effect = RuntimeError("CUDA out of memory")
    def factory(name, device, compute_type):
        return cuda_model
    with patch("backends.ct2.WhisperModel", side_effect=factory), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_nvidia_both()):
        t = Transcriber("medium", "auto", "auto", tmp_path, vulkan=vk)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        result = t.transcribe(wav)
    assert result.device == "vulkan"
    assert len(vk.calls) == 1


def test_vulkan_then_cpu_both_fail_propagates_cpu_error(tmp_path):
    vk = FakeVulkan(side_effect=RuntimeError("vk down"))
    patches, cpu_model = _make_with_vulkan(tmp_path, _amd(), vk, cpu_side_effect=ValueError("corrupt wav"))
    with patches[0], patches[1], patches[2]:
        t = Transcriber("medium", "auto", "auto", tmp_path, vulkan=vk)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        with pytest.raises(ValueError, match="corrupt wav"):
            t.transcribe(wav)
    assert t.device == "cpu"
```

- [ ] **Step 2: Rodar e ver falhar**

```powershell
cd audio-service; .venv\Scripts\python.exe -m pytest tests/test_transcriber.py -q
```
Expected: FAIL — `test_default_vulkan_backend_is_built_from_find_whispercli` (`AttributeError: transcriber has no attribute find_whispercli`) e `test_amd_boot_does_not_preload_ct2_model` / `test_amd_auto_uses_vulkan...` conforme o construtor atual.

- [ ] **Step 3: Alterar o construtor em `transcriber.py`**

Trocar o import e a linha do `self._vulkan`:

```python
from backends.ct2 import CT2Backend
from backends.whispercpp import WhisperCppBackend, find_whispercli
```
```python
        self._ct2 = ct2 or CT2Backend(model_name, compute_type)
        self._vulkan = vulkan if vulkan is not None else WhisperCppBackend(model_name, find_whispercli())
        if not self._vulkan.available:
            logging.info("whisper-cli not found; Vulkan backend disabled (GPU non-NVIDIA falls back to CPU)")
```

Nada mais muda: `_chain`, `_run` e `transcribe` da Task 3 já tratam `"vulkan"`. O `logging.info` roda uma vez, no boot — é o aviso único que o spec pede para o caso típico do `wails dev` sem `vendor/whispercpp`.

- [ ] **Step 4: Rodar a suíte inteira**

```powershell
cd audio-service; .venv\Scripts\python.exe -m pytest -q
```
Expected: tudo verde (≈ 83 passed), em menos de 5s.

- [ ] **Step 5: Commit**

```powershell
git add audio-service/transcriber.py audio-service/tests/test_transcriber.py
git commit -m "feat: Vulkan na cadeia de devices do Transcriber, com força por env e fallback por chamada"
```

---

### Task 6: API do audio-service — `/health` com vendor/backend/modelo e `/transcribe` com `gpu`

**Files:**
- Modify: `audio-service/main.py:43-55` (health) e `:96` (transcribe)
- Test: `audio-service/tests/test_main.py`

**Interfaces:**
- Consumes: `transcriber.gpu_vendor`, `gpu_backend`, `vulkan_model_ready` (Task 3/5).
- Produces: contrato JSON da spec (seção "Contrato da API").

- [ ] **Step 1: Atualizar os testes**

Em `tests/test_main.py`, na fixture `mock_transcriber`, acrescentar após `m.gpu_vram_mb = None`:
```python
    m.gpu_vendor = None
    m.gpu_backend = None
    m.vulkan_model_ready = False
```
Em `test_health_idle`, acrescentar ao dict esperado:
```python
        "gpu_vendor": None,
        "gpu_backend": None,
        "vulkan_model_ready": False,
```
Adicionar ao fim do arquivo:
```python
def test_health_reports_vulkan_backend(mock_recorder, mock_transcriber, client):
    mock_transcriber.gpu_available = True
    mock_transcriber.gpu_name = "AMD Radeon RX 7600"
    mock_transcriber.gpu_vram_mb = 8192
    mock_transcriber.gpu_vendor = "amd"
    mock_transcriber.gpu_backend = "vulkan"
    mock_transcriber.vulkan_model_ready = True
    mock_transcriber.device = "vulkan"
    body = client.get("/health").json()
    assert body["gpu_vendor"] == "amd"
    assert body["gpu_backend"] == "vulkan"
    assert body["vulkan_model_ready"] is True
    assert body["device"] == "vulkan"


def test_transcribe_accepts_gpu_device(mock_recorder, mock_transcriber, client):
    mock_transcriber.transcribe.return_value = TranscribeResult(
        transcript="oi", language="pt", duration_seconds=1.0, model="medium", device="vulkan"
    )
    r = client.post("/transcribe", json={"path": "tmp/rec.wav", "device": "gpu"})
    assert r.status_code == 200
    assert r.json()["device"] == "vulkan"
    args, kwargs = mock_transcriber.transcribe.call_args
    assert kwargs.get("device") == "gpu"
```
Conferir também `test_health_includes_gpu_scan_fields` (linha 64): se ele compara o dict inteiro, acrescentar os três campos.

- [ ] **Step 2: Rodar e ver falhar**

```powershell
cd audio-service; .venv\Scripts\python.exe -m pytest tests/test_main.py -q
```
Expected: FAIL nos testes de health (chaves ausentes).

- [ ] **Step 3: Implementar em `main.py`** — no `health()`, após `"gpu_vram_mb": transcriber.gpu_vram_mb,`:
```python
        "gpu_vendor": transcriber.gpu_vendor,
        "gpu_backend": transcriber.gpu_backend,
        "vulkan_model_ready": transcriber.vulkan_model_ready,
```
`/transcribe` não precisa mudar: `device` é `Optional[str]` livre e o Transcriber já entende `gpu`.

- [ ] **Step 4: Rodar a suíte inteira**

```powershell
cd audio-service; .venv\Scripts\python.exe -m pytest -q
```
Expected: tudo verde.

- [ ] **Step 5: Commit**

```powershell
git add audio-service/main.py audio-service/tests/test_main.py
git commit -m "feat: /health expõe vendor, backend e modelo Vulkan; /transcribe aceita device=gpu"
```

---

### Task 7: Go — client, whitelist `auto|gpu|cpu`, migration 020 e health mirrors

**Files:**
- Modify: `internal/audio/client.go:20-30`
- Modify: `internal/audio/client_test.go:159-172`
- Modify: `internal/services/settings_service.go:17`
- Modify: `internal/services/settings_service_test.go:118-128`
- Create: `internal/database/migrations/020_whisper_device_gpu.sql`
- Create: `internal/database/migration_020_test.go`
- Modify: `cmd/desktop/app.go:143-158`
- Modify: `cmd/api/main.go:95-110`

**Interfaces:**
- Produces: `HealthResponse.GPUVendor string`, `GPUBackend string`, `VulkanModelReady bool`; JSON do `/health` do Go com `gpu_vendor`, `gpu_backend`, `vulkan_model_ready`.

- [ ] **Step 1: Testes primeiro**

`internal/audio/client_test.go` — substituir `TestHealth_ParsesGPUFields`:
```go
func TestHealth_ParsesGPUFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","state":"idle","loopback_available":true,"model_loaded":true,"model_name":"medium","device":"vulkan","gpu_available":true,"gpu_name":"AMD Radeon RX 7600","gpu_vram_mb":8192,"gpu_vendor":"amd","gpu_backend":"vulkan","vulkan_model_ready":true}`))
	}))
	defer srv.Close()
	c := audio.NewHTTPClient(srv.URL)
	h, err := c.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !h.GPUAvailable || h.GPUName != "AMD Radeon RX 7600" || h.GPUVRAMMB != 8192 {
		t.Fatalf("gpu fields: %+v", h)
	}
	if h.GPUVendor != "amd" || h.GPUBackend != "vulkan" || !h.VulkanModelReady || h.Device != "vulkan" {
		t.Fatalf("vulkan fields: %+v", h)
	}
}

func TestHealth_MissingVulkanFieldsAreZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","gpu_available":true,"gpu_vendor":null,"gpu_backend":null}`))
	}))
	defer srv.Close()
	h, err := audio.NewHTTPClient(srv.URL).Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h.GPUVendor != "" || h.GPUBackend != "" || h.VulkanModelReady {
		t.Fatalf("expected zero values, got %+v", h)
	}
}
```

`internal/services/settings_service_test.go` — substituir `TestSettingsService_Update_WhisperDeviceValidValues`:
```go
func TestSettingsService_Update_WhisperDeviceValidValues(t *testing.T) {
	svc := newSettingsSvc(t)
	for _, v := range []string{"auto", "gpu", "cpu"} {
		if err := svc.Update(context.Background(), map[string]string{"whisper_device": v}); err != nil {
			t.Fatalf("%s: %v", v, err)
		}
	}
	for _, v := range []string{"cuda", "vulkan", "tpu"} {
		if err := svc.Update(context.Background(), map[string]string{"whisper_device": v}); err == nil {
			t.Fatalf("%s deveria ser rejeitado: o backend é escolha interna, a UI só conhece auto/gpu/cpu", v)
		}
	}
}
```

`internal/database/migration_020_test.go`:
```go
package database

import "testing"

func TestMigration020_RewritesCudaToGpu(t *testing.T) {
	db, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// No Open a 019 semeia 'auto' e a 020 roda sobre isso; para testar a
	// conversão é preciso recriar o estado da v2.9.0 e reexecutar a 020 —
	// ela é um UPDATE idempotente, então isso é legítimo.
	if _, err := db.Exec(`UPDATE settings SET value = 'cuda' WHERE key = 'whisper_device'`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stmt, err := migrationsFS.ReadFile("migrations/020_whisper_device_gpu.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := db.Exec(string(stmt)); err != nil {
		t.Fatalf("exec migration: %v", err)
	}

	var got string
	if err := db.QueryRow(`SELECT value FROM settings WHERE key = 'whisper_device'`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != "gpu" {
		t.Errorf("whisper_device = %q, want gpu", got)
	}

	for _, keep := range []string{"auto", "cpu", "gpu"} {
		if _, err := db.Exec(`UPDATE settings SET value = ? WHERE key = 'whisper_device'`, keep); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(stmt)); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT value FROM settings WHERE key = 'whisper_device'`).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != keep {
			t.Errorf("valor %q deveria ser preservado, got %q", keep, got)
		}
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

```powershell
go test ./internal/audio/ ./internal/services/ ./internal/database/
```
Expected: `internal/audio` não compila (`h.GPUVendor undefined`); `services` falha (`cuda` aceito); `database` falha (`open migrations/020...: file does not exist`).

- [ ] **Step 3: Implementar**

`internal/audio/client.go` — `HealthResponse`:
```go
type HealthResponse struct {
	Status            string `json:"status"`
	State             string `json:"state"`
	LoopbackAvailable bool   `json:"loopback_available"`
	ModelLoaded       bool   `json:"model_loaded"`
	ModelName         string `json:"model_name"`
	Device            string `json:"device"`
	GPUAvailable      bool   `json:"gpu_available"`
	GPUName           string `json:"gpu_name"`
	GPUVRAMMB         int    `json:"gpu_vram_mb"`
	GPUVendor         string `json:"gpu_vendor"`
	GPUBackend        string `json:"gpu_backend"`
	VulkanModelReady  bool   `json:"vulkan_model_ready"`
}
```

`internal/services/settings_service.go:17`:
```go
	"whisper_device":        validateEnum("auto", "gpu", "cpu"),
```

`internal/database/migrations/020_whisper_device_gpu.sql`:
```sql
-- 020_whisper_device_gpu.sql
-- O seletor deixa de expor o backend: "cuda" vira "gpu" (o audio-service escolhe
-- CUDA ou Vulkan conforme a placa). auto/cpu ficam como estão. Reversível.
UPDATE settings SET value = 'gpu' WHERE key = 'whisper_device' AND value = 'cuda';
```

`cmd/desktop/app.go` (health mirror) — o bloco inteiro passa a ser:
```go
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"status":             "ok",
			"model_loaded":       false,
			"gpu_available":      false,
			"gpu_name":           nil,
			"gpu_vram_mb":        nil,
			"gpu_vendor":         nil,
			"gpu_backend":        nil,
			"vulkan_model_ready": false,
			"device":             "",
		}
		if h, err := audioClient.Health(r.Context()); err == nil {
			resp["model_loaded"] = h.ModelLoaded
			resp["gpu_available"] = h.GPUAvailable
			resp["gpu_name"] = h.GPUName
			resp["gpu_vram_mb"] = h.GPUVRAMMB
			resp["gpu_vendor"] = h.GPUVendor
			resp["gpu_backend"] = h.GPUBackend
			resp["vulkan_model_ready"] = h.VulkanModelReady
			resp["device"] = h.Device
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	})
```

`cmd/api/main.go` — mesmas seis linhas (três no mapa inicial, três no `if`), preservando `"database": dbStatus` e o `statusCode` que já existem lá.

- [ ] **Step 4: Rodar tudo**

```powershell
gofmt -l ./internal ./cmd; go vet ./... ; go build ./... ; go test ./...
```
Expected: `gofmt -l` sem saída; vet/build limpos; todos os pacotes `ok`.

- [ ] **Step 5: Commit**

```powershell
git add internal/audio internal/services internal/database cmd/desktop/app.go cmd/api/main.go
git commit -m "feat: setting whisper_device auto/gpu/cpu (migration 020) e health do Go com vendor/backend Vulkan"
```

---

### Task 8: Frontend — seletor `gpu`, status com backend e nota do download

**Files:**
- Modify: `frontend/src/hooks/useAudioHealth.ts:4-11`
- Modify: `frontend/src/components/settings/SettingsModal.tsx:38-42`, `:166-170`, `:397-421`

**Interfaces:**
- Consumes: `/health` do Go (Task 7): `gpu_vendor`, `gpu_backend`, `vulkan_model_ready`, `device ∈ cuda|vulkan|cpu`.

- [ ] **Step 1: Tipos** — `useAudioHealth.ts`:
```ts
export interface AudioHealth {
  status: string
  model_loaded: boolean
  gpu_available: boolean
  gpu_name: string | null
  gpu_vram_mb: number | null
  gpu_vendor: "nvidia" | "amd" | "intel" | "other" | null
  gpu_backend: "cuda" | "vulkan" | null
  vulkan_model_ready: boolean
  device: string
}
```

- [ ] **Step 2: Seletor** — `SettingsModal.tsx:38-42`:
```ts
const WHISPER_DEVICES = [
  { value: "auto", label: "Auto (recomendado)" },
  { value: "gpu",  label: "GPU" },
  { value: "cpu",  label: "CPU" },
]
```

- [ ] **Step 3: Status** — substituir as linhas 166-170 por:
```ts
  const gpuVram = audioHealth?.gpu_vram_mb
  const backendLabel = audioHealth?.gpu_backend === "cuda" ? "CUDA" : audioHealth?.gpu_backend === "vulkan" ? "Vulkan" : null
  const gpuScan = audioHealth?.gpu_available
    ? `GPU detectada: ${audioHealth.gpu_name || "GPU"}${gpuVram ? ` (${Math.round(gpuVram / 1024)} GB)` : ""}${backendLabel ? ` · ${backendLabel}` : ""}`
    : audioHealth?.gpu_name
      ? `${audioHealth.gpu_name} sem suporte de GPU — transcrição em CPU`
      : "Nenhuma GPU compatível — transcrição em CPU"
  const effectiveDevice = audioHealth?.device ?? ""
  const showGgmlDownloadNote = audioHealth?.gpu_backend === "vulkan" && !audioHealth?.vulkan_model_ready
```

- [ ] **Step 4: Bloco "Processamento"** — no JSX, após o `<p>` "Em “Auto” a GPU é usada…", acrescentar:
```tsx
                {showGgmlDownloadNote && (
                  <p className="text-[10px] text-amber-500/80 mt-1">
                    O modelo para esta GPU será baixado na primeira transcrição (~540 MB para “medium”).
                  </p>
                )}
```
e trocar a linha `Última transcrição: {effectiveDevice === "cuda" ? "GPU" : "CPU"}` por:
```tsx
                    Última transcrição: {effectiveDevice === "cpu" ? "CPU" : `GPU (${effectiveDevice === "vulkan" ? "Vulkan" : "CUDA"})`}
```

- [ ] **Step 5: Verificar**

```powershell
cd frontend; npx tsc --noEmit; npm run build
```
Expected: sem erros. Procurar resíduos: `Select-String -Path src -Pattern '"cuda"' -Recurse` deve devolver só o `effectiveDevice`/`gpu_backend` (leitura de health), nunca um valor enviado ao setting.

- [ ] **Step 6: Commit**

```powershell
git add frontend/src/hooks/useAudioHealth.ts frontend/src/components/settings/SettingsModal.tsx
git commit -m "feat: seletor Auto/GPU/CPU com backend detectado e aviso de download do modelo Vulkan"
```

---

### Task 9: Empacotamento — `.spec`, `build.ps1` e rebuild do bundle

**Files:**
- Modify: `audio-service/build/pyinstaller/audio-service.spec:49-54` (após o bloco `nvidia.*`)
- Modify: `build.ps1:107-118` (pre-flight do bundle) e após a cópia do bundle (linha ~135)

**Interfaces:**
- Consumes: `audio-service/vendor/whispercpp/whisper-cli.exe` (Task 1). `find_whispercli()` procura `sys._MEIPASS/whispercpp/whisper-cli.exe` (Task 4) — o destino no spec **tem** que ser `whispercpp`.

- [ ] **Step 1: `.spec`** — inserir depois do loop `for pkg in ("nvidia.cudnn", "nvidia.cublas")`:

```python
# whisper.cpp (Vulkan) é o segundo motor de inferência (GPU não-NVIDIA) — ver
# DECISIONS 2026-08-30. O binário e as DLLs ggml vão para _internal/whispercpp/,
# onde backends/whispercpp.find_whispercli() procura via sys._MEIPASS.
WHISPERCPP_DIR = AUDIO_SERVICE_ROOT / "vendor" / "whispercpp"
if not (WHISPERCPP_DIR / "whisper-cli.exe").exists():
    raise SystemExit(
        f"whisper-cli.exe ausente em {WHISPERCPP_DIR} — rode audio-service\\build\\fetch-whispercpp.ps1"
    )
for entry in WHISPERCPP_DIR.iterdir():
    if entry.suffix.lower() in (".exe", ".dll"):
        binaries.append((str(entry), "whispercpp"))
```

- [ ] **Step 2: `build.ps1`** — dentro de `if (-not $NoNSIS) {` do pre-flight (junto ao check de `$AudioServiceSrc`), acrescentar:

```powershell
    $WhisperCli = Join-Path $ProjectRoot "audio-service\vendor\whispercpp\whisper-cli.exe"
    if (-not (Test-Path $WhisperCli)) {
        Write-Fail "whisper-cli.exe (Vulkan) não encontrado em: $WhisperCli"
        Write-Host "  Obtenha com:  .\audio-service\build\fetch-whispercpp.ps1" -ForegroundColor Yellow
        exit 1
    }
    $BundledCli = Join-Path $AudioServiceSrc "_internal\whispercpp\whisper-cli.exe"
    if (-not (Test-Path $BundledCli)) {
        Write-Fail "O bundle do audio-service não contém _internal\whispercpp\whisper-cli.exe — está desatualizado."
        Write-Host "  Rebuild com PyInstaller (Python do .venv):" -ForegroundColor Yellow
        Write-Host "    cd audio-service" -ForegroundColor Yellow
        Write-Host "    .venv\Scripts\python.exe -m PyInstaller build\pyinstaller\audio-service.spec --distpath build\dist --workpath build\work --noconfirm" -ForegroundColor Yellow
        exit 1
    }
```

- [ ] **Step 3: Rebuild do bundle (demorado, ~10 min)**

```powershell
cd audio-service
.venv\Scripts\python.exe -m PyInstaller build\pyinstaller\audio-service.spec --distpath build\dist --workpath build\work --noconfirm
Get-ChildItem build\dist\audio-service\_internal\whispercpp
```
Expected: `whisper-cli.exe` e DLLs `ggml*.dll`/`whisper.dll` presentes.

- [ ] **Step 4: Smoke local do bundle com Vulkan**

```powershell
$env:RECORDINGS_DIR = "<dir com um wav real>"; $env:WHISPER_FORCE_BACKEND = "vulkan"
Start-Process .\build\dist\audio-service\audio-service.exe -ArgumentList "--port","8794" -PassThru
# aguardar ~40s (load do modelo CT2 em CPU/CUDA no boot)
Invoke-RestMethod http://127.0.0.1:8794/health | ConvertTo-Json
```
Expected: `gpu_backend: "cuda"` (nesta máquina CUDA vence no scan; a força só afeta a cadeia) e `gpu_available: true`, `gpu_vendor: "nvidia"`, `gpu_name` da RTX 2050. Em seguida:
```powershell
Invoke-RestMethod -Method Post http://127.0.0.1:8794/transcribe -ContentType "application/json" -Body '{"path":"<wav>","device":"gpu"}' | ConvertTo-Json
```
Expected: HTTP 200, `device: "vulkan"`, transcript não vazio. Matar o processo ao final (`Stop-Process`). Remover as variáveis de ambiente da sessão.

- [ ] **Step 5: Commit** (bundle e vendor são gitignorados)

```powershell
git add audio-service/build/pyinstaller/audio-service.spec build.ps1
git commit -m "build: whisper-cli (Vulkan) no bundle do audio-service e pre-flight no build.ps1"
```

---

### Task 10: Homologação no app, documentação e registro (INTERATIVO, sessão principal)

**Files:**
- Modify: `.claude/DECISIONS.md` (nova entrada no topo)
- Modify: `.claude/BACKLOG.md`
- Modify: `CLAUDE.md` (seção "Build do installer")

- [ ] **Step 1: Homologação na janela nativa** — parar qualquer `wails dev` (SingleInstanceLock), então:

```powershell
$env:WHISPER_FORCE_BACKEND = "vulkan"
cd cmd\desktop; wails dev
```
Nas Configurações: status deve mostrar "GPU detectada: NVIDIA GeForce RTX 2050 (4 GB) · CUDA" (o scan não muda com a força). Seletor em "GPU". Gravar ~30s (ou reprocessar uma reunião existente) e conferir em "Última transcrição": **GPU (Vulkan)**. Fechar, remover a variável (`Remove-Item Env:WHISPER_FORCE_BACKEND`), subir de novo, reprocessar: **GPU (CUDA)**. Conferir que o setting salvo persiste como `gpu` (`SELECT value FROM settings WHERE key='whisper_device'` no banco de dev) e que um banco com `cuda` foi convertido (migration 020) — simular: `UPDATE settings SET value='cuda' WHERE key='whisper_device'`, reiniciar o app **não** reaplica a migration (já registrada); então a conversão só é verificável pelo teste Go da Task 7 — aceito.

- [ ] **Step 2: DECISIONS.md** — entrada no topo:

```markdown
## [2026-08-30] GPU não-NVIDIA via whisper.cpp/Vulkan como segundo motor; CUDA permanece o caminho NVIDIA

**Contexto:** A v2.9.0 só transcreve em GPU NVIDIA — faster-whisper/ctranslate2 não tem backend AMD no
Windows (ROCm é Linux-only, sem wheel). Pedido do usuário: suporte a placas AMD.

**Alternativas:** (a) um motor só, whisper.cpp/Vulkan para todas as GPUs — instalador cai de 631 para
~150 MB, mas descarta a homologação CUDA e perde 1,5–3× em NVIDIA; (b) ONNX Runtime + DirectML — sem
pipeline Whisper pronto em Python, semanas de plumbing; (c) ROCm/torch-directml — imaturo no Windows.

**Escolha:** Dois motores lado a lado. `backends/ct2.py` (faster-whisper) segue para CUDA e CPU;
`backends/whispercpp.py` roda o binário `whisper-cli` (Vulkan) via subprocess para AMD/Intel e como
fallback quando CUDA falha. Resolução por chamada em cadeia (`cuda → vulkan → cpu`), sem estado
pegajoso. Backend é escolha interna: o setting `whisper_device` passa a `auto|gpu|cpu` (migration 020
converte `cuda→gpu`). Modelo GGML quantizado (q5) baixado sob demanda do HF, não embarcado.
Binário whisper.cpp pinado em `audio-service/build/whispercpp.version`, obtido por
`fetch-whispercpp.ps1`, embarcado em `_internal/whispercpp/` (+20–40 MB no instalador).

**Trade-offs aceitos:** dois formatos de modelo (quem usa Vulkan baixa um segundo `medium`, ~540 MB);
subprocess em vez de wheel (isolamento de crash de driver vale o custo de um processo por
transcrição); homologação em hardware AMD **em aberto** — validado nesta máquina forçando Vulkan na
RTX 2050 (`WHISPER_FORCE_BACKEND=vulkan`), decisão consciente do usuário sem máquina AMD disponível.

---
```

- [ ] **Step 3: BACKLOG.md** — em "Débitos técnicos", acrescentar:
```markdown
- **Vulkan não homologado em GPU AMD real** — a feature de 2026-08-30 foi validada forçando Vulkan na RTX 2050. Primeira máquina AMD disponível: instalar, conferir `gpu_vendor: amd`, `gpu_backend: vulkan`, transcrever e comparar tempo com CPU. Se falhar, o fallback por chamada cai em CPU — não trava a reunião.
- **Modelo GGML baixado sob demanda na primeira transcrição Vulkan** — sem barra de progresso; ~540 MB (medium). Se incomodar, pré-baixar ao salvar o seletor em GPU ou mostrar progresso via `/health`.
```
Em "Features futuras", nada a remover (AMD não estava listado).

- [ ] **Step 4: CLAUDE.md** — na seção "Build do installer (Windows)", após "Pré-requisitos: bundle do audio-service…", acrescentar:
```markdown
Segundo pré-requisito desde 2026-08-30: o binário do whisper.cpp (Vulkan) em `audio-service/vendor/whispercpp/` — obtido com `.\audio-service\build\fetch-whispercpp.ps1` (versão pinada em `audio-service/build/whispercpp.version`). O `.spec` e o `build.ps1` abortam sem ele.
```
E na frase "O bundle embarca CUDA desde 2026-08-29…", acrescentar ao final: "GPU não-NVIDIA usa whisper.cpp/Vulkan (DECISIONS 2026-08-30)."

- [ ] **Step 5: Commit**

```powershell
git add .claude/DECISIONS.md .claude/BACKLOG.md CLAUDE.md
git commit -m "docs: registrar o segundo motor whisper.cpp/Vulkan e os débitos de homologação AMD"
```

---

## Ordem e paralelismo

Sequência obrigatória: **1 → 2 → 3 → 4 → 5 → 6**. Depois, **7** (Go) e **8** (frontend) são independentes entre si e do Python, podem correr em paralelo. **9** depende de 1 e 4. **10** depende de tudo.

## Verificação final antes do merge (controlador)

```powershell
cd audio-service; .venv\Scripts\python.exe -m pytest -q; cd ..
gofmt -l ./internal ./cmd; go vet ./...; go test ./...
cd frontend; npx tsc --noEmit; npm run build; cd ..
git diff master --stat
```
Nenhuma mudança em `internal/services/orchestrator.go` é esperada — se aparecer, questionar.
