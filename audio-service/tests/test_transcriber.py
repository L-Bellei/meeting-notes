import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from transcriber import Transcriber, TranscribeResult


@pytest.fixture
def transcriber(tmp_path):
    """Build a Transcriber with WhisperModel and DLL setup mocked."""
    fake_model = MagicMock()
    with patch("transcriber.WhisperModel", return_value=fake_model), \
         patch.object(Transcriber, "_setup_dll_paths"):
        t = Transcriber(
            model_name="medium",
            device="cuda",
            compute_type="int8_float16",
            default_language="pt",
            recordings_dir=tmp_path,
        )
    t._fake_model = fake_model
    return t


def test_init_loads_model_and_sets_attributes(tmp_path):
    fake_model = MagicMock()
    with patch("transcriber.WhisperModel", return_value=fake_model) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths") as mock_setup:
        t = Transcriber("medium", "cuda", "int8_float16", "pt", tmp_path)
    mock_cls.assert_called_once_with("medium", device="cuda", compute_type="int8_float16")
    mock_setup.assert_called_once()
    assert t.model_loaded is True
    assert t.model_name == "medium"
    assert t.device == "cuda"
    assert t.default_language == "pt"


def test_transcribe_path_outside_recordings_dir_raises(transcriber, tmp_path):
    outside = tmp_path.parent / "elsewhere.wav"
    with pytest.raises(ValueError, match="outside recordings dir"):
        transcriber.transcribe(outside)


def test_transcribe_path_does_not_exist_raises(transcriber, tmp_path):
    missing = tmp_path / "missing.wav"
    with pytest.raises(ValueError, match="does not exist"):
        transcriber.transcribe(missing)


def test_transcribe_concatenates_segments(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    seg1 = MagicMock()
    seg1.text = " oi "
    seg2 = MagicMock()
    seg2.text = "mundo"
    info = MagicMock()
    info.language = "pt"
    info.duration = 10.5
    transcriber._fake_model.transcribe.return_value = (iter([seg1, seg2]), info)

    result = transcriber.transcribe(wav)

    assert isinstance(result, TranscribeResult)
    assert result.transcript == "oi mundo"
    assert result.language == "pt"
    assert result.duration_seconds == 10.5
    assert result.model == "medium"


def test_transcribe_uses_default_language_when_none_provided(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    info = MagicMock()
    info.language = "pt"
    info.duration = 5.0
    transcriber._fake_model.transcribe.return_value = (iter([]), info)

    transcriber.transcribe(wav)

    transcriber._fake_model.transcribe.assert_called_once()
    args, kwargs = transcriber._fake_model.transcribe.call_args
    assert kwargs["language"] == "pt"


def test_transcribe_uses_provided_language(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    info = MagicMock()
    info.language = "en"
    info.duration = 5.0
    transcriber._fake_model.transcribe.return_value = (iter([]), info)

    transcriber.transcribe(wav, language="en")

    args, kwargs = transcriber._fake_model.transcribe.call_args
    assert kwargs["language"] == "en"


def test_setup_dll_paths_noop_on_non_windows(tmp_path, monkeypatch):
    """The DLL setup should be a no-op on non-Windows platforms."""
    monkeypatch.setattr(sys, "platform", "linux")
    fake_model = MagicMock()
    with patch("transcriber.WhisperModel", return_value=fake_model):
        t = Transcriber("medium", "cuda", "int8_float16", "pt", tmp_path)
    assert t.model_loaded is True
