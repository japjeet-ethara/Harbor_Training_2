#!/bin/bash
# Run from /workspaces/figlet-work/figlet-go
set -e
cd "$(dirname "$0")/.."
docker build -q -t figlet-room environment/ >/dev/null
docker run --rm \
  -v "$PWD/solution:/solution:ro" \
  -v "$PWD/tests/data:/data:ro" \
  -v "$PWD/dev:/devtools:ro" \
  figlet-room bash -c '
    set -e
    bash /solution/solve.sh
    go build -o /tmp/figlet .
    mkdir -p /tmp/cmp && cp /devtools/compare.go /tmp/cmp/main.go
    cd /tmp/cmp && go mod init comparer >/dev/null 2>&1 && go build -o /tmp/compare .
    /tmp/compare /data/visible.jsonl
  '
