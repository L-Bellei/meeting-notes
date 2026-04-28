#Requires -Version 7
<#
.SYNOPSIS
    Gera o instalador Windows do Meeting Notes.

.PARAMETER Version
    Versão do release (ex: 1.0.0). Padrão: lê do wails.json.

.PARAMETER SkipTests
    Pula os testes Go antes de compilar.

.PARAMETER NoNSIS
    Gera apenas o .exe portátil, sem instalador NSIS.

.EXAMPLE
    .\build.ps1
    .\build.ps1 -Version 1.2.0
    .\build.ps1 -Version 1.2.0 -SkipTests
#>
param(
    [string]$Version = "",
    [switch]$SkipTests,
    [switch]$NoNSIS
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ProjectRoot = $PSScriptRoot
$WailsDir    = Join-Path $ProjectRoot "cmd\desktop"
$WailsJson   = Join-Path $WailsDir "wails.json"
$DistDir     = Join-Path $ProjectRoot "dist"

# ── helpers ──────────────────────────────────────────────────────────────────

function Write-Step([string]$msg) {
    Write-Host "`n▶  $msg" -ForegroundColor Cyan
}

function Write-Ok([string]$msg) {
    Write-Host "✓  $msg" -ForegroundColor Green
}

function Write-Fail([string]$msg) {
    Write-Host "✗  $msg" -ForegroundColor Red
}

# ── pre-flight checks ─────────────────────────────────────────────────────────

Write-Step "Verificando dependências"

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    Write-Fail "Wails não encontrado. Instale com: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    exit 1
}

if (-not $NoNSIS) {
    if (-not (Get-Command makensis -ErrorAction SilentlyContinue)) {
        Write-Fail "NSIS não encontrado."
        Write-Host "  Instale com:  winget install NSIS.NSIS" -ForegroundColor Yellow
        Write-Host "  Ou use:       .\build.ps1 -NoNSIS  (gera .exe portátil)" -ForegroundColor Yellow
        exit 1
    }
    Write-Ok "NSIS encontrado"
}

Write-Ok "Wails $(wails version 2>&1 | Select-String 'v\d' | ForEach-Object { $_.Matches[0].Value })"

# ── version ───────────────────────────────────────────────────────────────────

$config = Get-Content $WailsJson -Raw | ConvertFrom-Json

if ($Version -eq "") {
    $Version = $config.info.productVersion
    Write-Host "  Usando versão do wails.json: $Version" -ForegroundColor DarkGray
} else {
    # Persist new version into wails.json
    $config.info.productVersion = $Version
    $config | ConvertTo-Json -Depth 10 | Set-Content $WailsJson -Encoding UTF8
    Write-Ok "Versão atualizada para $Version no wails.json"
}

# ── tests ─────────────────────────────────────────────────────────────────────

if (-not $SkipTests) {
    Write-Step "Executando testes Go"
    Push-Location $ProjectRoot
    go test ./internal/... 2>&1 | ForEach-Object { Write-Host "  $_" }
    if ($LASTEXITCODE -ne 0) {
        Write-Fail "Testes falharam — corrija antes de gerar o instalador."
        exit 1
    }
    Write-Ok "Todos os testes passaram"
    Pop-Location
}

# ── build ─────────────────────────────────────────────────────────────────────

Write-Step "Compilando aplicação (versão $Version)"

Push-Location $WailsDir

$wailsFlags = @("-clean")
if (-not $NoNSIS) { $wailsFlags += "-nsis" }

wails build @wailsFlags
$buildExit = $LASTEXITCODE
Pop-Location

if ($buildExit -ne 0) {
    Write-Fail "wails build falhou (código $buildExit)"
    exit 1
}

Write-Ok "Compilação concluída"

# ── collect artifacts ─────────────────────────────────────────────────────────

Write-Step "Coletando artefatos"

$BinDir = Join-Path $WailsDir "build\bin"
$null   = New-Item -ItemType Directory -Path $DistDir -Force

if ($NoNSIS) {
    $src = Get-ChildItem $BinDir -Filter "*.exe" | Where-Object { $_.Name -notlike "*installer*" } | Select-Object -First 1
    if (-not $src) { Write-Fail "Executável não encontrado em $BinDir"; exit 1 }
    $destName = "meeting-notes-$Version-windows-amd64.exe"
} else {
    $src = Get-ChildItem $BinDir -Filter "*installer*.exe" | Select-Object -First 1
    if (-not $src) { Write-Fail "Instalador não encontrado em $BinDir"; exit 1 }
    $destName = "meeting-notes-$Version-windows-amd64-installer.exe"
}

$dest = Join-Path $DistDir $destName
Copy-Item $src.FullName $dest -Force

# ── summary ───────────────────────────────────────────────────────────────────

$sizeMB = [math]::Round((Get-Item $dest).Length / 1MB, 1)

Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor DarkGray
Write-Ok "Artefato gerado com sucesso"
Write-Host "  Arquivo : $dest" -ForegroundColor White
Write-Host "  Tamanho : $sizeMB MB" -ForegroundColor White
Write-Host "  Versão  : $Version" -ForegroundColor White
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor DarkGray
Write-Host ""
