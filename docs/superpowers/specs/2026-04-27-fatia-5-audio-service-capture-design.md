# Spec: Fatia 5 — Audio Service (parte 1, captura)

## Objetivo

Implementar o serviço Python standalone que captura áudio simultâneo de microfone + saída do sistema (loopback WASAPI) e grava como WAV no disco. Sem Whisper — Fatia 6 integra transcrição. Comunicação será via HTTP local na porta 8765, consumido pelo backend Go na Fatia 7.

---

## Stack

- Python 3.12
- FastAPI + uvicorn (servidor HTTP)
- pyaudiowpatch 0.2.12.7 (PyAudio fork com WASAPI loopback no Windows)
- numpy + scipy (resampling, mixagem)
- soundfile (escrita incremental de WAV)

---

## File map

| Arquivo | Ação | Responsabilidade |
|---|---|---|
| `audio-service/main.py` | criar | App FastAPI, endpoints, DI do Recorder |
| `audio-service/recorder.py` | criar | Classe `Recorder` — captura, mixa, grava WAV; encapsula state machine |
| `audio-service/requirements.txt` | criar | Dependências |
| `audio-service/.env.example` | criar | `HTTP_PORT=8765`, `RECORDINGS_DIR=./tmp` |
| `audio-service/.gitignore` | criar | `.venv/`, `tmp/`, `__pycache__/`, `*.pyc` |
| `audio-service/README.md` | criar | Setup local (venv, deps, run) e testes manuais |
| `audio-service/tests/__init__.py` | criar | (vazio, marca pacote) |
| `audio-service/tests/test_main.py` | criar | Testes da state machine via `TestClient` + Recorder mockado |

---

## Endpoints

```
GET  /health           → 200 {"status":"ok","state":"idle|recording","loopback_available":true|false}
POST /recording/start  → 200 {"recording_id":"<uuid>","started_at":"<iso8601>"}
                       → 409 se já está em RECORDING
                       → 503 se loopback ou mic indisponíveis
POST /recording/stop   → 200 {"recording_id":"...","path":"./tmp/rec-<uuid>.wav","duration_seconds":180.5,"size_bytes":...,"partial":false}
                       → 409 se está em IDLE
                       → 500 em erro de escrita de WAV
GET  /recording/status → 200 {"state":"idle|recording","recording_id":null|"<uuid>","started_at":null|"<iso8601>"}
```

**Path no `/stop`:** absoluto ou relativo ao CWD do serviço. Implementação retorna o que `pathlib.Path` resolveu.

**`partial: true`:** indicado quando houve erro irrecuperável durante a gravação mas o WAV foi finalizado parcialmente.

---

## State machine

```
        POST /start
IDLE ───────────────► RECORDING
                          │
                          │ POST /stop
                          ▼
                        IDLE
```

- `POST /start` em RECORDING → 409
- `POST /stop` em IDLE → 409
- Transições protegidas por `threading.Lock`
- Apenas uma gravação por vez (desktop app, single user)

---

## Recorder — comportamento

### Inicialização

`Recorder.__init__(recordings_dir: Path)`:
- Inicializa `pyaudio.PyAudio()`
- Enumera devices: encontra mic default e loopback do speaker default
- Cacheia `loopback_available: bool` e `mic_available: bool`
- **Não abre streams ainda** (lazy — só em `start()`)

### `Recorder.start() -> str`

- Levanta `RecorderError` se já está RECORDING
- Levanta `RecorderError("loopback unavailable")` ou similar se devices não disponíveis
- Gera `recording_id = uuid4()`
- Abre dois streams via `pyaudio.PyAudio.open(...)` em modo callback:
  - Mic: input, default device, sample rate nativa do device, mono
  - Loopback: input, loopback device, sample rate nativa, stereo (será reduzido a mono no mixer)
- Cada callback empilha frames numa `queue.Queue` própria
- Spawn thread mixer:
  - Lê chunks de ambas as filas com timeout
  - Resample cada chunk para 16 kHz mono (`scipy.signal.resample_poly`)
  - Soma sample-a-sample, `np.clip(-1.0, 1.0)`
  - Escreve no `soundfile.SoundFile` aberto em modo `'w'` (PCM_16, 16000 Hz, mono)
- Atualiza state para RECORDING
- Retorna `recording_id`

### `Recorder.stop() -> StopResult`

- Levanta `RecorderError` se está IDLE
- Sinaliza thread mixer para parar (via `threading.Event`)
- Aguarda mixer drenar filas e fechar `SoundFile`
- Fecha streams pyaudio
- Calcula `duration_seconds` (frames escritos / 16000) e `size_bytes` (`os.stat`)
- Atualiza state para IDLE
- Retorna `StopResult(path, duration_seconds, size_bytes, partial)`

### `Recorder.status() -> dict`

- Snapshot thread-safe do estado: `state`, `recording_id`, `started_at`, `loopback_available`, `mic_available`

### Tratamento de erros mid-recording

- Overflow nas filas: log warning, descarta frames mais antigos para evitar memory leak
- Stream callback retorna erro: log, marca `_partial = True`, deixa mixer drenar o que tem
- Erro escrevendo WAV: log critical, sinaliza para `/stop` retornar 500

