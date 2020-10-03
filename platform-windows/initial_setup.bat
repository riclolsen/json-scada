@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

cd \json-scada\platform-windows\

call postgresql-initdb.bat
copy /Y ..\conf-templates\pg_hba.conf ..\postgresql-data\
copy /Y ..\conf-templates\postgresql.conf ..\postgresql-data\
call postgresql-create_service.bat
call postgresql-start.bat
..\postgresql-runtime\bin\psql -U postgres -h localhost -f ..\sql\create_tables.sql template1



