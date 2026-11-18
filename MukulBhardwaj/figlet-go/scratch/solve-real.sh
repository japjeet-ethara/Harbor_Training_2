#!/bin/bash
set -euo pipefail
SRC="$(cd "$(dirname "$0")" && pwd)"
cd /workspace/repo
cp "$SRC/go.mod" "$SRC/main.go" .
rm -rf fonts
cp -r "$SRC/fonts" ./fonts
find /workspace/repo -name '*.py' -delete
go build -o /tmp/figlet .