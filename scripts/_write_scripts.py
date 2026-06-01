"""
Utility: write PowerShell scripts as UTF-8 BOM so that Windows
PowerShell 5.1 reads them correctly without garbled characters.

Run: python scripts/_write_scripts.py
"""
import os

BASE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def w(rel_path: str, content: str) -> None:
    full = os.path.join(BASE, rel_path)
    os.makedirs(os.path.dirname(full), exist_ok=True)
    with open(full, "wb") as f:
        f.write(b"\xef\xbb\xbf")          # UTF-8 BOM
        f.write(content.encode("utf-8"))
    print(f"  written: {rel_path}")


# ── build.ps1 ────────────────────────────────────────────────────────────────
BUILD = """\
# Smart Command Generation Platform - Build Script
# Usage:
#   .\\scripts\\build.ps1                    # build Windows amd64 (default)
#   .\\scripts\\build.ps1 -Target linux      # build Linux amd64
#   .\\scripts\\build.ps1 -Target all        # build all platforms
#   .\\scripts\\build.ps1 -Target windows -Prod  # production (trimpath)
param(
    [string]$Target  = 'windows',
    [string]$Version = '1.0.0',
    [switch]$Prod
)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

if (-not (Test-Path 'bin')) { New-Item -ItemType Directory -Path 'bin' | Out-Null }

$ldflags = if ($Prod) { '-w -s -trimpath' } else { '-w -s' }

function Build-One($goos, $goarch, $out) {
    Write-Host "  Building $goos/$goarch -> bin/$out" -ForegroundColor Cyan
    $env:GOOS        = $goos
    $env:GOARCH      = $goarch
    $env:CGO_ENABLED = '0'
    go build -ldflags $ldflags -o "bin/$out" ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw "Build failed: $goos/$goarch" }
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
}

Write-Host ''
Write-Host '=== Smart Command Generation Platform - Build ===' -ForegroundColor Green
Write-Host "Go: $(go version)"
Write-Host "Target: $Target  Prod: $Prod"
Write-Host ''

Write-Host '[1/3] Downloading dependencies...' -ForegroundColor Yellow
go mod download
if ($LASTEXITCODE -ne 0) { throw 'go mod download failed' }

Write-Host '[2/3] Compiling...' -ForegroundColor Yellow
switch ($Target) {
    'windows' { Build-One 'windows' 'amd64' 'cmdgen-windows-amd64.exe' }
    'linux'   { Build-One 'linux'   'amd64' 'cmdgen-linux-amd64'       }
    'darwin'  { Build-One 'darwin'  'arm64' 'cmdgen-darwin-arm64'      }
    'all' {
        Build-One 'windows' 'amd64' 'cmdgen-windows-amd64.exe'
        Build-One 'linux'   'amd64' 'cmdgen-linux-amd64'
        Build-One 'linux'   'arm64' 'cmdgen-linux-arm64'
        Build-One 'darwin'  'amd64' 'cmdgen-darwin-amd64'
        Build-One 'darwin'  'arm64' 'cmdgen-darwin-arm64'
    }
    default { throw "Unknown target '$Target'. Use: windows / linux / darwin / all" }
}

Write-Host '[3/3] Artifacts in ./bin/:' -ForegroundColor Yellow
Get-ChildItem 'bin' | ForEach-Object {
    $kb = [math]::Round($_.Length / 1KB, 1)
    Write-Host "  $($_.Name.PadRight(40)) $kb KB"
}
Write-Host ''
Write-Host 'Build complete!' -ForegroundColor Green
"""

# ── start.ps1 ────────────────────────────────────────────────────────────────
START = """\
# Smart Command Generation Platform - Windows Start Script
# Usage:
#   .\\scripts\\start.ps1                        # dev mode (go run)
#   .\\scripts\\start.ps1 -SkipDeps              # skip Docker startup
#   .\\scripts\\start.ps1 -Mode prod -BuildFirst # build then run binary
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
    & "$PSScriptRoot\\build.ps1" -Target windows
}

# ---- Step 6: run application --------------------------------------------------
Write-Host ''
Write-Host '[5] Starting application ...' -ForegroundColor Cyan
Write-Host '    URL  : http://localhost:8080' -ForegroundColor Cyan
Write-Host '    Health: http://localhost:8080/health' -ForegroundColor Cyan
Write-Host '    Stop : Ctrl+C' -ForegroundColor Gray
Write-Host ''

if ($Mode -eq 'prod') {
    $exe = 'bin\\cmdgen-windows-amd64.exe'
    if (-not (Test-Path $exe)) {
        Write-Host "    Binary not found, building first ..." -ForegroundColor Yellow
        & "$PSScriptRoot\\build.ps1" -Target windows -Prod
    }
    & ".\\$exe" --config configs/config.yaml
} else {
    go run ./cmd/server --config configs/config.yaml
}
"""

# ── init_kb.ps1 ───────────────────────────────────────────────────────────────
INIT_KB = """\
# Smart Command Generation Platform - Knowledge Base Init
# Usage: .\\scripts\\init_kb.ps1 [-ESAddr <url>]
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
"""

# ── test.ps1 ──────────────────────────────────────────────────────────────────
TEST_PS1 = """\
# Smart Command Generation Platform - Test Script
# Usage:
#   .\\scripts\\test.ps1             # run all tests
#   .\\scripts\\test.ps1 -Coverage   # generate HTML coverage report
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
"""

print("Writing PowerShell scripts (UTF-8 BOM) ...")
w("scripts/build.ps1",   BUILD)
w("scripts/start.ps1",   START)
w("scripts/init_kb.ps1", INIT_KB)
w("scripts/test.ps1",    TEST_PS1)
print("Done.")
