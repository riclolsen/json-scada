echo This script requires administative rights!
echo Please execute it as administrator.

@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Stop services

call stop_services.bat

REM FORCE STOP OF ANY ACTIVE SERVICE OR PROCESS!
REM Use PowerShell to terminate processes matching common JSON-SCADA paths.
REM WMIC is deprecated; using Get-CimInstance + Invoke-CimMethod for termination.
powershell -NoProfile -ExecutionPolicy Bypass -Command "& '%~dp0kill_processes_by_pattern.ps1' \
	-Patterns 'c:\\\\json-scada\\\\sql\\\\','c:\\\\json-scada\\\\platform-windows\\\\grafana-runtime\\\\bin\\\\','c:\\\\json-scada\\\\platform-windows\\\\nginx_php-runtime\\\\php\\\\','c:\\\\json-scada\\\\platform-windows\\\\nginx_php-runtime\\\\','c:\\\\json-scada\\\\platform-windows\\\\nodejs-runtime\\\\','c:\\\\json-scada\\\\platform-windows\\\\browser-runtime\\\\','c:\\\\json-scada\\\\platform-windows\\\\mongodb-runtime\\\\','c:\\\\json-scada\\\\platform-windows\\\\inkscape-runtime\\\\','c:\\\\json-scada\\\\platform-windows\\\\telegraf-runtime\\\\','..\\\\platform-windows\\\\','c:\\\\json-scada\\\\bin\\\\'"

rem Remove every JSON_SCADA_* service without confirmation (WinSW, NSSM or pg_ctl registered),
rem with the WinSW binaries and live XML files in platform-windows\services (logs are kept)

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0winsw-services.ps1" -Action Remove

\json-scada\bin\OPC-DA_Server\OpcNetDaServer.exe -unregserver

PowerShell -NoProfile -ExecutionPolicy Bypass -Command "& {Start-Process PowerShell -ArgumentList '-NoProfile -ExecutionPolicy Bypass -File ""remove_jsonscada_services.ps1""' -Verb RunAs}"
