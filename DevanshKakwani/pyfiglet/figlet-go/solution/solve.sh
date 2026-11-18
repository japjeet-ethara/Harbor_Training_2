#!/bin/bash
# Reference solution: install the Go port of pyfiglet into /workspace/repo.
set -euo pipefail

SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/go"
REPO="${REPO:-/workspace/repo}"

cp "$SRC"/go.mod "$SRC"/*.go "$REPO"/

# No Python may remain anywhere in the repository.
find "$REPO" -type f -iname '*.py' -delete

cd "$REPO"
go vet ./...
go build -o /tmp/figlet .
