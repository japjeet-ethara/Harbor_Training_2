#!/bin/bash
# Oracle: install the Go port of pyfiglet into /workspace/repo.
# The Go source lives in /solution (uploaded only for the oracle run), never in the image.
set -euo pipefail
cd /workspace/repo

# 1. Fonts: keep the standard .flf files inside the repository, next to the Go code (go:embed).
mkdir -p fonts
cp pyfiglet/fonts/*.flf fonts/

# 2. The Go program (standard library only).
cp /solution/figlet.go /solution/main.go .
cat > go.mod <<'EOF'
module figlet

go 1.23
EOF

# 3. The Python is gone.
find . -name '*.py' -type f -delete
rm -rf pyfiglet

# 4. Check that it builds.
go build ./...
go build -o /tmp/figlet .
/tmp/figlet -f slant oracle
