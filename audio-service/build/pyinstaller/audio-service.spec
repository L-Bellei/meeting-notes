# -*- mode: python ; coding: utf-8 -*-
from pathlib import Path

from PyInstaller.utils.hooks import (
    collect_all,
    collect_data_files,
    collect_delvewheel_libs_directory,
    collect_submodules,
)

AUDIO_SERVICE_ROOT = Path(SPECPATH).parent.parent

datas = []
binaries = []
hiddenimports = []

# Packages with compiled extensions, dynamic imports and/or data files that
# PyInstaller's default static analysis cannot see through on its own.
for pkg in ("av", "ctranslate2", "faster_whisper", "onnxruntime", "tokenizers", "huggingface_hub"):
    pkg_datas, pkg_binaries, pkg_hiddenimports = collect_all(pkg)
    datas += pkg_datas
    binaries += pkg_binaries
    hiddenimports += pkg_hiddenimports

# soundfile (>=0.13) is a plain top-level module; its native lib ships in the
# sibling `_soundfile_data` package instead of inside `soundfile` itself.
datas += collect_data_files("_soundfile_data")

# uvicorn resolves its loop/protocol/lifespan implementations by dotted-path
# string lookup (uvicorn.config.LOOP_FACTORIES / HTTP_PROTOCOLS / ...), which
# static analysis never sees as an import.
hiddenimports += collect_submodules("uvicorn")

hiddenimports += ["_portaudiowpatch"]

# `av`'s bundled ffmpeg DLLs live in an `av.libs` directory that sits next to
# (not inside) the `av` package - delvewheel's loader patch in av/__init__.py
# looks it up as a real sibling directory, so it falls outside collect_all("av").
datas, binaries = collect_delvewheel_libs_directory("av", datas=datas, binaries=binaries)

# The nvidia.cudnn / nvidia.cublas GPU DLLs (~1.6 GB) are deliberately NOT
# collected here, to keep the installer small and match the size of prior
# releases. transcriber.py._setup_dll_paths() still tries to import them at
# startup; when that import fails (because they're absent from the bundle),
# it skips silently, ctranslate2.get_cuda_device_count() reports 0, and the
# service falls back to device="cpu" / compute_type="int8" - that fallback
# is the intended production path for this release, not a bug.

a = Analysis(
    [str(AUDIO_SERVICE_ROOT / "run.py")],
    pathex=[str(AUDIO_SERVICE_ROOT)],
    binaries=binaries,
    datas=datas,
    hiddenimports=hiddenimports,
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=[],
    noarchive=False,
    optimize=0,
)

pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name="audio-service",
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=False,
    console=False,
    disable_windowed_traceback=False,
    argv_emulation=False,
)

coll = COLLECT(
    exe,
    a.binaries,
    a.datas,
    strip=False,
    upx=False,
    name="audio-service",
)
