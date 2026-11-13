<#
.SYNOPSIS
  SOMUN 서버(와 cloudflared)를 기동한다. 작업 스케줄러·수동 실행 공용.
.EXAMPLE
  powershell -File scripts\start.ps1
  powershell -File scripts\start.ps1 -Env production -Port 8080
#>
param(
  [string]$Root = (Split-Path $PSScriptRoot -Parent),
  [string]$Env = "",
  [string]$Port = "",
  [string]$DataDir = "",
  [string]$MirrorDir = ""
)

$ErrorActionPreference = "Stop"
Set-Location $Root

if ($Env -ne "") { $env:SOMUN_ENV = $Env }
if ($Port -ne "") { $env:PORT = $Port }
if ($DataDir -ne "") { $env:DATA_DIR = $DataDir }
if ($MirrorDir -ne "") { $env:BACKUP_MIRROR_DIR = $MirrorDir }
if (-not $env:BACKUP_MIRROR_DIR) {
  # 외부 사본 기본값: 프로젝트·DATA_DIR 밖의 홈 폴더. 드라이브가 하나뿐이라
  # 물리 분리는 불가하며, 같은 디스크 내 경로 분리로 둔다.
  $env:BACKUP_MIRROR_DIR = Join-Path $HOME "somun-backup-mirror"
}

$exe = Join-Path $Root "backend\somun.exe"
if (-not (Test-Path $exe)) {
  Write-Output "building backend\somun.exe..."
  Push-Location (Join-Path $Root "backend")
  go build -o somun.exe ./cmd/server
  Pop-Location
}

Write-Output "starting somun (env=$($env:SOMUN_ENV), port=$($env:PORT))..."
$server = Start-Process $exe -PassThru -WindowStyle Hidden
Write-Output ("server pid=" + $server.Id)

$cloudflared = Get-Command cloudflared -ErrorAction SilentlyContinue
$tunnelArgs = $env:CLOUDFLARED_ARGS
if ($cloudflared -and $tunnelArgs) {
  Write-Output "starting cloudflared tunnel..."
  $tunnel = Start-Process $cloudflared.Source -ArgumentList $tunnelArgs -PassThru -WindowStyle Hidden
  Write-Output ("tunnel pid=" + $tunnel.Id)
} else {
  Write-Output "cloudflared not started (needs cloudflared + CLOUDFLARED_ARGS)."
}
Write-Output "START_OK"
