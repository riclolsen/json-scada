
set JSPATH=\json-scada
set SRCPATH=%JSPATH%\src
set BINPATH=%JSPATH%\bin

cd .\iec61850_client
go mod tidy
cd ..\iec61850_server
go mod tidy
cd ..
go build -ldflags="-s -w" -o %BINPATH%\iec61850_client.exe .\iec61850_client
go build -ldflags="-s -w" -o %BINPATH%\iec61850_server.exe .\iec61850_client

