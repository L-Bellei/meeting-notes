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
