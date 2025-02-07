@echo off
echo This script requires administative rights!
echo Please execute it as administrator.

if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Start protocol services

REM SELECT PROTOCOLS TO START

REM net start JSON_SCADA_demo_simul
REM net start JSON_SCADA_mongofw
REM net start JSON_SCADA_mongowr
net start JSON_SCADA_iec104client
rem net start JSON_SCADA_iec101client
rem net start JSON_SCADA_iccpclient
net start JSON_SCADA_mqttsparkplugclient
rem net start JSON_SCADA_plc4xclient
net start JSON_SCADA_telegraf_listener
net start JSON_SCADA_telegraf_runtime
REM net start JSON_SCADA_iec104server
REM net start JSON_SCADA_iec101server
REM net start JSON_SCADA_dnp3server
REM net start JSON_SCADA_iccpserver
REM net start JSON_SCADA_iec101client
REM net start JSON_SCADA_dnp3client
net start JSON_SCADA_opcuaclient
rem net start JSON_SCADA_opcdaclient
REM net start JSON_SCADA_iec61850client
REM net start JSON_SCADA_i104m
REM net start JSON_SCADA_plctags
net start JSON_SCADA_opcuaserver
