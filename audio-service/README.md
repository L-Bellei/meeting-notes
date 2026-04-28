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
GET  /health           service status, current state, loopback availability
POST /recording/start  start a new recording (only allowed when idle)
POST /recording/stop   stop the current recording, return WAV path and metadata
GET  /recording/status current state and recording info
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

The automated tests cover the state machine and the HTTP endpoints with PyAudio mocked. They run on any platform with the required dependencies installed.

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
