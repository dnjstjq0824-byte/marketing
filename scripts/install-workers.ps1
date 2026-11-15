<#
.SYNOPSIS
  팀 실행기(레인 T: 분석·전략·제작·검수 전 사슬) 1기를 에이전트로 등록하고 시작 폴더에 등록한다.
  W16 이후 레인별 상주 작업자(resident-a/b/c)는 폐지: 기존 등록·바로가기·러너를 정리한다.
  실AI 실행(SOMUN_REAL_AI=1)으로 동작하며, 로그온 때마다 자동 시작한다.
  관리자 권한 불필요. 토큰은 러너 파일에 저장된다(로컬 전용, git 제외).
  AI 계정이 여러 개면 계정마다 한 번씩 -ChatAccountId를 바꿔 실행한다.
.EXAMPLE
  powershell -File scripts\install-workers.ps1 -DryRun
  powershell -File scripts\install-workers.ps1 -ServerUrl http://127.0.0.1:18080 -ChatAccountId 3
#>
param(
  [string]$Root = (Split-Path $PSScriptRoot -Parent),
  [string]$ServerUrl = "http://127.0.0.1:18080",
  [string]$AdminUser = "admin",
  [string]$AdminPass = "changeme123",
  [int]$ChatAccountId = 3,
  [string]$Label = "resident-team",
  [switch]$DryRun
)

$ErrorActionPreference = "Stop"
$ServerUrl = $ServerUrl.TrimEnd("/")

function Invoke-Api($method, $path, $body, $token) {
  $headers = @{ "Content-Type" = "application/json" }
  if ($token) { $headers["Authorization"] = "Bearer $token" }
  $params = @{ Uri = "$ServerUrl$path"; Method = $method; Headers = $headers }
  if ($body -ne $null) { $params["Body"] = ($body | ConvertTo-Json -Depth 5) }
  return Invoke-RestMethod @params
}

try {
  $health = Invoke-RestMethod -Uri "$ServerUrl/api/health" -TimeoutSec 10
  if (-not $health.ok) { throw "health not ok" }
} catch {
  Write-Output "SERVER UNREACHABLE: $ServerUrl"
  exit 1
}

$login = Invoke-Api "POST" "/api/auth/login" @{ username = $AdminUser; password = $AdminPass } ""
$opToken = $login.token
Write-Output "operator login ok"

$python = (Get-Command python -ErrorAction SilentlyContinue).Source
if (-not $python) { Write-Output "PYTHON NOT FOUND on PATH"; exit 1 }

$workerDir = Join-Path $Root "worker"
$runDir = Join-Path $Root "scripts\workers"
$startup = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\Startup"
$existing = Invoke-Api "GET" "/api/agents?limit=200" $null $opToken

# W16-7: 레인별 상주 작업자 폐지. 기존 resident-a/b/c 등록·바로가기·러너를 정리한다.
foreach ($lane in @("A", "B", "C")) {
  $old = "resident-$($lane.ToLower())"
  foreach ($a in $existing.items) {
    if ($a.label -eq $old) {
      if ($DryRun) {
        Write-Output "DRYRUN: would delete retired agent $old"
      } else {
        try { Invoke-Api "DELETE" "/api/agents/$($a.id)" $null $opToken | Out-Null } catch {}
        Write-Output "retired agent $old removed (claims release on lease expiry)"
      }
    }
  }
  $lnk = Join-Path $startup "SOMUN Worker $lane.lnk"
  if (Test-Path $lnk) {
    if ($DryRun) { Write-Output "DRYRUN: would remove $lnk" }
    else { Remove-Item $lnk -Force; Write-Output "startup link for lane $lane removed" }
  }
  $runner = Join-Path $runDir "$old.cmd"
  if (Test-Path $runner) {
    if ($DryRun) { Write-Output "DRYRUN: would remove $runner" }
    else { Remove-Item $runner -Force; Write-Output "runner $old.cmd removed" }
  }
}

foreach ($a in $existing.items) {
  if ($a.label -eq $Label) {
    if ($DryRun) {
      Write-Output "DRYRUN: would re-register agent $Label"
    } else {
      try { Invoke-Api "DELETE" "/api/agents/$($a.id)" $null $opToken | Out-Null } catch {}
    }
  }
}
if ($DryRun) {
  Write-Output "DRYRUN: would register team executor $Label (lane T, account $ChatAccountId)"
  Write-Output "DRYRUN_OK (not registered)"
  exit 0
}

$reg = Invoke-Api "POST" "/api/agents" @{ label = $Label; lane = "T"; host = $env:COMPUTERNAME; accountId = $ChatAccountId } $opToken
$token = $reg.token
New-Item -ItemType Directory -Path $runDir -Force | Out-Null
$runner = Join-Path $runDir "$Label.cmd"
$pythonw = Join-Path (Split-Path $python -Parent) "pythonw.exe"
if (-not (Test-Path $pythonw)) { $pythonw = $python }
$runnerBody = "@echo off`r`n" +
  "cd /d `"$workerDir`"`r`n" +
  "set SERVER_URL=$ServerUrl`r`n" +
  "set AGENT_TOKEN=$token`r`n" +
  "set WORKER_NAME=$Label`r`n" +
  "set SOMUN_REAL_AI=1`r`n" +
  "set SOMUN_CHAT_ACCOUNT_ID=$ChatAccountId`r`n" +
  "start `"`" `"$pythonw`" supervisor.py $Label`r`n"
[System.IO.File]::WriteAllText($runner, $runnerBody)
$lnkPath = Join-Path $startup "SOMUN Team Executor.lnk"
$ws = New-Object -ComObject WScript.Shell
$lnk = $ws.CreateShortcut($lnkPath)
$lnk.TargetPath = $runner
$lnk.WorkingDirectory = $workerDir
$lnk.WindowStyle = 7
$lnk.Description = "SOMUN team executor (lane T)"
$lnk.Save()
[System.Runtime.Interopservices.Marshal]::ReleaseComObject($ws) | Out-Null
Start-Process -FilePath $runner -WindowStyle Minimized
Write-Output "TEAM EXECUTOR OK (started)"
