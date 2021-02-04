echo This script requires administative rights!
echo Please execute it as administrator.

@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Start services

cd \json-scada\platform-windows

net start JSON_SCADA_postgresql 
nssm start JSON_SCADA_grafana
nssm start JSON_SCADA_mongodb 
nssm start JSON_SCADA_cs_data_processor
rem nssm start JSON_SCADA_server_realtime
nssm start JSON_SCADA_server_realtime_auth
nssm start JSON_SCADA_calculations 
nssm start JSON_SCADA_process_rtdata
nssm start JSON_SCADA_process_hist 
REM nssm start JSON_SCADA_alarm_beep
REM nssm start JSON_SCADA_shell_api

nssm start JSON_SCADA_php
nssm start JSON_SCADA_nginx

REM SELECT PROTOCOLS TO START
nssm start JSON_SCADA_iec104client
REM nssm start JSON_SCADA_iec104server
REM nssm start JSON_SCADA_iec101server
REM nssm start JSON_SCADA_iec101client
REM nssm start JSON_SCADA_dnp3client
REM nssm start JSON_SCADA_opcuaclient
REM nssm start JSON_SCADA_i104m
REM nssm start JSON_SCADA_plctags
