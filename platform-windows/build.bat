echo This script builds JSON-SCADA Windows x64 binaries and restores NodeJS NPM modules.
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

rem copy %SRCPATH%\dnp3\Dnp3Client\Dependencies\OpenSSL\*.dll %BINALTPATH% /y

set DOTNET_CLI_TELEMETRY_OPTOUT=1

rem cd %SRCPATH%\libiec61850
rem rmdir build /S /Q
rem mkdir build
rem cd build	
rem rem Run the line below to create solution file for Visual Studio 2022/2026
rem cmake .. -A x64 -DCMAKE_SUPPRESS_REGENERATION=ON -DBUILD_EXAMPLES=OFF
rem msbuild libiec61850.sln /p:Configuration=Release
rem msbuild libiec61850.slnx /p:Configuration=Release

rem copy %SRCPATH%\libiec61850\build\src\Release\iec61850.dll %BINPATHrem %

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
nuget restore OPC-DA-Server.sln
msbuild OPC-DA-Server.sln /p:Configuration=Release /p:Platform=x64
mkdir %BINPATH%\OPC-DA_Server
copy /Y bin\x64\Release\*.* %BINPATH%\OPC-DA_Server\

rem cd %SRCPATH%\OPC-UA-Client\  
rem rmdir obj /S /Q
rem rmdir bin /S /Q
rem dotnet restore -p:Platform="Any CPU"
rem dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -p:Platform="Any CPU" -o %BINALTPATH% OPC-UA-Client.csproj

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
set GOOS=windows
set GOARCH=amd64

cd %SRCPATH%\calculations
go mod tidy 
go build -ldflags="-s -w"
copy /Y calculations.exe %BINPATH%

cd %SRCPATH%\dnp3-go
go mod tidy
go build -ldflags="-s -w" -o %BINPATH%\dnp3-client.exe .\cmd\dnp3client
go build -ldflags="-s -w" -o %BINPATH%\dnp3-server.exe .\cmd\dnp3server

rem cd %SRCPATH%\i104m
rem go mod tidy
rem go build -ldflags="-s -w"
rem copy /Y i104m.exe %BINALTPATH%

rem cd %SRCPATH%\plc4x-client
rem go mod tidy
rem go build -ldflags="-s -w"
rem copy /Y plc4x-client.exe %BINALTPATH%

cd %SRCPATH%\OPC-UA-Client-Go
go mod tidy
go build -ldflags="-s -w"
copy /Y opcua-client.exe %BINPATH%

rem Go implementation of the DNP3 client and server drivers, drop-in
rem replacements for the C++ Dnp3ClientCpp and Dnp3Server. These need no
rem opendnp3, mongo-cxx-driver or OpenSSL build.
cd %SRCPATH%\dnp3-go
go mod tidy
go build -ldflags="-s -w" -o %BINPATH%\dnp3-client.exe .\cmd\dnp3client
go build -ldflags="-s -w" -o %BINPATH%\dnp3-server.exe .\cmd\dnp3server

cd %SRCPATH%\iec60870-5
go mod tidy
go build -ldflags="-s -w" -o %BINPATH%\iec104client.exe .\cmd\iec104client
go build -ldflags="-s -w" -o %BINPATH%\iec104server.exe .\cmd\iec104server
go build -ldflags="-s -w" -o %BINPATH%\iec101client.exe .\cmd\iec101client
go build -ldflags="-s -w" -o %BINPATH%\iec101server.exe .\cmd\iec101server
go build -ldflags="-s -w" -o %BINPATH%\iec103client.exe .\cmd\iec103client

cd %SRCPATH%\iec61850\iec61850_client
go mod tidy
go build -ldflags="-s -w" -o %BINPATH%\iec61850_client.exe 

cd %SRCPATH%\iec61850\iec61850_server
go mod tidy
go build -ldflags="-s -w" -o %BINPATH%\iec61850_server.exe 

rem Go implementation of cs_data_processor, a drop-in replacement for the
rem Node.js one (run only one of them per instance number)
cd %SRCPATH%\cs_data_processor-go
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

rem cd %SRCPATH%\iccp\iccp-server
rem go mod tidy 
rem go build -ldflags="-s -w"
rem copy /Y iccp-server.exe %BINPATH%

rem set GOOS=linux
rem set GOARCH=arm64
rem go build -ldflags="-s -w" -o iccp-server-linux-arm64

rem set GOOS=linux
rem set GOARCH=amd64
rem go build -ldflags="-s -w" -o iccp-server-linux-amd64

rem cd %SRCPATH%\iccp\iccp-client
rem go mod tidy 
rem go build -ldflags="-s -w"
rem copy /Y iccp-client.exe %BINPATH%

rem set GOOS=linux
rem set GOARCH=arm64
rem go build -ldflags="-s -w" -o iccp-client-linux-arm64

rem set GOOS=linux
rem set GOARCH=amd64
rem go build -ldflags="-s -w" -o iccp-client-linux-amd64

cd %SRCPATH%\cs_data_processor
call %NPM% install

cd %SRCPATH%\cs_custom_processor
call %NPM% install
call %NPM% run build

cd %SRCPATH%\mcp-json-scada-db
call %NPM% install
call %NPM% run build

cd %SRCPATH%\oshmi2json
call %NPM% install

cd %SRCPATH%\oshmi_sync
call %NPM% install

cd %SRCPATH%\alarm_beep
call %NPM% install

cd %SRCPATH%\server_realtime_auth
call %NPM% install

cd %SRCPATH%\updateUser
call %NPM% install

cd %SRCPATH%\shell-api
call %NPM% install

cd %SRCPATH%\AdminUI
call %NPM% install
call %NPM% run build

cd %SRCPATH%\grafana_alert2event
call %NPM% install

cd %SRCPATH%\telegraf-listener
call %NPM% install

cd %SRCPATH%\mqtt-sparkplug
call %NPM% install

cd %SRCPATH%\config_server_for_excel
call %NPM% install

cd %SRCPATH%\OPC-UA-Server
call %NPM% install

cd %SRCPATH%\modbus
call %NPM% install
call %NPM% run build

cd %SRCPATH%\node-red-driver
call %NPM% install
call %NPM% run build

cd %SRCPATH%\n8n-client
call %NPM% install

cd %SRCPATH%\carbone-reports
call %NPM% install

cd %SRCPATH%\demo_simul
call %NPM% install

cd %SRCPATH%\backup-mongo
call %NPM% install

cd %SRCPATH%\mongofw
call %NPM% install

cd %SRCPATH%\mongowr
call %NPM% install

cd %SRCPATH%\camera-onvif
call %NPM% install

cd %SRCPATH%\log-io\ui
call %NPM% install
call %NPM% run build

cd %SRCPATH%\log-io\server
call %NPM% install
call %NPM% run build
call %NPM% prune --omit=dev

cd %SRCPATH%\log-io\inputs\file
call %NPM% install
call %NPM% run build
call %NPM% prune --omit=dev

set NODE_OPTIONS=--max-old-space-size=10000

cd %SRCPATH%\custom-developments\basic_bargraph
call %NPM% install
call %NPX% astro telemetry disable
call %NPM% run build

cd %SRCPATH%\custom-developments\advanced_dashboard
call %NPM% install
call %NPM% run build

cd %SRCPATH%\custom-developments\transformer_with_command
call %NPM% install
call %NPM% run build

cd %SRCPATH%\svgedit
call %NPM% install
call %NPM% run build

cd %JSPATH%\platform-windows
