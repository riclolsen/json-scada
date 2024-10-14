@ECHO OFF
set JSPATH=c:\json-scada
set TMPPATH=%JSPATH%\tmp
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

if [%1]==[] ( SET "INPUTFILE=" ) ELSE ( SET "INPUTFILE=%1" )

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
cd /d %TMPPATH%

if not [%INPUTFILE%]==[] (
    
    del %TMPPATH%\*.json /Q
    del %TMPPATH%\*.js /Q
    del %TMPPATH%\*.svg /Q
    del %TMPPATH%\*.conf /Q
    del %TMPPATH%\*.xml /Q
    del %TMPPATH%\*.ini /Q

     %JAVAPATH%\jar -xf %INPUTFILE% 
) 

set FLAGS=--mode=upsert

%MONGOBIN%\mongosh --quiet --eval "db.realtimeData.deleteMany({})" "%mongoConnectionString%"
%MONGOBIN%\mongosh --quiet --eval "db.processInstances.deleteMany({})" "%mongoConnectionString%"
%MONGOBIN%\mongosh --quiet --eval "db.protocolConnections.deleteMany({})" "%mongoConnectionString%"
%MONGOBIN%\mongosh --quiet --eval "db.protocolDriverInstances.deleteMany({})" "%mongoConnectionString%"
%MONGOBIN%\mongosh --quiet --eval "db.users.deleteMany({})" "%mongoConnectionString%"
%MONGOBIN%\mongosh --quiet --eval "db.roles.deleteMany({})" "%mongoConnectionString%"

%MONGOBIN%\mongoimport.exe --uri "%mongoConnectionString%" --db %database% --collection roles  %FLAGS% --file roles.json
%MONGOBIN%\mongoimport.exe --uri "%mongoConnectionString%" --db %database% --collection users %FLAGS% --file users.json 
%MONGOBIN%\mongoimport.exe --uri "%mongoConnectionString%" --db %database% --collection processInstances %FLAGS% --file processInstances.json
%MONGOBIN%\mongoimport.exe --uri "%mongoConnectionString%" --db %database% --collection protocolDriverInstances %FLAGS% --file protocolDriverInstances.json
%MONGOBIN%\mongoimport.exe --uri "%mongoConnectionString%" --db %database% --collection protocolConnections %FLAGS% --file protocolConnections.json
%MONGOBIN%\mongoimport.exe --uri "%mongoConnectionString%" --db %database% --collection realtimeData %FLAGS% --file realtimeData.json
rem optional historical data
rem %MONGOBIN%\mongoimport.exe --uri "%mongoConnectionString%" --db %database% --collection hist --file hist.json %FLAGS%
rem %MONGOBIN%\mongoimport.exe --uri "%mongoConnectionString%" --db %database% --collection backfillData --file backfillData.json %FLAGS%
rem %MONGOBIN%\mongoimport.exe --uri "%mongoConnectionString%" --db %database% --collection soeData --file soeData.json %FLAGS%
rem %MONGOBIN%\mongoimport.exe --uri "%mongoConnectionString%" --db %database% --collection userActions --file userActions.json %FLAGS%

copy %TMPPATH%\*.svg %SVGPATH%\*.svg /Y
copy %TMPPATH%\screen_list.js %SVGPATH%\ /Y
rem optional
rem copy %TMPPATH%\ %JSPATH%\conf\*.* 

cd /d %JSPATH%\platform-windows\