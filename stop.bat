@echo off
setlocal

set "ROOT_DIR=%~dp0"
if "%ROOT_DIR:~-1%"=="\" set "ROOT_DIR=%ROOT_DIR:~0,-1%"

echo Stopping SecMgmt processes for:
echo %ROOT_DIR%
echo.

powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$root = '%ROOT_DIR%';" ^
  "$ports = @(8000, 5173);" ^
  "$pids = New-Object System.Collections.Generic.HashSet[int];" ^
  "foreach ($port in $ports) {" ^
  "  Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue | ForEach-Object { [void]$pids.Add([int]$_.OwningProcess) }" ^
  "}" ^
  "$escapedRoot = [regex]::Escape($root);" ^
  "Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -and $_.CommandLine -match $escapedRoot -and ($_.Name -in @('go.exe','server.exe','node.exe','npm.cmd','cmd.exe')) } | ForEach-Object { [void]$pids.Add([int]$_.ProcessId) };" ^
  "foreach ($pid in $pids) {" ^
  "  if ($pid -le 0 -or $pid -eq $PID) { continue }" ^
  "  try { Stop-Process -Id $pid -Force -ErrorAction Stop; Write-Host ('Stopped PID ' + $pid) } catch { Write-Host ('Skip PID ' + $pid + ': ' + $_.Exception.Message) }" ^
  "}"

echo.
echo Done. You can also close the backend/frontend command windows manually.
exit /b 0
