@echo off
:: ============================================================
:: run.bat
:: Starts the OPC-DA Server driver (foreground / debug mode).
:: Usage: run.bat [instance] [logLevel] [x86|x64] [Debug|Release]
::   instance  : protocolDriverInstanceNumber (default 1)
::   logLevel  : 0=none 1=normal 2=detail 3=debug (default 1)
::   arch      : x86 or x64 (default x86)
::   config    : Debug or Release (default Release)
:: ============================================================

SET INSTANCE=%1
SET LOGLEVEL=%2
SET ARCH=%3
SET CONFIG=%4

IF "%INSTANCE%"==""  SET INSTANCE=1
IF "%LOGLEVEL%"==""  SET LOGLEVEL=1
IF "%ARCH%"==""      SET ARCH=x64
IF "%CONFIG%"==""    SET CONFIG=Release

SET BINDIR=%~dp0bin\%ARCH%\%CONFIG%
SET CONFFILE=%~dp0..\..\conf\json-scada.json

IF NOT EXIST "%BINDIR%\OpcNetDaServer.exe" (
    echo ERROR: OpcNetDaServer.exe not found in %BINDIR%
    exit /b 1
)
    
echo Starting JSON-SCADA OPC-DA Server instance=%INSTANCE% logLevel=%LOGLEVEL%
"%BINDIR%\OpcNetDaServer.exe" %INSTANCE% %LOGLEVEL% "%CONFFILE%"
