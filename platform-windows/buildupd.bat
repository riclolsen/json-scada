echo This script builds JSON-SCADA Windows x64 binaries and updates NodeJS NPM modules.
echo Required tools:
echo - Dotnet Core SDK 8.0
echo - Golang 1.22+
echo - Node.js 20+

set JSPATH=\json-scada
set SRCPATH=%JSPATH%\src
set BINPATH=%JSPATH%\bin
set BINALTPATH=%JSPATH%\bin_alt
set NPM=%JSPATH%\platform-windows\nodejs-runtime\npm.cmd
set NPX=%JSPATH%\platform-windows\nodejs-runtime\npx.cmd
if not exist %NPM% set NPM=npm
if not exist %NPX% set NPX=npx

cd %JSPATH%
mkdir bin
mkdir bin_alt

rem copy %SRCPATH%\dnp3\Dnp3Client\Dependencies\OpenSSL\*.dll %BINPATH% /y

set DOTNET_CLI_TELEMETRY_OPTOUT=1

rem cd %SRCPATH%\libiec61850
rem rmdir build /S /Q
rem mkdir build
rem cd build
rem rem Run the line below to create solution file for Visual Studio 2022/2026
rem cmake .. -A x64 -DCMAKE_SUPPRESS_REGENERATION=ON -DBUILD_EXAMPLES=OFF
rem msbuild libiec61850.sln /p:Configuration=Release
rem msbuild libiec61850.slnx /p:Configuration=Release

rem copy %SRCPATH%\libiec61850\build\src\Release\iec61850.dll %BINALTPATH%

rem cd %SRCPATH%\libiec61850\dotnet\core\2.0\
rem dotnet publish --no-self-contained --runtime win-x64 -c Release -o %BINALTPATH% IEC61850.NET.core.2.0 

rem cd %SRCPATH%\iec61850_client
rem dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -p:Platform="Any CPU" -c Release -o %BINALTPATH%

rem cd %SRCPATH%\iec61850_server
rem dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -p:Platform="Any CPU" -c Release -o %BINALTPATH%

rem IEC 60870-5-101/104 drivers are now built in Go (src\iec60870-5), see the Go section below.
rem cd %SRCPATH%\lib60870.netcore\lib60870.netcore\lib60870\
rem dotnet build --no-self-contained --runtime win-x64 -c Release
rem dotnet build --no-self-contained --runtime win-x64 -c Release -o %BINALTPATH%
rem cd %SRCPATH%\lib60870.netcore\iec101client\
rem dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINALTPATH%
rem cd %SRCPATH%\lib60870.netcore\iec101server\
rem dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINALTPATH%
rem cd %SRCPATH%\lib60870.netcore\iec104client\ 
rem dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINALTPATH%
rem cd %SRCPATH%\lib60870.netcore\iec104server\ 
rem dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINALTPATH%

rem cd %SRCPATH%\dnp3\Dnp3Client\
rem dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINALTPATH% Dnp3Client.csproj

rem cd %SRCPATH%\libplctag\libplctag.NET\src\libplctag
rem dotnet build --no-self-contained --runtime win-x64 -c Release -o %BINPATH%
rem cd %SRCPATH%\libplctag\PLCTagsClient
rem dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINALTPATH% PLCTagsClient.csproj

rem cd %SRCPATH%\logrotate\  
rem dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINPATH% logrotate.csproj

cd %SRCPATH%\opcdaaehda-client-solution-net\
dotnet build -f net8.0-windows DaAeHdaNetStandard.sln -p:Platform="Any CPU"

cd %SRCPATH%\OPC-DA-Client\  
rmdir obj /S /Q
rmdir bin /S /Q
dotnet publish --no-self-contained -p:PublishReadyToRun=true -f net8.0-windows -c Release -o %BINPATH% OPC-DA-Client.csproj

cd %SRCPATH%\OPC-DA-Server\
rmdir bin /S /Q
msbuild OPC-DA-Server.sln /p:Configuration=Release /p:Platform=x64
mkdir %BINPATH%\OPC-DA_Server
copy /Y bin\x64\Release\*.* %BINPATH%\OPC-DA_Server\

rem cd %SRCPATH%\OPC-UA-Client\  
rem rmdir obj /S /Q
rem rmdir bin /S /Q
rem dotnet restore -p:Platform="Any CPU"
rem dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -p:Platform="Any CPU" -o %BINPATH% OPC-UA-Client.csproj

rem C++ DNP3 client driver. Its native dependencies - OpenSSL, opendnp3 and
rem mongo-cxx-driver - are a one-time build: run src\dnp3\build-windows-deps.bat
rem once. This section is skipped while those dependencies are absent, so
rem build.bat keeps working on machines that have not set them up.
rem Pass "server" to build-windows.bat below to build Dnp3Server as well.
rem cd %SRCPATH%\dnp3
rem if exist opendnp3\build\cpp\lib\Release\opendnp3.lib (
rem   call build-windows.bat
rem ) else (
rem   echo run src\dnp3\build-windows-deps.bat first.
rem    call build-windows-deps.bat
rem    call build-windows.bat
rem )

