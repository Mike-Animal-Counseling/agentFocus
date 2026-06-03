# AgentFocus one-step build script (Windows).
#
# Builds the release executable (no console window) and, optionally, a debug
# build (with a console for log output). Runs vet + tests first so a broken
# tree fails fast.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts\build.ps1            # release only
#   powershell -ExecutionPolicy Bypass -File scripts\build.ps1 -Debug     # release + debug
#   powershell -ExecutionPolicy Bypass -File scripts\build.ps1 -SkipTests # skip vet/test

param(
    [switch]$Debug,
    [switch]$SkipTests
)

$ErrorActionPreference = 'Stop'

# Run from the repo root regardless of where this is invoked.
$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

# Resolve the Go toolchain.
$go = (Get-Command go -ErrorAction SilentlyContinue).Source
if (-not $go) { $go = "C:\Program Files\Go\bin\go.exe" }
if (-not (Test-Path $go)) {
    throw "Go not found. Install Go 1.24+ from https://go.dev/dl/"
}
Write-Host "Using Go: $go"
& $go version

if (-not $SkipTests) {
    Write-Host "`n== go vet ==" -ForegroundColor Cyan
    & $go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "go vet failed" }

    Write-Host "`n== go test ==" -ForegroundColor Cyan
    & $go test ./...
    if ($LASTEXITCODE -ne 0) { throw "go test failed" }
}

Write-Host "`n== build AgentFocus.exe (release, no console) ==" -ForegroundColor Cyan
& $go build -ldflags="-H windowsgui" -o AgentFocus.exe ./cmd/agentfocus
if ($LASTEXITCODE -ne 0) { throw "release build failed" }
Write-Host "  -> AgentFocus.exe" -ForegroundColor Green

if ($Debug) {
    Write-Host "`n== build AgentFocus-debug.exe (console, logs) ==" -ForegroundColor Cyan
    & $go build -o AgentFocus-debug.exe ./cmd/agentfocus
    if ($LASTEXITCODE -ne 0) { throw "debug build failed" }
    Write-Host "  -> AgentFocus-debug.exe" -ForegroundColor Green
}

Write-Host "`nBuild complete." -ForegroundColor Green
Get-ChildItem AgentFocus*.exe | Select-Object Name, @{n='MB';e={[math]::Round($_.Length/1MB,1)}}, LastWriteTime
