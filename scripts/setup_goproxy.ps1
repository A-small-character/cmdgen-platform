# One-time setup: configure Go module proxy for China mainland
# Run this ONCE on a new machine, then normal builds will work.
#
# Usage: .\scripts\setup_goproxy.ps1

Write-Host ''
Write-Host '=== Configuring Go Module Proxy ===' -ForegroundColor Green
Write-Host ''

go env -w GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy/,direct
go env -w GONOSUMDB='*'
go env -w GONOSUMCHECK='*'

Write-Host 'Done. Current Go environment:' -ForegroundColor Green
go env GOPROXY
go env GONOSUMDB

Write-Host ''
Write-Host 'Now run:  .\scripts\build.ps1 -Target windows' -ForegroundColor Cyan
