import os
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Optional

from faster_whisper import WhisperModel


@dataclass
class TranscribeResult:
    transcript: str
    language: str
    duration_seconds: float
    model: str


class Transcriber:
    def __init__(
        self,
        model_name: str,
        device: str,
        compute_type: str,
        default_language: str,
        recordings_dir: Path,
    ):
        self.model_name = model_name
        self.device = device
        self.default_language = default_language
        self.recordings_dir = Path(recordings_dir).resolve()
        self._setup_dll_paths()
        self._model = WhisperModel(model_name, device=device, compute_type=compute_type)
        self.model_loaded = True

    def _setup_dll_paths(self):
        if sys.platform != "win32":
            return
        for pkg in ("nvidia.cudnn", "nvidia.cublas"):
            try:
                module = __import__(pkg, fromlist=[""])
            except ImportError:
                continue
            base = Path(module.__file__).parent
            for candidate in (base / "bin", base / "lib"):
                if candidate.exists():
                    os.add_dll_directory(str(candidate))

    def transcribe(self, path: Path, language: Optional[str] = None) -> TranscribeResult:
        resolved = Path(path).resolve()
        if not str(resolved).startswith(str(self.recordings_dir)):
            raise ValueError(f"path outside recordings dir: {path}")
        if not resolved.exists():
            raise ValueError(f"path does not exist: {path}")

        lang = language or self.default_language
        segments, info = self._model.transcribe(str(resolved), language=lang)
        text = " ".join(seg.text.strip() for seg in segments).strip()
        return TranscribeResult(
            transcript=text,
            language=info.language,
            duration_seconds=info.duration,
            model=self.model_name,
        )
