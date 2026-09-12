@echo off
:: ============================================================
:: register.bat
:: Registers the OPC-DA Server with Windows DCOM/COM.
:: Must be run as Administrator.
:: Usage: register.bat [x86|x64] [Debug|Release]
:: Default: x86 Release
:: ============================================================

SET ARCH=%1
SET CONFIG=%2
IF "%ARCH%"==""   SET ARCH=x64
IF "%CONFIG%"=="" SET CONFIG=Release

SET BINDIR=%~dp0bin\%ARCH%\%CONFIG%

IF NOT EXIST "%BINDIR%\OpcNetDaServer.exe" (
    echo ERROR: OpcNetDaServer.exe not found in %BINDIR%
    echo Please copy OpcNetDaServer.exe from the Technosoftware distribution into that folder.
    exit /b 1
)

IF NOT EXIST "%BINDIR%\ServerPlugin.dll" (
    echo ERROR: ServerPlugin.dll not found in %BINDIR%
    echo Please build the solution first.
    exit /b 1
)

echo Registering JSON-SCADA OPC-DA Server (%ARCH% %CONFIG%)...
"%BINDIR%\OpcNetDaServer.exe" -regserver
IF ERRORLEVEL 1 (
    echo ERROR: Registration failed. Run as Administrator?
    exit /b 1
)
echo Done. Server registered successfully.
echo.
echo To test, run:  "%BINDIR%\OpcNetDaServer.exe" 1 2 ..\..\conf\json-scada.json
