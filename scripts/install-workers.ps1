<#
.SYNOPSIS
  상주 작업자(A/B/C 레인) 3기를 에이전트로 등록하고 시작 폴더에 등록한다.
  실AI 실행(SOMUN_REAL_AI=1)으로 동작하며, 로그온 때마다 자동 시작한다.
  관리자 권한 불필요. 토큰은 러너 파일에 저장된다(로컬 전용, git 제외).
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
$lanes = @("A", "B", "C")
$existing = Invoke-Api "GET" "/api/agents?limit=200" $null $opToken

foreach ($lane in $lanes) {
  $label = "resident-$($lane.ToLower())"
  foreach ($a in $existing.items) {
    if ($a.label -eq $label) {
      # Dead workers may hold claims: cancel them so delete + rotation work.
      # Cancelled jobs stay visible as failed and can be requeued in console.
      try {
        $jobs = Invoke-Api "GET" "/api/jobs" $null $opToken
        foreach ($j in $jobs.jobs) {
          if ($j.workerName -eq $label -and ($j.status -eq "claimed" -or $j.status -eq "pending")) {
            try { Invoke-Api "POST" "/api/jobs/$($j.id)/cancel" @{} $opToken | Out-Null } catch {}
          }
        }
      } catch {}
      try { Invoke-Api "DELETE" "/api/agents/$($a.id)" $null $opToken | Out-Null } catch {}
    }
  }
  if ($DryRun) {
    Write-Output "DRYRUN: would register agent $label (lane $lane)"
    continue
  }
  $reg = Invoke-Api "POST" "/api/agents" @{ label = $label; lane = $lane; host = $env:COMPUTERNAME } $opToken
  $token = $reg.token
  New-Item -ItemType Directory -Path $runDir -Force | Out-Null
  $runner = Join-Path $runDir "$label.cmd"
  $pythonw = Join-Path (Split-Path $python -Parent) "pythonw.exe"
  if (-not (Test-Path $pythonw)) { $pythonw = $python }
  $runnerBody = "@echo off`r`n" +
    "cd /d `"$workerDir`"`r`n" +
    "set SERVER_URL=$ServerUrl`r`n" +
    "set AGENT_TOKEN=$token`r`n" +
    "set WORKER_NAME=$label`r`n" +
    "set SOMUN_REAL_AI=1`r`n" +
    "set SOMUN_CHAT_ACCOUNT_ID=$ChatAccountId`r`n" +
    "start `"`" `"$pythonw`" supervisor.py $label`r`n"
  [System.IO.File]::WriteAllText($runner, $runnerBody)
  $lnkPath = Join-Path $startup "SOMUN Worker $lane.lnk"
  $ws = New-Object -ComObject WScript.Shell
  $lnk = $ws.CreateShortcut($lnkPath)
  $lnk.TargetPath = $runner
  $lnk.WorkingDirectory = $workerDir
  $lnk.WindowStyle = 7
  $lnk.Description = "SOMUN resident worker lane $lane"
  $lnk.Save()
  [System.Runtime.Interopservices.Marshal]::ReleaseComObject($ws) | Out-Null
  Start-Process -FilePath $runner -WindowStyle Minimized
  Write-Output "WORKER $lane OK (started)"
}

if ($DryRun) {
  Write-Output "DRYRUN_OK (not registered)"
} else {
  Write-Output "WORKERS_OK"
}
