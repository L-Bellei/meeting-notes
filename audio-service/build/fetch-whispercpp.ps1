#Requires -Version 7
<#
.SYNOPSIS
    Compila o whisper-cli (whisper.cpp) com backend Vulkan para audio-service/vendor/whispercpp/.

.DESCRIPTION
    A release oficial do whisper.cpp não publica build Vulkan para Windows (conferido em
    2026-08-30: só bin/blas/cublas), então o binário é compilado a partir da tag pinada em
    build/whispercpp.version. Pré-requisitos na máquina de build: git, CMake, Visual Studio
    Build Tools 2022 (workload C++) e Vulkan SDK (variável VULKAN_SDK definida).

.PARAMETER Force
    Recompila mesmo que vendor/whispercpp/whisper-cli.exe já exista.
#>
param([switch]$Force)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Root      = Split-Path $PSScriptRoot -Parent
$VendorDir = Join-Path $Root "vendor\whispercpp"
$Tag       = @(Get-Content (Join-Path $PSScriptRoot "whispercpp.version"))[0].Trim()
$Src       = Join-Path $env:TEMP "whisper.cpp-$Tag"
$BuildDir  = Join-Path $Src "build"

if ((Test-Path (Join-Path $VendorDir "whisper-cli.exe")) -and -not $Force) {
    Write-Host "whisper-cli já presente em $VendorDir ($Tag). Use -Force para recompilar."
    exit 0
}

foreach ($tool in "git", "cmake") {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
        Write-Error "$tool não encontrado no PATH."
        exit 1
    }
}
if (-not $env:VULKAN_SDK) {
    Write-Error "VULKAN_SDK não definido — instale o Vulkan SDK (winget install KhronosGroup.VulkanSDK) e abra um novo terminal."
    exit 1
}

if (-not (Test-Path $Src)) {
    Write-Host "Clonando whisper.cpp $Tag"
    git clone --depth 1 --branch $Tag https://github.com/ggml-org/whisper.cpp $Src
}

Write-Host "Configurando (GGML_VULKAN=1)"
cmake -S $Src -B $BuildDir -DGGML_VULKAN=1 -DBUILD_SHARED_LIBS=ON -DCMAKE_BUILD_TYPE=Release `
      -DWHISPER_BUILD_TESTS=OFF -DWHISPER_BUILD_SERVER=OFF
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Compilando whisper-cli"
cmake --build $BuildDir --config Release --target whisper-cli --parallel
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$OutDir = Join-Path $BuildDir "bin\Release"
if (-not (Test-Path (Join-Path $OutDir "whisper-cli.exe"))) {
    Write-Error "whisper-cli.exe não apareceu em $OutDir"
    exit 1
}

New-Item -ItemType Directory -Force $VendorDir | Out-Null
Get-ChildItem $VendorDir -File -ErrorAction SilentlyContinue | Remove-Item -Force
Copy-Item (Join-Path $OutDir "whisper-cli.exe") $VendorDir -Force
Copy-Item (Join-Path $OutDir "*.dll") $VendorDir -Force
Set-Content (Join-Path $VendorDir "VERSION") $Tag

$size = [math]::Round((Get-ChildItem $VendorDir -File | Measure-Object -Sum Length).Sum / 1MB, 1)
Write-Host "OK: $VendorDir ($Tag, $size MB)"
Get-ChildItem $VendorDir -File | Select-Object Name, @{n = "MB"; e = { [math]::Round($_.Length / 1MB, 1) } } | Format-Table -AutoSize
