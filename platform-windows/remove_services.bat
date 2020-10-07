echo This script requires administative rights!
echo Please execute it as administrator.

@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

rem Stop services

call stop_services.bat

rem Remove service without confimation

sc delete JSON_SCADA_postgresql 
nssm remove JSON_SCADA_grafana confirm
nssm remove JSON_SCADA_mongodb confirm 
nssm remove JSON_SCADA_cs_data_processor confirm
nssm remove JSON_SCADA_server_realtime confirm
nssm remove JSON_SCADA_calculations confirm
nssm remove JSON_SCADA_process_rtdata confirm
nssm remove JSON_SCADA_process_hist confirm
nssm remove JSON_SCADA_alarm_beep confirm

nssm remove JSON_SCADA_nginx confirm
nssm remove JSON_SCADA_php confirm

nssm remove JSON_SCADA_iec104server confirm
nssm remove JSON_SCADA_iec104client confirm
nssm remove JSON_SCADA_iec101server confirm
nssm remove JSON_SCADA_iec101client confirm
nssm remove JSON_SCADA_dnp3client confirm
nssm remove JSON_SCADA_i104m confirm
nssm remove JSON_SCADA_plctags confirm

