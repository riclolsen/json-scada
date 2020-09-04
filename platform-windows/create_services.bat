echo This script requires administative rights!
echo Please execute it as administrator.

rem Create services, they will work not interactively and independently of a logged user

cd \json-scada\bin

C:\json-scada\postgresql-runtime\bin\pg_ctl.exe register -N JSON_SCADA_postgresql -D "C:\json-scada\postgresql-data"

nssm install JSON_SCADA_mongodb "C:\json-scada\mongodb-runtime\bin\mongod.exe" --config  "c:\json-scada\mongodb-runtime\bin\mongod.cfg" 
nssm install JSON_SCADA_calculations "C:\json-scada\bin\calculations.exe"
nssm install JSON_SCADA_process_dumpdb "C:\json-scada\sql\process_pg_dumpdb.bat"
nssm install JSON_SCADA_process_hist "C:\json-scada\sql\process_pg_hist.bat"

