echo This script builds JSON-SCADA Windows x64 binaries and restores NodeJS NPM modules.
echo Required tools:
echo - Dotnet Core SDK 8.0
echo - Golang 1.22+
echo - Node.js 20+

set JSPATH=\json-scada
set SRCPATH=%JSPATH%\src
set BINPATH=%JSPATH%\bin
set BINWINPATH=%JSPATH%\demo-docker\bin_win
set NPM=%JSPATH%\platform-windows\nodejs-runtime\npm
rem _set NPM="%programfiles%\nodejs\npm"

cd %JSPATH%
mkdir bin

copy %SRCPATH%\dnp3\Dnp3Client\Dependencies\OpenSSL\*.dll %BINPATH% /y

set DOTNET_CLI_TELEMETRY_OPTOUT=1

cd %SRCPATH%\libiec61850\build
rem set VCTargetsPath=C:\ProgramFiles\Microsoft Visual Studio\2022\Community\MSBuild\Microsoft\VC\v170\
rem set VCTargetsPath=D:\ProgramFiles\Microsoft Visual Studio\2022\Community\MSBuild\Microsoft\VC\v170\
rem set VCToolsInstallDir=D:\ProgramFiles\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807\
rem dotnet clean -c Release libiec61850.sln

dotnet publish --no-self-contained --runtime win-x64 -c Release libiec61850.sln
copy %SRCPATH%\libiec61850\build\src\Release\iec61850.dll %BINPATH%

cd %SRCPATH%\libiec61850\dotnet\core\2.0\
dotnet publish --no-self-contained --runtime win-x64 -c Release -o %BINPATH% IEC61850.NET.core.2.0 

cd %SRCPATH%\libiec61850\dotnet\core\2.0\iec61850_client
dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINPATH%

cd %SRCPATH%\lib60870.netcore\lib60870.netcore\
dotnet build --no-self-contained --runtime win-x64 -c Release
dotnet build --no-self-contained --runtime win-x64 -c Release -o %BINPATH%
cd %SRCPATH%\lib60870.netcore\iec101client\
dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINPATH%
cd %SRCPATH%\lib60870.netcore\iec101server\
dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINPATH%
cd %SRCPATH%\lib60870.netcore\iec104client\ 
dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINPATH%
cd %SRCPATH%\lib60870.netcore\iec104server\ 
dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINPATH%
cd %SRCPATH%\dnp3\Dnp3Client\ 
dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINPATH% Dnp3Client.csproj
dotnet publish --self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINWINPATH% Dnp3Client.csproj

cd %SRCPATH%\libplctag\libplctag.NET\src\libplctag
dotnet build --no-self-contained --runtime win-x64 -c Release -o %BINPATH%
cd %SRCPATH%\libplctag\PLCTagsClient
dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINPATH% PLCTagsClient.csproj

cd %SRCPATH%\logrotate\  
dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINPATH% logrotate.csproj

cd %SRCPATH%\opcdaaehda-client-solution-net\
dotnet build -f net8.0-windows DaAeHdaNetStandard.sln

cd %SRCPATH%\OPC-DA-Client\  
rmdir obj /S /Q
rmdir bin /S /Q
dotnet publish --no-self-contained -p:PublishReadyToRun=true -f net8.0-windows -c Release -o %BINPATH% OPC-DA-Client.csproj
dotnet publish --no-self-contained -p:PublishReadyToRun=true -f net8.0-windows -c Release -o %BINWINPATH% OPC-DA-Client.csproj

cd %SRCPATH%\OPC-UA-Client\  
rmdir obj /S /Q
rmdir bin /S /Q
dotnet restore
dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o %BINPATH% OPC-UA-Client.csproj

go env -w GO111MODULE=auto
set GOBIN=c:\json-scada\bin
cd %SRCPATH%\calculations
go mod tidy 
go build 
copy /Y calculations.exe %BINPATH%

cd %SRCPATH%\i104m
go mod tidy
go build 
copy /Y i104m.exe %BINPATH%

cd %SRCPATH%\plc4x-client
go mod tidy 
go build 
copy /Y plc4x-client.exe %BINPATH%

cd %SRCPATH%\cs_data_processor
call %NPM% install

cd %SRCPATH%\cs_custom_processor
call %NPM% install

cd %SRCPATH%\oshmi2json
call %NPM% install

cd %SRCPATH%\oshmi_sync
call %NPM% install

cd %SRCPATH%\alarm_beep
call %NPM% install

cd %SRCPATH%\server_realtime
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

cd %SRCPATH%\carbone-reports
call %NPM% install

cd %SRCPATH%\demo_simul
call %NPM% install

cd %SRCPATH%\backup-mongo
call %NPM% install

cd %SRCPATH\mongofw
call %NPM% install

cd %SRCPATH\mongowr
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

rem deprecated
cd %SRCPATH%\htdocs-admin
set NODE_OPTIONS=--openssl-legacy-provider
call %NPM% install
call %NPM% run build
set NODE_OPTIONS=

cd %JSPATH%\platform-windows
