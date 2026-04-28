# Fatia 6 — Whisper Transcription Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `POST /transcribe` endpoint to the audio-service that transcribes WAV files via faster-whisper on the GPU. Model loaded eagerly at startup. cuDNN/cuBLAS shipped via pip wheels (no manual download).

**Architecture:** New `Transcriber` class wraps `faster_whisper.WhisperModel` with eager loading and path-traversal protection. `main.py` is refactored to use FastAPI's `lifespan` context for both `Recorder` and `Transcriber` (so module import doesn't trigger CUDA/PyAudio initialization, making tests cleaner). The new `/transcribe` endpoint validates the path, calls `Transcriber.transcribe()`, and returns the concatenated transcript.

**Tech Stack:** Python 3.12, FastAPI, faster-whisper, nvidia-cudnn-cu12, nvidia-cublas-cu12 (existing: PyAudioWPatch, numpy, scipy, soundfile).

---

## Project conventions

- Working directory: `F:\dev\meeting-notes`. Branch: `master`. Python project at `audio-service/`.
- Tests run from `audio-service/` via `python -m pytest -v`.
- All commits prefixed: `feat(audio):`, `test(audio):`, `docs(audio):`, `refactor(audio):`.
- Existing 23 tests must keep passing.

---

## Task 1: Add Whisper dependencies and env var

**Files:**
- Modify: `audio-service/requirements.txt`
- Modify: `audio-service/.env.example`

- [ ] **Step 1: Update `requirements.txt`**

Add three new lines (preserve existing entries):

```
faster-whisper==1.0.3
nvidia-cudnn-cu12==9.1.0.70
nvidia-cublas-cu12==12.4.5.8
```

The complete file should be:

```
fastapi==0.115.0
uvicorn[standard]==0.32.0
pyaudiowpatch==0.2.12.7
numpy==2.1.2
scipy==1.14.1
soundfile==0.12.1
pytest==8.3.3
httpx==0.27.2
faster-whisper==1.0.3
nvidia-cudnn-cu12==9.1.0.70
nvidia-cublas-cu12==12.4.5.8
```

- [ ] **Step 2: Update `.env.example`**

Append `WHISPER_LANGUAGE=pt` so the file becomes:

```
HTTP_PORT=8765
RECORDINGS_DIR=./tmp
WHISPER_LANGUAGE=pt
```

- [ ] **Step 3: Install the new dependencies**

```
py -3.12 -m pip install faster-whisper nvidia-cudnn-cu12 nvidia-cublas-cu12
```

This downloads ~1.5 GB of NVIDIA wheels. If pinned versions don't resolve, drop the version pins from the install command and let pip pick — but DO commit pinned versions in requirements.txt for reproducibility, adjusting if necessary.

Verify:
```
py -3.12 -c "import faster_whisper, nvidia.cudnn, nvidia.cublas; print('imports OK')"
```

Expected: `imports OK`.

- [ ] **Step 4: Commit**

```
git add audio-service/requirements.txt audio-service/.env.example
git commit -m "feat(audio): add faster-whisper and CUDA wheel dependencies"
```

---

## Task 2: Transcriber class

**Files:**
- Create: `audio-service/transcriber.py`
- Create: `audio-service/tests/test_transcriber.py`

- [ ] **Step 1: Write failing tests**

Create `audio-service/tests/test_transcriber.py`:

