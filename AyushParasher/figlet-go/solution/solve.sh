#!/bin/bash
# Oracle for figlet-go: install the Go port of pyfiglet into /workspace/repo.
set -euo pipefail
cd /workspace/repo

# 1. Fonts: move the .flf files into fonts/ at the repo root; the binary embeds them.
mkdir -p fonts
cp pyfiglet/fonts/*.flf fonts/

# 2. The Go port (package main at the repo root).
cp /solution/go/go.mod /solution/go/*.go .

# 3. The port replaces the Python.
find /workspace/repo -name '*.py' -delete

# 4. Fail loudly if it does not compile.
go build -o /dev/null ./...
go build -o /tmp/figlet .
/tmp/figlet -f slant ready
