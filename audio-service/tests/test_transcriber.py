import logging
from contextlib import contextmanager
from unittest.mock import MagicMock, patch

import pytest

from gpuscan import GPUInfo
from transcriber import Transcriber, TranscribeResult


def _gpu(avail=True, name=None, vram=None):
    return GPUInfo(cuda_available=avail, vendor="nvidia" if avail else None, name=name, vram_mb=vram)


@contextmanager
def _make_transcriber(tmp_path, device="cuda", compute_type="int8_float16"):
    """Patches ficam ativos durante o corpo do teste: um mock que lança dentro
    de transcribe() jamais pode alcançar o WhisperModel real (download de GB)."""
    fake_model = MagicMock()
    with patch("backends.ct2.WhisperModel", return_value=fake_model) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(device == "cuda")):
        t = Transcriber(model_name="medium", device=device, compute_type=compute_type, recordings_dir=tmp_path)
        t._fake_model = fake_model
        t._mock_cls = mock_cls
        yield t


@pytest.fixture
def transcriber(tmp_path):
    with _make_transcriber(tmp_path) as t:
        yield t


def _info(lang="pt", dur=1.0):
    info = MagicMock(); info.language = lang; info.duration = dur
    return info


def test_fixture_patch_active_during_test_body(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
    transcriber._fake_model.transcribe.side_effect = RuntimeError("boom cuda")
    transcriber._mock_cls.side_effect = RuntimeError("segundo load também falha")
    with pytest.raises(RuntimeError, match="segundo load também falha"):
        transcriber.transcribe(wav)
    assert transcriber._mock_cls.call_count == 2


def test_init_loads_model_and_sets_attributes(tmp_path):
    with patch("backends.ct2.WhisperModel", return_value=MagicMock()) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths") as mock_setup, \
         patch("transcriber.scan_gpu", return_value=_gpu(True, "NVIDIA GeForce RTX 2050", 4096)):
        t = Transcriber("medium", "cuda", "int8_float16", tmp_path)
    mock_cls.assert_called_once_with("medium", device="cuda", compute_type="int8_float16")
    mock_setup.assert_called_once()
    assert t.model_loaded is True
    assert t.model_name == "medium"
    assert t.device == "cuda"


def test_scan_exposed_on_attributes(tmp_path):
    with patch("backends.ct2.WhisperModel", return_value=MagicMock()), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(True, "NVIDIA GeForce RTX 2050", 4096)):
        t = Transcriber("medium", "auto", "auto", tmp_path)
    assert t.gpu_available is True
    assert t.gpu_name == "NVIDIA GeForce RTX 2050"
    assert t.gpu_vram_mb == 4096
    assert t.gpu_vendor == "nvidia"
    assert t.gpu_backend == "cuda"
    assert t.vulkan_model_ready is False
    assert t.device == "cuda"


def test_transcribe_path_outside_recordings_dir_raises(transcriber, tmp_path):
    with pytest.raises(ValueError, match="outside recordings dir"):
        transcriber.transcribe(tmp_path.parent / "elsewhere.wav")


def test_transcribe_path_does_not_exist_raises(transcriber, tmp_path):
    with pytest.raises(ValueError, match="does not exist"):
        transcriber.transcribe(tmp_path / "missing.wav")


