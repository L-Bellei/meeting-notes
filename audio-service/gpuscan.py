import ctypes
import subprocess
import sys
from ctypes import POINTER, Structure, byref, c_long, c_size_t, c_uint, c_void_p, wintypes
from dataclasses import dataclass
from typing import Callable, Optional

VENDOR_IDS = {0x10DE: "nvidia", 0x1002: "amd", 0x8086: "intel"}
DXGI_ADAPTER_FLAG_SOFTWARE = 2


@dataclass
class Adapter:
    name: str
    vendor_id: int
    vram_mb: int


@dataclass
class GPUInfo:
    cuda_available: bool = False
    vulkan_available: bool = False
    vendor: Optional[str] = None
    name: Optional[str] = None
    vram_mb: Optional[int] = None

    @property
    def available(self) -> bool:
        return self.cuda_available or self.vulkan_available

    @property
    def backend(self) -> Optional[str]:
        if self.cuda_available:
            return "cuda"
        if self.vulkan_available:
            return "vulkan"
        return None


def vendor_from_id(vendor_id: int) -> str:
    return VENDOR_IDS.get(vendor_id, "other")


def probe_cuda() -> bool:
    try:
        import ctranslate2
        if ctranslate2.get_cuda_device_count() <= 0:
            return False
        for dll in ("cublas64_12.dll", "cublas64_11.dll"):
            try:
                ctypes.CDLL(dll)
                return True
            except OSError:
                continue
    except Exception:
        pass
    return False


def nvidia_smi_info() -> tuple[Optional[str], Optional[int]]:
    try:
        flags = 0x08000000 if sys.platform == "win32" else 0
        out = subprocess.run(
            ["nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits"],
            capture_output=True, text=True, timeout=5, creationflags=flags,
        )
        first = out.stdout.strip().splitlines()[0]
        raw_name, raw_mem = first.rsplit(",", 1)
        return raw_name.strip(), int(float(raw_mem.strip()))
    except Exception:
        return None, None


def vulkan_loader_present() -> bool:
    if sys.platform != "win32":
        return False
    try:
        ctypes.CDLL("vulkan-1.dll")
        return True
    except OSError:
        return False


class _GUID(Structure):
    _fields_ = [("Data1", wintypes.DWORD), ("Data2", wintypes.WORD), ("Data3", wintypes.WORD),
                ("Data4", ctypes.c_ubyte * 8)]


class _LUID(Structure):
    _fields_ = [("LowPart", wintypes.DWORD), ("HighPart", wintypes.LONG)]


class _DXGI_ADAPTER_DESC1(Structure):
    _fields_ = [
        ("Description", wintypes.WCHAR * 128),
        ("VendorId", c_uint), ("DeviceId", c_uint), ("SubSysId", c_uint), ("Revision", c_uint),
        ("DedicatedVideoMemory", c_size_t), ("DedicatedSystemMemory", c_size_t),
        ("SharedSystemMemory", c_size_t),
        ("AdapterLuid", _LUID), ("Flags", c_uint),
    ]


_IID_IDXGIFactory1 = _GUID(0x770AAE78, 0xF26F, 0x4DBA,
                           (ctypes.c_ubyte * 8)(0xA8, 0x29, 0x25, 0x3C, 0x83, 0xD1, 0xB3, 0x87))

# Índices na vtable COM: IUnknown(0-2) + IDXGIObject(3-6) + IDXGIFactory(7-11) + IDXGIFactory1(12-13);
# IUnknown(0-2) + IDXGIObject(3-6) + IDXGIAdapter(7-9) + IDXGIAdapter1::GetDesc1(10).
_VT_RELEASE = 2
_VT_FACTORY1_ENUM_ADAPTERS1 = 12
_VT_ADAPTER1_GET_DESC1 = 10


def _method(obj: c_void_p, index: int, restype, *argtypes):
    vtable = ctypes.cast(obj, POINTER(POINTER(c_void_p)))[0]
    return ctypes.WINFUNCTYPE(restype, c_void_p, *argtypes)(vtable[index])


def dxgi_adapters() -> list[Adapter]:
    if sys.platform != "win32":
        return []
    dxgi = ctypes.WinDLL("dxgi")
    dxgi.CreateDXGIFactory1.argtypes = [POINTER(_GUID), POINTER(c_void_p)]
    dxgi.CreateDXGIFactory1.restype = c_long
    factory = c_void_p()
    if dxgi.CreateDXGIFactory1(byref(_IID_IDXGIFactory1), byref(factory)) != 0 or not factory:
        return []
    enum_adapters = _method(factory, _VT_FACTORY1_ENUM_ADAPTERS1, c_long, c_uint, POINTER(c_void_p))
    out: list[Adapter] = []
    try:
        index = 0
        while True:
            adapter = c_void_p()
            if enum_adapters(factory, index, byref(adapter)) != 0:
                break
            try:
                desc = _DXGI_ADAPTER_DESC1()
                get_desc = _method(adapter, _VT_ADAPTER1_GET_DESC1, c_long, POINTER(_DXGI_ADAPTER_DESC1))
                if get_desc(adapter, byref(desc)) == 0 and not (desc.Flags & DXGI_ADAPTER_FLAG_SOFTWARE):
                    out.append(Adapter(desc.Description, desc.VendorId,
                                       int(desc.DedicatedVideoMemory // (1024 * 1024))))
            finally:
                _method(adapter, _VT_RELEASE, c_uint)(adapter)
            index += 1
    finally:
        _method(factory, _VT_RELEASE, c_uint)(factory)
    return out


def _safe(fn: Callable, default):
    try:
        return fn()
    except Exception:
        return default


def scan(
    whispercli_present: bool,
    *,
    probe_cuda: Callable[[], bool] = probe_cuda,
    nvidia_smi_info: Callable[[], tuple[Optional[str], Optional[int]]] = nvidia_smi_info,
    dxgi_adapters: Callable[[], list[Adapter]] = dxgi_adapters,
    vulkan_loader_present: Callable[[], bool] = vulkan_loader_present,
) -> GPUInfo:
    cuda = _safe(probe_cuda, False)
    adapters = _safe(dxgi_adapters, [])
    loader = _safe(vulkan_loader_present, False)
    primary = max(adapters, key=lambda a: a.vram_mb, default=None)

    info = GPUInfo(cuda_available=cuda)
    if primary is not None:
        info.vendor = vendor_from_id(primary.vendor_id)
        info.name = primary.name
        info.vram_mb = primary.vram_mb
    if cuda:
        smi_name, smi_vram = _safe(nvidia_smi_info, (None, None))
        info.vendor = "nvidia"
        info.name = smi_name or info.name
        info.vram_mb = smi_vram or info.vram_mb
    info.vulkan_available = bool(loader and primary is not None and whispercli_present)
    return info
