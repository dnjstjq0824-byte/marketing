<#
.SYNOPSIS
  로그온 시 start.ps1을 실행하는 작업 스케줄러 항목을 등록한다.
  -DryRun으로 실제 등록 없이 명령만 확인한다.
.EXAMPLE
  powershell -File scripts\install-task.ps1 -DryRun
  powershell -File scripts\install-task.ps1 -Env production
#>
param(
  [string]$Root = (Split-Path $PSScriptRoot -Parent),
  [string]$TaskName = "SOMUN Marketing Core",
  [string]$Env = "production",
  [string]$Port = "8080",
  [string]$WorkerToken = "",
  [switch]$DryRun
)

$ErrorActionPreference = "Stop"
$startScript = Join-Path $Root "scripts\start.ps1"
if ($WorkerToken -ne "") {
  $action = "powershell.exe -NoProfile -ExecutionPolicy Bypass -Command `"& { `$env:WORKER_TOKEN='$WorkerToken'; & '$startScript' -Env $Env -Port $Port }`""
} else {
  $action = "powershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$startScript`" -Env $Env -Port $Port"
}
$cmd = "schtasks /Create /TN `"$TaskName`" /TR `"$action`" /SC ONLOGON /RL LIMITED /F"
Write-Output $cmd
if ($DryRun) {
  Write-Output "DRYRUN_OK (not registered)"
  return
}
$taskArgs = @("/Create", "/TN", $TaskName, "/TR", $action, "/SC", "ONLOGON", "/RL", "LIMITED", "/F")
& schtasks @taskArgs
if ($LASTEXITCODE -ne 0) { throw "schtasks failed with exit $LASTEXITCODE" }
Write-Output "TASK_OK"
