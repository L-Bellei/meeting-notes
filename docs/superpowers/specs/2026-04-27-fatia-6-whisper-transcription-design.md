# Spec: Fatia 6 — Audio Service (parte 2, transcrição Whisper)

## Objetivo

Adicionar transcrição via `faster-whisper` ao audio service. Novo endpoint `POST /transcribe` que recebe um path de WAV e retorna o texto transcrito. Modelo carregado eager no startup, GPU obrigatória (sem fallback CPU). cuDNN portátil via pip wheels (sem download manual).

---

## Stack adicional

- `faster-whisper` (Whisper otimizado via CTranslate2)
- `nvidia-cudnn-cu12` (cuDNN runtime via pip)
- `nvidia-cublas-cu12` (cuBLAS runtime via pip)

GPU NVIDIA RTX 2050 (4 GB VRAM). Modelo `medium` com `int8_float16` ocupa ~1.5 GB de VRAM.

---

## File map

| Arquivo | Ação | Responsabilidade |
|---|---|---|
| `audio-service/transcriber.py` | criar | Classe `Transcriber` — bootstrap de DLLs + carregamento eager + método `transcribe(path, language)` |
| `audio-service/main.py` | modificar | Inicializar `Transcriber` no startup; novo endpoint `/transcribe`; `/health` expõe info do modelo |
| `audio-service/requirements.txt` | modificar | Adicionar `faster-whisper`, `nvidia-cudnn-cu12`, `nvidia-cublas-cu12` |
| `audio-service/.env.example` | modificar | Adicionar `WHISPER_LANGUAGE=pt` |
| `audio-service/tests/test_transcriber.py` | criar | Testes de validação de path com `WhisperModel` mockado |
| `audio-service/tests/test_main.py` | modificar | Testes do endpoint `/transcribe` + `/health` atualizado |
| `audio-service/README.md` | modificar | Setup CUDA, novo endpoint, smoke test atualizado |

---

## Endpoints (atualizado)

```
GET  /health           → {"status":"ok",
                          "state":"idle|recording",
                          "loopback_available":true,
                          "model_loaded":true,
                          "model_name":"medium",
                          "device":"cuda"}

POST /recording/start  (sem mudança)
POST /recording/stop   (sem mudança)
GET  /recording/status (sem mudança)

POST /transcribe       body: {"path":"tmp/rec-<id>.wav","language":"pt"}  (language opcional)
                       → 200 {"transcript":"...",
                              "language":"pt",
                              "duration_seconds":11.86,
                              "model":"medium"}
                       → 400 path inválido / fora de RECORDINGS_DIR / arquivo não existe
                       → 500 erro interno do Whisper (OOM, arquivo corrompido, etc.)
```

---

## Transcriber

```python
@dataclass
class TranscribeResult:
    transcript: str
    language: str
    duration_seconds: float
    model: str


class Transcriber:
    def __init__(
        self,
        model_name: str,           # ex: "medium"
        device: str,                # "cuda"
        compute_type: str,          # "int8_float16"
        default_language: str,      # "pt"
        recordings_dir: Path,       # raiz permitida para validação de paths
    ):
        # 1. Adiciona DLLs cuDNN/cuBLAS ao DLL search path no Windows
        self._setup_dll_paths()
        # 2. Carrega WhisperModel agora (eager); exception se falhar
        from faster_whisper import WhisperModel
        self._model = WhisperModel(model_name, device=device, compute_type=compute_type)
        self.model_name = model_name
        self.device = device
        self.default_language = default_language
        self.recordings_dir = recordings_dir.resolve()
        self.model_loaded = True

    def _setup_dll_paths(self):
        # No-op em não-Windows. No Windows, faz os.add_dll_directory para nvidia.cudnn.lib e nvidia.cublas.lib
        ...

    def transcribe(self, path: Path, language: str | None = None) -> TranscribeResult:
        resolved = path.resolve()
        if not str(resolved).startswith(str(self.recordings_dir)):
            raise ValueError(f"path outside recordings dir: {path}")
        if not resolved.exists():
            raise ValueError(f"path does not exist: {path}")

        lang = language or self.default_language
        segments, info = self._model.transcribe(str(resolved), language=lang)
        text = " ".join(seg.text.strip() for seg in segments).strip()
        return TranscribeResult(
            transcript=text,
            language=info.language,
            duration_seconds=info.duration,
            model=self.model_name,
        )
```

---

## main.py — mudanças

`main.py` ganha:
- `WHISPER_MODEL`, `WHISPER_DEVICE`, `WHISPER_COMPUTE_TYPE`, `WHISPER_LANGUAGE` lidos do env (defaults: `"medium"`, `"cuda"`, `"int8_float16"`, `"pt"`)
- `transcriber = Transcriber(...)` instanciado no nível de módulo (eager). Se falhar, uvicorn não sobe.
- `/health` agora retorna `model_loaded`, `model_name`, `device` lidos do `transcriber`
- Novo handler `/transcribe`:

```python
class TranscribeRequest(BaseModel):
    path: str
    language: str | None = None


@app.post("/transcribe")
def transcribe(req: TranscribeRequest):
    try:
        result = transcriber.transcribe(Path(req.path), req.language)
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"transcription failed: {e}")
    return {
        "transcript": result.transcript,
        "language": result.language,
        "duration_seconds": result.duration_seconds,
        "model": result.model,
    }
```

