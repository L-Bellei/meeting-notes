import os
import sys
from pathlib import Path
from typing import Optional

from faster_whisper import WhisperModel

TRANSCRIBE_KWARGS = dict(
    # Evita realimentação de alucinação: cada trecho de 30s é decodificado
    # isoladamente, um segmento ruim não contamina os seguintes.
    condition_on_previous_text=False,
    compression_ratio_threshold=1.8,
    repetition_penalty=1.1,
)


class CT2Backend:
    def __init__(self, model_name: str, compute_type: str):
        self.model_name = model_name
        self.compute_type = compute_type
        self._models: dict[str, "WhisperModel"] = {}
        self._dll_handles = []

    def compute_for(self, device: str) -> str:
        if device == "cpu":
            # CPU sempre int8: um compute_type de GPU (int8_float16) herdado quebraria o modelo.
            return "int8"
        return self.compute_type if self.compute_type != "auto" else "int8_float16"

    def get_model(self, device: str):
        if device not in self._models:
            self._models[device] = WhisperModel(
                self.model_name, device=device, compute_type=self.compute_for(device)
            )
        return self._models[device]

    def transcribe(self, path: Path, lang: Optional[str], device: str) -> tuple[str, str, float]:
        model = self.get_model(device)
        segments, info = model.transcribe(str(path), language=lang, **TRANSCRIBE_KWARGS)
        text = " ".join(seg.text.strip() for seg in segments).strip()
        return text, info.language, info.duration

    def setup_dll_paths(self) -> None:
        if sys.platform != "win32":
            return
        import ctypes
        import importlib
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
        # Todos os diretórios entram no search path antes de qualquer carga:
        # cublas depende de cudart e a resolução precisa achar os dois.
        for d in nvidia_dirs:
            try:
                self._dll_handles.append(os.add_dll_directory(str(d)))
            except Exception:
                pass
        for d in nvidia_dirs:
            for dll in d.glob("*.dll"):
                try:
                    ctypes.CDLL(str(dll))
                except Exception:
                    pass
