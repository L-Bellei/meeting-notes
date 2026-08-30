import logging
import os
from dataclasses import dataclass
from pathlib import Path
from typing import Optional, Protocol

from backends.ct2 import CT2Backend
from backends.whispercpp import WhisperCppBackend, find_whispercli
from gpuscan import GPUInfo
from gpuscan import scan as scan_gpu


@dataclass
class TranscribeResult:
    transcript: str
    language: str
    duration_seconds: float
    model: str
    device: str = "cpu"


class VulkanBackend(Protocol):
    available: bool
    model_ready: bool

    def transcribe(self, path: Path, lang: Optional[str]) -> tuple[str, str, float]: ...


class Transcriber:
    def __init__(
        self,
        model_name: str,
        device: str,
        compute_type: str,
        recordings_dir: Path,
        *,
        ct2: Optional[CT2Backend] = None,
        vulkan: Optional[VulkanBackend] = None,
    ):
        self.model_name = model_name
        self.default_device = device
        self.compute_type = compute_type
        self.recordings_dir = Path(recordings_dir).resolve()
        self._ct2 = ct2 or CT2Backend(model_name, compute_type)
        self._vulkan = vulkan if vulkan is not None else WhisperCppBackend(model_name, find_whispercli())
        if not self._vulkan.available:
            logging.info("whisper-cli not found; Vulkan backend disabled (GPU non-NVIDIA falls back to CPU)")
        self._setup_dll_paths()
        self.gpu: GPUInfo = self._scan()
        first = self._chain(device)[0]
        if first in ("cuda", "cpu"):
            self._ct2.get_model(first)
        self.device = first
        self.model_loaded = True

    def _setup_dll_paths(self) -> None:
        self._ct2.setup_dll_paths()

    def _scan(self) -> GPUInfo:
        return scan_gpu(whispercli_present=bool(self._vulkan and self._vulkan.available))

    @property
    def gpu_available(self) -> bool:
        return self.gpu.available

    @property
    def gpu_name(self) -> Optional[str]:
        return self.gpu.name

    @property
    def gpu_vram_mb(self) -> Optional[int]:
        return self.gpu.vram_mb

    @property
    def gpu_vendor(self) -> Optional[str]:
        return self.gpu.vendor

    @property
    def gpu_backend(self) -> Optional[str]:
        return self.gpu.backend

    @property
    def vulkan_model_ready(self) -> bool:
        return bool(self._vulkan and self._vulkan.model_ready)

    def _chain(self, requested: Optional[str]) -> list[str]:
        if requested == "cpu":
            return ["cpu"]
        chain: list[str] = []
        if requested == "cuda":
            if self.gpu.cuda_available:
                chain.append("cuda")
                if self.gpu.vulkan_available:
                    chain.append("vulkan")
            return chain + ["cpu"]
        if os.getenv("WHISPER_FORCE_BACKEND") == "vulkan" and self.gpu.vulkan_available:
            return ["vulkan", "cpu"]
        if self.gpu.cuda_available:
            chain.append("cuda")
        if self.gpu.vulkan_available:
            chain.append("vulkan")
        return chain + ["cpu"]

    def _run(self, device: str, path: Path, lang: Optional[str]) -> tuple[str, str, float]:
        if device == "vulkan":
            return self._vulkan.transcribe(path, lang)
        return self._ct2.transcribe(path, lang, device)

    def transcribe(self, path: Path, language: Optional[str] = None, device: str = "auto") -> TranscribeResult:
        resolved = Path(path).resolve()
        try:
            resolved.relative_to(self.recordings_dir)
        except ValueError:
            raise ValueError(f"path outside recordings dir: {path}")
        if not resolved.exists():
            raise ValueError(f"path does not exist: {path}")

        lang = None if language in (None, "", "auto") else language
        chain = self._chain(device)
        for index, attempt in enumerate(chain):
            # Atribuído antes da tentativa: se a última também falhar, /health reporta onde parou.
            self.device = attempt
            try:
                text, detected, duration = self._run(attempt, resolved, lang)
            except Exception as e:
                if index == len(chain) - 1:
                    raise
                # Transcrição é o ativo primário: qualquer falha na GPU vale a próxima
                # tentativa da cadeia NESTA chamada; a próxima chamada re-resolve do zero.
                logging.warning("%s inference failed (%s), retrying this call on %s", attempt, e, chain[index + 1])
                continue
            return TranscribeResult(
                transcript=text,
                language=detected,
                duration_seconds=duration,
                model=self.model_name,
                device=attempt,
            )
        raise RuntimeError("unreachable: empty device chain")
