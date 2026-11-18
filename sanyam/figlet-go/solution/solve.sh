#!/bin/bash
# Oracle: install the reference Go port (src/main.go) into the repository, keep only the
# .flf fonts (embedded by the program), and remove every Python file.
set -euo pipefail

repo=/workspace/repo
here="$(cd "$(dirname "$0")" && pwd)"
cd "$repo"

mkdir -p fonts
mv pyfiglet/fonts/*.flf fonts/
rm -rf pyfiglet
find . -name '*.py' -delete

cp "$here/src/main.go" main.go
printf 'module figlet\n\ngo 1.23\n' > go.mod

go build ./...
