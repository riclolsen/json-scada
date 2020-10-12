@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

REM c:\json-scada\platform-windows\postgresql-runtime\bin\pg_ctl stop -D ../postgresql-data

net stop JSON_SCADA_postgresql
