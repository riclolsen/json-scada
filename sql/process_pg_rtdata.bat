@ECHO OFF

rem look for process already running, if found get out
wmic process get commandline |find /I "process_pg_rtdata.bat" |find /C /I "cmd" |find /I "2"

if %ERRORLEVEL% EQU 0 GOTO END

rem cd \json-scada\sql

FOR /L %%i IN (0,0,0) DO ( 
FORFILES -m pg_rtdata_*.sql -c "CMD /c ..\platform-windows\postgresql-runtime\bin\psql.exe -h 127.0.0.1 -d json_scada -U json_scada -w  < @FILE && del @FILE & ECHO @FILE" & PING -n 3 127.0.0.1 > nul 
)

:END

