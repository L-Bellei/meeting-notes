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
    device: str = "cpu"


class Transcriber:
    def __init__(
        self,
        model_name: str,
        device: str,
        compute_type: str,
        recordings_dir: Path,
    ):
        self.model_name = model_name
        self.default_device = device
        self.compute_type = compute_type
        self.recordings_dir = Path(recordings_dir).resolve()
        self._setup_dll_paths()
        self.gpu_available, self.gpu_name, self.gpu_vram_mb = self._scan_gpu()
        self._models: dict[str, "WhisperModel"] = {}
        effective = self._effective_device(device)
        self._get_model(effective)
        self.device = effective
        self.model_loaded = True

    def _effective_device(self, requested: str) -> str:
        if requested in (None, "", "auto", "cuda") and self.gpu_available:
            return "cuda"
        return "cpu"

    def _compute_for(self, device: str) -> str:
        if device == "cpu":
            # Fallback e CPU explícito sempre int8: um compute_type de GPU
            # (int8_float16) herdado quebraria o modelo de CPU.
            return "int8"
        return self.compute_type if self.compute_type != "auto" else "int8_float16"

    def _get_model(self, device: str):
        if device not in self._models:
            self._models[device] = WhisperModel(
                self.model_name, device=device, compute_type=self._compute_for(device)
            )
        return self._models[device]

    def _scan_gpu(self):
        available = False
        try:
            import ctranslate2
            if ctranslate2.get_cuda_device_count() > 0:
                # Verify the CUDA compute DLLs are actually loadable before committing to GPU
                import ctypes
                for dll in ("cublas64_12.dll", "cublas64_11.dll"):
                    try:
                        ctypes.CDLL(dll)
                        available = True
                        break
                    except OSError:
                        continue
        except Exception:
            pass
        name = None
        vram = None
        if available:
            try:
                import subprocess
                flags = 0x08000000 if sys.platform == "win32" else 0
                out = subprocess.run(
                    ["nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits"],
                    capture_output=True, text=True, timeout=5, creationflags=flags,
                )
                first = out.stdout.strip().splitlines()[0]
                raw_name, raw_mem = first.rsplit(",", 1)
                name = raw_name.strip()
                vram = int(float(raw_mem.strip()))
            except Exception:
                pass
        return available, name, vram

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

    def transcribe(self, path: Path, language: Optional[str] = None, device: str = "auto") -> TranscribeResult:
        resolved = Path(path).resolve()
        try:
            resolved.relative_to(self.recordings_dir)
        except ValueError:
            raise ValueError(f"path outside recordings dir: {path}")
        if not resolved.exists():
            raise ValueError(f"path does not exist: {path}")

        # "", "auto" and None all mean "detect": faster-whisper auto-detects
        # when language is None and returns the result in info.language.
        lang = None if language in (None, "", "auto") else language
        transcribe_kwargs = dict(
            language=lang,
            # Prevents hallucination feedback loops: each 30s chunk is decoded
            # independently, so a bad segment can't poison subsequent ones.
            condition_on_previous_text=False,
            # Discard segments that are already highly repetitive internally.
            compression_ratio_threshold=1.8,
            # Small penalty for token repetition within a segment.
            repetition_penalty=1.1,
        )
        effective = self._effective_device(device)
        model = self._get_model(effective)
        try:
            segments, info = model.transcribe(str(resolved), **transcribe_kwargs)
            # Consume the generator inside the try block — errors from lazy CUDA ops surface here
            text = " ".join(seg.text.strip() for seg in segments).strip()
        except Exception as e:
            if effective != "cuda":
                raise
            # Transcrição é o ativo primário: qualquer falha na GPU vale uma
            # retentativa em CPU nesta chamada; a resolução NÃO fica pegajosa —
            # a próxima chamada re-resolve e retenta CUDA.
            import logging
            logging.warning("GPU inference failed (%s), retrying this call on CPU", e)
            effective = "cpu"
            self.device = effective
            model = self._get_model("cpu")
            segments, info = model.transcribe(str(resolved), **transcribe_kwargs)
            text = " ".join(seg.text.strip() for seg in segments).strip()
        self.device = effective
        return TranscribeResult(
            transcript=text,
            language=info.language,
            duration_seconds=info.duration,
            model=self.model_name,
            device=effective,
        )
