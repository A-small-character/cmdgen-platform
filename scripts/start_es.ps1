# Start dual ES cluster + load knowledge base
# Usage: .\scripts\start_es.ps1
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

Write-Host ''
Write-Host '=== Start Dual ES Cluster (6 nodes) + Load Knowledge ===' -ForegroundColor Green

# Set vm.max_map_count for ES (WSL2 backend)
wsl -d docker-desktop sysctl -w vm.max_map_count=262144 2>$null | Out-Null

Write-Host '[1] Starting ES clusters (cmdgen-a:9200, cmdgen-b:9201) ...' -ForegroundColor Cyan
docker compose -f deployments/docker/docker-compose-es-cluster.yml up -d

Write-Host '[2] Waiting for clusters to be ready (up to 120s) ...' -ForegroundColor Cyan
$ready = $false
for ($i = 1; $i -le 20; $i++) {
    Start-Sleep -Seconds 6
    try {
        $a = Invoke-RestMethod 'http://localhost:9200/_cluster/health' -TimeoutSec 3 -EA Stop
        $b = Invoke-RestMethod 'http://localhost:9201/_cluster/health' -TimeoutSec 3 -EA Stop
        if ($a.status -in @('green','yellow') -and $b.status -in @('green','yellow')) {
            Write-Host "    Cluster A: $($a.status) ($($a.number_of_nodes) nodes)  Cluster B: $($b.status) ($($b.number_of_nodes) nodes)" -ForegroundColor Green
            $ready = $true; break
        }
    } catch {}
    Write-Host "    waiting... ($i/20)" -ForegroundColor Gray
}
if (-not $ready) { Write-Host '    [WARN] clusters not ready in time' -ForegroundColor Yellow; exit 1 }

Write-Host '[3] Loading knowledge base into ES ...' -ForegroundColor Cyan
go run scripts/load_knowledge.go http://localhost:9200

Write-Host ''
Write-Host 'Done! ES knowledge base ready.' -ForegroundColor Green
Write-Host '  Cluster A (primary): http://localhost:9200' -ForegroundColor Cyan
Write-Host '  Cluster B (standby): http://localhost:9201' -ForegroundColor Cyan
Write-Host '  Now run desktop app: .\bin\cmdgen-desktop.exe' -ForegroundColor Cyan
