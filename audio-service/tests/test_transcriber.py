import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from transcriber import Transcriber, TranscribeResult


def _make_transcriber(tmp_path, device="cuda", compute_type="int8_float16"):
    """Build a Transcriber with WhisperModel, DLL setup, and device resolution mocked."""
    fake_model = MagicMock()
    with patch("transcriber.WhisperModel", return_value=fake_model), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch.object(Transcriber, "_resolve_device_compute", return_value=(device, compute_type)):
        t = Transcriber(
            model_name="medium",
            device=device,
            compute_type=compute_type,
            default_language="pt",
            recordings_dir=tmp_path,
        )
    t._fake_model = fake_model
    return t


@pytest.fixture
def transcriber(tmp_path):
    return _make_transcriber(tmp_path)


def test_init_loads_model_and_sets_attributes(tmp_path):
    fake_model = MagicMock()
    with patch("transcriber.WhisperModel", return_value=fake_model) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths") as mock_setup, \
         patch.object(Transcriber, "_resolve_device_compute", return_value=("cuda", "int8_float16")):
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


def test_transcribe_cuda_dll_error_falls_back_to_cpu(tmp_path):
    """When CUDA inference fails with a DLL error (lazy generator), model reloads on CPU and retries."""
    cpu_seg = MagicMock()
    cpu_seg.text = "fallback"
    cpu_info = MagicMock()
    cpu_info.language = "pt"
    cpu_info.duration = 3.0

    cpu_model = MagicMock()
    cpu_model.transcribe.return_value = (iter([cpu_seg]), cpu_info)

    def bad_segments():
        raise RuntimeError("Library cublas64_12.dll is not found or cannot be loaded")
        yield  # make it a generator

    gpu_info = MagicMock()
    gpu_model = MagicMock()
    gpu_model.transcribe.return_value = (bad_segments(), gpu_info)

    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")

    with patch("transcriber.WhisperModel", side_effect=[gpu_model, cpu_model]) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch.object(Transcriber, "_resolve_device_compute", return_value=("cuda", "int8_float16")):
        t = Transcriber("medium", "cuda", "int8_float16", "pt", tmp_path)
        t._model = gpu_model

        result = t.transcribe(wav)

    assert result.transcript == "fallback"
    assert t.device == "cpu"
    assert mock_cls.call_count == 2
    mock_cls.assert_called_with("medium", device="cpu", compute_type="int8")


def test_transcribe_non_dll_error_propagates(tmp_path):
    """Non-DLL errors from GPU inference are re-raised without CPU fallback."""
    gpu_model = MagicMock()
    gpu_model.transcribe.side_effect = ValueError("invalid audio format")

    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")

    with patch("transcriber.WhisperModel", return_value=gpu_model), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch.object(Transcriber, "_resolve_device_compute", return_value=("cuda", "int8_float16")):
        t = Transcriber("medium", "cuda", "int8_float16", "pt", tmp_path)
        t._model = gpu_model

        with pytest.raises(ValueError, match="invalid audio format"):
            t.transcribe(wav)

    assert t.device == "cuda"


def test_setup_dll_paths_noop_on_non_windows(tmp_path, monkeypatch):
    """The DLL setup should be a no-op on non-Windows platforms."""
    monkeypatch.setattr(sys, "platform", "linux")
    fake_model = MagicMock()

    fake_add_dll = MagicMock()
    if hasattr(__import__("os"), "add_dll_directory"):
        monkeypatch.setattr("os.add_dll_directory", fake_add_dll)

    with patch("transcriber.WhisperModel", return_value=fake_model):
        t = Transcriber("medium", "cuda", "int8_float16", "pt", tmp_path)

    assert t.model_loaded is True
    fake_add_dll.assert_not_called()
