#!/bin/bash
# Oracle: install a Go port of pyfiglet into /workspace/repo, then remove all Python.
set -euo pipefail

REPO=/workspace/repo
SRC="$(cd "$(dirname "$0")" && pwd)/src"

cd "$REPO"

# 1. Fonts: keep the standard .flf/.flc files inside the repository (embedded via go:embed).
mkdir -p fonts
cp pyfiglet/fonts-standard/*.flf pyfiglet/fonts-standard/*.flc fonts/

# 2. Go module + sources (standard library only).
cat > go.mod <<'EOF'
module figlet

go 1.23
EOF
cp "$SRC"/*.go .

# 3. No Python left anywhere in the repository.
find "$REPO" -name '*.py' -type f -delete

# 4. Sanity check.
go build ./...
go build -o /tmp/figlet .
/tmp/figlet -f slant "oracle ready"
