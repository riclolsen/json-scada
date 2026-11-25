echo This script requires administative rights!
echo Please execute it as administrator.

@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

REM Create services, they will run in background independently of a logged user.
REM Services run under WinSW (platform-windows\WinSW.exe). Each service is defined by
REM platform-windows\service-templates\JSON_SCADA_<name>.xml; winsw-services.ps1 installs it as
REM platform-windows\services\JSON_SCADA_<name>.xml (live configuration, edit it and restart the
REM service) next to its WinSW binary JSON_SCADA_<name>.exe. Logs go to C:\json-scada\log\<name>.out.log
REM and .err.log (rolled at 10 MB, one rolled file kept). Already existing services keep their settings.
REM Start modes (Automatic/Manual, delayedAutoStart) are set in the templates: change startmode there
REM before running this script, or later with: sc config JSON_SCADA_<name> start= auto (or delayed-auto, demand, disabled)
REM See winsw-services.ps1 for the other actions (Migrate from NSSM, Remove, Status, -Force re-create).

cd /d "%~dp0"

REM POSTGRESQL (service registered by pg_ctl itself)
C:\json-scada\platform-windows\postgresql-runtime\bin\pg_ctl.exe register -N JSON_SCADA_postgresql -D "C:\json-scada\platform-windows\postgresql-data"
sc config JSON_SCADA_postgresql start= auto

REM telegraf may have left its own event log source for this name
reg delete "HKLM\SYSTEM\CurrentControlSet\Services\EventLog\Application\JSON_SCADA_telegraf_runtime" /f

REM All other services: Grafana, Metabase, MongoDB, core processes, Nginx/PHP, protocol drivers, tools
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0winsw-services.ps1" -Action Install

REM OPC-DA Server
\json-scada\bin\OPC-DA_Server\OpcNetDaServer.exe -regserver

REM Create scheduled task for log rotation (alternative log rotator), configure with logrotate.conf
REM Should stop services to force log file to close. See https://sourceforge.net/p/logrotatewin/wiki/LogRotate/
REM SCHTASKS /CREATE /SC DAILY /TN "MyTasks\JSON-SCADA logrotate task" /TR "C:\json-scada\bin\logrotate C:\json-scada\platform-windows\logrotate.conf" /ST 04:00
