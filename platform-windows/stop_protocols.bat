@echo off
echo This script requires administative rights!
echo Please execute it as administrator.

if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Stop protocol services

cd \json-scada\platform-windows

REM STOP PROTOCOL CLIENTS
nssm stop JSON_SCADA_demo_simul
nssm stop JSON_SCADA_mongofw
nssm stop JSON_SCADA_mongowr
nssm stop JSON_SCADA_iec104client
nssm stop JSON_SCADA_iec101client
nssm stop JSON_SCADA_dnp3client
nssm stop JSON_SCADA_opcuaclient
nssm stop JSON_SCADA_opcdaclient
nssm stop JSON_SCADA_iec61850client
nssm stop JSON_SCADA_i104m
nssm stop JSON_SCADA_plctags
nssm stop JSON_SCADA_iccpclient
nssm stop JSON_SCADA_mqttsparkplugclient
nssm stop JSON_SCADA_plc4xclient
net stop JSON_SCADA_telegraf_runtime
nssm stop JSON_SCADA_telegraf_listener
ping -n 2

REM STOP PROTOCOL SERVERS
nssm stop JSON_SCADA_iec104server
nssm stop JSON_SCADA_iec101server
nssm stop JSON_SCADA_opcuaserver
nssm stop JSON_SCADA_iccpserver
ping -n 2

