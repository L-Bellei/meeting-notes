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
