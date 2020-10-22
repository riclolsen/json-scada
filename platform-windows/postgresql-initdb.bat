echo THIS WILL INITIALIZE THE POSTGRESQL DATABASE! 

if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

REM In some cases the data folder should be given rights to group "Everyone"=S-1-1-0
icacls "C:\json-scada\platform-windows\postgresql-data" /t  /grant *S-1-1-0:F

c:\json-scada\platform-windows\postgresql-runtime\bin\initdb.exe -D c:\json-scada\platform-windows\postgresql-data --username=postgres --auth=trust -E UTF8 --locale=en-US