def test_transcribe_returns_result(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
    seg = MagicMock(); seg.text = " oi mundo "
    transcriber._fake_model.transcribe.return_value = (iter([seg]), _info("pt", 10.5))

    result = transcriber.transcribe(wav)

    assert result == TranscribeResult("oi mundo", "pt", 10.5, "medium", "cuda")


@pytest.mark.parametrize("language,expected", [("auto", None), ("", None), (None, None), ("en", "en")])
def test_transcribe_language_normalisation(transcriber, tmp_path, language, expected):
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
    transcriber._fake_model.transcribe.return_value = (iter([]), _info("en"))
    transcriber.transcribe(wav, language=language)
    assert transcriber._fake_model.transcribe.call_args.kwargs["language"] == expected


def test_transcribe_uses_cpu_when_device_cpu_requested(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
    transcriber._fake_model.transcribe.return_value = (iter([]), _info())
    result = transcriber.transcribe(wav, device="cpu")
    assert result.device == "cpu"
    assert transcriber._mock_cls.call_count == 2
    transcriber._mock_cls.assert_called_with("medium", device="cpu", compute_type="int8")


def test_transcribe_gpu_alias_resolves_to_cuda(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
    transcriber._fake_model.transcribe.return_value = (iter([]), _info())
    assert transcriber.transcribe(wav, device="gpu").device == "cuda"


def test_transcribe_model_cache_reuses_per_device(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
    transcriber._fake_model.transcribe.return_value = (iter([]), _info())
    transcriber.transcribe(wav); transcriber.transcribe(wav)
    assert transcriber._mock_cls.call_count == 1


def _factory(cuda_side_effect, cpu_side_effect):
    def model_factory(name, device, compute_type):
        m = MagicMock()
        m.transcribe.side_effect = cuda_side_effect if device == "cuda" else cpu_side_effect
        return m
    return model_factory


def test_transcribe_fallback_does_not_stick(tmp_path):
    calls = []
    def flaky(*a, **k):
        calls.append("cuda")
        if calls.count("cuda") == 1:
            raise RuntimeError("CUDA failed with error out of memory")
        return (iter([]), _info())
    def ok(*a, **k):
        calls.append("cpu"); return (iter([]), _info())

    with patch("backends.ct2.WhisperModel", side_effect=_factory(flaky, ok)), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(True, "RTX", 4096)):
        t = Transcriber("medium", "auto", "auto", tmp_path)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        r1 = t.transcribe(wav)
        r2 = t.transcribe(wav)

    assert (r1.device, r2.device) == ("cpu", "cuda")
    assert calls == ["cuda", "cpu", "cuda"]


def test_transcribe_cuda_error_falls_back_to_cpu_and_logs_once(tmp_path, caplog):
    seg = MagicMock(); seg.text = "fallback"
    with patch("backends.ct2.WhisperModel",
               side_effect=_factory(RuntimeError("CUDA failed with error out of memory"),
                                    lambda *a, **k: (iter([seg]), _info("pt", 3.0)))) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(True, "RTX", 4096)):
        t = Transcriber("medium", "cuda", "int8_float16", tmp_path)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        with caplog.at_level(logging.WARNING):
            result = t.transcribe(wav)

    assert result.transcript == "fallback"
    assert result.device == "cpu"
    assert t.device == "cpu"
    mock_cls.assert_called_with("medium", device="cpu", compute_type="int8")
    warnings = [r for r in caplog.records if r.levelno == logging.WARNING]
    assert len(warnings) == 1
    assert "out of memory" in warnings[0].getMessage()


def test_transcribe_error_on_cpu_propagates(tmp_path):
    cpu_model = MagicMock(); cpu_model.transcribe.side_effect = ValueError("invalid audio format")
    with patch("backends.ct2.WhisperModel", return_value=cpu_model) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(False)):
        t = Transcriber("medium", "cpu", "int8", tmp_path)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        with pytest.raises(ValueError, match="invalid audio format"):
            t.transcribe(wav)
    assert t.device == "cpu"
    assert mock_cls.call_count == 1


def test_transcribe_cpu_retry_failure_propagates(tmp_path):
    with patch("backends.ct2.WhisperModel",
               side_effect=_factory(RuntimeError("CUDA failed with error out of memory"),
                                    ValueError("corrupt wav"))), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(True, "RTX", 4096)):
        t = Transcriber("medium", "cuda", "int8_float16", tmp_path)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        with pytest.raises(ValueError, match="corrupt wav"):
            t.transcribe(wav)
    assert t.device == "cpu"


def test_chain_without_gpu_is_cpu_only(tmp_path):
    with patch("backends.ct2.WhisperModel", return_value=MagicMock()), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(False)):
        t = Transcriber("medium", "auto", "auto", tmp_path)
    assert t._chain("auto") == ["cpu"]
    assert t._chain("gpu") == ["cpu"]
    assert t._chain("cuda") == ["cpu"]
    assert t._chain("cpu") == ["cpu"]
    assert t.gpu_backend is None


