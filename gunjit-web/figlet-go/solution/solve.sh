#!/bin/bash
# Oracle: install the Go port of pyfiglet into /workspace/repo and remove the Python.
# Harbor uploads this whole folder to /solution, so the Go sources sit next to this script.
set -euo pipefail
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd /workspace/repo

# fonts stay in the repository, embedded into the binary by go:embed
mkdir -p fonts
cp pyfiglet/fonts-standard/*.flf fonts/

cp "$SRC/go.mod" "$SRC/main.go" "$SRC/figlet.go" .

# the Python is gone
rm -rf pyfiglet tools setup.py pyrightconfig.json test_font.sh
find . -name '*.py' -delete

go build -o /dev/null ./...
go build -o /tmp/figlet .
/tmp/figlet -f slant "ported"
