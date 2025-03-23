echo This script requires administative rights!
echo Please execute it as administrator.

@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Stop services

call stop_services.bat

REM FORCE STOP OF ANY ACTIVE SERVICE OR PROCESS!
wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\sql\\%'" CALL TERMINATE
wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\grafana-runtime\\bin\\%'" CALL TERMINATE
wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\nginx_php-runtime\\php\\%'" CALL TERMINATE
wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\nginx_php-runtime\\%'" CALL TERMINATE
wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\nodejs-runtime\\%'" CALL TERMINATE
wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\browser-runtime\\%'" CALL TERMINATE
wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\mongodb-runtime\\%'" CALL TERMINATE
wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\inkscape-runtime\\%'" CALL TERMINATE
wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\telegraf-runtime\\%'" CALL TERMINATE
wmic PROCESS WHERE "COMMANDLINE LIKE '%..\\platform-windows\\%'" CALL TERMINATE
wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\bin\\%'" CALL TERMINATE

rem Remove service without confimation

sc delete JSON_SCADA_postgresql 
nssm remove JSON_SCADA_grafana confirm
nssm remove JSON_SCADA_metabase confirm
nssm remove JSON_SCADA_mongodb confirm 
nssm remove JSON_SCADA_cs_data_processor confirm
nssm remove JSON_SCADA_cs_custom_processor confirm
nssm remove JSON_SCADA_server_realtime_auth confirm
nssm remove JSON_SCADA_calculations confirm
nssm remove JSON_SCADA_process_rtdata confirm
nssm remove JSON_SCADA_process_hist confirm
nssm remove JSON_SCADA_alarm_beep confirm
nssm remove JSON_SCADA_demo_simul confirm
nssm remove JSON_SCADA_mongofw confirm
nssm remove JSON_SCADA_mongowr confirm
nssm remove JSON_SCADA_config_server_excel confirm

nssm remove JSON_SCADA_nginx confirm
nssm remove JSON_SCADA_php confirm

nssm remove JSON_SCADA_iec104server confirm
nssm remove JSON_SCADA_iec104client confirm
nssm remove JSON_SCADA_iec101server confirm
nssm remove JSON_SCADA_iec101client confirm
nssm remove JSON_SCADA_dnp3client confirm
nssm remove JSON_SCADA_dnp3server confirm
nssm remove JSON_SCADA_opcuaclient confirm
nssm remove JSON_SCADA_opcdaclient confirm
nssm remove JSON_SCADA_iec61850client confirm
nssm remove JSON_SCADA_i104m confirm
nssm remove JSON_SCADA_plctags confirm
nssm remove JSON_SCADA_plc4xclient confirm
nssm remove JSON_SCADA_telegraf_runtime confirm
nssm remove JSON_SCADA_telegraf_listener confirm
nssm remove JSON_SCADA_mqttsparkplugclient confirm
nssm remove JSON_SCADA_opcuaserver confirm
nssm remove JSON_SCADA_log_io_file confirm
nssm remove JSON_SCADA_log_io_server confirm
nssm remove JSON_SCADA_onvif confirm

PowerShell -NoProfile -ExecutionPolicy Bypass -Command "& {Start-Process PowerShell -ArgumentList '-NoProfile -ExecutionPolicy Bypass -File ""remove_jsonscada_services.ps1""' -Verb RunAs}"

