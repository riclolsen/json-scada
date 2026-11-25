echo This script requires administative rights!
echo Please execute it as administrator.

@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Stop services

call "%~dp0stop_services.bat" am_admin

REM FORCE STOP OF ANY ACTIVE SERVICE OR PROCESS!
REM Terminate leftover processes matching paths under this JSON-SCADA installation.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0kill_processes_by_pattern.ps1"

rem Remove every JSON_SCADA_* service without confirmation (WinSW, NSSM or pg_ctl registered),
rem with the WinSW binaries and live XML files in platform-windows\services (logs are kept)

powershell -NoProfile -ExecutionPolicy Bypass -File "winsw-services.ps1" -Action Remove

\json-scada\bin\OPC-DA_Server\OpcNetDaServer.exe -unregserver

PowerShell -NoProfile -ExecutionPolicy Bypass -Command "& {Start-Process PowerShell -ArgumentList '-NoProfile -ExecutionPolicy Bypass -File ""remove_jsonscada_services.ps1""' -Verb RunAs}"
