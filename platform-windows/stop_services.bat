echo This script requires administative rights!
echo Please execute it as administrator.

@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Stop services

cd \json-scada\platform-windows

net stop JSON_SCADA_postgresql 
nssm stop JSON_SCADA_grafana
nssm stop JSON_SCADA_mongodb 
nssm stop JSON_SCADA_cs_data_processor
nssm stop JSON_SCADA_server_realtime
nssm stop JSON_SCADA_calculations 
nssm stop JSON_SCADA_process_rtdata
nssm stop JSON_SCADA_process_hist 

nssm stop JSON_SCADA_iec104server
nssm stop JSON_SCADA_iec104client

nssm stop JSON_SCADA_nginx
nssm stop JSON_SCADA_php

REM nssm stop JSON_SCADA_iec101server
REM nssm stop JSON_SCADA_iec101client
REM nssm stop JSON_SCADA_dnp3client
REM nssm stop JSON_SCADA_i104m
