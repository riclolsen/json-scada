echo This script requires administative rights!
echo Please execute it as administrator.

rem Start services

cd \json-scada\bin

net start JSON_SCADA_postgresql 

nssm start JSON_SCADA_mongodb 
nssm start JSON_SCADA_calculations 
nssm start JSON_SCADA_process_dumpdb 
nssm start JSON_SCADA_process_hist 