---

## Bootstrap de DLLs (Windows)

```python
import os
import sys

def _setup_dll_paths(self):
    if sys.platform != "win32":
        return
    import nvidia.cudnn
    import nvidia.cublas
    cudnn_bin = Path(nvidia.cudnn.__file__).parent / "bin"
    cublas_bin = Path(nvidia.cublas.__file__).parent / "bin"
    for d in (cudnn_bin, cublas_bin):
        if d.exists():
            os.add_dll_directory(str(d))
```

Nota: o caminho exato `bin/` dentro do pacote pip pode variar entre versões. Implementador verifica e ajusta.

---

## Variáveis de ambiente

```
WHISPER_MODEL=medium
WHISPER_DEVICE=cuda
WHISPER_COMPUTE_TYPE=int8_float16
WHISPER_LANGUAGE=pt
```

(Já presentes em `internal/config/config.go`; agora também consumidas pelo Python.)

---

## HTTP status codes

| Operação | Sucesso | Erros |
|---|---|---|
| GET /health | 200 | (sempre 200 — model_loaded reflete o estado) |
| POST /transcribe | 200 | 400 (path inválido), 500 (Whisper falhou) |

---

## Testes

### `tests/test_transcriber.py`

Mocka `faster_whisper.WhisperModel` para evitar carregar modelo real:

- `test_transcribe_path_outside_recordings_dir` — `Transcriber.transcribe(Path("/etc/passwd"))` → ValueError
- `test_transcribe_path_does_not_exist` — path dentro de recordings_dir mas inexistente → ValueError
- `test_transcribe_concatenates_segments` — mock retorna `[Segment(text="oi"), Segment(text="mundo")]` + `Info(language="pt", duration=10.0)` → result tem `transcript="oi mundo"`, `language="pt"`, `duration_seconds=10.0`
- `test_transcribe_uses_default_language_when_none_provided` — `language=None` → mock recebe `language="pt"` (default)
- `test_transcribe_uses_provided_language` — `language="en"` → mock recebe `language="en"`

### `tests/test_main.py` (adicionar)

- `test_health_includes_model_info` — `mock_transcriber.model_loaded=True`, `model_name="medium"`, `device="cuda"` → `/health` retorna esses campos
- `test_transcribe_ok` — mock retorna `TranscribeResult(transcript="texto", language="pt", duration_seconds=10.0, model="medium")` → endpoint retorna 200 com payload exato
- `test_transcribe_path_invalid` — mock levanta `ValueError("path outside recordings dir")` → 400
- `test_transcribe_internal_error` — mock levanta `Exception("CUDA OOM")` → 500
- `test_transcribe_optional_language` — body sem `language` → handler chama `transcriber.transcribe(path, None)`

`Transcriber` injetado em `main.transcriber`; tests fazem `monkeypatch.setattr(main, "transcriber", mock)` igual ao Recorder.

### Smoke test manual

1. `pip install -r requirements.txt` (instala faster-whisper + nvidia wheels, ~1.5 GB)
2. `uvicorn main:app --port 8765` — primeira execução baixa modelo `medium` (~1.5 GB) em `~/.cache/huggingface/`. Subsequente: ~5s de boot.
3. `curl /health` mostra `model_loaded:true, model_name:"medium", device:"cuda"`
4. Reaproveita um WAV gerado pelo smoke test da Fatia 5 (ou faz uma nova gravação curta)
5. `curl -X POST /transcribe -H "Content-Type: application/json" -d '{"path":"tmp/rec-xxx.wav"}'` retorna texto coerente em pt-BR em ~5-10s para áudio de 12s
6. Path traversal: `'{"path":"../../etc/passwd"}'` → 400

---

## Decisões de design

- **Eager loading no startup:** falha rápida se CUDA/modelo não disponíveis; primeiro `/transcribe` rápido. Trade-off: ~30s no primeiro boot e ~1.5GB de VRAM ocupados continuamente. Aceitável para app desktop.
- **Sem fallback CPU:** GPU é o objetivo explícito do projeto (RTX 2050). CPU rodando `medium` numa reunião de 1h levaria ~1h só para transcrever — inutilizável.
- **cuDNN/cuBLAS via pip wheels:** evita download manual de tarballs NVIDIA. Tudo na venv. Custo: +1.5 GB de site-packages.
- **Endpoint separado de `/recording/stop`:** permite re-transcrever um WAV sem regravar; separa responsabilidades; cliente decide quando esperar.
- **WAV não deletado pelo serviço:** consistente com Fatia 5 — caller (Go orchestrator na Fatia 7) é responsável pelo cleanup.
- **Validação de path traversal:** path resolvido tem que estar dentro de `RECORDINGS_DIR`. Endpoint não é exposto na internet, mas validar é barato e impede acidentes.
- **Resposta sem segmentos / timestamps:** YAGNI — Anthropic só precisa do texto. Pode ser adicionado depois.
- **`info.language` na resposta:** mesmo que o caller passe `language="pt"`, o Whisper retorna o que detectou (que será "pt" no happy path; útil pra diagnóstico se algo der errado).
- **Erro genérico mapeia para 500:** OOM, arquivo corrompido, modelo travado — tudo cai no mesmo bucket. Mensagem detalhada no body. Granularidade fica para refinamento futuro.
