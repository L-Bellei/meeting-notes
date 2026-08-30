import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Callable, Optional

from huggingface_hub import hf_hub_download, try_to_load_from_cache

HF_REPO = "ggerganov/whisper.cpp"
GGML_FILES = {
    "tiny": "ggml-tiny-q5_1.bin",
    "base": "ggml-base-q5_1.bin",
    "small": "ggml-small-q5_1.bin",
    "medium": "ggml-medium-q5_0.bin",
    "large": "ggml-large-v3-q5_0.bin",
    "large-v3": "ggml-large-v3-q5_0.bin",
}
EXE_NAME = "whisper-cli.exe"
CREATE_NO_WINDOW = 0x08000000 if sys.platform == "win32" else 0
# Alinhado ao teto de 4h do cliente Go; um hang no driver Vulkan/CUDA trava o processo
# filho indefinidamente sem isso, e o timeout mata o filho e vira fallback para CPU.
TRANSCRIBE_TIMEOUT_SECONDS = 4 * 60 * 60


def _search_roots() -> list[Path]:
    roots = []
    frozen = getattr(sys, "_MEIPASS", None)
    if frozen:
        roots.append(Path(frozen))
    roots.append(Path(__file__).resolve().parent.parent / "vendor")
    return roots


def find_whispercli() -> Optional[Path]:
    override = os.getenv("WHISPER_CPP_BIN")
    if override:
        p = Path(override)
        return p if p.exists() else None
    for root in _search_roots():
        candidate = root / "whispercpp" / EXE_NAME
        if candidate.exists():
            return candidate
    return None


class WhisperCppBackend:
    def __init__(
        self,
        model_name: str,
        exe: Optional[Path] = None,
        *,
        runner: Callable = subprocess.run,
        downloader: Callable = hf_hub_download,
        cache_lookup: Callable = try_to_load_from_cache,
    ):
        self.model_name = model_name
        self.exe = Path(exe) if exe else None
        self._runner = runner
        self._downloader = downloader
        self._cache_lookup = cache_lookup
        self._model_path: Optional[Path] = None

    @property
    def available(self) -> bool:
        return bool(self.exe and self.exe.exists())

    @property
    def ggml_filename(self) -> str:
        return GGML_FILES.get(self.model_name, GGML_FILES["medium"])

    @property
    def model_ready(self) -> bool:
        if self._model_path is not None:
            return True
        cached = self._cache_lookup(HF_REPO, self.ggml_filename)
        return isinstance(cached, str) and Path(cached).exists()

    def model_path(self) -> Path:
        if self._model_path is None:
            self._model_path = Path(self._downloader(repo_id=HF_REPO, filename=self.ggml_filename))
        return self._model_path

    def build_command(self, model: Path, wav: Path, lang: Optional[str], out_prefix: Path) -> list[str]:
        return [
            str(self.exe),
            "-m", str(model),
            "-f", str(wav),
            "-l", lang or "auto",
            "-oj",
            "-of", str(out_prefix),
            "-np",
        ]

    def transcribe(self, path: Path, lang: Optional[str]) -> tuple[str, str, float]:
        if not self.available:
            raise RuntimeError("whisper-cli not available")
        model = self.model_path()
        with tempfile.TemporaryDirectory(prefix="whispercpp-") as tmp:
            out_prefix = Path(tmp) / "out"
            proc = self._runner(
                self.build_command(model, path, lang, out_prefix),
                capture_output=True, text=True, creationflags=CREATE_NO_WINDOW,
                timeout=TRANSCRIBE_TIMEOUT_SECONDS,
            )
            if proc.returncode != 0:
                raise RuntimeError(f"whisper-cli exited {proc.returncode}: {proc.stderr.strip()[-2000:]}")
            json_path = out_prefix.with_suffix(".json")
            if not json_path.exists():
                raise RuntimeError("whisper-cli produced no JSON output")
            data = json.loads(json_path.read_text(encoding="utf-8"))
        segments = data.get("transcription", [])
        text = " ".join(seg.get("text", "").strip() for seg in segments).strip()
        language = data.get("result", {}).get("language", lang or "")
        last_ms = max((seg.get("offsets", {}).get("to", 0) for seg in segments), default=0)
        return text, language, last_ms / 1000.0
