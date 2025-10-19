@ECHO OFF

rem obtain pid from process, then kill it

set _pid=0

FOR /F "tokens=2 delims= " %%G IN ('tasklist /V /FI "IMAGENAME eq cmd.exe" /NH /FO TABLE ^| findstr /I "process_pg_rtdata.bat" ^| findstr /V /I "terminate_pg_rtdata.bat"') DO (
IF %_pid% EQU 0 ( 
  SET _pid=%%G 
  GOTO BRK 
  )
)

:BRK

IF %_pid% EQU 0 GOTO END

TASKKILL /F /PID %_pid%

:END