```python
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from transcriber import Transcriber, TranscribeResult


@pytest.fixture
def transcriber(tmp_path):
    """Build a Transcriber with WhisperModel and DLL setup mocked."""
    fake_model = MagicMock()
    with patch("transcriber.WhisperModel", return_value=fake_model), \
         patch.object(Transcriber, "_setup_dll_paths"):
        t = Transcriber(
            model_name="medium",
            device="cuda",
            compute_type="int8_float16",
            default_language="pt",
            recordings_dir=tmp_path,
        )
    t._fake_model = fake_model
    return t


def test_init_loads_model_and_sets_attributes(tmp_path):
    fake_model = MagicMock()
    with patch("transcriber.WhisperModel", return_value=fake_model) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths") as mock_setup:
        t = Transcriber("medium", "cuda", "int8_float16", "pt", tmp_path)
    mock_cls.assert_called_once_with("medium", device="cuda", compute_type="int8_float16")
    mock_setup.assert_called_once()
    assert t.model_loaded is True
    assert t.model_name == "medium"
    assert t.device == "cuda"
    assert t.default_language == "pt"


def test_transcribe_path_outside_recordings_dir_raises(transcriber, tmp_path):
    outside = tmp_path.parent / "elsewhere.wav"
    with pytest.raises(ValueError, match="outside recordings dir"):
        transcriber.transcribe(outside)


def test_transcribe_path_does_not_exist_raises(transcriber, tmp_path):
    missing = tmp_path / "missing.wav"
    with pytest.raises(ValueError, match="does not exist"):
        transcriber.transcribe(missing)


def test_transcribe_concatenates_segments(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    seg1 = MagicMock()
    seg1.text = " oi "
    seg2 = MagicMock()
    seg2.text = "mundo"
    info = MagicMock()
    info.language = "pt"
    info.duration = 10.5
    transcriber._fake_model.transcribe.return_value = (iter([seg1, seg2]), info)

    result = transcriber.transcribe(wav)

    assert isinstance(result, TranscribeResult)
    assert result.transcript == "oi mundo"
    assert result.language == "pt"
    assert result.duration_seconds == 10.5
    assert result.model == "medium"


def test_transcribe_uses_default_language_when_none_provided(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    info = MagicMock()
    info.language = "pt"
    info.duration = 5.0
    transcriber._fake_model.transcribe.return_value = (iter([]), info)

    transcriber.transcribe(wav)

    transcriber._fake_model.transcribe.assert_called_once()
    args, kwargs = transcriber._fake_model.transcribe.call_args
    assert kwargs["language"] == "pt"


def test_transcribe_uses_provided_language(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    info = MagicMock()
    info.language = "en"
    info.duration = 5.0
    transcriber._fake_model.transcribe.return_value = (iter([]), info)

    transcriber.transcribe(wav, language="en")

    args, kwargs = transcriber._fake_model.transcribe.call_args
    assert kwargs["language"] == "en"


def test_setup_dll_paths_noop_on_non_windows(tmp_path, monkeypatch):
    """The DLL setup should be a no-op on non-Windows platforms."""
    monkeypatch.setattr(sys, "platform", "linux")
    fake_model = MagicMock()
    with patch("transcriber.WhisperModel", return_value=fake_model):
        # Should not raise even without nvidia.cudnn / nvidia.cublas paths existing.
        t = Transcriber("medium", "cuda", "int8_float16", "pt", tmp_path)
    assert t.model_loaded is True
```

- [ ] **Step 2: Run tests to verify they fail**

```
cd audio-service
python -m pytest tests/test_transcriber.py -v
```

Expected: ImportError — `transcriber` module not found.

- [ ] **Step 3: Implement `transcriber.py`**

Create `audio-service/transcriber.py`:

```python
import os
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Optional

from faster_whisper import WhisperModel


@dataclass
class TranscribeResult:
    transcript: str
    language: str
    duration_seconds: float
    model: str


class Transcriber:
    def __init__(
        self,
        model_name: str,
        device: str,
        compute_type: str,
        default_language: str,
        recordings_dir: Path,
    ):
        self.model_name = model_name
        self.device = device
        self.default_language = default_language
        self.recordings_dir = Path(recordings_dir).resolve()
        self._setup_dll_paths()
        self._model = WhisperModel(model_name, device=device, compute_type=compute_type)
        self.model_loaded = True

    def _setup_dll_paths(self):
        if sys.platform != "win32":
            return
        for pkg in ("nvidia.cudnn", "nvidia.cublas"):
            try:
                module = __import__(pkg, fromlist=[""])
            except ImportError:
                continue
            base = Path(module.__file__).parent
            for candidate in (base / "bin", base / "lib"):
                if candidate.exists():
                    os.add_dll_directory(str(candidate))

    def transcribe(self, path: Path, language: Optional[str] = None) -> TranscribeResult:
        resolved = Path(path).resolve()
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

- [ ] **Step 4: Run tests to verify they pass**

```
python -m pytest tests/test_transcriber.py -v
```

Expected: 7 passing tests.

- [ ] **Step 5: Run full suite**

```
python -m pytest -v
```

Expected: 30 passing tests (23 existing + 7 new).

- [ ] **Step 6: Commit**

```
git add audio-service/transcriber.py audio-service/tests/test_transcriber.py
git commit -m "feat(audio): add Transcriber wrapping faster-whisper with path validation"
```

---

## Task 3: main.py — lifespan refactor and `/transcribe` endpoint

**Goal:** Refactor `main.py` to use FastAPI's `lifespan` for both `Recorder` and `Transcriber` so the module imports cleanly without requiring CUDA at import time. Add the `/transcribe` endpoint and update `/health` with model info. Add 5 endpoint tests.

**Files:**
- Modify: `audio-service/main.py`
- Modify: `audio-service/tests/test_main.py`

- [ ] **Step 1: Write the failing tests**

Append to `audio-service/tests/test_main.py` (and update existing fixtures). The complete updated file:

```python
from datetime import datetime, timezone
from pathlib import Path
from unittest.mock import MagicMock

