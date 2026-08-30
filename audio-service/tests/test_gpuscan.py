from gpuscan import Adapter, GPUInfo, scan, vendor_from_id


def _scan(**overrides):
    probes = dict(
        whispercli_present=True,
        probe_cuda=lambda: False,
        nvidia_smi_info=lambda: (None, None),
        dxgi_adapters=lambda: [],
        vulkan_loader_present=lambda: False,
    )
    probes.update(overrides)
    return scan(**probes)


def test_vendor_from_id():
    assert vendor_from_id(0x10DE) == "nvidia"
    assert vendor_from_id(0x1002) == "amd"
    assert vendor_from_id(0x8086) == "intel"
    assert vendor_from_id(0x1414) == "other"


def test_nothing_detected():
    info = _scan()
    assert info == GPUInfo()
    assert info.available is False
    assert info.backend is None


def test_cuda_wins_and_uses_nvidia_smi_for_name():
    info = _scan(
        probe_cuda=lambda: True,
        nvidia_smi_info=lambda: ("NVIDIA GeForce RTX 2050", 4096),
        dxgi_adapters=lambda: [Adapter("NVIDIA GeForce RTX 2050", 0x10DE, 3900)],
        vulkan_loader_present=lambda: True,
    )
    assert info.cuda_available is True
    assert info.vulkan_available is True
    assert info.backend == "cuda"
    assert info.vendor == "nvidia"
    assert info.name == "NVIDIA GeForce RTX 2050"
    assert info.vram_mb == 4096


def test_cuda_without_nvidia_smi_falls_back_to_dxgi_name():
    info = _scan(
        probe_cuda=lambda: True,
        dxgi_adapters=lambda: [Adapter("NVIDIA GeForce RTX 2050", 0x10DE, 3900)],
    )
    assert info.name == "NVIDIA GeForce RTX 2050"
    assert info.vram_mb == 3900


def test_amd_via_vulkan():
    info = _scan(
        dxgi_adapters=lambda: [Adapter("AMD Radeon RX 7600", 0x1002, 8192)],
        vulkan_loader_present=lambda: True,
    )
    assert info.cuda_available is False
    assert info.vulkan_available is True
    assert info.backend == "vulkan"
    assert info.vendor == "amd"
    assert info.name == "AMD Radeon RX 7600"
    assert info.vram_mb == 8192


def test_vulkan_requires_whispercli_binary():
    info = _scan(
        whispercli_present=False,
        dxgi_adapters=lambda: [Adapter("AMD Radeon RX 7600", 0x1002, 8192)],
        vulkan_loader_present=lambda: True,
    )
    assert info.vulkan_available is False
    assert info.backend is None
    assert info.vendor == "amd"


def test_vulkan_requires_loader():
    info = _scan(dxgi_adapters=lambda: [Adapter("AMD Radeon RX 7600", 0x1002, 8192)])
    assert info.vulkan_available is False


def test_vulkan_requires_an_adapter():
    info = _scan(vulkan_loader_present=lambda: True)
    assert info.vulkan_available is False


def test_picks_adapter_with_most_dedicated_vram():
    info = _scan(
        dxgi_adapters=lambda: [
            Adapter("Intel(R) UHD Graphics", 0x8086, 128),
            Adapter("AMD Radeon RX 6700", 0x1002, 12288),
        ],
        vulkan_loader_present=lambda: True,
    )
    assert info.name == "AMD Radeon RX 6700"
    assert info.vendor == "amd"


def test_probe_exceptions_are_swallowed():
    def boom():
        raise OSError("dxgi.dll missing")
    info = _scan(dxgi_adapters=boom, vulkan_loader_present=lambda: True)
    assert info == GPUInfo()
