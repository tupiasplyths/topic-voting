# topic-voting all-services startup script
# Starts classifier (Python), backend (Go), and frontend (Vite) in separate windows.
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path

# Check prerequisites
$missing = @()
if (-not (Get-Command python -ErrorAction SilentlyContinue)) { $missing += "python" }
if (-not (Get-Command go     -ErrorAction SilentlyContinue)) { $missing += "go" }
if (-not (Get-Command node   -ErrorAction SilentlyContinue)) { $missing += "node" }
if ($missing.Count -gt 0) {
    Write-Host "Missing prerequisites: $($missing -join ', ')" -ForegroundColor Red
    exit 1
}

Write-Host "Starting all services..." -ForegroundColor Green

Start-Process pwsh -WorkingDirectory "$root" -ArgumentList (
    '-NoExit', '-File', "$root\start-classifier.ps1"
)

Start-Process pwsh -WorkingDirectory "$root" -ArgumentList (
    '-NoExit', '-File', "$root\start-backend.ps1"
)

Start-Process pwsh -WorkingDirectory "$root" -ArgumentList (
    '-NoExit', '-File', "$root\start-frontend.ps1"
)

Write-Host "All services started in separate windows:" -ForegroundColor Green
Write-Host "  Classifier -> http://localhost:4747" -ForegroundColor Gray
Write-Host "  Backend    -> http://localhost:8585" -ForegroundColor Gray
Write-Host "  Frontend   -> http://localhost:5442" -ForegroundColor Gray
Write-Host "`nClose this window or individual service windows to stop." -ForegroundColor DarkGray
