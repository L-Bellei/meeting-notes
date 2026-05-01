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
        effective_device, effective_compute = self._resolve_device_compute(device, compute_type)
        self.device = effective_device
        self._model = WhisperModel(model_name, device=effective_device, compute_type=effective_compute)
        self.model_loaded = True

    def _resolve_device_compute(self, device: str, compute_type: str) -> tuple[str, str]:
        if device not in ("auto", "cuda"):
            return device, compute_type

        cuda_available = False
        try:
            import torch
            cuda_available = torch.cuda.is_available()
        except Exception:
            pass

        effective_device = "cuda" if cuda_available else "cpu"
        if compute_type == "auto":
            effective_compute = "int8_float16" if cuda_available else "int8"
        else:
            effective_compute = compute_type
        return effective_device, effective_compute

    def _setup_dll_paths(self):
        self._dll_handles = []
        if sys.platform != "win32":
            return
        import importlib
        extra_paths = []
        for pkg in ("nvidia.cudnn", "nvidia.cublas"):
            try:
                module = importlib.import_module(pkg)
            except ImportError:
                continue
            if module.__file__ is None:
                continue
            base = Path(module.__file__).parent
            for candidate in (base / "bin", base / "lib"):
                if candidate.exists():
                    self._dll_handles.append(os.add_dll_directory(str(candidate)))
                    extra_paths.append(str(candidate))
        if extra_paths:
            os.environ["PATH"] = os.pathsep.join(extra_paths) + os.pathsep + os.environ.get("PATH", "")

    def transcribe(self, path: Path, language: Optional[str] = None) -> TranscribeResult:
        resolved = Path(path).resolve()
        try:
            resolved.relative_to(self.recordings_dir)
        except ValueError:
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
