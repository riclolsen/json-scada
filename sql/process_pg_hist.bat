@ECHO OFF

rem look for process already running, if found get out
rem Get-Process -Name cmd | Where-Object {$_.CommandLine -like '*process_pg_hist.bat*'} | Measure-Object | Select-Object -ExpandProperty Count
rem if %ERRORLEVEL% EQU 0 GOTO END

rem cd \json-scada\sql

FOR /L %%i IN (0,0,0) DO ( 
FORFILES -m pg_hist_*.sql   -c "CMD /c ..\platform-windows\postgresql-runtime\bin\psql.exe -h 127.0.0.1 -d json_scada -U json_scada -w  < @FILE && del @FILE & ECHO @FILE" & PING -n 2 127.0.0.1 > nul 
)

:END


