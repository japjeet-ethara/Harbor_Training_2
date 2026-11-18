#!/bin/sh
set -eu

cd /workspace/repo
cp /solution/main.go main.go
cat > go.mod <<'EOF'
module figlet-go

go 1.23.0
EOF
mkdir -p fonts
cp pyfiglet/pyfiglet/fonts/*.flf fonts/
cp pyfiglet/pyfiglet/fonts/*.tlf fonts/ 2>/dev/null || true
find . -type f -name '*.py' -delete
export PATH="/usr/local/go/bin:$PATH"
go build -o /tmp/figlet .
