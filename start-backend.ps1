# Start the Go Backend API service (port 8585)
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path

# Check prerequisites
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "ERROR: go is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

# Ensure .env exists in backend/ (godotenv.Load looks in working dir)
if (-not (Test-Path "$root\.env")) {
    Write-Host "[setup] Copying .env.example to .env" -ForegroundColor Yellow
    Copy-Item "$root\.env.example" "$root\.env"
}
if (-not (Test-Path "$root\backend\.env")) {
    Write-Host "[setup] Copying .env to backend/ (required by Go server)" -ForegroundColor Yellow
    Copy-Item "$root\.env" "$root\backend\.env"
}

# Build if needed
if (-not (Test-Path "$root\backend\server.exe")) {
    Write-Host "[backend] Building Go server..." -ForegroundColor Cyan
    go build -C "$root\backend" -o server.exe ./cmd/server
}
if (-not (Test-Path "$root\backend\server.exe")) {
    Write-Host "[backend] ERROR: Build failed — check Go installation" -ForegroundColor Red
    exit 1
}

Write-Host "[backend] Starting Go server..." -ForegroundColor Cyan

Set-Location "$root\backend"
Write-Host "Backend running on http://localhost:8585" -ForegroundColor Green

.\server.exe