def test_chain_with_cuda(tmp_path):
    with patch("backends.ct2.WhisperModel", return_value=MagicMock()), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(True)):
        t = Transcriber("medium", "auto", "auto", tmp_path)
    assert t._chain("auto") == ["cuda", "cpu"]
    assert t._chain("gpu") == ["cuda", "cpu"]
    assert t._chain("cuda") == ["cuda", "cpu"]
    assert t._chain("cpu") == ["cpu"]


class FakeVulkan:
    def __init__(self, available=True, model_ready=False, side_effect=None, result=("vk text", "pt", 4.0)):
        self.available = available
        self.model_ready = model_ready
        self.side_effect = side_effect
        self.result = result
        self.calls = []

    def transcribe(self, path, lang):
        self.calls.append((path, lang))
        if self.side_effect:
            raise self.side_effect
        return self.result


def _amd():
    return GPUInfo(cuda_available=False, vulkan_available=True, vendor="amd", name="AMD Radeon RX 7600", vram_mb=8192)


def _nvidia_both():
    return GPUInfo(cuda_available=True, vulkan_available=True, vendor="nvidia", name="RTX", vram_mb=4096)


def _make_with_vulkan(tmp_path, gpu, vulkan, device="auto", cpu_side_effect=None):
    cpu_model = MagicMock()
    cpu_model.transcribe.side_effect = cpu_side_effect or (lambda *a, **k: (iter([]), _info("pt", 1.0)))
    patches = (
        patch("backends.ct2.WhisperModel", return_value=cpu_model),
        patch.object(Transcriber, "_setup_dll_paths"),
        patch("transcriber.scan_gpu", return_value=gpu),
    )
    return patches, cpu_model


def test_default_vulkan_backend_is_built_from_find_whispercli(tmp_path):
    with patch("backends.ct2.WhisperModel", return_value=MagicMock()), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_gpu(False)) as scan, \
         patch("transcriber.find_whispercli", return_value=None):
        t = Transcriber("medium", "auto", "auto", tmp_path)
    assert t._vulkan is not None
    assert t._vulkan.available is False
    assert scan.call_args.kwargs["whispercli_present"] is False


def test_scan_receives_whispercli_presence(tmp_path):
    with patch("backends.ct2.WhisperModel", return_value=MagicMock()), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_amd()) as scan:
        Transcriber("medium", "auto", "auto", tmp_path, vulkan=FakeVulkan(available=True))
    assert scan.call_args.kwargs["whispercli_present"] is True


def test_amd_auto_uses_vulkan_and_reports_it(tmp_path):
    vk = FakeVulkan(model_ready=True)
    patches, cpu_model = _make_with_vulkan(tmp_path, _amd(), vk)
    with patches[0], patches[1], patches[2]:
        t = Transcriber("medium", "auto", "auto", tmp_path, vulkan=vk)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        result = t.transcribe(wav, language="pt")

    assert t.device == "vulkan"
    assert t.gpu_backend == "vulkan"
    assert t.gpu_vendor == "amd"
    assert t.vulkan_model_ready is True
    assert result == TranscribeResult("vk text", "pt", 4.0, "medium", "vulkan")
    assert vk.calls == [(wav.resolve(), "pt")]
    cpu_model.transcribe.assert_not_called()


def test_amd_boot_does_not_preload_ct2_model(tmp_path):
    with patch("backends.ct2.WhisperModel") as cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_amd()):
        t = Transcriber("medium", "auto", "auto", tmp_path, vulkan=FakeVulkan())
    cls.assert_not_called()
    assert t.model_loaded is True
    assert t.device == "vulkan"


def test_amd_chains():
    class _T(Transcriber):
        def __init__(self): self.gpu = _amd()
    t = _T()
    assert t._chain("auto") == ["vulkan", "cpu"]
    assert t._chain("gpu") == ["vulkan", "cpu"]
    assert t._chain("cuda") == ["cpu"]
    assert t._chain("cpu") == ["cpu"]


