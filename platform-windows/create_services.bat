echo This script requires administative rights!
echo Please execute it as administrator.

@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Create services, they will work not interactively and independently of a logged user

cd \json-scada\bin

C:\json-scada\platform-windows\postgresql-runtime\bin\pg_ctl.exe register -N JSON_SCADA_postgresql -D "C:\json-scada\platform-windows\postgresql-data"
nssm set JSON_SCADA_postgresql Start SERVICE_AUTO_START

nssm install JSON_SCADA_grafana "C:\json-scada\platform-windows\grafana-runtime\bin\grafana-server.exe"
nssm set JSON_SCADA_grafana AppDirectory "C:\json-scada\platform-windows\grafana-runtime\bin"
nssm set JSON_SCADA_grafana AppEnvironmentExtra GF_SERVER_DOMAIN=127.0.0.1 GF_SERVER_ROOT_URL=%(protocol)s://%(domain)s:80/grafana/ GF_SERVER_SERVE_FROM_SUB_PATH=true
nssm set JSON_SCADA_grafana Start SERVICE_AUTO_START

nssm install JSON_SCADA_mongodb "C:\json-scada\platform-windows\mongodb-runtime\bin\mongod.exe" --config  "c:\json-scada\platform-windows\mongodb-conf\mongod.cfg" 
nssm set JSON_SCADA_mongodb Start SERVICE_AUTO_START

nssm install JSON_SCADA_calculations "C:\json-scada\bin\calculations.exe" 1 1 2.0 "c:\json-scada\conf\json-scada.json"
REM STDOUT logging
nssm set JSON_SCADA_calculations AppStdout C:\json-scada\log\calculations.log
nssm set JSON_SCADA_calculations Start SERVICE_AUTO_START
REM See log rotation options https://nssm.cc/usage#io

nssm install JSON_SCADA_cs_data_processor "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\cs_data_processor\cs_data_processor.js" 1 1 "c:\json-scada\conf\json-scada.json"
nssm set JSON_SCADA_cs_data_processor AppDirectory  "C:\json-scada\src\cs_data_processor"
nssm set JSON_SCADA_cd_data_processor Start SERVICE_AUTO_START

REM CHOOSE ONE: server_realtime (no user control) or server_realtime_auth (token based auth and RBAC)

REM nssm install JSON_SCADA_server_realtime  "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\server_realtime\index.js" NOAUTH
REM nssm set JSON_SCADA_server_realtime AppDirectory "C:\json-scada\src\server_realtime"
REM nssm set JSON_SCADA_server_realtime Start SERVICE_AUTO_START

nssm install JSON_SCADA_server_realtime_auth  "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\server_realtime_auth\index.js" 
nssm set JSON_SCADA_server_realtime_auth AppDirectory "C:\json-scada\src\server_realtime_auth"
nssm set JSON_SCADA_server_realtime_auth Start SERVICE_AUTO_START


nssm install JSON_SCADA_demo_simul  "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\demo_simul\index.js" 
nssm set JSON_SCADA_demo_simul AppDirectory "C:\json-scada\src\demo_simul"
nssm set JSON_SCADA_demo_simul SERVICE_DEMAND_START

nssm install JSON_SCADA_alarm_beep  "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\alarm_beep\alarm_beep.js" 
nssm set JSON_SCADA_alarm_beep AppDirectory "C:\json-scada\src\alarm_beep"
nssm set JSON_SCADA_alarm_beep Start SERVICE_AUTO_START

nssm install JSON_SCADA_process_rtdata "C:\json-scada\sql\process_pg_rtdata.bat"
nssm set JSON_SCADA_process_rtdata AppDirectory "C:\json-scada\sql"
nssm set JSON_SCADA_process_rtdata Start SERVICE_AUTO_START

nssm install JSON_SCADA_process_hist "C:\json-scada\sql\process_pg_hist.bat"
nssm set JSON_SCADA_process_hist AppDirectory "C:\json-scada\sql"
nssm set JSON_SCADA_process_hist Start SERVICE_AUTO_START

nssm install JSON_SCADA_php "c:\json-scada\platform-windows\nginx_php-runtime\php\php-cgi.exe" -b 127.0.0.1:9000 -c c:\json-scada\conf\php.ini
nssm set JSON_SCADA_php Start SERVICE_AUTO_START

nssm install JSON_SCADA_nginx "c:\json-scada\platform-windows\nginx_php-runtime\nginx.exe" -c c:\json-scada\conf\nginx.conf
nssm set JSON_SCADA_php Start SERVICE_AUTO_START

REM SELECT THE DESIRED PROTOCOL DRIVERS (service startup options: SERVICE_AUTO_START, SERVICE_DELAYED_START, SERVICE_DEMAND_START, SERVICE_DISABLED)

nssm install JSON_SCADA_iec104client "C:\json-scada\bin\iec104client.exe" 1 1 
nssm set JSON_SCADA_iec104client Start SERVICE_DELAYED_START

nssm install JSON_SCADA_iec104server "C:\json-scada\bin\iec104server.exe" 1 1
nssm set JSON_SCADA_iec104server Start SERVICE_DEMAND_START

nssm install JSON_SCADA_iec101client "C:\json-scada\bin\iec101client.exe" 1 1 
nssm set JSON_SCADA_iec101client Start SERVICE_DEMAND_START

nssm install JSON_SCADA_iec101server "C:\json-scada\bin\iec101server.exe" 1 1
nssm set JSON_SCADA_iec101server Start SERVICE_DEMAND_START

nssm install JSON_SCADA_dnp3client "C:\json-scada\bin\Dnp3Client.exe" 1 1 
nssm set JSON_SCADA_dnp3client Start SERVICE_DEMAND_START

nssm install JSON_SCADA_i104m "C:\json-scada\bin\i104m.exe" 1 1 
nssm set JSON_SCADA_i104m Start SERVICE_DEMAND_START

nssm install JSON_SCADA_plctags "C:\json-scada\bin\PLCTagsClient.exe" 1 1 
nssm set JSON_SCADA_plctags Start SERVICE_DEMAND_START
