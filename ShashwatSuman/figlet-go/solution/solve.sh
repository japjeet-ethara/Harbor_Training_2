#!/bin/bash
# Oracle: install the Go port of pyfiglet into /workspace/repo.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO=/workspace/repo

cp "$HERE/figlet/go.mod" "$HERE/figlet/main.go" "$REPO/"

# The fonts stay inside the repository (pyfiglet/fonts) and are embedded into
# the binary with go:embed, so /tmp/figlet works from any directory.
test -f "$REPO/pyfiglet/fonts/standard.flf"

# No Python may remain.
find "$REPO" -name '*.py' -type f -delete
find "$REPO" -name '__pycache__' -type d -prune -exec rm -rf {} +

# solve.sh runs as root; hand everything back to the agent user.
if [ "$(id -u)" = 0 ] && id player >/dev/null 2>&1; then
    chown -R player:player "$REPO"
fi

cd "$REPO"
go build ./...
go build -o /tmp/figlet .
/tmp/figlet -f slant "oracle ready"
