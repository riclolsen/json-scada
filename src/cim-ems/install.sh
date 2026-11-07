#!/bin/bash
# CIM-EMS Python environment: creates .venv here and installs the pinned dependencies (pyproject.toml).
# Linux x86_64 only (pypowsybl has no Linux ARM64 wheel); elsewhere it prints a notice and exits 0 so that
# platform builds continue. Needs Python 3.10 to 3.14 with the venv module (Ubuntu: python3-venv,
# RHEL 9: python3.12).
cd "$(dirname "$0")" || exit 1
if [ "$(uname -m)" != "x86_64" ]; then
    echo "CIM-EMS: no pypowsybl wheel for $(uname -m), driver not installed"
    exit 0
fi
PY=""
for c in python3.14 python3.13 python3.12 python3.11 python3.10 python3; do
    if command -v "$c" >/dev/null 2>&1 && \
       "$c" -c 'import sys, venv; sys.exit(0 if (3, 10) <= sys.version_info[:2] <= (3, 14) else 1)' 2>/dev/null; then
        PY="$c"
        break
    fi
done
if [ -z "$PY" ]; then
    echo "CIM-EMS: needs Python 3.10 to 3.14 with the venv module"
    exit 1
fi
echo "CIM-EMS: using $($PY --version)"
"$PY" -m venv --clear .venv || exit 1
.venv/bin/python -m pip install --quiet --upgrade pip || exit 1
.venv/bin/python -m pip install --quiet . || exit 1
.venv/bin/python -c "import cim_ems, pypowsybl, pandapower; print('CIM-EMS: installed', cim_ems.CGMES_VERSION and 'ok')"
