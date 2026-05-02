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
            import ctranslate2
            cuda_available = ctranslate2.get_cuda_device_count() > 0
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
        import ctypes
        import importlib
        # Collect all NVIDIA DLL directories first so dependency resolution works
        # when we later pre-load individual DLLs (cublas depends on cudart, etc.)
        nvidia_dirs: list[Path] = []
        for pkg in ("nvidia.cuda_runtime", "nvidia.cudnn", "nvidia.cublas", "nvidia.cufft"):
            try:
                module = importlib.import_module(pkg)
            except ImportError:
                continue
            if module.__file__ is None:
                continue
            base = Path(module.__file__).parent
            for candidate in (base / "bin", base / "lib"):
                if candidate.exists():
                    nvidia_dirs.append(candidate)
        # Add all dirs to search path before loading any DLL
        for d in nvidia_dirs:
            try:
                self._dll_handles.append(os.add_dll_directory(str(d)))
            except Exception:
                pass
        # Pre-load each DLL now that all directories are in the search path
        for d in nvidia_dirs:
            for dll in d.glob("*.dll"):
                try:
                    ctypes.CDLL(str(dll))
                except Exception:
                    pass

    def transcribe(self, path: Path, language: Optional[str] = None) -> TranscribeResult:
        resolved = Path(path).resolve()
        try:
            resolved.relative_to(self.recordings_dir)
        except ValueError:
            raise ValueError(f"path outside recordings dir: {path}")
        if not resolved.exists():
            raise ValueError(f"path does not exist: {path}")

        lang = language or self.default_language
        try:
            segments, info = self._model.transcribe(str(resolved), language=lang)
        except Exception as e:
            err = str(e).lower()
            if self.device == "cuda" and any(kw in err for kw in ("dll", "cublas", "cudnn", "library", "not found", "cannot be loaded")):
                # GPU inference failed due to missing CUDA DLL — reload model on CPU
                import logging
                logging.warning("CUDA inference failed (%s), reloading model on CPU", e)
                self._model = WhisperModel(self.model_name, device="cpu", compute_type="int8")
                self.device = "cpu"
                segments, info = self._model.transcribe(str(resolved), language=lang)
            else:
                raise
        text = " ".join(seg.text.strip() for seg in segments).strip()
        return TranscribeResult(
            transcript=text,
            language=info.language,
            duration_seconds=info.duration,
            model=self.model_name,
        )
