echo This script requires administative rights!
echo Please execute it as administrator.

@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

REM cd C:\json-scada\platform-windows\grafana-runtime\bin\
REM grafana-server
net start JSON_SCADA_grafana