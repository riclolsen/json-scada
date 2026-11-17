@echo off
call mvn package
if errorlevel 1 exit /b %errorlevel%
copy /Y target\plc4j-client.jar \json-scada\bin\plc4j-client.jar
