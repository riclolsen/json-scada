@echo off
echo This script requires administative rights!
echo Please execute it as administrator.

if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Start protocol services

cd \json-scada\platform-windows

REM SELECT PROTOCOLS TO START

REM nssm start JSON_SCADA_demo_simul
REM nssm start JSON_SCADA_mongofw
REM nssm start JSON_SCADA_mongowr
nssm start JSON_SCADA_iec104client
rem nssm start JSON_SCADA_iec101client
rem nssm start JSON_SCADA_iccpclient
nssm start JSON_SCADA_mqttsparkplugclient
rem nssm start JSON_SCADA_plc4xclient
nssm start JSON_SCADA_telegraf_listener
net start JSON_SCADA_telegraf_runtime
REM nssm start JSON_SCADA_iec104server
REM nssm start JSON_SCADA_iec101server
REM nssm start JSON_SCADA_iccpserver
REM nssm start JSON_SCADA_iec101client
REM nssm start JSON_SCADA_dnp3client
nssm start JSON_SCADA_opcuaclient
rem nssm start JSON_SCADA_opcdaclient
REM nssm start JSON_SCADA_iec61850client
REM nssm start JSON_SCADA_i104m
REM nssm start JSON_SCADA_plctags
nssm start JSON_SCADA_opcuaserver
