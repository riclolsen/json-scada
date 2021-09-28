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
nssm set JSON_SCADA_grafana AppEnvironmentExtra GF_SERVER_DOMAIN=127.0.0.1 GF_SERVER_ROOT_URL=%(protocol)s://%(domain)s:80/grafana/ GF_SERVER_SERVE_FROM_SUB_PATH=true GF_AUTH_PROXY_ENABLED=true GF_AUTH_PROXY_ENABLE_LOGIN_TOKEN=true GF_AUTH_DISABLE_SIGNOUT_MENU=true
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

nssm install JSON_SCADA_cs_custom_processor "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\cs_custom_processor\cs_custom_processor.js" 1 1 "c:\json-scada\conf\json-scada.json"
nssm set JSON_SCADA_cs_custom_processor AppDirectory  "C:\json-scada\src\cs_custom_processor"
nssm set JSON_SCADA_cd_custom_processor Start SERVICE_AUTO_START


REM CHOOSE ONE: server_realtime (no user control) or server_realtime_auth (token based auth and RBAC)

REM nssm install JSON_SCADA_server_realtime  "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\server_realtime\index.js" NOAUTH
REM nssm set JSON_SCADA_server_realtime AppDirectory "C:\json-scada\src\server_realtime"
REM nssm set JSON_SCADA_server_realtime Start SERVICE_AUTO_START
rem Use environment variables to connect (for reading) to PostgreSQL historian (https://www.postgresql.org/docs/current/libpq-envars.html)
rem nssm set JSON_SCADA_server_realtime AppEnvironmentExtra PGHOSTADDR=127.0.0.1 PGPORT=27017 PGDATABASE=json_scada PGUSER=json_scada PGPASSWORD=json_scada

REM CREATE A RANDOM SECRET FOR JWT ENCRYPTION
setlocal EnableDelayedExpansion
set char=abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789
set count=0
set /a length=25
:Loop
set /a count+=1
set /a rand=%Random%%%62
set buffer=!buffer!!char:~%rand%,1!
if !count! leq !length! goto Loop

nssm install JSON_SCADA_server_realtime_auth  "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\server_realtime_auth\index.js" 
nssm set JSON_SCADA_server_realtime_auth AppDirectory "C:\json-scada\src\server_realtime_auth"
nssm set JSON_SCADA_server_realtime_auth Start SERVICE_AUTO_START
nssm set JSON_SCADA_server_realtime_auth AppEnvironmentExtra JS_JWT_SECRET=%buffer%
rem Use environment variables to connect (for reading) to PostgreSQL historian (https://www.postgresql.org/docs/current/libpq-envars.html)
rem nssm set JSON_SCADA_server_realtime_auth AppEnvironmentExtra PGHOSTADDR=127.0.0.1 PGPORT=27017 PGDATABASE=json_scada PGUSER=json_scada PGPASSWORD=json_scada

nssm install JSON_SCADA_demo_simul  "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\demo_simul\index.js" 
nssm set JSON_SCADA_demo_simul AppDirectory "C:\json-scada\src\demo_simul"
nssm set JSON_SCADA_demo_simul SERVICE_DEMAND_START

nssm install JSON_SCADA_alarm_beep  "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\alarm_beep\alarm_beep.js" 
nssm set JSON_SCADA_alarm_beep AppDirectory "C:\json-scada\src\alarm_beep"
nssm set JSON_SCADA_alarm_beep Start SERVICE_AUTO_START

nssm install JSON_SCADA_config_server_excel  "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\config_server_for_excel\index.js" 
nssm set JSON_SCADA_config_server_excel AppDirectory "C:\json-scada\src\config_server_for_excel"
nssm set JSON_SCADA_config_server_excel Start SERVICE_AUTO_START

rem For use with OSHMI HMI Shell
rem nssm install JSON_SCADA_shell_api  "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\shell-api\shell-api.js" 
rem nssm set JSON_SCADA_shell_api AppDirectory "C:\json-scada\src\shell-api"
rem nssm set JSON_SCADA_shell_api Start SERVICE_AUTO_START

nssm install JSON_SCADA_process_rtdata "C:\json-scada\sql\process_pg_rtdata.bat"
nssm set JSON_SCADA_process_rtdata AppDirectory "C:\json-scada\sql"
nssm set JSON_SCADA_process_rtdata Start SERVICE_AUTO_START
rem Use environment variables to connect (for writing) to PostgreSQL historian (https://www.postgresql.org/docs/current/libpq-envars.html)
rem nssm set JSON_SCADA_process_rtdata AppEnvironmentExtra PGHOSTADDR=127.0.0.1 PGPORT=27017 PGDATABASE=json_scada PGUSER=json_scada PGPASSWORD=json_scada

nssm install JSON_SCADA_process_hist "C:\json-scada\sql\process_pg_hist.bat"
nssm set JSON_SCADA_process_hist AppDirectory "C:\json-scada\sql"
nssm set JSON_SCADA_process_hist Start SERVICE_AUTO_START
rem Use environment variables to connect (for writing) to PostgreSQL historian (https://www.postgresql.org/docs/current/libpq-envars.html)
rem nssm set JSON_SCADA_process_hist AppEnvironmentExtra PGHOSTADDR=127.0.0.1 PGPORT=27017 PGDATABASE=json_scada PGUSER=json_scada PGPASSWORD=json_scada

nssm install JSON_SCADA_php "c:\json-scada\platform-windows\nginx_php-runtime\php\php-cgi.exe" -b 127.0.0.1:9000 -c c:\json-scada\conf\php.ini
nssm set JSON_SCADA_php Start SERVICE_AUTO_START

nssm install JSON_SCADA_nginx "c:\json-scada\platform-windows\nginx_php-runtime\nginx.exe" -c c:\json-scada\conf\nginx.conf
nssm set JSON_SCADA_nginx Start SERVICE_AUTO_START

REM SELECT THE DESIRED PROTOCOL DRIVERS (service startup options: SERVICE_AUTO_START, SERVICE_DELAYED_START, SERVICE_DEMAND_START, SERVICE_DISABLED)

nssm install JSON_SCADA_mqttsparkplugclient "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\mqtt-sparkplug\index.js" 
nssm set JSON_SCADA_mqttsparkplugclient AppDirectory "C:\json-scada\src\mqtt-sparkplug"
nssm set JSON_SCADA_mqttsparkplugclient Start SERVICE_AUTO_START

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

nssm install JSON_SCADA_opcuaclient "C:\json-scada\bin\OPC-UA-Client.exe" 1 1 
nssm set JSON_SCADA_opcuaclient Start SERVICE_DEMAND_START

nssm install JSON_SCADA_i104m "C:\json-scada\bin\i104m.exe" 1 1 
nssm set JSON_SCADA_i104m Start SERVICE_DEMAND_START

nssm install JSON_SCADA_plctags "C:\json-scada\bin\PLCTagsClient.exe" 1 1 
nssm set JSON_SCADA_plctags Start SERVICE_DEMAND_START

REM service for OPC-UA Server
nssm install JSON_SCADA_opcuaserver "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\OPC-UA-Server\index.js"  1 1 "c:\json-scada\conf\json-scada.json"
nssm set JSON_SCADA_opcuaserver AppDirectory "C:\json-scada\src\OPC-UA-Server"
nssm set JSON_SCADA_opcuaserver Start SERVICE_DEMAND_START

REM service for telegraf listener
nssm install JSON_SCADA_telegraf_listener "C:\json-scada\platform-windows\nodejs-runtime\node.exe" "C:\json-scada\src\telegraf-listener\index.js" 
nssm set JSON_SCADA_telegraf_listener AppDirectory "C:\json-scada\src\telegraf-listener"
nssm set JSON_SCADA_telegraf_listener Start SERVICE_AUTO_START

cd \json-scada\platform-windows\telegraf-runtime

REM service for telegraf runtime
C:\json-scada\platform-windows\telegraf-runtime\telegraf --service install --service-name="JSON_SCADA_telegraf_runtime" --service-display-name="JSON_SCADA_telegraf_runtime" --config "C:\json-scada\conf\telegraf.conf"

