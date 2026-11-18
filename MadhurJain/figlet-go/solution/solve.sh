#!/bin/bash
set -euo pipefail

cp -r /solution/go/* /workspace/repo/
find /workspace/repo -name "*.py" -delete
cd /workspace/repo
go build -o /tmp/figlet .
go build -o /dev/null ./...
