#!/bin/bash
set -euo pipefail

repo=/workspace/repo
solution_dir="$(cd "$(dirname "$0")" && pwd)"

test -f "$solution_dir/main.go"
test -d "$repo/pyfiglet/fonts-standard"

cat > "$repo/go.mod" <<'EOF'
module figlet

go 1.23
EOF

mkdir -p "$repo/fonts"
cp "$solution_dir/main.go" "$repo/main.go"
cp "$repo/pyfiglet/fonts-standard/"*.flf "$repo/fonts/"
find "$repo" -type f -name '*.py' -delete

if find "$repo" -type f -name '*.py' -print -quit | grep -q .; then
    echo "Python files remain in $repo" >&2
    exit 1
fi

cd "$repo"
go build ./...
