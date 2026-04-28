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
