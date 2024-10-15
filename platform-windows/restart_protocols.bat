@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

call stop_protocols.bat
ping -n 10
call start_protocols.bat
