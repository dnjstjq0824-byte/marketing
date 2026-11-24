<#
.SYNOPSIS
  SOMUN 완전 무인 실행기 일괄 등록·자동 기동 스크립트.
  - 팀 실행기 1기(작성·분석·전략, lane T)
  - 교차 검수기 1기(독립 AI 검수, lane T, -ReviewerAccountId 지정 시)
  - 발행·성과 수집 어댑터 1기(네이버 블로그 등 직접군, lane P, -PublishAccountId 지정 시)
  전부 adapter\.venv (Playwright 포함) + UTF-8 + PYTHONPATH=. 로 기동하며,
  Windows 시작 폴더에 등록해 재부팅 후에도 무인으로 자동 시작한다.
.EXAMPLE
  powershell -File scripts\install-workers.ps1 -DryRun
  powershell -File scripts\install-workers.ps1 -ServerUrl http://127.0.0.1:8080 -ChatAccountId 3 -ReviewerAccountId 4 -PublishAccountId 31
#>
param(
  [string]$Root = (Split-Path $PSScriptRoot -Parent),
  [string]$ServerUrl = "http://127.0.0.1:8080",
  [string]$AdminUser = "admin",
  [string]$AdminPass = "changeme123",
  [int]$ChatAccountId = 3,
  [int]$ReviewerAccountId = 4,
  [int]$PublishAccountId = 31,
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

$venvPythonw = Join-Path $Root "adapter\.venv\Scripts\pythonw.exe"
$venvPython  = Join-Path $Root "adapter\.venv\Scripts\python.exe"
if (Test-Path $venvPythonw) {
  $pythonExec = $venvPythonw
} elseif (Test-Path $venvPython) {
  $pythonExec = $venvPython
} else {
  $pythonExec = (Get-Command python -ErrorAction SilentlyContinue).Source
}
if (-not $pythonExec) { Write-Output "PYTHON NOT FOUND"; exit 1 }

$workerDir = Join-Path $Root "worker"
$runDir = Join-Path $Root "scripts\workers"
$startup = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\Startup"
$existing = Invoke-Api "GET" "/api/agents?limit=200" $null $opToken

# W16-7: 레인별 구형 상주 작업자 정리
foreach ($lane in @("A", "B", "C")) {
  $old = "resident-$($lane.ToLower())"
  foreach ($a in $existing.items) {
    if ($a.label -eq $old) {
      if ($DryRun) { Write-Output "DRYRUN: would delete retired agent $old" }
      else { try { Invoke-Api "DELETE" "/api/agents/$($a.id)" $null $opToken | Out-Null } catch {} }
    }
  }
  $lnk = Join-Path $startup "SOMUN Worker $lane.lnk"
  if ((Test-Path $lnk) -and (-not $DryRun)) { Remove-Item $lnk -Force }
  $runner = Join-Path $runDir "$old.cmd"
  if ((Test-Path $runner) -and (-not $DryRun)) { Remove-Item $runner -Force }
}

function Register-Runner($agentLabel, $lane, $acctId, $mode, $shortcutName) {
  foreach ($a in $existing.items) {
    if ($a.label -eq $agentLabel) {
      if ($DryRun) { Write-Output "DRYRUN: would re-register $agentLabel" }
      else { try { Invoke-Api "DELETE" "/api/agents/$($a.id)" $null $opToken | Out-Null } catch {} }
    }
  }
  if ($DryRun) {
    Write-Output "DRYRUN: would register $agentLabel (lane $lane, account $acctId, mode $mode)"
    return
  }
  $reg = Invoke-Api "POST" "/api/agents" @{ label = $agentLabel; lane = $lane; host = $env:COMPUTERNAME; accountId = $acctId } $opToken
  $token = $reg.token
  New-Item -ItemType Directory -Path $runDir -Force | Out-Null
  $runner = Join-Path $runDir "$agentLabel.cmd"
  if ($mode -eq "publisher") {
    $runnerBody = "@echo off`r`n" +
      "cd /d `"$Root`"`r`n" +
      "set SERVER_URL=$ServerUrl`r`n" +
      "set AGENT_TOKEN=$token`r`n" +
      "set WORKER_NAME=$agentLabel`r`n" +
      "set PYTHONUTF8=1`r`n" +
      "set PYTHONPATH=$Root`r`n" +
      "start `"`" `"$pythonExec`" -m adapter.worker_adapter`r`n"
  } else {
    $runnerBody = "@echo off`r`n" +
      "cd /d `"$workerDir`"`r`n" +
      "set SERVER_URL=$ServerUrl`r`n" +
      "set AGENT_TOKEN=$token`r`n" +
      "set WORKER_NAME=$agentLabel`r`n" +
      "set SOMUN_REAL_AI=1`r`n" +
      "set SOMUN_CHAT_ACCOUNT_ID=$acctId`r`n" +
      "set PYTHONUTF8=1`r`n" +
      "set PYTHONPATH=$Root`r`n" +
      "start `"`" `"$pythonExec`" supervisor.py $agentLabel`r`n"
  }
  [System.IO.File]::WriteAllText($runner, $runnerBody)
  $lnkPath = Join-Path $startup "$shortcutName.lnk"
  $ws = New-Object -ComObject WScript.Shell
  $lnk = $ws.CreateShortcut($lnkPath)
  $lnk.TargetPath = $runner
  $lnk.WorkingDirectory = if ($mode -eq "publisher") { $Root } else { $workerDir }
  $lnk.WindowStyle = 7
  $lnk.Description = "SOMUN $agentLabel (lane $lane)"
  $lnk.Save()
  [System.Runtime.Interopservices.Marshal]::ReleaseComObject($ws) | Out-Null
  Start-Process -FilePath $runner -WindowStyle Minimized
  Write-Output "REGISTERED & STARTED: $agentLabel (lane $lane, account $acctId)"
}

Register-Runner $Label "T" $ChatAccountId "team" "SOMUN Team Executor"
if ($ReviewerAccountId -gt 0 -and $ReviewerAccountId -ne $ChatAccountId) {
  Register-Runner "$Label-reviewer" "T" $ReviewerAccountId "team" "SOMUN Team Reviewer"
}
if ($PublishAccountId -gt 0) {
  Register-Runner "resident-publisher" "P" $PublishAccountId "publisher" "SOMUN Publisher Adapter"
}

if ($DryRun) { Write-Output "DRYRUN_OK (not registered)" }
else { Write-Output "ALL SOMUN RUNNERS READY" }
