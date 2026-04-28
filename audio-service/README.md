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
7. Transcribe: `curl -X POST http://localhost:8765/transcribe -H "Content-Type: application/json" -d "{\"path\":\"<path>\"}"` (replace `<path>` with the value from step 6). Should return a coherent transcript in Portuguese in 5-10 seconds.

State conflicts:
- `POST /recording/start` while recording → 409.
- `POST /recording/stop` while idle → 409.
- `POST /transcribe` with a path outside the recordings directory → 400.
