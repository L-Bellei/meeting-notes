import json
import subprocess
from pathlib import Path
from unittest.mock import MagicMock

import pytest

from backends.whispercpp import GGML_FILES, HF_REPO, TRANSCRIBE_TIMEOUT_SECONDS, WhisperCppBackend, find_whispercli


def _json(text_parts, language="pt", last_to_ms=12500):
    return {
        "result": {"language": language},
        "transcription": [
            {"text": t, "offsets": {"from": i * 1000, "to": last_to_ms if i == len(text_parts) - 1 else (i + 1) * 1000}}
            for i, t in enumerate(text_parts)
        ],
    }


@pytest.fixture
def exe(tmp_path):
    p = tmp_path / "vendor" / "whisper-cli.exe"
    p.parent.mkdir(parents=True)
    p.write_bytes(b"MZ")
    return p


@pytest.fixture
def model_file(tmp_path):
    m = tmp_path / "hf" / "ggml-medium-q5_0.bin"
    m.parent.mkdir(parents=True)
    m.write_bytes(b"ggml")
    return m


def _runner_writing(json_obj, returncode=0, stderr=""):
    def run(cmd, **kwargs):
        of = Path(cmd[cmd.index("-of") + 1])
        if json_obj is not None:
            of.with_suffix(".json").write_text(json.dumps(json_obj), encoding="utf-8")
        return subprocess.CompletedProcess(cmd, returncode, stdout="", stderr=stderr)
    return MagicMock(side_effect=run)


def test_ggml_map_is_quantized_and_large_maps_to_v3():
    assert GGML_FILES["medium"] == "ggml-medium-q5_0.bin"
    assert GGML_FILES["small"] == "ggml-small-q5_1.bin"
    assert GGML_FILES["large"] == "ggml-large-v3-q5_0.bin"
    assert HF_REPO == "ggerganov/whisper.cpp"


def test_available_reflects_exe_presence(exe, tmp_path):
    assert WhisperCppBackend("medium", exe).available is True
    assert WhisperCppBackend("medium", tmp_path / "nope.exe").available is False
    assert WhisperCppBackend("medium", None).available is False


def test_find_whispercli_prefers_env_override(monkeypatch, exe):
    monkeypatch.setenv("WHISPER_CPP_BIN", str(exe))
    assert find_whispercli() == exe


def test_find_whispercli_returns_none_when_absent(monkeypatch, tmp_path):
    monkeypatch.delenv("WHISPER_CPP_BIN", raising=False)
    monkeypatch.setattr("backends.whispercpp._search_roots", lambda: [tmp_path])
    assert find_whispercli() is None


def test_find_whispercli_in_search_root(monkeypatch, tmp_path):
    monkeypatch.delenv("WHISPER_CPP_BIN", raising=False)
    p = tmp_path / "whispercpp" / "whisper-cli.exe"
    p.parent.mkdir(); p.write_bytes(b"MZ")
    monkeypatch.setattr("backends.whispercpp._search_roots", lambda: [tmp_path])
    assert find_whispercli() == p


def test_model_ready_uses_cache_lookup(exe, model_file):
    b = WhisperCppBackend("medium", exe, cache_lookup=lambda repo, fn: str(model_file))
    assert b.model_ready is True
    b2 = WhisperCppBackend("medium", exe, cache_lookup=lambda repo, fn: None)
    assert b2.model_ready is False


def test_model_path_downloads_once(exe, model_file):
    downloader = MagicMock(return_value=str(model_file))
    b = WhisperCppBackend("medium", exe, downloader=downloader, cache_lookup=lambda r, f: None)
    assert b.model_path() == model_file
    assert b.model_path() == model_file
    downloader.assert_called_once_with(repo_id=HF_REPO, filename="ggml-medium-q5_0.bin")


def test_build_command(exe, model_file, tmp_path):
    b = WhisperCppBackend("medium", exe)
    cmd = b.build_command(model_file, tmp_path / "rec.wav", "pt", tmp_path / "out")
    assert cmd[0] == str(exe)
    assert cmd[cmd.index("-m") + 1] == str(model_file)
    assert cmd[cmd.index("-f") + 1] == str(tmp_path / "rec.wav")
    assert cmd[cmd.index("-l") + 1] == "pt"
    assert cmd[cmd.index("-of") + 1] == str(tmp_path / "out")
    assert "-oj" in cmd and "-np" in cmd


def test_build_command_language_none_is_auto(exe, model_file, tmp_path):
    cmd = WhisperCppBackend("medium", exe).build_command(model_file, tmp_path / "r.wav", None, tmp_path / "o")
    assert cmd[cmd.index("-l") + 1] == "auto"


def test_transcribe_parses_json(exe, model_file, tmp_path):
    runner = _runner_writing(_json([" Olá ", " mundo "], "pt", 12500))
    b = WhisperCppBackend("medium", exe, runner=runner, downloader=lambda **k: str(model_file),
                          cache_lookup=lambda r, f: None)
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"RIFF")

    text, language, duration = b.transcribe(wav, None)

    assert (text, language, duration) == ("Olá mundo", "pt", 12.5)
    kwargs = runner.call_args.kwargs
    assert kwargs["capture_output"] is True
    assert kwargs["text"] is True
    assert kwargs.get("creationflags", 0) == 0x08000000
    assert kwargs["timeout"] == TRANSCRIBE_TIMEOUT_SECONDS


def test_transcribe_raises_on_nonzero_exit_with_stderr(exe, model_file, tmp_path):
    runner = _runner_writing(None, returncode=1, stderr="ggml_vulkan: device lost")
    b = WhisperCppBackend("medium", exe, runner=runner, downloader=lambda **k: str(model_file),
                          cache_lookup=lambda r, f: None)
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"RIFF")
    with pytest.raises(RuntimeError, match="device lost"):
        b.transcribe(wav, None)


def test_transcribe_raises_when_json_missing(exe, model_file, tmp_path):
    runner = _runner_writing(None, returncode=0)
    b = WhisperCppBackend("medium", exe, runner=runner, downloader=lambda **k: str(model_file),
                          cache_lookup=lambda r, f: None)
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"RIFF")
    with pytest.raises(RuntimeError, match="no JSON output"):
        b.transcribe(wav, None)


def test_transcribe_raises_when_unavailable(tmp_path):
    b = WhisperCppBackend("medium", None)
    with pytest.raises(RuntimeError, match="whisper-cli not available"):
        b.transcribe(tmp_path / "rec.wav", None)


def test_transcribe_cleans_temp_output(exe, model_file, tmp_path, monkeypatch):
    created = []
    runner = _runner_writing(_json(["x"]))
    b = WhisperCppBackend("medium", exe, runner=runner, downloader=lambda **k: str(model_file),
                          cache_lookup=lambda r, f: None)
    wav = tmp_path / "rec.wav"; wav.write_bytes(b"RIFF")
    b.transcribe(wav, None)
    of = Path(runner.call_args.args[0][runner.call_args.args[0].index("-of") + 1])
    assert not of.parent.exists()
