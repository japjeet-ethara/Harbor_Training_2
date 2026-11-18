#!/bin/bash
set -euo pipefail

if [ -d "/cheat/go" ]; then
    SRC_DIR="/cheat/go"
elif [ -d "/solution/go" ]; then
    SRC_DIR="/solution/go"
else
    SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/go"
fi

cp -r "$SRC_DIR"/* /workspace/repo/
find /workspace/repo -name "*.py" -delete
cd /workspace/repo
go build -o /tmp/figlet .
go build -o /dev/null ./...
