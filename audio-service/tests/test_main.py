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
