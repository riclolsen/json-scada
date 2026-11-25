@echo off
setlocal EnableExtensions DisableDelayedExpansion
set "NODE_DIR=%~dp0nodejs-runtime"
set "RUNTIME=%~dp0n8n-runtime"
set "CLI=%RUNTIME%\node_modules\n8n\bin\n8n"
set "PATH=%NODE_DIR%;%PATH%"

if not exist "%NODE_DIR%\node.exe" goto missing_node
if /i "%~1"=="start" goto start
if not "%~1"=="" (
  echo Usage: install_n8n.bat [start]
  exit /b 1
)
if not exist "%NODE_DIR%\npm.cmd" goto missing_node
if not exist "%RUNTIME%\" mkdir "%RUNTIME%"
if not exist "%RUNTIME%\" exit /b 1

call "%NODE_DIR%\npm.cmd" install --prefix "%RUNTIME%" n8n@2
if errorlevel 1 exit /b 1
"%NODE_DIR%\node.exe" "%CLI%" --version
if errorlevel 1 exit /b 1
echo Run "%~f0" start to launch the n8n editor.
exit /b 0

:start
if not exist "%CLI%" (
  echo Install n8n first by running "%~f0" without arguments.
  exit /b 1
)
set "N8N_LISTEN_ADDRESS=127.0.0.1"
set "N8N_PORT=5678"
set "N8N_PATH=/n8n/"
if not defined N8N_EDITOR_BASE_URL set "N8N_EDITOR_BASE_URL=http://localhost/n8n/"
echo Open /n8n/ in the JSON-SCADA web UI.
"%NODE_DIR%\node.exe" "%CLI%" start
exit /b %errorlevel%

:missing_node
echo The bundled Node.js runtime is required in "%NODE_DIR%".
exit /b 1
