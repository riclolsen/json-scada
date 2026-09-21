#!/bin/bash
# Build, vet and test every Go module of the workspace.
#
# Go has no single command that covers every module of a workspace: src/ is not
# itself a module, so "go build ./..." from here is rejected. Hence the loop.
#
# Usage: cd src && ./go-check.sh
set -u

cd "$(dirname "$0")" || exit 1

modules=$(sed -n 's|^\t\./||p' go.work)
rc=0

for m in $modules; do
    printf '%-32s ' "$m"
    (
        cd "$m" || exit 1

        # -o /dev/null: a "go build ./..." in a package main module drops the
        # executable in the module directory, which would overwrite the
        # prebuilt iccp-*.exe binaries that build.sh copies into bin/.
        out=$(go build -o /dev/null ./... 2>&1) || { echo "BUILD FAIL"; echo "$out"; exit 1; }
        out=$(go vet ./... 2>&1)               || { echo "VET FAIL";   echo "$out"; exit 1; }
        out=$(go test -count=1 ./... 2>&1)     || { echo "TEST FAIL";  echo "$out"; exit 1; }
        echo "OK"
    ) || rc=1
done

exit $rc
