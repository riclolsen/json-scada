set JSPATH=\json-scada
set SRCPATH=%JSPATH%\src
set BINPATH=%JSPATH%\bin

go mod tidy
go build -ldflags="-s -w" -o %BINPATH%\modbus-client.exe .\cmd\modbusclient
go build -ldflags="-s -w" -o %BINPATH%\modbus-server.exe .\cmd\modbusserver
