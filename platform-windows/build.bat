echo This script builds JSON-SCADA Windows x64 binaries and restores NodeJS NPM modules.
echo Required tools:
echo - Dotnet Core SDK 5.0
echo - Golang 1.14+
echo - Node.js 14+

cd c:\json-scada
mkdir bin

set DOTNET_CLI_TELEMETRY_OPTOUT=1

cd \json-scada\src\lib60870.netcore\lib60870.netcore\
dotnet build --runtime win-x64 -c Release
dotnet build --runtime win-x64 -c Release -o ..\..\..\bin\ 
cd \json-scada\src\lib60870.netcore\iec101client\
dotnet publish --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ..\..\..\bin\ 
cd \json-scada\src\lib60870.netcore\iec101server\
dotnet publish --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ..\..\..\bin\
cd \json-scada\src\lib60870.netcore\iec104client\ 
dotnet publish --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ..\..\..\bin\
cd \json-scada\src\lib60870.netcore\iec104server\ 
dotnet publish --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ..\..\..\bin\
cd \json-scada\src\dnp3\Dnp3Client\ 
dotnet publish --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ..\..\..\bin\ Dnp3Client.csproj
cd \json-scada\src\OPC-UA-Client\ 
dotnet publish --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ..\..\bin\
cd \json-scada\src\libplctag\libplctag.NET\src\libplctag
dotnet build --runtime win-x64 -c Release -o ..\..\bin\
cd \json-scada\src\libplctag\PLCTagsClient
dotnet publish --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ..\..\..\bin\

go env -w GO111MODULE=auto
set GOBIN=c:\json-scada\bin
cd \json-scada\src\calculations
go get ./... 
go build 
copy /Y calculations ..\..\bin\

cd \json-scada\src\i104m
go get ./... 
go build 
copy /Y i104m ..\..\bin\

cd \json-scada\src\cs_data_processor
call \json-scada\platform-windows\nodejs-runtime\npm install
cd \json-scada\src\cs_custom_processor
call \json-scada\platform-windows\nodejs-runtime\npm install
cd \json-scada\src\oshmi2json
call \json-scada\platform-windows\nodejs-runtime\npm install
cd \json-scada\src\oshmi_sync
call \json-scada\platform-windows\nodejs-runtime\npm install
cd \json-scada\src\alarm_beep
call \json-scada\platform-windows\nodejs-runtime\npm install
cd \json-scada\src\server_realtime
call \json-scada\platform-windows\nodejs-runtime\npm install
cd \json-scada\src\server_realtime_auth
call \json-scada\platform-windows\nodejs-runtime\npm install
cd \json-scada\src\shell-api
call \json-scada\platform-windows\nodejs-runtime\npm install
cd \json-scada\src\htdocs-admin
call \json-scada\platform-windows\nodejs-runtime\npm install
call \json-scada\platform-windows\nodejs-runtime\npm run build
cd \json-scada\src\grafana_alert2event
call \json-scada\platform-windows\nodejs-runtime\npm install
cd \json-scada\src\telegraf-listener
call \json-scada\platform-windows\nodejs-runtime\npm install
cd \json-scada\src\mqtt-sparkplug
call \json-scada\platform-windows\nodejs-runtime\npm install
cd \json-scada\src\config_server_for_excel
call \json-scada\platform-windows\nodejs-runtime\npm install
cd \json-scada\src\OPC-UA-Server
call \json-scada\platform-windows\nodejs-runtime\npm install

cd ..\..\platform-windows