import pytest
from fastapi.testclient import TestClient

import main
from recorder import RecorderError, StopResult
from transcriber import TranscribeResult


@pytest.fixture
def mock_recorder(monkeypatch):
    m = MagicMock()
    m.state = "idle"
    m.loopback_available = True
    m.status.return_value = {"state": "idle", "recording_id": None, "started_at": None}
    monkeypatch.setattr(main, "recorder", m)
    return m


@pytest.fixture
def mock_transcriber(monkeypatch):
    m = MagicMock()
    m.model_loaded = True
    m.model_name = "medium"
    m.device = "cuda"
    monkeypatch.setattr(main, "transcriber", m)
    return m


@pytest.fixture
def client():
    return TestClient(main.app)


def test_health_idle(mock_recorder, mock_transcriber, client):
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json() == {
        "status": "ok",
        "state": "idle",
        "loopback_available": True,
        "model_loaded": True,
        "model_name": "medium",
        "device": "cuda",
    }


def test_health_loopback_unavailable(mock_recorder, mock_transcriber, client):
    mock_recorder.loopback_available = False
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json()["loopback_available"] is False


def test_start_idle(mock_recorder, mock_transcriber, client):
    started = datetime(2026, 4, 27, 12, 0, 0, tzinfo=timezone.utc)
    mock_recorder.start.return_value = ("abc-123", started)
    r = client.post("/recording/start")
    assert r.status_code == 200
    assert r.json() == {"recording_id": "abc-123", "started_at": "2026-04-27T12:00:00+00:00"}


def test_start_already_recording(mock_recorder, mock_transcriber, client):
    mock_recorder.start.side_effect = RecorderError("already recording")
    r = client.post("/recording/start")
    assert r.status_code == 409
    assert "already recording" in r.json()["detail"]


def test_start_loopback_unavailable(mock_recorder, mock_transcriber, client):
    mock_recorder.start.side_effect = RecorderError("loopback unavailable")
    r = client.post("/recording/start")
    assert r.status_code == 503
    assert "loopback unavailable" in r.json()["detail"]


def test_start_mic_unavailable(mock_recorder, mock_transcriber, client):
    mock_recorder.start.side_effect = RecorderError("mic unavailable")
    r = client.post("/recording/start")
    assert r.status_code == 503


def test_stop_recording(mock_recorder, mock_transcriber, client):
    mock_recorder.stop.return_value = StopResult(
        recording_id="abc-123",
        path=Path("./tmp/rec-abc-123.wav"),
        duration_seconds=180.5,
        size_bytes=12345,
        partial=False,
    )
    r = client.post("/recording/stop")
    assert r.status_code == 200
    body = r.json()
    assert body["recording_id"] == "abc-123"
    assert body["duration_seconds"] == 180.5
    assert body["size_bytes"] == 12345
    assert body["partial"] is False
    assert "rec-abc-123.wav" in body["path"]


def test_stop_idle(mock_recorder, mock_transcriber, client):
    mock_recorder.stop.side_effect = RecorderError("not recording")
    r = client.post("/recording/stop")
    assert r.status_code == 409
    assert "not recording" in r.json()["detail"]


def test_status_idle(mock_recorder, mock_transcriber, client):
    r = client.get("/recording/status")
    assert r.status_code == 200
    assert r.json() == {"state": "idle", "recording_id": None, "started_at": None}


def test_status_recording(mock_recorder, mock_transcriber, client):
    mock_recorder.status.return_value = {
        "state": "recording",
        "recording_id": "abc-123",
        "started_at": "2026-04-27T12:00:00+00:00",
    }
    r = client.get("/recording/status")
    assert r.status_code == 200
    assert r.json() == {
        "state": "recording",
        "recording_id": "abc-123",
        "started_at": "2026-04-27T12:00:00+00:00",
    }


def test_transcribe_ok(mock_recorder, mock_transcriber, client):
    mock_transcriber.transcribe.return_value = TranscribeResult(
        transcript="texto transcrito",
        language="pt",
        duration_seconds=10.5,
        model="medium",
    )
    r = client.post("/transcribe", json={"path": "tmp/rec-abc.wav"})
    assert r.status_code == 200
    assert r.json() == {
        "transcript": "texto transcrito",
        "language": "pt",
        "duration_seconds": 10.5,
        "model": "medium",
    }
    args, kwargs = mock_transcriber.transcribe.call_args
    assert str(args[0]) == "tmp/rec-abc.wav" or str(args[0]).endswith("rec-abc.wav")
    assert args[1] is None


