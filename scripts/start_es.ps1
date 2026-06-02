# Start dual ES cluster (Security + unified P12) + load knowledge + create Kibana data views
# Usage: .\scripts\start_es.ps1
param(
  [string]$ElasticPassword = 'Cmdgen@ES2024'
)
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

Write-Host ''
Write-Host '=== Start Dual ES Cluster (Security) + Knowledge + Kibana DataViews ===' -ForegroundColor Green

wsl -d docker-desktop sysctl -w vm.max_map_count=262144 2>$null | Out-Null

$auth = "elastic:$ElasticPassword"
$b64  = [System.Convert]::ToBase64String([System.Text.Encoding]::ASCII.GetBytes($auth))
$esHdr = @{ Authorization = "Basic $b64" }
$kbHdr = @{ Authorization = "Basic $b64"; 'kbn-xsrf' = 'true'; 'Content-Type' = 'application/json' }

Write-Host '[1] Starting clusters (cmdgen-a:9200, cmdgen-b:9201) ...' -ForegroundColor Cyan
docker compose -f deployments/docker/docker-compose-es-cluster.yml up -d

Write-Host '[2] Waiting for both clusters (up to 150s) ...' -ForegroundColor Cyan
$ready = $false
for ($i = 1; $i -le 25; $i++) {
  Start-Sleep -Seconds 6
  try {
    $a = Invoke-RestMethod 'http://localhost:9200/_cluster/health' -Headers $esHdr -TimeoutSec 3 -EA Stop
    $b = Invoke-RestMethod 'http://localhost:9201/_cluster/health' -Headers $esHdr -TimeoutSec 3 -EA Stop
    if ($a.status -in @('green','yellow') -and $b.status -in @('green','yellow')) {
      Write-Host "    A: $($a.status)/$($a.number_of_nodes)nodes  B: $($b.status)/$($b.number_of_nodes)nodes" -ForegroundColor Green
      $ready = $true; break
    }
  } catch {}
  Write-Host "    waiting... ($i/25)" -ForegroundColor Gray
}
if (-not $ready) { Write-Host '    [WARN] clusters not ready' -ForegroundColor Yellow; exit 1 }

Write-Host '[3] Loading knowledge base (59 official commands) ...' -ForegroundColor Cyan
go run scripts/load_knowledge.go http://localhost:9200 elastic $ElasticPassword

Write-Host '[4] Waiting for Kibana A (5601) ...' -ForegroundColor Cyan
for ($i = 1; $i -le 30; $i++) {
  try { $s = Invoke-RestMethod 'http://localhost:5601/api/status' -Headers $esHdr -TimeoutSec 3 -EA Stop; if ($s) { Write-Host '    Kibana A ready' -ForegroundColor Green; break } } catch {}
  Start-Sleep -Seconds 5
}

Write-Host '[5] Creating Kibana Data View (cmdgen_history) ...' -ForegroundColor Cyan
$dvBody = '{"data_view":{"title":"cmdgen_history","name":"Command Knowledge Base","timeFieldName":"created_at"}}'
try { Invoke-RestMethod 'http://localhost:5601/api/data_views/data_view' -Method Post -Headers $kbHdr -Body $dvBody | Out-Null; Write-Host '    Data View created on Kibana A' -ForegroundColor Green } catch { Write-Host '    Data View may already exist (A)' -ForegroundColor Gray }

Write-Host ''
Write-Host 'All done!' -ForegroundColor Green
Write-Host "  ES   A/B : http://localhost:9200  /  http://localhost:9201  (elastic / $ElasticPassword)" -ForegroundColor Cyan
Write-Host '  Kibana A : http://localhost:5601   (login: elastic)' -ForegroundColor Cyan
Write-Host '  Kibana B : http://localhost:5602' -ForegroundColor Cyan
Write-Host '  In Kibana: Menu -> Discover -> select "Command Knowledge Base" -> 59 commands' -ForegroundColor Cyan
