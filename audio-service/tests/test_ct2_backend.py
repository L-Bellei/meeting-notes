from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from backends.ct2 import CT2Backend


@pytest.fixture
def backend():
    with patch("backends.ct2.WhisperModel") as cls:
        b = CT2Backend("medium", "int8_float16")
        b._cls = cls
        yield b


def test_compute_for_cpu_is_int8_even_with_gpu_compute_type(backend):
    assert backend.compute_for("cpu") == "int8"
    assert backend.compute_for("cuda") == "int8_float16"


def test_compute_for_auto_maps_to_int8_float16_on_cuda():
    with patch("backends.ct2.WhisperModel"):
        b = CT2Backend("medium", "auto")
    assert b.compute_for("cuda") == "int8_float16"
    assert b.compute_for("cpu") == "int8"


def test_get_model_caches_per_device(backend):
    backend.get_model("cuda"); backend.get_model("cuda"); backend.get_model("cpu")
    assert backend._cls.call_count == 2
    backend._cls.assert_any_call("medium", device="cuda", compute_type="int8_float16")
    backend._cls.assert_any_call("medium", device="cpu", compute_type="int8")


def test_transcribe_concatenates_segments_and_returns_info(backend, tmp_path):
    seg1 = MagicMock(); seg1.text = " oi "
    seg2 = MagicMock(); seg2.text = "mundo"
    info = MagicMock(); info.language = "pt"; info.duration = 10.5
    backend._cls.return_value.transcribe.return_value = (iter([seg1, seg2]), info)

    text, language, duration = backend.transcribe(tmp_path / "rec.wav", None, "cpu")

    assert (text, language, duration) == ("oi mundo", "pt", 10.5)
    args, kwargs = backend._cls.return_value.transcribe.call_args
    assert args == (str(tmp_path / "rec.wav"),)
    assert kwargs["language"] is None
    assert kwargs["condition_on_previous_text"] is False
    assert kwargs["compression_ratio_threshold"] == 1.8
    assert kwargs["repetition_penalty"] == 1.1


def test_transcribe_passes_language(backend, tmp_path):
    info = MagicMock(); info.language = "en"; info.duration = 1.0
    backend._cls.return_value.transcribe.return_value = (iter([]), info)
    backend.transcribe(tmp_path / "rec.wav", "en", "cpu")
    assert backend._cls.return_value.transcribe.call_args.kwargs["language"] == "en"


def test_transcribe_surfaces_lazy_generator_errors(backend, tmp_path):
    def bad():
        raise RuntimeError("Library cublas64_12.dll is not found")
        yield
    backend._cls.return_value.transcribe.return_value = (bad(), MagicMock())
    with pytest.raises(RuntimeError, match="cublas64_12"):
        backend.transcribe(tmp_path / "rec.wav", None, "cuda")


def test_setup_dll_paths_noop_on_non_windows(monkeypatch):
    monkeypatch.setattr("backends.ct2.sys.platform", "linux")
    fake = MagicMock()
    if hasattr(__import__("os"), "add_dll_directory"):
        monkeypatch.setattr("os.add_dll_directory", fake)
    with patch("backends.ct2.WhisperModel"):
        CT2Backend("medium", "int8").setup_dll_paths()
    fake.assert_not_called()
