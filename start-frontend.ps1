# Start the React Frontend dev server (port 5442)
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path

# Check prerequisites
if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
    Write-Host "ERROR: node is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

# Install deps if needed
if (-not (Test-Path "$root\frontend\node_modules")) {
    Write-Host "[frontend] Installing npm dependencies..." -ForegroundColor Cyan
    npm install --prefix "$root\frontend"
}

Write-Host "[frontend] Starting Vite dev server..." -ForegroundColor Cyan

Set-Location "$root\frontend"
npm run dev
