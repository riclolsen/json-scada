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
net stop JSON_SCADA_iec61850goclient
net stop JSON_SCADA_iec61850goserver
net stop JSON_SCADA_iec61850server
net stop JSON_SCADA_i104m
rem net stop JSON_SCADA_plctags
net stop JSON_SCADA_iccpclient
net stop JSON_SCADA_mqttsparkplugclient
rem net stop JSON_SCADA_plc4xclient
net stop JSON_SCADA_plc4jclient
net stop JSON_SCADA_telegraf_runtime
net stop JSON_SCADA_telegraf_listener
net stop JSON_SCADA_modbusclient
net stop JSON_SCADA_modbusserver
net stop JSON_SCADA_nodered_driver
net stop JSON_SCADA_n8nclient
net stop JSON_SCADA_iec104server
net stop JSON_SCADA_iec101server
net stop JSON_SCADA_opcuaserver
net stop JSON_SCADA_iccpserver
net stop JSON_SCADA_dnp3server

REM ADJUST HERE THE SERVICES YOU WANT TO START

REM net start JSON_SCADA_demo_simul
REM net start JSON_SCADA_mongofw
REM net start JSON_SCADA_mongowr
net start JSON_SCADA_iec104client
rem net start JSON_SCADA_iec101client
net start JSON_SCADA_mqttsparkplugclient
rem net start JSON_SCADA_plc4xclient
rem net start JSON_SCADA_plc4jclient
net start JSON_SCADA_telegraf_listener
net start JSON_SCADA_telegraf_runtime
rem net start JSON_SCADA_nodered_driver
rem net start JSON_SCADA_n8nclient
REM net start JSON_SCADA_iec104server
REM net start JSON_SCADA_iec101server
rem net start JSON_SCADA_iccpclient
REM net start JSON_SCADA_iccpserver
REM net start JSON_SCADA_iec101client
REM net start JSON_SCADA_dnp3client
net start JSON_SCADA_opcuaclient
rem net start JSON_SCADA_opcdaclient
REM net start JSON_SCADA_iec61850client
REM net start JSON_SCADA_iec61850server
REM net start JSON_SCADA_i104m
REM net start JSON_SCADA_plctags
REM net start JSON_SCADA_onvif
net start JSON_SCADA_opcuaserver

