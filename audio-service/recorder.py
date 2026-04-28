import threading
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

import pyaudiowpatch as pyaudio


class RecorderError(Exception):
    pass


@dataclass
class StopResult:
    recording_id: str
    path: Path
    duration_seconds: float
    size_bytes: int
    partial: bool


class Recorder:
    def __init__(self, recordings_dir):
        self.recordings_dir = Path(recordings_dir)
        self.recordings_dir.mkdir(parents=True, exist_ok=True)

        self._lock = threading.Lock()
        self._state = "idle"
        self._recording_id = None
        self._started_at = None
        self._path = None
        self._partial = False

        self._pa = pyaudio.PyAudio()
        self._mic_info = None
        self._loopback_info = None
        self.mic_available = False
        self.loopback_available = False
        self._enumerate_devices()

    def _enumerate_devices(self):
        try:
            mic = self._pa.get_default_input_device_info()
            self._mic_info = mic
            self.mic_available = True
        except (OSError, IOError):
            self.mic_available = False

        try:
            speakers = self._pa.get_default_output_device_info()
            for lb in self._pa.get_loopback_device_info_generator():
                if speakers["name"] in lb["name"]:
                    self._loopback_info = lb
                    self.loopback_available = True
                    break
        except (OSError, IOError):
            self.loopback_available = False

    @property
    def state(self):
        with self._lock:
            return self._state

    def status(self):
        with self._lock:
            return {
                "state": self._state,
                "recording_id": self._recording_id,
                "started_at": self._started_at.isoformat() if self._started_at else None,
            }

    def start(self):
        with self._lock:
            if self._state == "recording":
                raise RecorderError("already recording")
            if not self.loopback_available:
                raise RecorderError("loopback unavailable")
            if not self.mic_available:
                raise RecorderError("mic unavailable")

            self._recording_id = str(uuid.uuid4())
            self._started_at = datetime.now(timezone.utc)
            self._path = self.recordings_dir / f"rec-{self._recording_id}.wav"
            self._partial = False
            self._open_streams()
            self._state = "recording"
            return self._recording_id, self._started_at

    def stop(self):
        with self._lock:
            if self._state == "idle":
                raise RecorderError("not recording")

            self._close_streams()
            duration = self._compute_duration()
            size_bytes = self._path.stat().st_size if self._path.exists() else 0
            result = StopResult(
                recording_id=self._recording_id,
                path=self._path,
                duration_seconds=duration,
                size_bytes=size_bytes,
                partial=self._partial,
            )
            self._state = "idle"
            self._recording_id = None
            self._started_at = None
            self._path = None
            return result

    def _open_streams(self):
        pass

    def _close_streams(self):
        pass

    def _compute_duration(self):
        return 0.0
