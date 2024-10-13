@ECHO OFF
set JSPATH=c:\json-scada
set TMPPATH=%JSPATH%\tmp
set MONGO_CONNECT_STRING=mongodb://192.168.239.133/?tls=false&directConnection=true
set MONGOBIN=%JSPATH%\platform-windows\mongodb-runtime\bin
set JAVAPATH=%JSPATH%\platform-windows\jdk-runtime\bin
set TARPATH=%JSPATH%\platform-windows
set SVGPATH=%JSPATH%\svg
set mongoConnectionString=mongodb://127.0.0.1/json_scada?tls=false&directConnection=true
set database=json_scada
set tlsCaPemFile=
set tlsClientPemFile=
set tlsClientKeyFile=
set tlsClientPfxFile=
set tlsClientKeyPassword=
set tlsAllowInvalidHostnames=true
set tlsAllowChainErrors=true
set tlsInsecure=false

cd /d %JSPATH%\platform-windows\

if [%1]==[] ( SET "OUTPUTFILE=jsproject.zip" ) ELSE ( SET "OUTPUTFILE=%1" )

for /f "tokens=* delims=" %%a in ('jsonextractor.bat ..\conf\json-scada.json mongoConnectionString') do set "mongoConnectionString=%%~a"
for /f "tokens=* delims=" %%a in ('jsonextractor.bat ..\conf\json-scada.json database') do set "database=%%~a"

rem for /f "tokens=* delims=" %%a in ('jsonextractor.bat ..\conf\json-scada.json tlsCaPemFile') do set "tlsCaPemFile=%%~a"
rem for /f "tokens=* delims=" %%a in ('jsonextractor.bat ..\conf\json-scada.json tlsClientPemFile') do set "tlsClientPemFile=%%~a"
rem for /f "tokens=* delims=" %%a in ('jsonextractor.bat ..\conf\json-scada.json tlsClientKeyFile') do set "tlsClientKeyFile=%%~a"
rem for /f "tokens=* delims=" %%a in ('jsonextractor.bat ..\conf\json-scada.json tlsClientPfxFile') do set "tlsClientPfxFile=%%~a"
rem for /f "tokens=* delims=" %%a in ('jsonextractor.bat ..\conf\json-scada.json tlsClientKeyPassword') do set "tlsClientKeyPassword=%%~a"
rem for /f "tokens=* delims=" %%a in ('jsonextractor.bat ..\conf\json-scada.json tlsAllowInvalidHostnames') do set "tlsAllowInvalidHostnames=%%~a"
rem for /f "tokens=* delims=" %%a in ('jsonextractor.bat ..\conf\json-scada.json tlsAllowChainErrors') do set "tlsAllowChainErrors=%%~a"
rem for /f "tokens=* delims=" %%a in ('jsonextractor.bat ..\conf\json-scada.json tlsInsecure') do set "tlsInsecure=%%~a"
rem set TLSFLAGS=--sslCAFile=%tlsCaPemFile% --sslPEMKeyFile=%tlsClientPemFile% --sslPEMKeyPassword=%tlsClientKeyPassword% --sslAllowInvalidHostnames=%tlsAllowInvalidHostnames% --sslAllowInvalidCertificates=%tlsAllowChainErrors%
rem echo "%TLSFLAGS%"

if not exist "%TMPPATH%" mkdir "%TMPPATH%"
del %TMPPATH%\*.* /Q
%MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %database% --collection realtimeData --out %TMPPATH%\realtimeData.json
%MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %database% --collection processInstances --out %TMPPATH%\processInstances.json
%MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %database% --collection protocolDriverInstances --out %TMPPATH%\protocolDriverInstances.json
%MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %database% --collection protocolConnections --out %TMPPATH%\protocolConnections.json
%MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %database% --collection users --out %TMPPATH%\users.json
%MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %database% --collection roles --out %TMPPATH%\roles.json
rem optional historical data
rem %MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %database% --collection hist --out %TMPPATH%\hist.json
rem %MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %database% --collection backfillData --out %TMPPATH%\backfillData.json
rem %MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %database% --collection soeData --out %TMPPATH%\soeData.json
rem %MONGOBIN%\mongoexport.exe --uri "%mongoConnectionString%" --db %database% --collection userActions --out %TMPPATH%\userActions.json

copy %SVGPATH%\*.svg %TMPPATH%\
copy %SVGPATH%\screen_list.js %TMPPATH%\
rem optional
rem copy %JSPATH%\conf\*.* %TMPPATH%\

rem %TARPATH%\tar -a -c -f  %TMPPATH%\jsproject.zip -C %TMPPATH% *.json 
cd /d %TMPPATH%
%JAVAPATH%\jar -cfM %OUTPUTFILE% *.json 
%JAVAPATH%\jar -ufM %OUTPUTFILE% *.js
%JAVAPATH%\jar -ufM %OUTPUTFILE% *.svg 
cd /d %JSPATH%\platform-windows\