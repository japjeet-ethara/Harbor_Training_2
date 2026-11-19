#!/bin/bash
# Reference solution: install the Go port of pyfiglet into /workspace/repo.
set -euo pipefail

SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/src"
REPO=/workspace/repo
cd "$REPO"

# Fonts stay inside the repository and are embedded into the binary
# (go:embed), so the program works from any working directory.
mkdir -p fonts
cp pyfiglet/fonts-standard/*.flf fonts/

# Go sources (standard library only).
cp "$SRC"/*.go .
cat > go.mod <<'EOF'
module figlet

go 1.23
EOF

# The port replaces the Python implementation: no .py files may remain.
find . -name '*.py' ! -type d -delete

go build ./...
go build -o /tmp/figlet .
echo "figlet built: $(/tmp/figlet -f standard ok | wc -l) lines rendered for a smoke test"
