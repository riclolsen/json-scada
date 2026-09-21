set JSPATH=\json-scada
set BINPATH=%JSPATH%\bin

rmdir bin /S /Q
rem Generic server EXE (C++/CLI), copied to bin\ as OpcNetDaServer.exe by the ServerPlugin post-build
rem msbuild ..\ClassicServerSolutions\src\Technosoftware\Server\ClassicServer\OpcNetDaAeServer.vcxproj /p:Configuration=Release /p:Platform=x64 /p:SolutionDir=%~dp0..\ClassicServerSolutions\
nuget restore OPC-DA-Server.sln
msbuild OPC-DA-Server.sln /p:Configuration=Release /p:Platform=x64
mkdir %BINPATH%\OPC-DA_Server
copy /Y bin\x64\Release\*.* %BINPATH%\OPC-DA_Server\

