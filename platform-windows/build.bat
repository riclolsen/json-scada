echo This script builds JSON-SCADA Windows x64 binaries and restores NodeJS NPM modules.
echo Required tools:
echo - Dotnet Core SDK 3.1
echo - Golang 1.14+
echo - Node.js 14+

cd c:\json-scada
mkdir bin

set DOTNET_CLI_TELEMETRY_OPTOUT=1

cd \json-scada\src\lib60870.netcore\lib60870.netcore\
dotnet build --runtime win-x64 -c Release -o ..\..\bin\ 
cd \json-scada\src\lib60870.netcore\iec101client\
dotnet publish --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ..\..\bin\ 
cd \json-scada\src\lib60870.netcore\iec101server\
dotnet publish --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ..\..\bin\
cd \json-scada\src\lib60870.netcore\iec104client\ 
dotnet publish --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ..\..\bin\
cd \json-scada\src\lib60870.netcore\iec104server\ 
dotnet publish --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ..\..\bin\
cd \json-scada\src\dnp3\Dnp3Client\ 
dotnet publish --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ..\..\bin\
cd \json-scada\src\libplctag\libplctag.NET\src\libplctag
dotnet build --runtime win-x64 -c Release -o ..\..\bin\
cd \json-scada\src\libplctag\PLCTagsClient
dotnet publish --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ..\..\..\bin\

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
npm update
cd \json-scada\src\server_realtime
npm update
cd \json-scada\src\oshmi2json
npm update
cd \json-scada\src\alarm_beep
npm update