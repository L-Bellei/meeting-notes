# Fatia 5 — Audio Service (parte 1, captura) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Python FastAPI service that captures simultaneous microphone + system loopback audio on Windows and saves a 16 kHz mono WAV per recording. No Whisper yet.

**Architecture:** Single-process FastAPI app on port 8765. A `Recorder` class encapsulates the state machine (`IDLE` ↔ `RECORDING`) and the audio pipeline (PyAudioWPatch streams → per-stream queues → mixer thread that resamples, mixes mono, writes WAV via `soundfile`). Endpoints are thin wrappers around `Recorder` methods. The state machine is implemented and tested before any audio hardware is touched.

**Tech Stack:** Python 3.12, FastAPI, uvicorn, pyaudiowpatch (WASAPI loopback), numpy, scipy (resampling), soundfile (WAV writing), pytest.

---

## Project conventions

- All code lives in `audio-service/`. The Go code in `internal/` is unaffected by Fatia 5.
- Python: 4-space indentation. No type annotations are required, but the plan uses them where they aid clarity.
- Tests use `pytest` from the `audio-service/` directory: `cd audio-service && pytest`.
- Each task ends with a commit. Commit messages start with `feat(audio):`, `test(audio):`, `docs(audio):`, etc.
- Working directory `F:\dev\meeting-notes`. Branch `master`.
- The audio capture pipeline is Windows-specific. Tests that mock pyaudiowpatch run anywhere; the manual smoke test requires Windows + WASAPI.

---

## Task 1: Project bootstrap

**Files:**
- Create: `audio-service/requirements.txt`
- Create: `audio-service/.env.example`
- Create: `audio-service/.gitignore`
- Create: `audio-service/README.md` (skeleton; finalized in Task 6)
- Create: `audio-service/tests/__init__.py` (empty)
- Create: `audio-service/conftest.py` (empty placeholder for pytest discovery)

- [ ] **Step 1: Create `requirements.txt`**

Create `audio-service/requirements.txt`:

```
fastapi==0.115.0
uvicorn[standard]==0.32.0
pyaudiowpatch==0.2.12.7
numpy==2.1.2
scipy==1.14.1
soundfile==0.12.1
pytest==8.3.3
httpx==0.27.2
```

