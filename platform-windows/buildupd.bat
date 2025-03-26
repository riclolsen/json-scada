echo This script builds JSON-SCADA Windows x64 binaries and updates NodeJS NPM modules.
echo Required tools:
echo - Dotnet Core SDK 8.0
echo - Golang 1.22+
echo - Node.js 20+

set JSPATH=\json-scada
set SRCPATH=%JSPATH%\src
set BINPATH=%JSPATH%\bin
set BINWINPATH=%JSPATH%\demo-docker\bin_win
set NPM=%JSPATH%\platform-windows\nodejs-runtime\npm
set NPX=%JSPATH%\platform-windows\nodejs-runtime\npx
rem _set NPM="%programfiles%\nodejs\npm"

cd %JSPATH%
mkdir bin

copy %SRCPATH%\dnp3\Dnp3Client\Dependencies\OpenSSL\*.dll %BINPATH% /y

set DOTNET_CLI_TELEMETRY_OPTOUT=1

cd %SRCPATH%\libiec61850
rem mkdir build
cd build	
rem Run the line below to create solution file for Visual Studio 2022
rem cmake -G "Visual Studio 17 2022" .. -A x64 -DCMAKE_SUPPRESS_REGENERATION=ON
msbuild libiec61850.sln /p:Configuration=Release

rem cd %SRCPATH%\libiec61850\build
rem set VCTargetsPath=C:\ProgramFiles\Microsoft Visual Studio\2022\Community\MSBuild\Microsoft\VC\v170\
rem set VCTargetsPath=D:\ProgramFiles\Microsoft Visual Studio\2022\Community\MSBuild\Microsoft\VC\v170\
rem set VCTargetsPath=C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\MSBuild\Microsoft\VC\v170\
rem set VCToolsInstallDir=D:\ProgramFiles\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807\
rem set VCToolsInstallDir=C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Tools\MSVC\14.41.34120\
rem dotnet clean -c Release libiec61850.sln
rem dotnet publish --no-self-contained --runtime win-x64 -c Release libiec61850.sln

copy %SRCPATH%\libiec61850\build\src\Release\iec61850.dll %BINPATH%

cd %SRCPATH%\libiec61850\dotnet\core\2.0\
dotnet publish --no-self-contained --runtime win-x64 -c Release -o %BINPATH% IEC61850.NET.core.2.0 

cd %SRCPATH%\iec61850_client
dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -p:Platform="Any CPU" -c Release -o %BINPATH%

cd %SRCPATH%\lib60870.netcore\lib60870.netcore\lib60870\
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
dotnet build -f net8.0-windows DaAeHdaNetStandard.sln -p:Platform="Any CPU"

cd %SRCPATH%\OPC-DA-Client\  
rmdir obj /S /Q
rmdir bin /S /Q
dotnet publish --no-self-contained -p:PublishReadyToRun=true -f net8.0-windows -c Release -o %BINPATH% OPC-DA-Client.csproj
dotnet publish --no-self-contained -p:PublishReadyToRun=true -f net8.0-windows -c Release -o %BINWINPATH% OPC-DA-Client.csproj

cd %SRCPATH%\OPC-UA-Client\  
rmdir obj /S /Q
rmdir bin /S /Q
dotnet restore -p:Platform="Any CPU"
dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -p:Platform="Any CPU" -o %BINPATH% OPC-UA-Client.csproj

rem cd %SRCPATH%\dnp3\opendnp3
rem mkdir build
rem cd build
rem cmake -DDNP3_EXAMPLES=OFF -DDNP3_TLS=ON -DOPENSSL_ROOT_DIR="C:\Program Files\OpenSSL-Win64" -DOPENSSL_USE_STATIC_LIBS=TRUE -DOPENSSL_MSVC_STATIC_RT=TRUE ..
rem msbuild opendnp3.sln /p:Configuration=Release

rem cd %SRCPATH%\dnp3\Dnp3Server
rem mkdir build
rem cd build
rem cmake -DOPENSSL_ROOT_DIR="C:\Program Files\OpenSSL-Win64" -DOPENSSL_USE_STATIC_LIBS=TRUE -DOPENSSL_MSVC_STATIC_RT=TRUE ..
rem msbuild Dnp3Server.sln /p:Configuration=Release
rem copy /Y %SRCPATH%\dnp3\Dnp3Server\build\Release\Dnp3Server.exe %BINPATH%

go env -w GO111MODULE=auto
set GOBIN=c:\json-scada\bin
cd %SRCPATH%\calculations
go get -u ./...
go mod tidy 
go build -ldflags="-s -w"
copy /Y calculations.exe %BINPATH%

cd %SRCPATH%\i104m
go get -u ./...
go mod tidy 
go build -ldflags="-s -w"
copy /Y i104m.exe %BINPATH%

cd %SRCPATH%\plc4x-client
go get -u ./...
go mod tidy 
go build -ldflags="-s -w"
copy /Y plc4x-client.exe %BINPATH%

cd %SRCPATH%\cs_data_processor
call %NPM% i --package-lock-only
call %NPM% update
cd %SRCPATH%\cs_custom_processor
call %NPM% i --package-lock-only
call %NPM% update
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

cd %JSPATH%\platform-windows
