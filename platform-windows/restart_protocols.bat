@echo off
echo This script requires administative rights!
echo Please execute it as administrator.
echo This script is to be called from a windows service (do not use call or start or powershell)

rem RESTART protocol services

rem stop protocol services
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
net stop JSON_SCADA_iec104server
net stop JSON_SCADA_iec101server
net stop JSON_SCADA_opcuaserver
net stop JSON_SCADA_iccpserver

REM ADJUST HERE THE SERVICES YOU WANT TO START

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
REM net start JSON_SCADA_iccpserver
REM net start JSON_SCADA_iec101client
REM net start JSON_SCADA_dnp3client
net start JSON_SCADA_opcuaclient
rem net start JSON_SCADA_opcdaclient
REM net start JSON_SCADA_iec61850client
REM net start JSON_SCADA_i104m
REM net start JSON_SCADA_plctags
net start JSON_SCADA_opcuaserver

