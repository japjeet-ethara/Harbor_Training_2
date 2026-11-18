#!/usr/bin/env bash
set -euo pipefail

repo=/workspace/repo
script_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
if [[ ! -f "$script_dir/main.go" && -f /solution/main.go ]]; then
    script_dir=/solution
fi

cp "$script_dir/main.go" "$repo/main.go"
cp "$script_dir/go.mod" "$repo/go.mod"
mkdir -p "$repo/fonts"
find "$repo/pyfiglet/fonts" -maxdepth 1 -type f \( -name '*.flf' -o -name '*.tlf' \) \
    -exec cp '{}' "$repo/fonts/" \;
find "$repo" -type f -name '*.py' -delete

cd "$repo"
gofmt -w main.go
go build ./...
go build -o /tmp/figlet .
