echo This script requires administative rights!
echo Please execute it as administrator.

@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Stop services

cd \json-scada\platform-windows

REM FIRST STOP USERS
nssm stop JSON_SCADA_nginx
nssm stop JSON_SCADA_php
nssm stop JSON_SCADA_server_realtime
nssm stop JSON_SCADA_server_realtime_auth
nssm stop JSON_SCADA_alarm_beep
nssm stop JSON_SCADA_shell_api
ping -n 2

REM STOP PROTOCOL CLIENTS
nssm stop JSON_SCADA_iec104client
nssm stop JSON_SCADA_iec101client
nssm stop JSON_SCADA_dnp3client
nssm stop JSON_SCADA_opcuaclient
nssm stop JSON_SCADA_i104m
nssm stop JSON_SCADA_plctags
net stop JSON_SCADA_telegraf_runtime
nssm stop JSON_SCADA_telegraf_listener
ping -n 3

REM STOP PROTOCOL SERVERS
nssm stop JSON_SCADA_iec104server
nssm stop JSON_SCADA_iec101server
ping -n 2

REM STOP OTHER PROCESSES
nssm stop JSON_SCADA_calculations 
nssm stop JSON_SCADA_cs_data_processor
ping -n 2
nssm stop JSON_SCADA_process_rtdata
nssm stop JSON_SCADA_process_hist 
ping -n 3

REM STOP GRAFANA AND DATABASE SERVERS
nssm stop JSON_SCADA_grafana
nssm stop JSON_SCADA_mongodb 
net stop JSON_SCADA_postgresql 