---

## main.py — estrutura

```python
import os, threading
from pathlib import Path
from fastapi import FastAPI, HTTPException
from recorder import Recorder, RecorderError

RECORDINGS_DIR = Path(os.getenv("RECORDINGS_DIR", "./tmp"))
HTTP_PORT = int(os.getenv("HTTP_PORT", "8765"))

app = FastAPI()
recorder = Recorder(RECORDINGS_DIR)

@app.get("/health")
def health():
    return {"status": "ok", "state": recorder.state, "loopback_available": recorder.loopback_available}

@app.post("/recording/start")
def start():
    try:
        rec_id, started_at = recorder.start()
    except RecorderError as e:
        raise HTTPException(status_code=409 if "already" in str(e).lower() else 503, detail=str(e))
    return {"recording_id": rec_id, "started_at": started_at.isoformat()}

@app.post("/recording/stop")
def stop():
    try:
        result = recorder.stop()
    except RecorderError as e:
        raise HTTPException(status_code=409, detail=str(e))
    return {
        "recording_id": result.recording_id,
        "path": str(result.path),
        "duration_seconds": result.duration_seconds,
        "size_bytes": result.size_bytes,
        "partial": result.partial,
    }

@app.get("/recording/status")
def status():
    return recorder.status()
```

---

## WAV format

- Sample rate: **16 000 Hz**
- Canais: **1 (mono)**
- Bit depth: **16-bit PCM**
- Container: **WAV (RIFF)**
- Filename: `rec-<recording_id>.wav` em `RECORDINGS_DIR`

---

## Testes automatizados

`tests/test_main.py` — usa `fastapi.testclient.TestClient` e mocka `Recorder`:

- `test_health_idle` — mock state=idle, loopback_available=true → `/health` retorna 200 com payload esperado
- `test_start_idle` — mock recorder.start retorna (id, datetime) → `/start` retorna 200
- `test_start_already_recording` — mock recorder.start levanta `RecorderError("already recording")` → `/start` retorna 409
- `test_start_loopback_unavailable` — mock levanta `RecorderError("loopback unavailable")` → `/start` retorna 503
- `test_stop_recording` — mock recorder.stop retorna `StopResult(...)` → `/stop` retorna 200
- `test_stop_idle` — mock levanta `RecorderError("not recording")` → `/stop` retorna 409
- `test_status_idle` — mock retorna dict idle → `/status` retorna 200 com state=idle
- `test_status_recording` — mock retorna dict recording → `/status` retorna 200 com recording_id

`pytest` deve passar sem audio hardware.

---

## Testes manuais

Documentados no `README.md`:

1. **Setup**: `py -3.12 -m venv .venv` → `.venv\Scripts\activate` → `pip install -r requirements.txt`
2. **Run**: `uvicorn main:app --port 8765`
3. **Health check**: `curl http://localhost:8765/health` deve retornar `loopback_available: true` num Windows com WASAPI funcional
4. **Smoke test de gravação**:
   - `curl -X POST http://localhost:8765/recording/start`
   - Falar no microfone enquanto toca uma música no Spotify por 10s
   - `curl -X POST http://localhost:8765/recording/stop`
   - Abrir o WAV gerado e verificar que mic + música aparecem mixados
5. **State conflicts**: `/start` duas vezes seguidas → segunda retorna 409. `/stop` sem `/start` → 409.

---

## Critério de aceite

- [ ] `pytest` passa
- [ ] `uvicorn main:app --port 8765` sobe sem erros
- [ ] `/health` retorna 200 com `loopback_available: true` em Windows
- [ ] Smoke test produz WAV válido em `RECORDINGS_DIR`
- [ ] State conflicts retornam 409
- [ ] WAV é mono, 16kHz, PCM 16-bit (verificável via `ffprobe` ou `soundfile.info`)

---

## Decisões de design

- **Lazy stream open:** abrir os streams pyaudio só em `/start` (não no `__init__`) evita segurar handles desnecessariamente quando o serviço só está respondendo `/health`.
- **Resample no mixer (não no callback):** callbacks PyAudio devem ser rápidos; resampling pesado fica no mixer thread separado.
- **Não auto-deletar WAVs:** caller (Go orchestrator na Fatia 7) é responsável pelo cleanup. Alinha com "áudio descartado após transcrição" do roadmap mestre — o descarte é responsabilidade do caller depois que o Whisper consumir.
- **`partial: true` em vez de falhar:** preserva o que foi gravado em vez de descartar o áudio em erros transitórios. Caller decide se quer usar ou re-tentar.
- **Sem auth:** roda no localhost, app desktop single-user. Tarefa do Wails depois é amarrar tudo num mesmo processo (binding direto), HTTP é só boundary de desenvolvimento/debug.
- **Versões fixadas em requirements.txt:** evita quebras silenciosas. `pyaudiowpatch==0.2.12.7` em particular tem ABI ligada a versão do PyAudio upstream.
- **Sample rate fixa 16 kHz no output:** otimiza para Whisper (next fatia), reduz tamanho do WAV em ~3x vs 48 kHz, qualidade suficiente para fala.