def test_transcribe_path_invalid(mock_recorder, mock_transcriber, client):
    mock_transcriber.transcribe.side_effect = ValueError("path outside recordings dir")
    r = client.post("/transcribe", json={"path": "../etc/passwd"})
    assert r.status_code == 400
    assert "outside recordings dir" in r.json()["detail"]


def test_transcribe_internal_error(mock_recorder, mock_transcriber, client):
    mock_transcriber.transcribe.side_effect = RuntimeError("CUDA OOM")
    r = client.post("/transcribe", json={"path": "tmp/rec-abc.wav"})
    assert r.status_code == 500
    assert "CUDA OOM" in r.json()["detail"]


def test_transcribe_optional_language(mock_recorder, mock_transcriber, client):
    mock_transcriber.transcribe.return_value = TranscribeResult(
        transcript="x", language="en", duration_seconds=1.0, model="medium"
    )
    r = client.post("/transcribe", json={"path": "tmp/rec.wav", "language": "en"})
    assert r.status_code == 200
    args, kwargs = mock_transcriber.transcribe.call_args
    assert args[1] == "en"


def test_transcribe_path_required(mock_recorder, mock_transcriber, client):
    r = client.post("/transcribe", json={})
    assert r.status_code == 422
```

- [ ] **Step 2: Run tests to verify they fail**

```
python -m pytest tests/test_main.py -v
```

Expected: many failures — `main.transcriber` doesn't exist, `/transcribe` endpoint not defined, `/health` doesn't include model info.

- [ ] **Step 3: Refactor `main.py` with lifespan and add `/transcribe`**

Replace `audio-service/main.py` with:

```python
import os
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from recorder import Recorder, RecorderError
from transcriber import Transcriber

RECORDINGS_DIR = Path(os.getenv("RECORDINGS_DIR", "./tmp")).resolve()
WHISPER_MODEL = os.getenv("WHISPER_MODEL", "medium")
WHISPER_DEVICE = os.getenv("WHISPER_DEVICE", "cuda")
WHISPER_COMPUTE_TYPE = os.getenv("WHISPER_COMPUTE_TYPE", "int8_float16")
WHISPER_LANGUAGE = os.getenv("WHISPER_LANGUAGE", "pt")

recorder: Optional[Recorder] = None
transcriber: Optional[Transcriber] = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global recorder, transcriber
    recorder = Recorder(RECORDINGS_DIR)
    transcriber = Transcriber(
        model_name=WHISPER_MODEL,
        device=WHISPER_DEVICE,
        compute_type=WHISPER_COMPUTE_TYPE,
        default_language=WHISPER_LANGUAGE,
        recordings_dir=RECORDINGS_DIR,
    )
    yield


app = FastAPI(lifespan=lifespan)


class TranscribeRequest(BaseModel):
    path: str
    language: Optional[str] = None


@app.get("/health")
def health():
    return {
        "status": "ok",
        "state": recorder.state,
        "loopback_available": recorder.loopback_available,
        "model_loaded": transcriber.model_loaded,
        "model_name": transcriber.model_name,
        "device": transcriber.device,
    }


@app.post("/recording/start")
def start_recording():
    try:
        rec_id, started_at = recorder.start()
    except RecorderError as e:
        msg = str(e)
        if "already recording" in msg:
            raise HTTPException(status_code=409, detail=msg)
        raise HTTPException(status_code=503, detail=msg)
    return {"recording_id": rec_id, "started_at": started_at.isoformat()}


@app.post("/recording/stop")
def stop_recording():
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


@app.post("/transcribe")
def transcribe(req: TranscribeRequest):
    try:
        result = transcriber.transcribe(Path(req.path), req.language)
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
    return {
        "transcript": result.transcript,
        "language": result.language,
        "duration_seconds": result.duration_seconds,
        "model": result.model,
    }
```

**Important notes about the lifespan refactor:**
- `recorder` and `transcriber` are now module-level `None` until lifespan runs.
- `TestClient(main.app)` without `with` does NOT trigger lifespan — tests rely on monkeypatch to set `main.recorder` and `main.transcriber`.
- In production, uvicorn runs the lifespan on startup, populating both globals before any request arrives.
- If lifespan fails (CUDA unavailable, model not found), uvicorn does NOT start serving — fail-fast as designed.

- [ ] **Step 4: Run tests to verify they pass**

```
python -m pytest tests/test_main.py -v
```

Expected: 15 passing tests (10 existing updated + 5 new).

- [ ] **Step 5: Run full suite**

```
python -m pytest -v
```

Expected: 35 passing tests total (15 main + 13 recorder + 7 transcriber).

- [ ] **Step 6: Commit**

```
git add audio-service/main.py audio-service/tests/test_main.py
git commit -m "feat(audio): add /transcribe endpoint and refactor main.py to lifespan"
```

---

## Task 4: README + final verification

**Files:**
- Modify: `audio-service/README.md`

- [ ] **Step 1: Replace `README.md`**

Overwrite `audio-service/README.md`:

```markdown
# Audio Service