(`httpx` is required by FastAPI's `TestClient`.)

- [ ] **Step 2: Create `.env.example`**

Create `audio-service/.env.example`:

```
HTTP_PORT=8765
RECORDINGS_DIR=./tmp
```

- [ ] **Step 3: Create `.gitignore`**

Create `audio-service/.gitignore`:

```
.venv/
venv/
tmp/
__pycache__/
*.pyc
.pytest_cache/
.env
```

- [ ] **Step 4: Create skeleton `README.md`**

Create `audio-service/README.md`:

```markdown
# Audio Service

Captures microphone + system loopback audio on Windows and saves a WAV file per recording.
Runs on port 8765. Consumed by the Go backend (Fatia 7).

## Setup

```
py -3.12 -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
```

## Run

```
uvicorn main:app --port 8765
```

## Tests

```
pytest
```

Manual smoke tests are documented at the end of this file (filled in Task 6).
```

- [ ] **Step 5: Create empty test scaffolding**

Create `audio-service/tests/__init__.py` (empty file).
Create `audio-service/conftest.py` (empty file — kept for future fixtures).

- [ ] **Step 6: Create empty `tmp/` directory placeholder**

Create `audio-service/tmp/.gitkeep` (empty file). This keeps the recordings directory in source control even though the WAVs themselves are git-ignored.

- [ ] **Step 7: Commit**

```
git add audio-service/
git commit -m "feat(audio): bootstrap audio-service project skeleton"
```

---

## Task 2: Recorder state machine (no audio yet)

**Goal:** Implement the `Recorder` class with state transitions, `RecorderError`, and `StopResult`. No PyAudio calls. Audio integration in Task 5.

**Files:**
- Create: `audio-service/recorder.py`
- Create: `audio-service/tests/test_recorder.py`

- [ ] **Step 1: Write the failing tests**

Create `audio-service/tests/test_recorder.py`:

```python
import threading
from pathlib import Path

import pytest

from recorder import Recorder, RecorderError, StopResult


@pytest.fixture
def recorder(tmp_path):
    return Recorder(tmp_path)


def test_initial_state_is_idle(recorder):
    assert recorder.state == "idle"


def test_status_when_idle(recorder):
    s = recorder.status()
    assert s["state"] == "idle"
    assert s["recording_id"] is None
    assert s["started_at"] is None


def test_start_transitions_to_recording(recorder):
    rec_id, started_at = recorder.start()
    assert isinstance(rec_id, str) and len(rec_id) > 0
    assert started_at is not None
    assert recorder.state == "recording"


def test_status_when_recording(recorder):
    rec_id, _ = recorder.start()
    s = recorder.status()
    assert s["state"] == "recording"
    assert s["recording_id"] == rec_id
    assert s["started_at"] is not None


def test_start_when_already_recording_raises(recorder):
    recorder.start()
    with pytest.raises(RecorderError, match="already recording"):
        recorder.start()


def test_stop_when_idle_raises(recorder):
    with pytest.raises(RecorderError, match="not recording"):
        recorder.stop()


def test_stop_returns_stopresult_and_transitions_to_idle(recorder):
    rec_id, _ = recorder.start()
    result = recorder.stop()
    assert isinstance(result, StopResult)
    assert result.recording_id == rec_id
    assert result.path.name == f"rec-{rec_id}.wav"
    assert result.partial is False
    assert recorder.state == "idle"


def test_start_when_loopback_unavailable_raises(tmp_path):
    rec = Recorder(tmp_path)
    rec.loopback_available = False
    with pytest.raises(RecorderError, match="loopback unavailable"):
        rec.start()


def test_start_when_mic_unavailable_raises(tmp_path):
    rec = Recorder(tmp_path)
    rec.mic_available = False
    with pytest.raises(RecorderError, match="mic unavailable"):
        rec.start()


def test_recordings_dir_is_created(tmp_path):
    target = tmp_path / "subdir"
    Recorder(target)
    assert target.exists() and target.is_dir()
```

- [ ] **Step 2: Run tests to verify they fail**

Run from `audio-service/`:
```
pytest tests/test_recorder.py -v
```

Expected: ImportError — `recorder` module not found.

- [ ] **Step 3: Implement `recorder.py` skeleton**

Create `audio-service/recorder.py`:

```python
import threading
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path


class RecorderError(Exception):
    pass


@dataclass
class StopResult:
    recording_id: str
    path: Path
    duration_seconds: float
    size_bytes: int
    partial: bool


class Recorder:
    def __init__(self, recordings_dir):
        self.recordings_dir = Path(recordings_dir)
        self.recordings_dir.mkdir(parents=True, exist_ok=True)

        self._lock = threading.Lock()
        self._state = "idle"
        self._recording_id = None
        self._started_at = None
        self._path = None
        self._partial = False

        self.loopback_available = True
        self.mic_available = True

    @property
    def state(self):
        with self._lock:
            return self._state

    def status(self):
        with self._lock:
            return {
                "state": self._state,
                "recording_id": self._recording_id,
                "started_at": self._started_at.isoformat() if self._started_at else None,
            }

    def start(self):
        with self._lock:
            if self._state == "recording":
                raise RecorderError("already recording")
            if not self.loopback_available:
                raise RecorderError("loopback unavailable")
            if not self.mic_available:
                raise RecorderError("mic unavailable")

            self._recording_id = str(uuid.uuid4())
            self._started_at = datetime.now(timezone.utc)
            self._path = self.recordings_dir / f"rec-{self._recording_id}.wav"
            self._partial = False
            self._open_streams()
            self._state = "recording"
            return self._recording_id, self._started_at

    def stop(self):
        with self._lock:
            if self._state == "idle":
                raise RecorderError("not recording")

            self._close_streams()
            duration = self._compute_duration()
            size_bytes = self._path.stat().st_size if self._path.exists() else 0
            result = StopResult(
                recording_id=self._recording_id,
                path=self._path,
                duration_seconds=duration,
                size_bytes=size_bytes,
                partial=self._partial,
            )
            self._state = "idle"
            self._recording_id = None
            self._started_at = None
            self._path = None
            return result

    def _open_streams(self):
        pass

    def _close_streams(self):
        pass

    def _compute_duration(self):
        return 0.0
```

- [ ] **Step 4: Run tests to verify they pass**

```
pytest tests/test_recorder.py -v
```

Expected: 10 passing tests.

- [ ] **Step 5: Commit**

```
git add audio-service/recorder.py audio-service/tests/test_recorder.py
git commit -m "feat(audio): add Recorder state machine with start/stop/status"
```

---

## Task 3: FastAPI endpoints

**Goal:** Implement `main.py` with the four endpoints, wired to `Recorder`. Tests use `TestClient` with a monkeypatched `Recorder`.

**Files:**
- Create: `audio-service/main.py`
- Create: `audio-service/tests/test_main.py`

- [ ] **Step 1: Write the failing tests**

Create `audio-service/tests/test_main.py`:

```python
from datetime import datetime, timezone
from pathlib import Path
from unittest.mock import MagicMock

import pytest
from fastapi.testclient import TestClient

import main
from recorder import RecorderError, StopResult


@pytest.fixture
def mock_recorder(monkeypatch):
    m = MagicMock()
    m.state = "idle"
    m.loopback_available = True
    m.status.return_value = {"state": "idle", "recording_id": None, "started_at": None}
    monkeypatch.setattr(main, "recorder", m)
    return m


@pytest.fixture
def client():
    return TestClient(main.app)


def test_health_idle(mock_recorder, client):
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json() == {"status": "ok", "state": "idle", "loopback_available": True}


def test_health_loopback_unavailable(mock_recorder, client):
    mock_recorder.loopback_available = False
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json()["loopback_available"] is False


def test_start_idle(mock_recorder, client):
    started = datetime(2026, 4, 27, 12, 0, 0, tzinfo=timezone.utc)
    mock_recorder.start.return_value = ("abc-123", started)
    r = client.post("/recording/start")
    assert r.status_code == 200
    assert r.json() == {"recording_id": "abc-123", "started_at": "2026-04-27T12:00:00+00:00"}


def test_start_already_recording(mock_recorder, client):
    mock_recorder.start.side_effect = RecorderError("already recording")
    r = client.post("/recording/start")
    assert r.status_code == 409
    assert "already recording" in r.json()["detail"]


def test_start_loopback_unavailable(mock_recorder, client):
    mock_recorder.start.side_effect = RecorderError("loopback unavailable")
    r = client.post("/recording/start")
    assert r.status_code == 503
    assert "loopback unavailable" in r.json()["detail"]


def test_start_mic_unavailable(mock_recorder, client):
    mock_recorder.start.side_effect = RecorderError("mic unavailable")
    r = client.post("/recording/start")
    assert r.status_code == 503


def test_stop_recording(mock_recorder, client):
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


def test_stop_idle(mock_recorder, client):
    mock_recorder.stop.side_effect = RecorderError("not recording")
    r = client.post("/recording/stop")
    assert r.status_code == 409
    assert "not recording" in r.json()["detail"]


def test_status_idle(mock_recorder, client):
    r = client.get("/recording/status")
    assert r.status_code == 200
    assert r.json() == {"state": "idle", "recording_id": None, "started_at": None}


def test_status_recording(mock_recorder, client):
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
```

- [ ] **Step 2: Run tests to verify they fail**

```
pytest tests/test_main.py -v
```

Expected: ImportError — `main` not found.

- [ ] **Step 3: Implement `main.py`**

Create `audio-service/main.py`:

```python
import os
from pathlib import Path

from fastapi import FastAPI, HTTPException

from recorder import Recorder, RecorderError

RECORDINGS_DIR = Path(os.getenv("RECORDINGS_DIR", "./tmp"))

app = FastAPI()
recorder = Recorder(RECORDINGS_DIR)


@app.get("/health")
def health():
    return {
        "status": "ok",
        "state": recorder.state,
        "loopback_available": recorder.loopback_available,
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
```

- [ ] **Step 4: Run tests to verify they pass**

```
pytest tests/test_main.py -v
```

Expected: 10 passing tests.

- [ ] **Step 5: Run full test suite**

```
pytest -v
```

Expected: all tests in `test_recorder.py` and `test_main.py` pass (20 total).

- [ ] **Step 6: Smoke-test the server starts**

```
uvicorn main:app --port 8765
```

In a second terminal:
```
curl http://localhost:8765/health
```

Expected: 200 with JSON body. Stop the server (`Ctrl+C`).

- [ ] **Step 7: Commit**

```
git add audio-service/main.py audio-service/tests/test_main.py
git commit -m "feat(audio): add FastAPI endpoints for recording lifecycle"
```

---

## Task 4: Device enumeration (capability check)

**Goal:** On `Recorder.__init__`, enumerate PyAudio devices and detect availability of the default mic and the WASAPI loopback for the default speakers. Cache `loopback_available` and `mic_available`. Stash device info for use in Task 5.

**Files:**
- Modify: `audio-service/recorder.py`
- Modify: `audio-service/tests/test_recorder.py`

- [ ] **Step 1: Write the failing tests**

Append to `audio-service/tests/test_recorder.py`:

```python
from unittest.mock import MagicMock, patch


def _fake_pyaudio_with(default_input=True, loopback=True):
    """Build a MagicMock that simulates a PyAudio instance."""
    pa = MagicMock()
    if default_input:
        pa.get_default_input_device_info.return_value = {
            "index": 0, "name": "Microfone", "defaultSampleRate": 48000.0, "maxInputChannels": 1,
        }
    else:
        pa.get_default_input_device_info.side_effect = OSError("no default input")
    if loopback:
        pa.get_default_output_device_info.return_value = {
            "index": 1, "name": "Alto-falantes", "defaultSampleRate": 48000.0,
        }
        pa.get_loopback_device_info_generator.return_value = iter([
            {"index": 2, "name": "Alto-falantes [Loopback]", "defaultSampleRate": 48000.0, "maxInputChannels": 2},
        ])
    else:
        pa.get_default_output_device_info.return_value = {
            "index": 1, "name": "Alto-falantes", "defaultSampleRate": 48000.0,
        }
        pa.get_loopback_device_info_generator.return_value = iter([])
    return pa


def test_init_detects_devices_when_present(tmp_path):
    fake_pa = _fake_pyaudio_with(default_input=True, loopback=True)
    with patch("recorder.pyaudio.PyAudio", return_value=fake_pa):
        rec = Recorder(tmp_path)
    assert rec.mic_available is True
    assert rec.loopback_available is True


def test_init_marks_loopback_unavailable_when_no_match(tmp_path):
    fake_pa = _fake_pyaudio_with(default_input=True, loopback=False)
    with patch("recorder.pyaudio.PyAudio", return_value=fake_pa):
        rec = Recorder(tmp_path)
    assert rec.loopback_available is False


def test_init_marks_mic_unavailable_when_no_default_input(tmp_path):
    fake_pa = _fake_pyaudio_with(default_input=False, loopback=True)
    with patch("recorder.pyaudio.PyAudio", return_value=fake_pa):
        rec = Recorder(tmp_path)
    assert rec.mic_available is False
```

The earlier `test_initial_state_is_idle`-style tests already construct `Recorder(tmp_path)` directly. Those still work because the real PyAudioWPatch import succeeds (it's installed). On a Windows machine the real device check may pass too. To keep those tests platform-independent, change them to patch PyAudio. Update the existing fixture:

Replace the `recorder` fixture and the standalone `Recorder(tmp_path)` calls in earlier tests with a fixture that patches:

```python
@pytest.fixture
def recorder(tmp_path):
    fake_pa = _fake_pyaudio_with(default_input=True, loopback=True)
    with patch("recorder.pyaudio.PyAudio", return_value=fake_pa):
        yield Recorder(tmp_path)
```

And update `test_start_when_loopback_unavailable_raises`, `test_start_when_mic_unavailable_raises`, and `test_recordings_dir_is_created` to also use the patch:

```python
def test_start_when_loopback_unavailable_raises(tmp_path):
    fake_pa = _fake_pyaudio_with(default_input=True, loopback=True)
    with patch("recorder.pyaudio.PyAudio", return_value=fake_pa):
        rec = Recorder(tmp_path)
        rec.loopback_available = False
        with pytest.raises(RecorderError, match="loopback unavailable"):
            rec.start()


def test_start_when_mic_unavailable_raises(tmp_path):
    fake_pa = _fake_pyaudio_with(default_input=True, loopback=True)
    with patch("recorder.pyaudio.PyAudio", return_value=fake_pa):
        rec = Recorder(tmp_path)
        rec.mic_available = False
        with pytest.raises(RecorderError, match="mic unavailable"):
            rec.start()


def test_recordings_dir_is_created(tmp_path):
    fake_pa = _fake_pyaudio_with(default_input=True, loopback=True)
    target = tmp_path / "subdir"
    with patch("recorder.pyaudio.PyAudio", return_value=fake_pa):
        Recorder(target)
    assert target.exists() and target.is_dir()
```

- [ ] **Step 2: Run tests to verify the new ones fail**

```
pytest tests/test_recorder.py -v
```

Expected: the three new tests fail because `recorder.pyaudio` doesn't exist yet.

- [ ] **Step 3: Update `recorder.py` to enumerate devices**

Modify `audio-service/recorder.py`. Add `import pyaudiowpatch as pyaudio` at the top, and replace `__init__` with the version that enumerates devices:

```python
import threading
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

import pyaudiowpatch as pyaudio


class RecorderError(Exception):
    pass


@dataclass
class StopResult:
    recording_id: str
    path: Path
    duration_seconds: float
    size_bytes: int
    partial: bool


class Recorder:
    def __init__(self, recordings_dir):
        self.recordings_dir = Path(recordings_dir)
        self.recordings_dir.mkdir(parents=True, exist_ok=True)

        self._lock = threading.Lock()
        self._state = "idle"
        self._recording_id = None
        self._started_at = None
        self._path = None
        self._partial = False

        self._pa = pyaudio.PyAudio()
        self._mic_info = None
        self._loopback_info = None
        self.mic_available = False
        self.loopback_available = False
        self._enumerate_devices()

    def _enumerate_devices(self):
        try:
            mic = self._pa.get_default_input_device_info()
            self._mic_info = mic
            self.mic_available = True
        except (OSError, IOError):
            self.mic_available = False

        try:
            speakers = self._pa.get_default_output_device_info()
            for lb in self._pa.get_loopback_device_info_generator():
                if speakers["name"] in lb["name"]:
                    self._loopback_info = lb
                    self.loopback_available = True
                    break
        except (OSError, IOError):
            self.loopback_available = False
```

The rest of the class (`state`, `status`, `start`, `stop`, `_open_streams`, `_close_streams`, `_compute_duration`) stays unchanged.

- [ ] **Step 4: Run tests to verify they pass**

```
pytest tests/test_recorder.py -v
```

Expected: all tests pass (13 total in this file).

- [ ] **Step 5: Run full suite**

```
pytest -v
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```
git add audio-service/recorder.py audio-service/tests/test_recorder.py
git commit -m "feat(audio): enumerate mic and loopback devices on Recorder init"
```

---

## Task 5: Audio capture pipeline

**Goal:** Implement the actual audio capture inside `Recorder._open_streams`, `_close_streams`, and `_compute_duration`. PyAudio streams open in callback mode, frames go into per-stream queues, a mixer thread drains queues, resamples to 16 kHz mono, mixes, and writes incrementally to a `soundfile.SoundFile`.

This task does not introduce new automated tests — the audio path requires real hardware. It is verified manually in Task 6. The state-machine and endpoint tests from Tasks 2/3 must continue to pass after the changes.

**Files:**
- Modify: `audio-service/recorder.py`

- [ ] **Step 1: Add the imports and constants**

At the top of `audio-service/recorder.py`, alongside the existing imports, add:

```python
import logging
import queue

import numpy as np
import soundfile as sf
from scipy.signal import resample_poly

TARGET_SAMPLE_RATE = 16000
CHUNK_FRAMES = 1024
QUEUE_MAX_CHUNKS = 200

log = logging.getLogger("recorder")
```

- [ ] **Step 2: Add the audio fields to `__init__`**

Inside `Recorder.__init__`, add after the existing field initialization (and after `_enumerate_devices()`):

```python
        self._mic_stream = None
        self._loopback_stream = None
        self._mic_queue = None
        self._loopback_queue = None
        self._mic_native_rate = None
        self._loopback_native_rate = None
        self._loopback_channels = None
        self._writer = None
        self._frames_written = 0
        self._stop_event = None
        self._mixer_thread = None
```

- [ ] **Step 3: Implement `_open_streams`**

Replace the existing `_open_streams` body in `recorder.py` with:

```python
    def _open_streams(self):
        self._mic_queue = queue.Queue(maxsize=QUEUE_MAX_CHUNKS)
        self._loopback_queue = queue.Queue(maxsize=QUEUE_MAX_CHUNKS)
        self._mic_native_rate = int(self._mic_info["defaultSampleRate"])
        self._loopback_native_rate = int(self._loopback_info["defaultSampleRate"])
        self._loopback_channels = int(self._loopback_info["maxInputChannels"])
        self._frames_written = 0
        self._stop_event = threading.Event()

        self._writer = sf.SoundFile(
            str(self._path),
            mode="w",
            samplerate=TARGET_SAMPLE_RATE,
            channels=1,
            subtype="PCM_16",
        )

        self._mic_stream = self._pa.open(
            format=pyaudio.paInt16,
            channels=1,
            rate=self._mic_native_rate,
            input=True,
            input_device_index=self._mic_info["index"],
            frames_per_buffer=CHUNK_FRAMES,
            stream_callback=self._make_callback(self._mic_queue),
        )

        self._loopback_stream = self._pa.open(
            format=pyaudio.paInt16,
            channels=self._loopback_channels,
            rate=self._loopback_native_rate,
            input=True,
            input_device_index=self._loopback_info["index"],
            frames_per_buffer=CHUNK_FRAMES,
            stream_callback=self._make_callback(self._loopback_queue),
        )

        self._mic_stream.start_stream()
        self._loopback_stream.start_stream()

        self._mixer_thread = threading.Thread(target=self._run_mixer, daemon=True)
        self._mixer_thread.start()

    def _make_callback(self, q):
        def callback(in_data, frame_count, time_info, status):
            try:
                q.put_nowait(in_data)
            except queue.Full:
                try:
                    q.get_nowait()
                except queue.Empty:
                    pass
                try:
                    q.put_nowait(in_data)
                except queue.Full:
                    self._partial = True
                    log.warning("queue full; dropped audio chunk")
            return (None, pyaudio.paContinue)
        return callback
```

- [ ] **Step 4: Implement `_run_mixer`**

Add this method to `Recorder`:

```python
    def _run_mixer(self):
        try:
            while not self._stop_event.is_set():
                mic_chunk = self._safe_get(self._mic_queue, 0.05)
                lb_chunk = self._safe_get(self._loopback_queue, 0.05)
                if mic_chunk is None and lb_chunk is None:
                    continue
                mic_mono16 = self._chunk_to_mono16k(mic_chunk, 1, self._mic_native_rate)
                lb_mono16 = self._chunk_to_mono16k(lb_chunk, self._loopback_channels, self._loopback_native_rate)
                mixed = self._mix(mic_mono16, lb_mono16)
                if mixed.size > 0:
                    self._writer.write(mixed)
                    self._frames_written += mixed.shape[0]
            self._drain_queues()
        except Exception as e:
            log.exception("mixer thread error: %s", e)
            self._partial = True

    def _safe_get(self, q, timeout):
        try:
            return q.get(timeout=timeout)
        except queue.Empty:
            return None

    def _drain_queues(self):
        while True:
            mic_chunk = self._safe_get(self._mic_queue, 0)
            lb_chunk = self._safe_get(self._loopback_queue, 0)
            if mic_chunk is None and lb_chunk is None:
                return
            mic_mono16 = self._chunk_to_mono16k(mic_chunk, 1, self._mic_native_rate)
            lb_mono16 = self._chunk_to_mono16k(lb_chunk, self._loopback_channels, self._loopback_native_rate)
            mixed = self._mix(mic_mono16, lb_mono16)
            if mixed.size > 0:
                self._writer.write(mixed)
                self._frames_written += mixed.shape[0]

    def _chunk_to_mono16k(self, raw, channels, native_rate):
        if raw is None:
            return np.zeros(0, dtype=np.float32)
        samples = np.frombuffer(raw, dtype=np.int16).astype(np.float32) / 32768.0
        if channels > 1:
            samples = samples.reshape(-1, channels).mean(axis=1)
        if native_rate != TARGET_SAMPLE_RATE:
            samples = resample_poly(samples, TARGET_SAMPLE_RATE, native_rate)
        return samples.astype(np.float32)

    def _mix(self, a, b):
        if a.size == 0 and b.size == 0:
            return np.zeros(0, dtype=np.float32)
        n = max(a.size, b.size)
        if a.size < n:
            a = np.pad(a, (0, n - a.size))
        if b.size < n:
            b = np.pad(b, (0, n - b.size))
        mixed = a + b
        return np.clip(mixed, -1.0, 1.0)
```

- [ ] **Step 5: Implement `_close_streams`**

Replace the existing `_close_streams` body with:

```python
    def _close_streams(self):
        if self._stop_event is not None:
            self._stop_event.set()

        for stream in (self._mic_stream, self._loopback_stream):
            if stream is None:
                continue
            try:
                if stream.is_active():
                    stream.stop_stream()
                stream.close()
            except Exception as e:
                log.warning("error closing stream: %s", e)
                self._partial = True

        if self._mixer_thread is not None:
            self._mixer_thread.join(timeout=5.0)
            if self._mixer_thread.is_alive():
                log.warning("mixer thread did not exit in time")
                self._partial = True

        if self._writer is not None:
            try:
                self._writer.close()
            except Exception as e:
                log.warning("error closing writer: %s", e)
                self._partial = True

        self._mic_stream = None
        self._loopback_stream = None
        self._mic_queue = None
        self._loopback_queue = None
        self._mixer_thread = None
        self._stop_event = None
        self._writer = None
```

- [ ] **Step 6: Implement `_compute_duration`**

Replace `_compute_duration`:

```python
    def _compute_duration(self):
        return self._frames_written / TARGET_SAMPLE_RATE
```

- [ ] **Step 7: Run the existing test suite**

```
pytest -v
```

Expected: all tests pass. The state-machine and endpoint tests are unaffected because they mock PyAudio.

- [ ] **Step 8: Manual smoke test**

In a Windows environment with a microphone and audio output:

1. Activate the venv: `audio-service\.venv\Scripts\activate`
2. Run: `uvicorn main:app --port 8765`
3. In another shell:
   ```
   curl http://localhost:8765/health
   ```
   Expect `loopback_available: true`.
4. Start a song in a music player.
5. `curl -X POST http://localhost:8765/recording/start` — expect `{"recording_id":"...","started_at":"..."}`
6. Speak into the microphone for ~10 seconds while the music plays.
7. `curl -X POST http://localhost:8765/recording/stop` — expect a path, a `duration_seconds` ≈ 10, `partial: false`.
8. Open the WAV file (`audio-service/tmp/rec-<id>.wav`) in any audio player. Both your voice and the music must be audible. The file must be playable end-to-end.
9. Stop uvicorn (`Ctrl+C`).

If the smoke test fails (no audio, partial=true unexpectedly, file unplayable), debug before committing. Common issues:
- WASAPI device naming: `_loopback_info` lookup uses `speakers["name"] in lb["name"]` — verify the names match in your environment by adding a `print()` if needed.
- Sample rate mismatch: stream may refuse the requested format; check the exception in uvicorn logs.

- [ ] **Step 9: Commit**

```
git add audio-service/recorder.py
git commit -m "feat(audio): implement mic+loopback capture, mixing, and WAV writing"
```

---

## Task 6: Documentation and final verification

**Goal:** Finalize the README with full setup, run, and manual test instructions. Run the full test suite and a final smoke test.

**Files:**
- Modify: `audio-service/README.md`

- [ ] **Step 1: Replace `README.md` with the full version**

Overwrite `audio-service/README.md`:

```markdown
# Audio Service

Captures microphone + system loopback audio on Windows and saves a 16 kHz mono WAV per recording. Runs as a local HTTP server on port 8765. Consumed by the Go backend (Fatia 7).

## Requirements

- Windows 10/11
- Python 3.12 (`py -3.12 --version` should report 3.12.x)
- A working default microphone and a default audio output device

## Setup

```
cd audio-service
py -3.12 -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
copy .env.example .env
```

Edit `.env` if you want to change the port or recordings directory.

## Run

```
.venv\Scripts\activate
uvicorn main:app --port 8765
```

The service stays in `idle` state until the first `POST /recording/start`.

## Endpoints

```
GET  /health           → service status, current state, loopback availability
POST /recording/start  → start a new recording (only allowed when idle)
POST /recording/stop   → stop the current recording, return WAV path and metadata
GET  /recording/status → current state and recording info
```

Examples:

```
curl http://localhost:8765/health

curl -X POST http://localhost:8765/recording/start

curl -X POST http://localhost:8765/recording/stop
```

## Automated tests

```
pytest -v
```

The automated tests cover the state machine and the HTTP endpoints with PyAudio mocked. They run on any platform.

## Manual smoke test (Windows only)

1. Start the service: `uvicorn main:app --port 8765`
2. Confirm health: `curl http://localhost:8765/health` returns `loopback_available: true`.
3. Start playing music in a media player (Spotify, browser, anything).
4. Start a recording: `curl -X POST http://localhost:8765/recording/start`
5. Speak into the microphone for ~10 seconds.
6. Stop: `curl -X POST http://localhost:8765/recording/stop`
7. The response includes a `path`. Open that WAV file. Both your voice and the music must be audible.

State conflicts:
- Calling `/recording/start` twice in a row returns `409`.
- Calling `/recording/stop` without a prior `/recording/start` returns `409`.
```

- [ ] **Step 2: Run the full test suite one last time**

```
cd audio-service
pytest -v
```

Expected: all tests pass.

- [ ] **Step 3: Final smoke test**

Repeat the manual smoke test from Task 5 Step 8 to confirm everything still works end-to-end.

- [ ] **Step 4: Commit**

```
git add audio-service/README.md
git commit -m "docs(audio): finalize README with setup, endpoints, and manual smoke test"
```

---

## Final verification checklist

After all six tasks are complete:

- [ ] `cd audio-service && pytest -v` passes (≥ 23 tests)
- [ ] `cd audio-service && uvicorn main:app --port 8765` starts without errors
- [ ] `curl http://localhost:8765/health` returns 200 with `loopback_available: true`
- [ ] Manual smoke test produces a playable WAV with mic + system audio mixed
- [ ] State conflicts (`/start` while recording, `/stop` while idle) return 409
- [ ] WAV is mono, 16 kHz, PCM 16-bit (verifiable via `python -c "import soundfile; print(soundfile.info('tmp/rec-<id>.wav'))"`)
