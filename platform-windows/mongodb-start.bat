@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem --dbpath="c:\json-scada\mongodb-data" --replSet rs1
rem "C:\json-scada\platform-windows\mongodb-runtime\bin\mongod.exe" --config  "c:\json-scada\platform-windows\mongodb-conf\mongod.cfg" 
net start JSON_SCADA_mongodb
