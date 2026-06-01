# Smart Command Generation Platform - Test Script
# Usage:
#   .\scripts\test.ps1             # run all tests
#   .\scripts\test.ps1 -Coverage   # generate HTML coverage report
param([switch]$Coverage)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

Write-Host ''
Write-Host '=== Running Tests ===' -ForegroundColor Green

if ($Coverage) {
    go test ./... -v -race -coverprofile=coverage.out -timeout 120s
    if ($LASTEXITCODE -eq 0) {
        go tool cover -html=coverage.out -o coverage.html
        Write-Host ''
        Write-Host 'Coverage report: coverage.html' -ForegroundColor Cyan
        Start-Process coverage.html   # open in browser
    }
} else {
    go test ./... -v -race -timeout 120s
}

if ($LASTEXITCODE -eq 0) {
    Write-Host ''
    Write-Host 'All tests passed!' -ForegroundColor Green
} else {
    Write-Host ''
    Write-Host 'Some tests failed.' -ForegroundColor Red
    exit 1
}