go env -w GO111MODULE=auto
set GOBIN=c:\json-scada\bin
cd %SRCPATH%\calculations
go get -u ./...
go mod tidy 
go build -ldflags="-s -w"
copy /Y calculations.exe %BINPATH%

cd %SRCPATH%\dnp3-go
go get -u ./...
go mod tidy
go build -ldflags="-s -w" -o %BINPATH%\dnp3-client.exe .\cmd\dnp3client
go build -ldflags="-s -w" -o %BINPATH%\dnp3-server.exe .\cmd\dnp3server

rem cd %SRCPATH%\i104m
rem go get -u ./...
rem go mod tidy 
rem go build -ldflags="-s -w"
rem copy /Y i104m.exe %BINALTPATH%

rem cd %SRCPATH%\plc4x-client
rem go get -u ./...
rem go mod tidy
rem go build -ldflags="-s -w"
rem copy /Y plc4x-client.exe %BINPATH%

cd %SRCPATH%\iec60870-5
go get -u ./...
go mod tidy
go build -ldflags="-s -w" -o %BINPATH%\iec104client.exe .\cmd\iec104client
go build -ldflags="-s -w" -o %BINPATH%\iec104server.exe .\cmd\iec104server
go build -ldflags="-s -w" -o %BINPATH%\iec101client.exe .\cmd\iec101client
go build -ldflags="-s -w" -o %BINPATH%\iec101server.exe .\cmd\iec101server
go build -ldflags="-s -w" -o %BINPATH%\iec103client.exe .\cmd\iec103client

cd %SRCPATH%\iec61850\iec61850_client
go get -u ./...
go mod tidy
go build -ldflags="-s -w" -o %BINPATH%\iec61850_client.exe 

cd %SRCPATH%\iec61850\iec61850_server
go get -u ./...
go mod tidy
go build -ldflags="-s -w" -o %BINPATH%\iec61850_server.exe 

rem Go implementation of cs_data_processor, a drop-in replacement for the
rem Node.js one (run only one of them per instance number)
cd %SRCPATH%\cs_data_processor-go
go get -u ./...
go mod tidy
go build -ldflags="-s -w" -o %BINPATH%\cs_data_processor-go.exe

rem PLC4J client (Java alternative for the PLC4X driver) - built only when JDK 17+ and Maven are available
where mvn >nul 2>nul
if %ERRORLEVEL% neq 0 goto skip_plc4j
cd %SRCPATH%\plc4j-client
call mvn -B -ntp -DskipTests package
copy /Y target\plc4j-client.jar %BINPATH%
copy /Y plc4j-client.bat %BINPATH%
:skip_plc4j

rem ICCP client/server
copy /Y  %SRCPATH%\iccp\iccp-server\iccp-server.exe %BINPATH%
copy /Y  %SRCPATH%\iccp\iccp-client\iccp-client.exe %BINPATH%

cd %SRCPATH%\cs_data_processor
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\cs_custom_processor
call %NPM% i --package-lock-only
call %NPM% update
call %NPM% run build
cd %SRCPATH%\mcp-json-scada-db
call %NPM% i --package-lock-only
call %NPM% update
call %NPM% run build
cd %SRCPATH%\oshmi2json
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\oshmi_sync
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\alarm_beep
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\server_realtime_auth
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\updateUser
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\shell-api
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\AdminUI
call %NPM% i --package-lock-only
call %NPM% update
call %NPM% run build
cd %SRCPATH%\grafana_alert2event
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\telegraf-listener
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\mqtt-sparkplug
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\config_server_for_excel
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\OPC-UA-Server
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\modbus
call %NPM% i --package-lock-only
call %NPM% update
call %NPM% run build
cd %SRCPATH%\node-red-driver
call %NPM% i --package-lock-only
call %NPM% update
call %NPM% run build
cd %SRCPATH%\carbone-reports
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\demo_simul
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\backup-mongo
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\mongofw
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\mongowr
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\camera-onvif
call %NPM% i --package-lock-only
call %NPM% update

cd %SRCPATH%\log-io\ui
call %NPM% i --package-lock-only
rem call %NPM% update
call %NPM% install
call %NPM% run build

cd %SRCPATH%\log-io\server
call %NPM% i --package-lock-only
rem call %NPM% update
call %NPM% install
call %NPM% run build
call %NPM% prune --omit=dev

cd %SRCPATH%\log-io\inputs\file
call %NPM% i --package-lock-only
rem call %NPM% update
call %NPM% install
call %NPM% run build
call %NPM% prune --omit=dev

set NODE_OPTIONS=--max-old-space-size=8000
set ASTRO_TELEMETRY_DISABLED=1

cd %SRCPATH%\custom-developments\basic_bargraph
call %NPM% i --package-lock-only
call %NPX% astro telemetry disable
call %NPM% update
call %NPM% run build

cd %SRCPATH%\custom-developments\advanced_dashboard
call %NPM% i --package-lock-only
call %NPM% update
call %NPM% run build

cd %SRCPATH%\custom-developments\transformer_with_command
call %NPM% i --package-lock-only
call %NPM% update
call %NPM% run build

cd %SRCPATH%\svgedit
call %NPM% install
call %NPM% run build

cd %JSPATH%\platform-windows
