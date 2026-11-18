#!/usr/bin/env bash
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
if [[ $(id -u) -eq 0 ]]; then
    exec su -s /bin/bash player -c 'bash "$1"' bash "$here/solve.sh"
fi
cd /workspace/repo
cp "$here"/port/*.go "$here/port/go.mod" .
find . -name '*.py' -delete
go build -o /tmp/figlet .
go build ./...
test -z "$(find . -name '*.py' -print -quit)"