Captures microphone + system loopback audio on Windows, saves a 16 kHz mono WAV per recording, and transcribes WAV files via faster-whisper on the GPU. Runs as a local HTTP server on port 8765. Consumed by the Go backend (Fatia 7).

## Requirements

- Windows 10/11
- Python 3.12 (`py -3.12 --version` should report 3.12.x)
- A working default microphone and a default audio output device
- NVIDIA GPU with CUDA 12 driver (RTX 2050 4GB or better recommended)

## Setup

```
cd audio-service
py -3.12 -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
copy .env.example .env
```

The first install downloads ~1.5 GB of NVIDIA wheels (cuDNN + cuBLAS) into the venv. Edit `.env` if you want to change the port, recordings directory, or Whisper settings.

The first time the service runs it will also download the Whisper `medium` model (~1.5 GB) into `~/.cache/huggingface/`.

## Run

```
.venv\Scripts\activate
uvicorn main:app --port 8765
```

The service loads the Whisper model into the GPU on startup. Boot takes ~30 seconds the first time (model download + load) and ~5 seconds on subsequent runs.

## Endpoints

```
GET  /health           service status, recording state, loopback availability, model info
POST /recording/start  start a new recording (only allowed when idle)
POST /recording/stop   stop the current recording, return WAV path and metadata
GET  /recording/status current state and recording info
POST /transcribe       transcribe a WAV file at the given path, return text
```

Examples:

```
curl http://localhost:8765/health

curl -X POST http://localhost:8765/recording/start
curl -X POST http://localhost:8765/recording/stop

curl -X POST http://localhost:8765/transcribe \
     -H "Content-Type: application/json" \
     -d '{"path":"tmp/rec-<id>.wav"}'
```

The `/transcribe` body accepts an optional `language` field (e.g. `"en"`); when omitted, the value of `WHISPER_LANGUAGE` from `.env` is used.

## Automated tests

```
pytest -v
```

The automated tests cover the state machine, the HTTP endpoints, and the Transcriber path validation with PyAudio and WhisperModel mocked. They run on any platform with the required Python packages installed.

## Manual smoke test (Windows + GPU only)

1. Start the service: `uvicorn main:app --port 8765` and wait for "Application startup complete" (~30s on first run while the model loads).
2. Confirm health: `curl http://localhost:8765/health` returns `loopback_available: true`, `model_loaded: true`, `device: "cuda"`.
3. Start playing music in a media player (Spotify, browser, anything).
4. Start a recording: `curl -X POST http://localhost:8765/recording/start`
5. Speak into the microphone for ~10 seconds.
6. Stop: `curl -X POST http://localhost:8765/recording/stop` — the response includes the WAV `path`.
7. Transcribe: `curl -X POST http://localhost:8765/transcribe -H "Content-Type: application/json" -d "{\"path\":\"<path>\"}"` (replace `<path>` with the value from step 6). Should return a coherent transcript in Portuguese in 5–10 seconds.

State conflicts:
- `POST /recording/start` while recording → 409.
- `POST /recording/stop` while idle → 409.
- `POST /transcribe` with a path outside the recordings directory → 400.
```

- [ ] **Step 2: Run the full test suite one last time**

```
cd audio-service
python -m pytest -v
```

Expected: 35 passing tests.

- [ ] **Step 3: Final smoke test (manual, on the user's Windows machine)**

The user runs the smoke test from the README and confirms the transcription quality.

- [ ] **Step 4: Commit**

```
git add audio-service/README.md
git commit -m "docs(audio): document /transcribe endpoint and Whisper setup"
```

---

## Final verification checklist

After all four tasks are complete:

- [ ] `cd audio-service && pytest -v` passes (35 tests)
- [ ] `cd audio-service && uvicorn main:app --port 8765` boots, model loads on GPU
- [ ] `curl http://localhost:8765/health` shows `model_loaded: true`, `device: "cuda"`
- [ ] Manual smoke test produces a coherent pt-BR transcript from a recorded WAV
- [ ] `POST /transcribe` with a path-traversal attempt returns 400
- [ ] WAV files are NOT deleted by the service (caller's responsibility)
