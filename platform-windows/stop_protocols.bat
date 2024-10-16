@echo off
echo This script requires administative rights!
echo Please execute it as administrator.

if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Stop protocol services

REM STOP PROTOCOL CLIENTS
net stop JSON_SCADA_demo_simul
net stop JSON_SCADA_mongofw
net stop JSON_SCADA_mongowr
net stop JSON_SCADA_iec104client
net stop JSON_SCADA_iec101client
net stop JSON_SCADA_dnp3client
net stop JSON_SCADA_opcuaclient
net stop JSON_SCADA_opcdaclient
net stop JSON_SCADA_iec61850client
net stop JSON_SCADA_i104m
net stop JSON_SCADA_plctags
net stop JSON_SCADA_iccpclient
net stop JSON_SCADA_mqttsparkplugclient
net stop JSON_SCADA_plc4xclient
net stop JSON_SCADA_telegraf_runtime
net stop JSON_SCADA_telegraf_listener
ping -n 2

REM STOP PROTOCOL SERVERS
net stop JSON_SCADA_iec104server
net stop JSON_SCADA_iec101server
net stop JSON_SCADA_opcuaserver
net stop JSON_SCADA_iccpserver
ping -n 2

