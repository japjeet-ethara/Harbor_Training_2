#!/bin/bash
set -euo pipefail
cd /workspace/repo
mkdir -p fonts
cp /solution/main.go ./main.go
cp /solution/go.mod ./go.mod
cp pyfiglet/fonts/*.flf fonts/ 2>/dev/null || true
cp pyfiglet/fonts/*.tlf fonts/ 2>/dev/null || true
if ! find fonts -type f \( -name '*.flf' -o -name '*.tlf' \) -print -quit | grep -q .; then
  echo 'No FIGfont files found under pyfiglet/fonts' >&2
  exit 1
fi
# The Go implementation and go.mod are part of the reference solution.
# The Python source is removed only after the font files have been copied.
find . -type f -name '*.py' -delete
