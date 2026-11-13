<#
.SYNOPSIS
  롤백: somun.prev.exe를 현행으로 복귀하고 재기동한다.
.EXAMPLE
  powershell -File scripts\rollback.ps1
  powershell -File scripts\rollback.ps1 -Root C:\somun-stage -Port 18240 -NoStart
#>
param(
  [string]$Root = (Split-Path $PSScriptRoot -Parent),
  [string]$Port = "8080",
  [string]$TaskName = "",
  [switch]$NoStart
)

$ErrorActionPreference = "Stop"
Set-Location $Root

$exe = Join-Path $Root "backend\somun.exe"
$prev = Join-Path $Root "backend\somun.prev.exe"
if (-not (Test-Path $prev)) {
  throw "no somun.prev.exe to roll back to"
}

if ($TaskName -ne "") {
  schtasks /End /TN "$TaskName" 2>$null
  Start-Sleep -Seconds 3
} else {
  $conns = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
  foreach ($c in $conns) {
    Stop-Process -Id $c.OwningProcess -Force -ErrorAction SilentlyContinue
  }
  Start-Sleep -Seconds 2
}

$bad = Join-Path $Root ("backend\somun.bad-" + (Get-Date -Format "yyyyMMdd-HHmmss") + ".exe")
if (Test-Path $exe) {
  Move-Item $exe $bad -Force
}
Move-Item $prev $exe -Force
Write-Output "restored previous binary"

if ($NoStart) {
  Write-Output "ROLLBACK_OK (not started)"
  return
}

if ($TaskName -ne "") {
  schtasks /Run /TN "$TaskName" | Out-Null
} else {
  $env:PORT = $Port
  Start-Process $exe -WindowStyle Hidden | Out-Null
}
for ($i = 0; $i -lt 30; $i++) {
  try {
    $r = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/api/health" -TimeoutSec 3 -UseBasicParsing
    if ($r.StatusCode -eq 200) {
      Write-Output "ROLLBACK_OK healthy"
      return
    }
  } catch { }
  Start-Sleep -Seconds 2
}
throw "server did not become healthy after rollback"
