# Smart Command Generation Platform - Windows Start Script
# Usage:
#   .\scripts\start.ps1                        # dev mode (go run)
#   .\scripts\start.ps1 -SkipDeps              # skip Docker startup
#   .\scripts\start.ps1 -Mode prod -BuildFirst # build then run binary
param(
    [string]$Mode      = 'dev',
    [switch]$SkipDeps,
    [switch]$BuildFirst
)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

Write-Host ''
Write-Host '================================================' -ForegroundColor Green
Write-Host ' Smart Command Generation Platform (Windows)' -ForegroundColor Green
Write-Host '================================================' -ForegroundColor Green

# ---- Step 1: check .env -------------------------------------------------------
if (-not (Test-Path '.env')) {
    if (Test-Path '.env.example') {
        Copy-Item '.env.example' '.env'
        Write-Host ''
        Write-Host '[WARN] .env not found. Copied from .env.example.' -ForegroundColor Yellow
        Write-Host '  Edit .env and fill in at least one AI API key, then re-run.' -ForegroundColor Yellow
        Write-Host '  Opening .env ...' -ForegroundColor Gray
        Start-Process notepad '.env' -Wait
        exit 1
    }
    Write-Host '[ERROR] Neither .env nor .env.example found.' -ForegroundColor Red
    exit 1
}

# ---- Step 2: load .env --------------------------------------------------------
Write-Host ''
Write-Host '[1] Loading .env ...' -ForegroundColor Cyan
Get-Content '.env' | ForEach-Object {
    if ($_ -match '^\s*([^#\s=][^=]*)=(.*)$') {
        $k = $Matches[1].Trim()
        $v = $Matches[2].Trim()
        [System.Environment]::SetEnvironmentVariable($k, $v, 'Process')
        $disp = if ($k -match 'KEY|SECRET|PASSWORD|TOKEN') { '****' } else { $v }
        Write-Host "    $k = $disp"
    }
}

# ---- Step 3: warn if no AI key ------------------------------------------------
if (-not ($env:OPENAI_API_KEY -or $env:CLAUDE_API_KEY -or $env:DEEPSEEK_API_KEY)) {
    Write-Host ''
    Write-Host '[WARN] No AI API key detected.' -ForegroundColor Yellow
    Write-Host '  Add one to .env:' -ForegroundColor Gray
    Write-Host '    OPENAI_API_KEY=sk-...' -ForegroundColor Gray
    Write-Host '    CLAUDE_API_KEY=sk-ant-...' -ForegroundColor Gray
    Write-Host '    DEEPSEEK_API_KEY=sk-...' -ForegroundColor Gray
}

# ---- Step 4: start Docker deps ------------------------------------------------
if (-not $SkipDeps) {
    Write-Host ''
    Write-Host '[2] Checking Docker Desktop ...' -ForegroundColor Cyan
    $dockerOk = $false
    try {
        docker info 2>&1 | Out-Null
        $dockerOk = $true
        Write-Host '    Docker is running.' -ForegroundColor Green
    } catch {
        Write-Host '    Docker not running or not installed - skipping deps.' -ForegroundColor Yellow
        Write-Host '    Install Docker Desktop: https://www.docker.com/products/docker-desktop/' -ForegroundColor Gray
    }

    if ($dockerOk) {
        Write-Host '[3] Starting PostgreSQL + Redis + Elasticsearch ...' -ForegroundColor Cyan
        docker compose -f deployments/docker/docker-compose.yml `
            up -d postgres redis elasticsearch

        Write-Host '    Waiting for Elasticsearch (up to 90 s) ...' -ForegroundColor Gray
        $ready = $false
        for ($i = 1; $i -le 18; $i++) {
            Start-Sleep -Seconds 5
            try {
                $h = Invoke-RestMethod 'http://localhost:9200/_cluster/health' -TimeoutSec 3 -EA Stop
                if ($h.status -in @('green', 'yellow')) {
                    Write-Host "    Elasticsearch ready (status: $($h.status))" -ForegroundColor Green
                    $ready = $true; break
                }
            } catch {}
            Write-Host "    Still waiting ... ($i/18)" -ForegroundColor Gray
        }
        if (-not $ready) {
            Write-Host '    [WARN] ES not ready after 90 s. RAG search may be unavailable.' -ForegroundColor Yellow
        }
    }
} else {
    Write-Host ''
    Write-Host '[2][3] Dependency startup skipped (-SkipDeps).' -ForegroundColor Gray
}

# ---- Step 5: optional build ---------------------------------------------------
if ($BuildFirst) {
    Write-Host ''
    Write-Host '[4] Building ...' -ForegroundColor Cyan
    & "$PSScriptRoot\build.ps1" -Target windows
}

# ---- Step 6: run application --------------------------------------------------
Write-Host ''
Write-Host '[5] Starting application ...' -ForegroundColor Cyan
Write-Host '    URL  : http://localhost:8080' -ForegroundColor Cyan
Write-Host '    Health: http://localhost:8080/health' -ForegroundColor Cyan
Write-Host '    Stop : Ctrl+C' -ForegroundColor Gray
Write-Host ''

if ($Mode -eq 'prod') {
    $exe = 'bin\cmdgen-windows-amd64.exe'
    if (-not (Test-Path $exe)) {
        Write-Host "    Binary not found, building first ..." -ForegroundColor Yellow
        & "$PSScriptRoot\build.ps1" -Target windows -Prod
    }
    & ".\$exe" --config configs/config.yaml
} else {
    go run ./cmd/server --config configs/config.yaml
}
