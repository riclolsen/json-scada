@echo off
:: ============================================================
:: unregister.bat
:: Unregisters the OPC-DA Server from Windows DCOM/COM.
:: Must be run as Administrator.
:: Usage: unregister.bat [x86|x64] [Debug|Release]
:: ============================================================

SET ARCH=%1
SET CONFIG=%2
IF "%ARCH%"==""   SET ARCH=x64
IF "%CONFIG%"=="" SET CONFIG=Release

SET BINDIR=%~dp0bin\%ARCH%\%CONFIG%

IF NOT EXIST "%BINDIR%\OpcNetDaServer.exe" (
    echo ERROR: OpcNetDaServer.exe not found in %BINDIR%
    exit /b 1
)

echo Unregistering JSON-SCADA OPC-DA Server (%ARCH% %CONFIG%)...
"%BINDIR%\OpcNetDaServer.exe" -unregserver
echo Done.
