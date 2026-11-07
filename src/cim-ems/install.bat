@echo off
rem CIM-EMS Python environment: creates .venv here and installs the pinned dependencies (pyproject.toml).
rem Needs Python 3.10 to 3.14 (x64) on PATH or the py launcher. The Windows installer does not bundle Python yet.
cd /d "%~dp0"
set PY=
for %%v in (3.14 3.13 3.12 3.11 3.10) do (
  if not defined PY ( py -%%v -c "import venv" >nul 2>&1 && set PY=py -%%v )
)
if not defined PY ( python -c "import sys,venv; sys.exit(0 if (3,10)<=sys.version_info[:2]<=(3,14) else 1)" >nul 2>&1 && set PY=python )
if not defined PY (
  echo CIM-EMS: needs Python 3.10 to 3.14 x64
  exit /b 1
)
%PY% -m venv .venv || exit /b 1
.venv\Scripts\python -m pip install --quiet --upgrade pip || exit /b 1
.venv\Scripts\python -m pip install --quiet . || exit /b 1
.venv\Scripts\python -c "import cim_ems, pypowsybl, pandapower; print('CIM-EMS: installed')"
