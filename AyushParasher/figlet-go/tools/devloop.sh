#!/bin/bash
# Fast fix loop (no Harbor): build the port the way solve.sh lays it out, inside the
# pinned Go 1.23 image, then diff it against the VISIBLE cases only.
# (Runs in Docker because Windows Application Control blocks go vet / local .exe files.)
# Usage (Git Bash, from the task folder):  bash tools/devloop.sh [N] [CASES]
set -euo pipefail
cd "$(dirname "$0")/.."
show="${1:-5}"
cases="${2:-tests/data/visible.jsonl}"
MSYS_NO_PATHCONV=1 docker run --rm \
  -v "$(cygpath -w "$PWD")":/task:ro \
  -v "$(cygpath -w "$PWD/../pyfiglet/pyfiglet/fonts-standard")":/fonts:ro \
  -e GOTOOLCHAIN=local -e CGO_ENABLED=0 -e GOFLAGS=-buildvcs=false \
  golang:1.23-alpine sh -c "
    set -e
    mkdir /build /build/fonts && cd /build
    cp /task/solution/go/go.mod /task/solution/go/*.go . && cp /fonts/*.flf fonts/
    go vet . && go build -o /tmp/figlet .
    cd /task && go run tools/diffcases/main.go /tmp/figlet $cases $show"
