<#
.SYNOPSIS
  업데이트: 데이터 스냅샷 -> 빌드 -> somun.prev.exe 보존 -> 교체 -> 재기동 -> 헬스 확인.
  서버가 떠 있으면 먼저 중지한다(-TaskName이 있으면 작업 중지, 없으면 포트 프로세스 중지).
.EXAMPLE
  powershell -File scripts\update.ps1
  powershell -File scripts\update.ps1 -Root C:\somun-stage -Port 18240 -NoStart
#>
param(
  [string]$Root = (Split-Path $PSScriptRoot -Parent),
  [string]$Port = "8080",
  [string]$TaskName = "",
  [string]$DataDir = "data",
  [switch]$NoStart
)

$ErrorActionPreference = "Stop"
Set-Location $Root
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"

function Stop-Server {
  if ($TaskName -ne "") {
    schtasks /End /TN "$TaskName" 2>$null
    Start-Sleep -Seconds 3
    return
  }
  $conns = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
  foreach ($c in $conns) {
    Stop-Process -Id $c.OwningProcess -Force -ErrorAction SilentlyContinue
  }
  Start-Sleep -Seconds 2
}

function Wait-Healthy {
  for ($i = 0; $i -lt 30; $i++) {
    try {
      $r = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/api/health" -TimeoutSec 3 -UseBasicParsing
      if ($r.StatusCode -eq 200) { return $true }
    } catch { }
    Start-Sleep -Seconds 2
  }
  return $false
}

# 1. snapshot live data (file copy, no auth needed).
$dataPath = Join-Path $Root $DataDir
$snap = Join-Path $Root ("data-pre-update-" + $stamp)
if (Test-Path $dataPath) {
  Copy-Item -Recurse $dataPath $snap
  Write-Output ("snapshot=" + $snap)
} else {
  Write-Output "no live data dir; skipping snapshot"
}

# 2. build to a temp name first.
Push-Location (Join-Path $Root "backend")
go build -o ("somun.new-" + $stamp + ".exe") ./cmd/server
if ($LASTEXITCODE -ne 0) { throw "go build failed" }
Pop-Location

# 3. stop, preserve, swap.
Stop-Server
$exe = Join-Path $Root "backend\somun.exe"
$prev = Join-Path $Root "backend\somun.prev.exe"
$new = Join-Path $Root ("backend\somun.new-" + $stamp + ".exe")
if (Test-Path $exe) {
  Move-Item $exe $prev -Force
  Write-Output "preserved previous binary as somun.prev.exe"
}
Move-Item $new $exe -Force
Write-Output "swapped new binary"

if ($NoStart) {
  Write-Output "UPDATE_OK (not started)"
  return
}

# 4. start + health.
if ($TaskName -ne "") {
  schtasks /Run /TN "$TaskName" | Out-Null
} else {
  $env:PORT = $Port
  Start-Process $exe -WindowStyle Hidden | Out-Null
}
if (Wait-Healthy) {
  Write-Output "UPDATE_OK healthy"
} else {
  throw "server did not become healthy after update"
}
