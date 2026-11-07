@echo off
rem CIM-EMS Python environment (Windows x64).
rem With the bundled runtime ..\..\platform-windows\python-runtime (NuGet "python" 3.14 package, tools folder):
rem   installs the package and its pinned dependencies (pyproject.toml) into that runtime. Used by build.bat and
rem   shipped by the installer; the service runs python-runtime\python.exe -m cim_ems.
rem Without it: creates .venv here with Python 3.10 to 3.14 x64 from the py launcher or PATH (development).
setlocal
cd /d "%~dp0"
set RT=%~dp0..\..\platform-windows\python-runtime\python.exe
if exist "%RT%" (
  "%RT%" -c "import sys; sys.exit(0 if (3,10)<=sys.version_info[:2]<=(3,14) else 1)" || (
    echo CIM-EMS: platform-windows\python-runtime must be Python 3.10 to 3.14 x64
    exit /b 1
  )
  "%RT%" -m pip install --quiet --disable-pip-version-check --no-warn-script-location . || exit /b 1
  "%RT%" -c "import cim_ems, pypowsybl, pandapower; print('CIM-EMS: installed into python-runtime')" || exit /b 1
  exit /b 0
)
set PY=
for %%v in (3.14 3.13 3.12 3.11 3.10) do (
  if not defined PY ( py -%%v -c "import venv" >nul 2>&1 && set PY=py -%%v )
)
if not defined PY ( python -c "import sys,venv; sys.exit(0 if (3,10)<=sys.version_info[:2]<=(3,14) else 1)" >nul 2>&1 && set PY=python )
if not defined PY (
  echo CIM-EMS: needs platform-windows\python-runtime or Python 3.10 to 3.14 x64
  exit /b 1
)
%PY% -m venv .venv || exit /b 1
.venv\Scripts\python -m pip install --quiet --upgrade pip || exit /b 1
.venv\Scripts\python -m pip install --quiet . || exit /b 1
.venv\Scripts\python -c "import cim_ems, pypowsybl, pandapower; print('CIM-EMS: installed')"
