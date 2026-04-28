from pathlib import Path

import pytest

from recorder import Recorder, RecorderError, StopResult

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


@pytest.fixture
def recorder(tmp_path):
    fake_pa = _fake_pyaudio_with(default_input=True, loopback=True)
    with patch("recorder.pyaudio.PyAudio", return_value=fake_pa):
        yield Recorder(tmp_path)


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
