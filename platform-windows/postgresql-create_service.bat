echo This script requires administative rights!
echo Please execute it as administrator.

rem Create services, they will work not interactively and independently of a logged user

C:\json-scada\postgresql-runtime\bin\pg_ctl.exe register -N JSON_SCADA_postgresql -D "C:\json-scada\postgresql-data"
