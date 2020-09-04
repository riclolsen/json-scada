echo This script requires administative rights!
echo Please execute it as administrator.

rem Stop services

cd \json-scada\bin

net stop JSON_SCADA_postgresql 

nssm stop JSON_SCADA_mongodb 
nssm stop JSON_SCADA_calculations 
nssm stop JSON_SCADA_process_dumpdb 
nssm stop JSON_SCADA_process_hist 