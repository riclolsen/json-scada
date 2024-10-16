echo This script requires administative rights!
echo Please execute it as administrator.

@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Stop services

REM FIRST STOP USERS
net stop JSON_SCADA_nginx
net stop JSON_SCADA_php
net stop JSON_SCADA_server_realtime_auth
net stop JSON_SCADA_alarm_beep
net stop JSON_SCADA_config_server_excel

REM STOP PROTOCOL DRIVERS
call stop_protocols.bat
ping -n 2

REM STOP OTHER PROCESSES
net stop JSON_SCADA_calculations 
net stop JSON_SCADA_cs_data_processor
net stop JSON_SCADA_cs_custom_processor
ping -n 2
net stop JSON_SCADA_process_rtdata
net stop JSON_SCADA_process_hist 
ping -n 3

REM STOP GRAFANA/METABASE AND DATABASE SERVERS
net stop JSON_SCADA_grafana
net stop JSON_SCADA_metabase
net stop JSON_SCADA_mongodb 
net stop JSON_SCADA_postgresql 

net stop JSON_SCADA_log_io_file
net stop JSON_SCADA_log_io_server
