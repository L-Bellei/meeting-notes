import logging
import queue
import threading
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

import numpy as np
import pyaudiowpatch as pyaudio
import soundfile as sf
from scipy.signal import resample_poly

TARGET_SAMPLE_RATE = 16000
CHUNK_FRAMES = 1024
QUEUE_MAX_CHUNKS = 200

log = logging.getLogger("recorder")


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
        self._stopping = False

        self._pa = pyaudio.PyAudio()
        self._mic_info = None
        self._loopback_info = None
        self.mic_available = False
        self.loopback_available = False
        self._enumerate_devices()

        self._mic_stream = None
        self._loopback_stream = None
        self._mic_queue = None
        self._loopback_queue = None
        self._mic_native_rate = None
        self._loopback_native_rate = None
        self._loopback_channels = None
        self._mic_channels = None
        self._writer = None
        self._frames_written = 0
        self._stop_event = None
        self._mixer_thread = None

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
            if self._stopping:
                raise RecorderError("already stopping")
            self._stopping = True
            path = self._path
            recording_id = self._recording_id
            partial_at_entry = self._partial

        try:
            self._close_streams()
            duration = self._compute_duration()
            size_bytes = path.stat().st_size if path.exists() else 0
            result = StopResult(
                recording_id=recording_id,
                path=path,
                duration_seconds=duration,
                size_bytes=size_bytes,
                partial=self._partial,
            )
        finally:
            with self._lock:
                self._state = "idle"
                self._recording_id = None
                self._started_at = None
                self._path = None
                self._stopping = False

        return result

    def _open_streams(self):
        self._mic_queue = queue.Queue(maxsize=QUEUE_MAX_CHUNKS)
        self._loopback_queue = queue.Queue(maxsize=QUEUE_MAX_CHUNKS)
        self._mic_native_rate = int(self._mic_info["defaultSampleRate"])
        self._loopback_native_rate = int(self._loopback_info["defaultSampleRate"])
        self._loopback_channels = int(self._loopback_info["maxInputChannels"])
        self._mic_channels = int(self._mic_info["maxInputChannels"])
        self._frames_written = 0
        self._stop_event = threading.Event()

        self._writer = sf.SoundFile(
            str(self._path),
            mode="w",
            samplerate=TARGET_SAMPLE_RATE,
            channels=1,
            subtype="PCM_16",
        )

        try:
            self._mic_stream = self._pa.open(
                format=pyaudio.paInt16,
                channels=self._mic_channels,
                rate=self._mic_native_rate,
                input=True,
                input_device_index=self._mic_info["index"],
                frames_per_buffer=CHUNK_FRAMES,
                stream_callback=self._make_callback(self._mic_queue),
            )

            self._loopback_stream = self._pa.open(
                format=pyaudio.paInt16,
                channels=self._loopback_channels,
                rate=self._loopback_native_rate,
                input=True,
                input_device_index=self._loopback_info["index"],
                frames_per_buffer=CHUNK_FRAMES,
                stream_callback=self._make_callback(self._loopback_queue),
            )
        except OSError:
            # PyAudio instance may be in a stale state — reinitialize and retry once.
            log.warning("Stream open failed; reinitializing PyAudio and retrying")
            self._reset_pyaudio()
            self._mic_stream = self._pa.open(
                format=pyaudio.paInt16,
                channels=self._mic_channels,
                rate=self._mic_native_rate,
                input=True,
                input_device_index=self._mic_info["index"],
                frames_per_buffer=CHUNK_FRAMES,
                stream_callback=self._make_callback(self._mic_queue),
            )
            self._loopback_stream = self._pa.open(
                format=pyaudio.paInt16,
                channels=self._loopback_channels,
                rate=self._loopback_native_rate,
                input=True,
                input_device_index=self._loopback_info["index"],
                frames_per_buffer=CHUNK_FRAMES,
                stream_callback=self._make_callback(self._loopback_queue),
            )

        self._mic_stream.start_stream()
        self._loopback_stream.start_stream()

        self._mixer_thread = threading.Thread(target=self._run_mixer, daemon=True)
        self._mixer_thread.start()

    def _reset_pyaudio(self):
        """Terminate the current PyAudio instance and create a fresh one with re-enumerated devices."""
        try:
            self._pa.terminate()
        except Exception:
            pass
        self._pa = pyaudio.PyAudio()
        self._enumerate_devices()

    def _make_callback(self, q):
        def callback(in_data, frame_count, time_info, status):
            try:
                q.put_nowait(in_data)
            except queue.Full:
                try:
                    q.get_nowait()
                except queue.Empty:
                    pass
                try:
                    q.put_nowait(in_data)
                except queue.Full:
                    self._partial = True
                    log.warning("queue full; dropped audio chunk")
            return (None, pyaudio.paContinue)
        return callback

    def _run_mixer(self):
        try:
            while not self._stop_event.is_set():
                mic_chunk = self._safe_get(self._mic_queue, 0.05)
                lb_chunk = self._safe_get(self._loopback_queue, 0.05)
                if mic_chunk is None and lb_chunk is None:
                    continue
                mic_mono16 = self._chunk_to_mono16k(mic_chunk, self._mic_channels, self._mic_native_rate)
                lb_mono16 = self._chunk_to_mono16k(lb_chunk, self._loopback_channels, self._loopback_native_rate)
                mixed = self._mix(mic_mono16, lb_mono16)
                if mixed.size > 0:
                    self._writer.write(mixed)
                    self._frames_written += mixed.shape[0]
            self._drain_queues()
        except Exception as e:
            log.exception("mixer thread error: %s", e)
            self._partial = True

    def _safe_get(self, q, timeout):
        try:
            return q.get(timeout=timeout)
        except queue.Empty:
            return None

    def _drain_queues(self):
        while True:
            mic_chunk = self._safe_get(self._mic_queue, 0)
            lb_chunk = self._safe_get(self._loopback_queue, 0)
            if mic_chunk is None and lb_chunk is None:
                return
            mic_mono16 = self._chunk_to_mono16k(mic_chunk, self._mic_channels, self._mic_native_rate)
            lb_mono16 = self._chunk_to_mono16k(lb_chunk, self._loopback_channels, self._loopback_native_rate)
            mixed = self._mix(mic_mono16, lb_mono16)
            if mixed.size > 0:
                self._writer.write(mixed)
                self._frames_written += mixed.shape[0]

    def _chunk_to_mono16k(self, raw, channels, native_rate):
        if raw is None:
            return np.zeros(0, dtype=np.float32)
        samples = np.frombuffer(raw, dtype=np.int16).astype(np.float32) / 32768.0
        if channels > 1:
            samples = samples.reshape(-1, channels).mean(axis=1)
        if native_rate != TARGET_SAMPLE_RATE:
            samples = resample_poly(samples, TARGET_SAMPLE_RATE, native_rate)
        return samples.astype(np.float32)

    def _mix(self, a, b):
        if a.size == 0 and b.size == 0:
            return np.zeros(0, dtype=np.float32)
        n = max(a.size, b.size)
        if a.size < n:
            a = np.pad(a, (0, n - a.size))
        if b.size < n:
            b = np.pad(b, (0, n - b.size))
        mixed = a + b
        return np.clip(mixed, -1.0, 1.0)

    def _close_streams(self):
        for stream in (self._mic_stream, self._loopback_stream):
            if stream is None:
                continue
            try:
                if stream.is_active():
                    stream.stop_stream()
                stream.close()
            except Exception as e:
                log.warning("error closing stream: %s", e)
                self._partial = True

        if self._stop_event is not None:
            self._stop_event.set()

        if self._mixer_thread is not None:
            self._mixer_thread.join(timeout=5.0)
            if self._mixer_thread.is_alive():
                log.warning("mixer thread did not exit in time")
                self._partial = True

        if self._writer is not None:
            try:
                self._writer.close()
            except Exception as e:
                log.warning("error closing writer: %s", e)
                self._partial = True

        self._mic_stream = None
        self._loopback_stream = None
        self._mic_queue = None
        self._loopback_queue = None
        self._mixer_thread = None
        self._stop_event = None
        self._writer = None
        self._mic_channels = None

    def _compute_duration(self):
        return self._frames_written / TARGET_SAMPLE_RATE
