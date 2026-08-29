import logging
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from transcriber import Transcriber, TranscribeResult


from contextlib import contextmanager


@contextmanager
def _make_transcriber(tmp_path, device="cuda", compute_type="int8_float16"):
    """Patches ficam ativos durante o corpo do teste: um mock que lança dentro
    de transcribe() jamais pode alcançar o WhisperModel real (download de GB)."""
    fake_model = MagicMock()
    with patch("transcriber.WhisperModel", return_value=fake_model) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch.object(Transcriber, "_resolve_device_compute", return_value=(device, compute_type)):
        t = Transcriber(
            model_name="medium",
            device=device,
            compute_type=compute_type,
            recordings_dir=tmp_path,
        )
        t._fake_model = fake_model
        t._mock_cls = mock_cls
        yield t


@pytest.fixture
def transcriber(tmp_path):
    with _make_transcriber(tmp_path) as t:
        yield t


def test_fixture_patch_active_during_test_body(transcriber, tmp_path):
    """Regressão do harness: WhisperModel deve continuar mockado no corpo do teste."""
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    transcriber._fake_model.transcribe.side_effect = RuntimeError("boom cuda")
    with pytest.raises(RuntimeError):
        # device é cuda: o except tenta recarregar em CPU — que DEVE bater no mock,
        # não no WhisperModel real. side_effect abaixo prova que bateu no mock.
        transcriber._mock_cls.side_effect = RuntimeError("segundo load também falha")
        transcriber.transcribe(wav)
    assert transcriber._mock_cls.call_count >= 1


def test_init_loads_model_and_sets_attributes(tmp_path):
    fake_model = MagicMock()
    with patch("transcriber.WhisperModel", return_value=fake_model) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths") as mock_setup, \
         patch.object(Transcriber, "_resolve_device_compute", return_value=("cuda", "int8_float16")):
        t = Transcriber("medium", "cuda", "int8_float16", tmp_path)
    mock_cls.assert_called_once_with("medium", device="cuda", compute_type="int8_float16")
    mock_setup.assert_called_once()
    assert t.model_loaded is True
    assert t.model_name == "medium"
    assert t.device == "cuda"


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


def test_transcribe_auto_passes_none_language(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    info = MagicMock()
    info.language = "en"
    info.duration = 5.0
    transcriber._fake_model.transcribe.return_value = (iter([]), info)

    result = transcriber.transcribe(wav, language="auto")

    args, kwargs = transcriber._fake_model.transcribe.call_args
    assert kwargs["language"] is None
    assert result.language == "en"


def test_transcribe_empty_string_passes_none_language(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    info = MagicMock()
    info.language = "es"
    info.duration = 5.0
    transcriber._fake_model.transcribe.return_value = (iter([]), info)

    transcriber.transcribe(wav, language="")

    args, kwargs = transcriber._fake_model.transcribe.call_args
    assert kwargs["language"] is None


def test_transcribe_none_passes_none_language(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    info = MagicMock()
    info.language = "pt"
    info.duration = 5.0
    transcriber._fake_model.transcribe.return_value = (iter([]), info)

    transcriber.transcribe(wav)

    args, kwargs = transcriber._fake_model.transcribe.call_args
    assert kwargs["language"] is None


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
        t = Transcriber("medium", "cuda", "int8_float16", tmp_path)
        t._model = gpu_model

        result = t.transcribe(wav)

    assert result.transcript == "fallback"
    assert t.device == "cpu"
    assert mock_cls.call_count == 2
    mock_cls.assert_called_with("medium", device="cpu", compute_type="int8")


def test_transcribe_cuda_oom_falls_back_to_cpu(tmp_path, caplog):
    """Falta de VRAM na GPU recarrega em CPU e transcreve, em vez de falhar a reunião."""
    cpu_seg = MagicMock()
    cpu_seg.text = "fallback"
    cpu_info = MagicMock()
    cpu_info.language = "pt"
    cpu_info.duration = 3.0

    cpu_model = MagicMock()
    cpu_model.transcribe.return_value = (iter([cpu_seg]), cpu_info)

    oom_message = "CUDA failed with error out of memory"
    gpu_model = MagicMock()
    gpu_model.transcribe.side_effect = RuntimeError(oom_message)

    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")

    with patch("transcriber.WhisperModel", side_effect=[gpu_model, cpu_model]) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch.object(Transcriber, "_resolve_device_compute", return_value=("cuda", "int8_float16")):
        t = Transcriber("medium", "cuda", "int8_float16", tmp_path)
        t._model = gpu_model

        with caplog.at_level(logging.WARNING):
            result = t.transcribe(wav)

    assert result.transcript == "fallback"
    assert t.device == "cpu"
    mock_cls.assert_called_with("medium", device="cpu", compute_type="int8")
    warnings = [r for r in caplog.records if r.levelno == logging.WARNING]
    assert len(warnings) == 1
    assert oom_message in warnings[0].getMessage()


def test_transcribe_error_on_cpu_propagates(tmp_path):
    """Em CPU não há para onde cair: o erro propaga em vez de recarregar em loop."""
    cpu_model = MagicMock()
    cpu_model.transcribe.side_effect = ValueError("invalid audio format")

    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")

    with patch("transcriber.WhisperModel", return_value=cpu_model) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch.object(Transcriber, "_resolve_device_compute", return_value=("cpu", "int8")):
        t = Transcriber("medium", "cpu", "int8", tmp_path)
        t._model = cpu_model

        with pytest.raises(ValueError, match="invalid audio format"):
            t.transcribe(wav)

    assert t.device == "cpu"
    assert mock_cls.call_count == 1


def test_transcribe_cpu_retry_failure_propagates(tmp_path):
    """Se a retentativa em CPU também falha, o erro sobe — nada é engolido."""
    gpu_model = MagicMock()
    gpu_model.transcribe.side_effect = RuntimeError("CUDA failed with error out of memory")

    cpu_model = MagicMock()
    cpu_model.transcribe.side_effect = ValueError("corrupt wav")

    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")

    with patch("transcriber.WhisperModel", side_effect=[gpu_model, cpu_model]), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch.object(Transcriber, "_resolve_device_compute", return_value=("cuda", "int8_float16")):
        t = Transcriber("medium", "cuda", "int8_float16", tmp_path)
        t._model = gpu_model

        with pytest.raises(ValueError, match="corrupt wav"):
            t.transcribe(wav)

    assert t.device == "cpu"


def test_setup_dll_paths_noop_on_non_windows(tmp_path, monkeypatch):
    """The DLL setup should be a no-op on non-Windows platforms."""
    monkeypatch.setattr(sys, "platform", "linux")
    fake_model = MagicMock()

    fake_add_dll = MagicMock()
    if hasattr(__import__("os"), "add_dll_directory"):
        monkeypatch.setattr("os.add_dll_directory", fake_add_dll)

    with patch("transcriber.WhisperModel", return_value=fake_model):
        t = Transcriber("medium", "cuda", "int8_float16", tmp_path)

    assert t.model_loaded is True
    fake_add_dll.assert_not_called()
