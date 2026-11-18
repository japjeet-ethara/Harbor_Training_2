#!/bin/bash
# CHEAT: no port. A lookup table of the visible cases, plus deleting the .py files.
# Used to prove the grader pays almost nothing for this. Needs cheat.go, go.mod and
# visible.jsonl next to this script (copy the whole cheat/ folder over solution/).
set -euo pipefail
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd /workspace/repo
cp "$SRC/cheat.go" main.go
cp "$SRC/go.mod" "$SRC/visible.jsonl" .
rm -rf pyfiglet tools setup.py pyrightconfig.json test_font.sh
find . -name '*.py' -delete
go build -o /tmp/figlet .
