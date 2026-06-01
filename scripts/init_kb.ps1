# Smart Command Generation Platform - Knowledge Base Init
# Usage: .\scripts\init_kb.ps1 [-ESAddr <url>]
param(
    [string]$ESAddr = 'http://localhost:9200',
    [string]$Config = 'configs/config.yaml'
)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

Write-Host ''
Write-Host '=== Knowledge Base Initialization ===' -ForegroundColor Green

# Check ES
Write-Host ''
Write-Host "[1] Connecting to Elasticsearch: $ESAddr" -ForegroundColor Cyan
try {
    $h = Invoke-RestMethod "$ESAddr/_cluster/health" -TimeoutSec 5
    Write-Host "    Status: $($h.status)   Nodes: $($h.number_of_nodes)" -ForegroundColor Green
} catch {
    Write-Host "    [ERROR] Cannot connect to $ESAddr" -ForegroundColor Red
    Write-Host '    Start ES with:' -ForegroundColor Yellow
    Write-Host '      docker compose -f deployments/docker/docker-compose.yml up -d elasticsearch' -ForegroundColor Gray
    exit 1
}

# Load .env
if (Test-Path '.env') {
    Get-Content '.env' | ForEach-Object {
        if ($_ -match '^\s*([^#\s=][^=]*)=(.*)$') {
            [System.Environment]::SetEnvironmentVariable($Matches[1].Trim(), $Matches[2].Trim(), 'Process')
        }
    }
}

Write-Host ''
Write-Host '[2] Running indexer ...' -ForegroundColor Cyan
go run scripts/init_knowledge.go --config $Config

if ($LASTEXITCODE -eq 0) {
    Write-Host ''
    Write-Host 'Knowledge base initialized successfully!' -ForegroundColor Green
} else {
    Write-Host ''
    Write-Host 'Initialization failed. See output above.' -ForegroundColor Red
    exit 1
}
