echo This script requires administative rights!
echo Please execute it as administrator.

rem Create services, they will work not interactively and independently of a logged user

cd \json-scada\bin

C:\json-scada\postgresql-runtime\bin\pg_ctl.exe register -N JSON_SCADA_postgresql -D "C:\json-scada\postgresql-data"

nssm install JSON_SCADA_mongodb "C:\json-scada\mongodb-runtime\bin\mongod.exe" --config  "c:\json-scada\mongodb-runtime\bin\mongod.cfg" 
nssm install JSON_SCADA_calculations "C:\json-scada\bin\calculations.exe" 1 1 "c:\json-scada\json-scada.json"

REM STDOUT logging
nssm set JSON_SCADA_calculations AppStdout C:\json-scada\log\calculations.log
REM See log rotation options https://nssm.cc/usage#io

nssm install JSON_SCADA_cs_data_processor "c:\Program Files\nodejs\node.exe" "C:\json-scada\src\cs_data_processor\cs_data_processor.js" 1 1 "c:\json-scada\json-scada.json"
nssm set AppDirectory JSON_SCADA_cs_data_processor  "C:\json-scada\src\cs_data_processor"
nssm install JSON_SCADA_server_realtime  "c:\Program Files\nodejs\node.exe" "C:\json-scada\src\server_realtime\index.js" 1 1 "c:\json-scada\json-scada.json"
nssm set AppDirectory JSON_SCADA_server_realtime "C:\json-scada\src\server_realtime"

nssm install JSON_SCADA_process_rtdata "C:\json-scada\sql\process_pg_rtdata.bat"
nssm set JSON_SCADA_process_rtdata AppDirectory "C:\json-scada\sql"
nssm install JSON_SCADA_process_hist "C:\json-scada\sql\process_pg_hist.bat"
nssm set JSON_SCADA_process_hist AppDirectory "C:\json-scada\sql"

