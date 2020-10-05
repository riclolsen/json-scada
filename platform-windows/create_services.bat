echo This script requires administative rights!
echo Please execute it as administrator.

@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Create services, they will work not interactively and independently of a logged user

cd \json-scada\bin

C:\json-scada\platform-windows\postgresql-runtime\bin\pg_ctl.exe register -N JSON_SCADA_postgresql -D "C:\json-scada\platform-windows\postgresql-data"

nssm install JSON_SCADA_grafana "C:\json-scada\platform-windows\grafana-runtime\bin\grafana-server.exe"
nssm set AppDirectory JSON_SCADA_grafana  "C:\json-scada\platform-windows\grafana-runtime\bin"

nssm install JSON_SCADA_mongodb "C:\json-scada\platform-windows\mongodb-runtime\bin\mongod.exe" --config  "c:\json-scada\platform-windows\mongodb-conf\mongod.cfg" 

nssm install JSON_SCADA_calculations "C:\json-scada\bin\calculations.exe" 1 1 "c:\json-scada\json-scada.json"
REM STDOUT logging
nssm set JSON_SCADA_calculations AppStdout C:\json-scada\log\calculations.log
REM See log rotation options https://nssm.cc/usage#io

nssm install JSON_SCADA_cs_data_processor "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\cs_data_processor\cs_data_processor.js" 1 1 "c:\json-scada\json-scada.json"
nssm set AppDirectory JSON_SCADA_cs_data_processor  "C:\json-scada\src\cs_data_processor"
nssm install JSON_SCADA_server_realtime  "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\server_realtime\index.js" 1 1 "c:\json-scada\json-scada.json"
nssm set AppDirectory JSON_SCADA_server_realtime "C:\json-scada\src\server_realtime"

nssm install JSON_SCADA_process_rtdata "C:\json-scada\sql\process_pg_rtdata.bat"
nssm set JSON_SCADA_process_rtdata AppDirectory "C:\json-scada\sql"
nssm install JSON_SCADA_process_hist "C:\json-scada\sql\process_pg_hist.bat"
nssm set JSON_SCADA_process_hist AppDirectory "C:\json-scada\sql"

nssm install JSON_SCADA_iec104client "C:\json-scada\bin\iec104client.exe" 1 1 
nssm install JSON_SCADA_iec104server "C:\json-scada\bin\iec104server.exe" 1 1

REM nssm install JSON_SCADA_iec101client "C:\json-scada\bin\iec101client.exe" 1 1 
REM nssm install JSON_SCADA_iec101server "C:\json-scada\bin\iec101server.exe" 1 1
REM nssm install JSON_SCADA_dnp3client "C:\json-scada\bin\iec104client.exe" 1 1 
REM nssm install JSON_SCADA_i104m "C:\json-scada\bin\i104m.exe" 1 1 

