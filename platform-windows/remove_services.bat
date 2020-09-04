echo This script requires administative rights!
echo Please execute it as administrator.

rem Stop services

call stop_services.bat

rem Remove service without confimation

sc delete JSON_SCADA_postgresql

nssm remove JSON_SCADA_mongodb confirm
nssm remove JSON_SCADA_calculations confirm
nssm remove JSON_SCADA_process_dumpdb confirm
nssm remove JSON_SCADA_process_hist confirm

