# Smart Command Generation Platform - Build Script
# Usage:
#   .\scripts\build.ps1                      # build Windows amd64 (default)
#   .\scripts\build.ps1 -Target linux        # build Linux amd64
#   .\scripts\build.ps1 -Target all          # build all platforms
#   .\scripts\build.ps1 -Offline             # skip go mod download (use vendor/ or cache)
#   .\scripts\build.ps1 -SetProxy            # set GOPROXY=goproxy.cn then build
param(
    [string]$Target   = 'windows',
    [switch]$Prod,
    [switch]$Offline,
    [switch]$SetProxy
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
    if ($Offline) {
        go build -mod=mod -ldflags $ldflags -o "bin/$out" ./cmd/server
    } else {
        go build -ldflags $ldflags -o "bin/$out" ./cmd/server
    }
    if ($LASTEXITCODE -ne 0) { throw "Build failed: $goos/$goarch" }
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
}

Write-Host ''
Write-Host '=== Smart Command Generation Platform - Build ===' -ForegroundColor Green
Write-Host "Go  : $(go version)"
Write-Host "Mode: Target=$Target  Prod=$Prod  Offline=$Offline  SetProxy=$SetProxy"
Write-Host ''

# ---- Step 0: sync knowledge_base -> embedded kb (for offline engine) ------------
Write-Host '[0/3] Syncing knowledge base into offline engine ...' -ForegroundColor Yellow
$kbDir = 'internal/application/generator/kb'
if (-not (Test-Path $kbDir)) { New-Item -ItemType Directory -Path $kbDir | Out-Null }
$map = @{
  'knowledge_base/linux/commands.yaml'         = 'linux.yaml'
  'knowledge_base/elasticsearch/commands.yaml' = 'elasticsearch.yaml'
  'knowledge_base/network/huawei_switch.yaml'  = 'network.yaml'
  'knowledge_base/docker/commands.yaml'        = 'docker.yaml'
  'knowledge_base/kubernetes/commands.yaml'    = 'kubernetes.yaml'
  'knowledge_base/mysql/commands.yaml'         = 'mysql.yaml'
}
foreach ($src in $map.Keys) {
  if (Test-Path $src) { Copy-Item $src (Join-Path $kbDir $map[$src]) -Force }
}
Write-Host "      kb files: $((Get-ChildItem $kbDir -Filter *.yaml).Count)" -ForegroundColor Gray

# ---- Step 1: dependency handling ------------------------------------------------
if ($Offline) {
    Write-Host '[1/3] Offline mode - skipping go mod download.' -ForegroundColor Gray
    if (Test-Path 'vendor') {
        Write-Host '      vendor/ directory found - will use -mod=vendor flag.' -ForegroundColor Green
        $script:useVendor = $true
    } else {
        Write-Host '      No vendor/ - relying on local module cache.' -ForegroundColor Yellow
        Write-Host '      Cache path: ' -NoNewline
        Write-Host (go env GOPATH) -ForegroundColor Cyan
        $script:useVendor = $false
    }
} elseif ($SetProxy) {
    Write-Host '[1/3] Setting GOPROXY to goproxy.cn ...' -ForegroundColor Yellow
    go env -w GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy/,direct
    go env -w GONOSUMDB='*'
    go env -w GOFLAGS='-mod=mod'
    Write-Host '      GOPROXY set. Downloading dependencies ...' -ForegroundColor Green
    go mod download
    if ($LASTEXITCODE -ne 0) { throw 'go mod download failed' }
} else {
    Write-Host '[1/3] Downloading dependencies ...' -ForegroundColor Yellow
    go mod download
    if ($LASTEXITCODE -ne 0) {
        Write-Host ''
        Write-Host '  Download failed. Try one of:' -ForegroundColor Red
        Write-Host '    .\scripts\build.ps1 -SetProxy     # use goproxy.cn mirror' -ForegroundColor Yellow
        Write-Host '    .\scripts\build.ps1 -Offline      # use local cache / vendor/' -ForegroundColor Yellow
        throw 'go mod download failed'
    }
}

# ---- Step 2: compile -------------------------------------------------------------
Write-Host '[2/3] Compiling ...' -ForegroundColor Yellow

# override Build-One if using vendor
if ($script:useVendor) {
    function Build-One($goos, $goarch, $out) {
        Write-Host "  Building $goos/$goarch -> bin/$out  [vendor]" -ForegroundColor Cyan
        $env:GOOS        = $goos
        $env:GOARCH      = $goarch
        $env:CGO_ENABLED = '0'
        go build -mod=vendor -ldflags $ldflags -o "bin/$out" ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw "Build failed: $goos/$goarch" }
        Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
    }
}

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

# ---- Step 3: list artifacts -------------------------------------------------------
Write-Host '[3/3] Artifacts in ./bin/:' -ForegroundColor Yellow
Get-ChildItem 'bin' | ForEach-Object {
    $kb = [math]::Round($_.Length / 1KB, 1)
    Write-Host "  $($_.Name.PadRight(42)) $kb KB"
}
Write-Host ''
Write-Host 'Build complete!' -ForegroundColor Green
