# Start the Python NLP Classifier service (port 4747)
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path

# Check prerequisites
if (-not (Get-Command python -ErrorAction SilentlyContinue)) {
    Write-Host "ERROR: python is not installed or not in PATH" -ForegroundColor Red
    exit 1
}
if (-not (Test-Path "$root\.venv")) {
    Write-Host "ERROR: Virtual environment not found at $root\.venv" -ForegroundColor Red
    exit 1
}

Write-Host "[classifier] Activating venv and starting uvicorn..." -ForegroundColor Cyan

Set-Location "$root\classifier"
& "$root\.venv\Scripts\Activate.ps1"
Write-Host "Classifier running on http://localhost:4747" -ForegroundColor Green

python main.py
