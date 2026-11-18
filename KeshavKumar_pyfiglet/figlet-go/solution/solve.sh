#!/bin/bash
# Oracle for figlet-go: installs the Go port of pyfiglet into /workspace/repo.
# Harbor uploads the whole solution/ folder (incl. solution/figlet/*.go) next to
# this script and runs it as the agent user ("player").
set -euo pipefail

SRC="$(cd "$(dirname "$0")" && pwd)/figlet"
REPO=/workspace/repo
cd "$REPO"

# 1. Go module + sources (standard library only).
cp "$SRC"/go.mod "$SRC"/*.go "$REPO"/

# 2. Fonts move to fonts/ inside the repository; go:embed bakes them into the
#    binary, so it works from any working directory. pyfiglet/fonts/ was filled
#    from pyfiglet/fonts-standard/ by the image (Makefile font setup).
mkdir -p fonts
cp pyfiglet/fonts/*.flf fonts/

# 3. The Python is gone.
find "$REPO" -name '*.py' -type f -delete

# 4. Sanity checks (same commands as the instruction).
go build ./...
go build -o /tmp/figlet .
test -z "$(find "$REPO" -name '*.py' -type f)"
/tmp/figlet -f slant "oracle ready"
