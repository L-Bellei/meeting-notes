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
    m.gpu_available = False
    m.gpu_name = None
    m.gpu_vram_mb = None
    m.gpu_vendor = None
    m.gpu_backend = None
    m.vulkan_model_ready = False
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
        "gpu_available": False,
        "gpu_name": None,
        "gpu_vram_mb": None,
        "gpu_vendor": None,
        "gpu_backend": None,
        "vulkan_model_ready": False,
    }


def test_health_loopback_unavailable(mock_recorder, mock_transcriber, client):
    mock_recorder.loopback_available = False
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json()["loopback_available"] is False


def test_health_includes_gpu_scan_fields(mock_recorder, mock_transcriber, client):
    mock_transcriber.gpu_available = True
    mock_transcriber.gpu_name = "NVIDIA GeForce RTX 2050"
    mock_transcriber.gpu_vram_mb = 4096
    r = client.get("/health")
    body = r.json()
    assert body["gpu_available"] is True
    assert body["gpu_name"] == "NVIDIA GeForce RTX 2050"
    assert body["gpu_vram_mb"] == 4096


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
        device="cpu",
    )
    r = client.post("/transcribe", json={"path": "tmp/rec-abc.wav"})
    assert r.status_code == 200
    assert r.json() == {
        "transcript": "texto transcrito",
        "language": "pt",
        "duration_seconds": 10.5,
        "model": "medium",
        "device": "cpu",
    }
    args, kwargs = mock_transcriber.transcribe.call_args
    assert str(args[0]) == "tmp/rec-abc.wav" or str(args[0]).endswith("rec-abc.wav")
    assert args[1] is None
    assert kwargs.get("device") == "auto"


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


def test_transcribe_passes_device_and_returns_effective(mock_recorder, mock_transcriber, client):
    mock_transcriber.transcribe.return_value = TranscribeResult(
        transcript="oi", language="pt", duration_seconds=1.0, model="medium", device="cuda"
    )
    r = client.post("/transcribe", json={"path": "tmp/rec.wav", "device": "cuda"})
    assert r.status_code == 200
    assert r.json()["device"] == "cuda"
    args, kwargs = mock_transcriber.transcribe.call_args
    assert kwargs.get("device") == "cuda" or (len(args) >= 3 and args[2] == "cuda")


def test_transcribe_device_defaults_to_auto(mock_recorder, mock_transcriber, client):
    mock_transcriber.transcribe.return_value = TranscribeResult(
        transcript="oi", language="pt", duration_seconds=1.0, model="medium", device="cpu"
    )
    r = client.post("/transcribe", json={"path": "tmp/rec.wav"})
    assert r.status_code == 200
    args, kwargs = mock_transcriber.transcribe.call_args
    assert kwargs.get("device") == "auto" or (len(args) >= 3 and args[2] == "auto")


def test_transcribe_path_required(mock_recorder, mock_transcriber, client):
    r = client.post("/transcribe", json={})
    assert r.status_code == 422


def test_health_reports_vulkan_backend(mock_recorder, mock_transcriber, client):
    mock_transcriber.gpu_available = True
    mock_transcriber.gpu_name = "AMD Radeon RX 7600"
    mock_transcriber.gpu_vram_mb = 8192
    mock_transcriber.gpu_vendor = "amd"
    mock_transcriber.gpu_backend = "vulkan"
    mock_transcriber.vulkan_model_ready = True
    mock_transcriber.device = "vulkan"
    body = client.get("/health").json()
    assert body["gpu_vendor"] == "amd"
    assert body["gpu_backend"] == "vulkan"
    assert body["vulkan_model_ready"] is True
    assert body["device"] == "vulkan"


def test_transcribe_accepts_gpu_device(mock_recorder, mock_transcriber, client):
    mock_transcriber.transcribe.return_value = TranscribeResult(
        transcript="oi", language="pt", duration_seconds=1.0, model="medium", device="vulkan"
    )
    r = client.post("/transcribe", json={"path": "tmp/rec.wav", "device": "gpu"})
    assert r.status_code == 200
    assert r.json()["device"] == "vulkan"
    args, kwargs = mock_transcriber.transcribe.call_args
    assert kwargs.get("device") == "gpu"
