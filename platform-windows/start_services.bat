echo This script requires administative rights!
echo Please execute it as administrator.

@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Start services, uncomment the services you need

net start JSON_SCADA_log_io_server
net start JSON_SCADA_postgresql 
net start JSON_SCADA_mongodb 

ping -n 5 127.0.0.1

net start JSON_SCADA_log_io_file
net start JSON_SCADA_cs_data_processor
net start JSON_SCADA_cs_custom_processor
net start JSON_SCADA_server_realtime_auth
net start JSON_SCADA_calculations 
net start JSON_SCADA_process_rtdata
net start JSON_SCADA_process_hist 
REM net start JSON_SCADA_config_server_excel
REM net start JSON_SCADA_alarm_beep

call start_protocols.bat

ping -n 3 127.0.0.1

net start JSON_SCADA_grafana
net start JSON_SCADA_metabase

net start JSON_SCADA_php
net start JSON_SCADA_nginx
