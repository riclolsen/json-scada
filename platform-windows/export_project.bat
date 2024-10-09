set JSPATH=c:\json-scada
set TMPPATH=%JSPATH%\tmp
set DATABASE=json_scada
set MONGO_CONNECT_STRING=mongodb://192.168.239.133/?tls=false&directConnection=true
set MONGOBIN=%JSPATH%\platform-windows\mongodb-runtime\bin
set JAVAPATH=%JSPATH%\platform-windows\jdk-runtime\bin
set TARPATH=%JSPATH%\platform-windows
set SVGPATH=%JSPATH%\src\AdminUI\dist\svg

IF [%1]==[] ( SET "OUTPUTFILE=jsproject.zip" ) ELSE ( SET "OUTPUTFILE=%1" )

for /f "tokens=* delims=" %%a in ('jsonextractor.bat ..\conf\json-scada.json mongoConnectionString') do set "mongoConnectionString=%%~a"

mkdir %TMPPATH%
del %TMPPATH%\*.* /Q
%MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %DATABASE% --collection realtimeData --out %TMPPATH%\realtimeData.json
%MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %DATABASE% --collection processInstances --out %TMPPATH%\processInstances.json
%MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %DATABASE% --collection protocolDriverInstances --out %TMPPATH%\mongodb-runtime\bin\mongoexport.exe --uri %MONGO_CONNECT_STRING% --db %DATABASE% --collection protocolDriverInstances --out %JSPATH%\tmp\realtimeData.json
%MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %DATABASE% --collection protocolConnections --out %TMPPATH%\protocolConnections.json
%MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %DATABASE% --collection users --out %TMPPATH%\users.json
%MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %DATABASE% --collection roles --out %TMPPATH%\roles.json
rem optional historical data
rem %MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %DATABASE% --collection hist --out %TMPPATH%\hist.json
rem %MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %DATABASE% --collection backfillData --out %TMPPATH%\backfillData.json
rem %MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %DATABASE% --collection soeData --out %TMPPATH%\soeData.json
rem %MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %DATABASE% --collection userActions --out %TMPPATH%\userActions.json

copy %SVGPATH%\*.svg %TMPPATH%\
copy %SVGPATH%\screen_list.js %TMPPATH%\
rem optional
rem copy %JSPATH%\conf\*.* %TMPPATH%\

rem %TARPATH%\tar -a -c -f  %TMPPATH%\jsproject.zip -C %TMPPATH% *.json 
cd %TMPPATH%
%JAVAPATH%\jar -cfM %OUTPUTFILE% *.json 
%JAVAPATH%\jar -ufM %OUTPUTFILE% *.js
%JAVAPATH%\jar -ufM %OUTPUTFILE% *.svg 
cd %JSPATH%\platform-windows\