def test_nvidia_with_both_chains_cuda_then_vulkan(monkeypatch):
    monkeypatch.delenv("WHISPER_FORCE_BACKEND", raising=False)
    class _T(Transcriber):
        def __init__(self): self.gpu = _nvidia_both()
    t = _T()
    assert t._chain("auto") == ["cuda", "vulkan", "cpu"]
    assert t._chain("cuda") == ["cuda", "vulkan", "cpu"]


def test_force_backend_vulkan_env(monkeypatch):
    monkeypatch.setenv("WHISPER_FORCE_BACKEND", "vulkan")
    class _T(Transcriber):
        def __init__(self): self.gpu = _nvidia_both()
    t = _T()
    assert t._chain("auto") == ["vulkan", "cpu"]
    assert t._chain("gpu") == ["vulkan", "cpu"]
    assert t._chain("cuda") == ["cuda", "vulkan", "cpu"]
    assert t._chain("cpu") == ["cpu"]


def test_force_backend_ignored_when_vulkan_unavailable(monkeypatch):
    monkeypatch.setenv("WHISPER_FORCE_BACKEND", "vulkan")
    class _T(Transcriber):
        def __init__(self): self.gpu = _gpu(True)
    assert _T()._chain("auto") == ["cuda", "cpu"]


def test_vulkan_failure_falls_back_to_cpu_in_same_call(tmp_path, caplog):
    vk = FakeVulkan(side_effect=RuntimeError("whisper-cli exited 1: device lost"))
    patches, cpu_model = _make_with_vulkan(tmp_path, _amd(), vk)
    with patches[0], patches[1], patches[2]:
        t = Transcriber("medium", "auto", "auto", tmp_path, vulkan=vk)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        with caplog.at_level(logging.WARNING):
            result = t.transcribe(wav)

    assert result.device == "cpu"
    assert t.device == "cpu"
    cpu_model.transcribe.assert_called_once()
    assert any("vulkan inference failed" in r.getMessage() for r in caplog.records)


def test_vulkan_failure_does_not_stick(tmp_path):
    vk = FakeVulkan()
    vk.side_effect = RuntimeError("first fails")
    patches, cpu_model = _make_with_vulkan(tmp_path, _amd(), vk)
    with patches[0], patches[1], patches[2]:
        t = Transcriber("medium", "auto", "auto", tmp_path, vulkan=vk)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        r1 = t.transcribe(wav)
        vk.side_effect = None
        r2 = t.transcribe(wav)
    assert (r1.device, r2.device) == ("cpu", "vulkan")
    assert len(vk.calls) == 2


def test_cuda_failure_tries_vulkan_before_cpu(tmp_path):
    vk = FakeVulkan()
    cuda_model = MagicMock(); cuda_model.transcribe.side_effect = RuntimeError("CUDA out of memory")
    def factory(name, device, compute_type):
        return cuda_model
    with patch("backends.ct2.WhisperModel", side_effect=factory), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch("transcriber.scan_gpu", return_value=_nvidia_both()):
        t = Transcriber("medium", "auto", "auto", tmp_path, vulkan=vk)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        result = t.transcribe(wav)
    assert result.device == "vulkan"
    assert len(vk.calls) == 1


def test_vulkan_then_cpu_both_fail_propagates_cpu_error(tmp_path):
    vk = FakeVulkan(side_effect=RuntimeError("vk down"))
    patches, cpu_model = _make_with_vulkan(tmp_path, _amd(), vk, cpu_side_effect=ValueError("corrupt wav"))
    with patches[0], patches[1], patches[2]:
        t = Transcriber("medium", "auto", "auto", tmp_path, vulkan=vk)
        wav = tmp_path / "rec.wav"; wav.write_bytes(b"fake")
        with pytest.raises(ValueError, match="corrupt wav"):
            t.transcribe(wav)
    assert t.device == "cpu"
