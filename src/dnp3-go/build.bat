
set JSPATH=\json-scada
set SRCPATH=%JSPATH%\src
set BINPATH=%JSPATH%\bin

go mod tidy
go build -ldflags="-s -w" -o %BINPATH%\dnp3-client.exe .\cmd\dnp3client
go build -ldflags="-s -w" -o %BINPATH%\dnp3-server.exe .\cmd\dnp3server
