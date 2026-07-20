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
WHISPER_DEVICE = os.getenv("WHISPER_DEVICE", "auto")
WHISPER_COMPUTE_TYPE = os.getenv("WHISPER_COMPUTE_TYPE", "auto")

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
    except Exception as e:
        import traceback
        raise HTTPException(status_code=503, detail=f"{type(e).__name__}: {e}\n{traceback.format_exc()}")
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
