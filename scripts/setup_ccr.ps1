# Setup CCR (Cross-Cluster Replication): cluster A (leader) -> cluster B (follower)
# Prerequisite: both clusters running with UNIFIED elastic-certificates.p12 (already configured)
# Note: CCR requires Platinum/Enterprise license or a trial. Start a trial first if needed.
#
# Usage: .\scripts\setup_ccr.ps1
param(
  [string]$ElasticPassword = 'Cmdgen@ES2024',
  [string]$LeaderHttp  = 'http://localhost:9200',   # cluster A (leader)
  [string]$FollowerHttp= 'http://localhost:9201'    # cluster B (follower)
)
$ErrorActionPreference = 'Stop'
$auth = "elastic:$ElasticPassword"
$pair = [System.Text.Encoding]::ASCII.GetBytes($auth)
$b64  = [System.Convert]::ToBase64String($pair)
$headers = @{ Authorization = "Basic $b64"; 'Content-Type'='application/json' }

Write-Host '=== Setup CCR: A(leader) -> B(follower) ===' -ForegroundColor Green

# 0) (Optional) start trial license on BOTH clusters (CCR is a commercial feature)
Write-Host '[0] Starting trial license on both clusters (CCR requires it) ...' -ForegroundColor Cyan
try { Invoke-RestMethod "$LeaderHttp/_license/start_trial?acknowledge=true"   -Method Post -Headers $headers | Out-Null } catch {}
try { Invoke-RestMethod "$FollowerHttp/_license/start_trial?acknowledge=true" -Method Post -Headers $headers | Out-Null } catch {}

# 1) On FOLLOWER (cluster B): register LEADER (cluster A) as a remote cluster
#    transport seed uses container DNS es-a1:9300 (same docker network, unified P12 -> trusted)
Write-Host '[1] Registering remote cluster (leader) on follower ...' -ForegroundColor Cyan
$remoteBody = @{
  persistent = @{
    'cluster.remote.leader.mode'       = 'sniff'
    'cluster.remote.leader.seeds'      = @('es-a1:9300','es-a2:9300','es-a3:9300')
  }
} | ConvertTo-Json -Depth 5
Invoke-RestMethod "$FollowerHttp/_cluster/settings" -Method Put -Headers $headers -Body $remoteBody | Out-Null

# 2) Verify remote connection
Write-Host '[2] Verifying remote connection ...' -ForegroundColor Cyan
$ri = Invoke-RestMethod "$FollowerHttp/_remote/info" -Headers $headers
Write-Host ($ri | ConvertTo-Json -Depth 5)

# 3) Create a follower index on B that replicates 'cmdgen_history' from A
Write-Host '[3] Creating follower index cmdgen_history_follower on B ...' -ForegroundColor Cyan
$followBody = @{
  remote_cluster = 'leader'
  leader_index   = 'cmdgen_history'
} | ConvertTo-Json
try {
  Invoke-RestMethod "$FollowerHttp/cmdgen_history_follower/_ccr/follow?wait_for_active_shards=1" -Method Put -Headers $headers -Body $followBody | Out-Null
  Write-Host '    Follower index created.' -ForegroundColor Green
} catch {
  Write-Host "    [WARN] follow failed: $_" -ForegroundColor Yellow
  Write-Host '    (Ensure trial license active and leader index exists)' -ForegroundColor Gray
}

# 4) Show CCR status
Write-Host '[4] CCR follow stats:' -ForegroundColor Cyan
try { Invoke-RestMethod "$FollowerHttp/cmdgen_history_follower/_ccr/stats" -Headers $headers | ConvertTo-Json -Depth 6 } catch {}

Write-Host ''
Write-Host 'CCR setup attempted. Because all 6 nodes share the SAME elastic-certificates.p12,' -ForegroundColor Green
Write-Host 'the cross-cluster transport trust is automatic - no extra cert exchange needed.' -ForegroundColor Green
